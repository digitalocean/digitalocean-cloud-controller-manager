/*
Copyright 2020 DigitalOcean

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package do

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/digitalocean/godo"
	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/component-base/metrics/prometheus/ratelimiter"
	"k8s.io/klog"
)

const (
	// Interval of synchronizing service status from apiserver.
	serviceSyncPeriod = 30 * time.Second
	minRetryDelay     = 5 * time.Second
	maxRetryDelay     = 300 * time.Second

	// The format we should expect for ccm worker firewall names.
	firewallWorkerCCMNameFormat = "k8s-%s-ccm"
)

type cachedFirewall struct {
	// The cached state of the firewall.
	state *godo.Firewall
}

type firewallCache struct {
	mu       sync.RWMutex // protects firewallMap.
	firewall *cachedFirewall
}

// Controller helps to keep cloud provider service firewalls in sync.
type Controller struct {
	kubeClient         clientset.Interface
	client             *godo.Client
	fwCache            *firewallCache
	queue              workqueue.RateLimitingInterface
	firewallService    *godo.FirewallsServiceOp
	workerFirewallName string
	tags               []string
	serviceLister      corelisters.ServiceLister
	firewallManager    FirewallManager
}

type FirewallManager interface {
	// Get returns the current CCM worker firewall representation (i.e., the DO Firewall object).
	Get(ctx context.Context, inboundRules []godo.InboundRule) (godo.Firewall, error)

	// Set applies the given inbound rules to the CCM worker firewall.
	Set(ctx context.Context, inboundRules []godo.InboundRule) error
}

// NewFirewallController returns a new firewall controller to reconcile CCM worker firewall state.
func NewFirewallController(
	workerFirewallName string,
	kubeClient clientset.Interface,
	client *godo.Client,
	firewallService *godo.FirewallsService,
	serviceInformer coreinformers.ServiceInformer,
	tags []string,
	ctx context.Context,
) (*Controller, error) {
	if kubeClient != nil && kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		if err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage("firewall_controller", kubeClient.CoreV1().RESTClient().GetRateLimiter()); err != nil {
			return nil, err
		}
	}

	c := &Controller{
		kubeClient:         kubeClient,
		client:             client,
		fwCache:            &firewallCache{firewall: *cachedFirewall},
		queue:              workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(minRetryDelay, maxRetryDelay), "firewall"),
		workerFirewallName: workerFirewallName,
		tags:               tags,
	}

	serviceInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(cur interface{}) {
				c.onServiceChange(ctx)
			},
			UpdateFunc: func(old, cur interface{}) {
				c.onServiceChange(ctx)
			},
			DeleteFunc: func(obj interface{}) {
				c.onServiceChange(ctx)
			},
		},
		serviceSyncPeriod,
	)
	c.serviceLister = serviceInformer.Lister()

	return c, nil
}

// Get returns the current CCM worker firewall representation.
func (c *Controller) Get(ctx context.Context, inboundRules []godo.InboundRule) (*godo.Firewall, error) {
	// return the firewall stored in the cache.
	if c.firewallCacheExists() {
		fw, _, err := c.firewallService.Get(ctx, c.fwCache.firewall.state.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get firewall: %s", err)
		}
		return fw, nil
	}
	// cached firewall does not exist, so iterate through firewall API provided list and return
	// the firewall with the matching firewall name.
	firewallList, err := allFirewallList(ctx, c.client)
	if err != nil {
		return nil, fmt.Errorf("failed to list firewalls: %s", err)
	}
	for _, fw := range firewallList {
		if fw.Name == c.workerFirewallName {
			return &fw, nil
		}
	}
	// firewall is not found via firewalls API, so we need to create it.
	fw, err := c.createFirewallAndUpdateCache(ctx, inboundRules)
	if err != nil {
		return nil, fmt.Errorf("failed to create firewall: %s", err)
	}
	return fw, nil
}

// Set applies the given inbound rules to the CCM worker firewall when the current rules and target rules differ.
func (c *Controller) Set(ctx context.Context, inboundRules []godo.InboundRule) error {
	// retrieve the target firewall representation (CCM worker firewall from cache) and the
	// current firewall representation from the DO firewalls API. If there are any differences,
	// handle it.
	fw, err := c.firewallManager.Get(ctx, inboundRules)
	if err != nil {
		return fmt.Errorf("failed to get firewall state: %s", err)
	}
	cachedFw := c.fwCache.firewall.state
	firewallsAreEqual := cmp.Equal(cachedFw, fw)
	if cmp.Equal(cachedFw.InboundRules, inboundRules) && cmp.Equal(fw.InboundRules, inboundRules) {
		if firewallsAreEqual {
			return nil
		}
	} else if !cmp.Equal(cachedFw.InboundRules, inboundRules) && cmp.Equal(fw.InboundRules, inboundRules) {
		c.updateCache(&fw)
	} else if !cmp.Equal(fw.InboundRules, inboundRules) {
		err := c.updateFirewallRules(ctx, inboundRules)
		if err != nil {
			return fmt.Errorf("failed to update firewall state: %s", err)
		}
	}
	if !firewallsAreEqual {
		err := c.reconcileFirewall(ctx, *cachedFw, fw)
		if err != nil {
			return fmt.Errorf("failed to reconcile firewall state: %s", err)
		}
		return nil
	}
	return nil
}

func (c *Controller) Run(ctx context.Context, inboundRules []godo.InboundRule, stopCh <-chan struct{}) {
	wait.Until(func() {
		firewall, err := c.firewallManager.Get(ctx, inboundRules)
		if err != nil {
			klog.Error("failed to get worker firewall: %s", err)
			return
		}

		firewallCachedState := c.fwCache.firewall.state
		if cmp.Equal(firewallCachedState, firewall) {
			return
		}
		err = c.onServiceChange(ctx)
		if err != nil {
			klog.Error("Failed to reconcile worker firewall: %s", err)
		}
	}, 5*time.Minute, stopCh)
}

func (c *Controller) onServiceChange(ctx context.Context) error {
	var nodePortInboundRules []godo.InboundRule
	serviceList, err := c.serviceLister.List(labels.Nothing())
	if err != nil {
		return fmt.Errorf("failed to get service state: %s", err)
	}
	for _, svc := range serviceList {
		if svc.Spec.Type == v1.ServiceTypeNodePort {
			// this is a nodeport service so we should check for existing inbound rules on all ports.
			for _, servicePort := range svc.Spec.Ports {
				nodePortInboundRules = append(nodePortInboundRules, godo.InboundRule{
					Protocol:  "tcp",
					PortRange: strconv.Itoa(int(servicePort.NodePort)),
					Sources: &godo.Sources{
						Tags: c.tags,
					},
				})
			}
		}
	}
	if len(nodePortInboundRules) == 0 {
		return nil
	}
	return c.firewallManager.Set(ctx, nodePortInboundRules)
}

func (c *Controller) updateCache(firewall *godo.Firewall) {
	c.fwCache.mu.Lock()
	defer c.fwCache.mu.Unlock()
	fw := &cachedFirewall{state: firewall}
	c.fwCache.firewall.state = fw.state
}

func (c *Controller) firewallCacheExists() bool {
	c.fwCache.mu.RLock()
	defer c.fwCache.mu.RUnlock()
	if c.fwCache.firewall != nil {
		return true
	}
	return false
}

func (c *Controller) updateFirewallRules(ctx context.Context, inboundRules []godo.InboundRule) error {
	rr := godo.FirewallRulesRequest{
		InboundRules: inboundRules,
	}
	resp, err := c.firewallService.AddRules(ctx, c.fwCache.firewall.state.ID, &rr)
	if err != nil {
		return fmt.Errorf("failed to add firewall inbound rules: %s", err)
	}
	if resp.StatusCode == 404 {
		c.createFirewallAndUpdateCache(ctx, inboundRules)
		return nil
	}
	return nil
}

func (c *Controller) createFirewallAndUpdateCache(ctx context.Context, inboundRules []godo.InboundRule) (*godo.Firewall, error) {
	// make create request since firewall does not exist, then cache firewall state.
	fr := &godo.FirewallRequest{
		Name:         c.workerFirewallName,
		InboundRules: inboundRules,
	}
	fw, _, err := c.firewallService.Create(ctx, fr)
	if err != nil {
		return nil, fmt.Errorf("failed to create firewall: %s", err)
	}
	c.updateCache(fw)
	return fw, nil
}

// check each field of the cached firewall and the DO API firewall and update any discrepancies until it
// matches the target state (cached firewall).
func (c *Controller) reconcileFirewall(ctx context.Context, targetState godo.Firewall, currentState godo.Firewall) error {
	updateStateRequest := &godo.FirewallRequest{}
	if targetState.Name != currentState.Name {
		updateStateRequest.Name = targetState.Name
	}
	if !cmp.Equal(targetState.InboundRules, currentState.InboundRules) {
		updateStateRequest.InboundRules = targetState.InboundRules
	}
	if !cmp.Equal(targetState.OutboundRules, currentState.OutboundRules) {
		updateStateRequest.OutboundRules = targetState.OutboundRules
	}
	if !cmp.Equal(targetState.DropletIDs, currentState.DropletIDs) {
		updateStateRequest.DropletIDs = targetState.DropletIDs
	}
	if !cmp.Equal(targetState.Tags, currentState.Tags) {
		updateStateRequest.Tags = targetState.Tags
	}
	_, resp, err := c.firewallService.Update(ctx, targetState.ID, updateStateRequest)
	if err != nil {
		return fmt.Errorf("failed to update firewall state: %s", err)
	}
	if resp.StatusCode == 404 {
		c.createFirewallAndUpdateCache(ctx, targetState.InboundRules)
		return nil
	}
	return nil
}
