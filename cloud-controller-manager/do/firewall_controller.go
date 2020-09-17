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
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/digitalocean/godo"
	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

const (
	// Interval of synchronizing service status from apiserver.
	serviceSyncPeriod = 30 * time.Second
	// Frequency at which the firewall controller runs.
	firewallReconcileFrequency = 5 * time.Minute
	originEvent                = "event"
	originFirewallLoop         = "firewall_loop"

	// How long to wait before retrying the processing of a firewall change.
	minRetryDelay = 1 * time.Second
	maxRetryDelay = 5 * time.Minute
)

var (
	allowAllOutboundRules = []godo.OutboundRule{
		{
			Protocol:  "tcp",
			PortRange: "all",
			Destinations: &godo.Destinations{
				Addresses: []string{"0.0.0.0/0", "::/0"},
			},
		},
		{
			Protocol:  "udp",
			PortRange: "all",
			Destinations: &godo.Destinations{
				Addresses: []string{"0.0.0.0/0", "::/0"},
			},
		},
		{
			Protocol: "icmp",
			Destinations: &godo.Destinations{
				Addresses: []string{"0.0.0.0/0", "::/0"},
			},
		},
	}
)

// firewallCache stores a cached firewall and mutex to handle concurrent access.
type firewallCache struct {
	mu       *sync.RWMutex // protects firewall.
	firewall *godo.Firewall
}

// firewallManager manages the interaction with the DO Firewalls API.
type firewallManager struct {
	client             *godo.Client
	fwCache            firewallCache
	workerFirewallName string
	workerFirewallTags []string
	metrics            metrics
}

// FirewallController helps to keep cloud provider service firewalls in sync.
type FirewallController struct {
	kubeClient         clientset.Interface
	client             *godo.Client
	workerFirewallTags []string
	workerFirewallName string
	serviceLister      corelisters.ServiceLister
	fwManager          firewallManager
	metrics            metrics
	queue              workqueue.RateLimitingInterface
}

// NewFirewallController returns a new firewall controller to reconcile public access firewall state.
func NewFirewallController(
	ctx context.Context,
	kubeClient clientset.Interface,
	client *godo.Client,
	serviceInformer coreinformers.ServiceInformer,
	fwManager firewallManager,
	workerFirewallTags []string,
	workerFirewallName string,
	metrics metrics,
) *FirewallController {
	fc := &FirewallController{
		kubeClient:         kubeClient,
		client:             client,
		workerFirewallTags: workerFirewallTags,
		workerFirewallName: workerFirewallName,
		fwManager:          fwManager,
		metrics:            metrics,
		queue:              workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(minRetryDelay, maxRetryDelay), "firewall"),
	}

	serviceInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(cur interface{}) {
				fc.queue.Add("service")
			},
			UpdateFunc: func(old, cur interface{}) {
				fc.queue.Add("service")
			},
			DeleteFunc: func(cur interface{}) {
				fc.queue.Add("service")
			},
		},
		serviceSyncPeriod,
	)
	fc.serviceLister = serviceInformer.Lister()

	return fc
}

// Run starts the firewall controller loop.
func (fc *FirewallController) Run(ctx context.Context, stopCh <-chan struct{}, fwReconcileFrequency time.Duration) {
	wait.Until(func() {
		err := fc.observeRunLoopDuration(ctx)
		if err != nil {
			klog.Errorf("failed to run firewall controller loop: %v", err)
		}
	}, fwReconcileFrequency, stopCh)
	fc.queue.ShutDown()
}

func (fc *FirewallController) runWorker() {
	for fc.processNextItem() {
	}
}

func (fc *FirewallController) processNextItem() bool {
	ctx, cancel := context.WithTimeout(context.Background(), firewallReconcileFrequency)
	defer cancel()
	// Wait until there is a new item in the working queue
	key, quit := fc.queue.Get()
	if quit {
		return false
	}
	// Tell the queue that we are done with processing this key. This unblocks the key for other workers
	// This allows safe parallel processing because items with the same key are never processed in
	// parallel.
	defer fc.queue.Done(key)

	err := fc.observeReconcileDuration(ctx, originEvent)
	if err != nil {
		klog.Errorf("failed to run firewall controller loop: %v", err)
		fc.queue.AddRateLimited(key)
	}
	fc.queue.Forget(key)
	return true
}

func (fc *FirewallController) reconcileCloudFirewallChanges(ctx context.Context) error {
	currentFirewall, err := fc.fwManager.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get worker firewall: %s", err)
	}
	if currentFirewall != nil {
		if fc.fwManager.fwCache.isEqual(currentFirewall) {
			return nil
		}
	}
	fc.fwManager.fwCache.updateCache(currentFirewall)
	err = fc.observeReconcileDuration(ctx, originFirewallLoop)
	if err != nil {
		return fmt.Errorf("failed to reconcile worker firewall: %s", err)
	}

	klog.Info("successfully reconciled firewall")
	return nil
}

// Get returns the current public access firewall representation.
func (fm *firewallManager) Get(ctx context.Context) (*godo.Firewall, error) {
	// check cache and query the API firewall service to get firewall by ID, if it exists. Return it. If not, continue.
	fw := fm.fwCache.getCachedFirewall()
	if fw != nil {
		var (
			resp *godo.Response
			err  error
		)
		fw, resp, err := func() (*godo.Firewall, *godo.Response, error) {
			var (
				code   int
				method string
			)
			// The ObserverFunc gets called by the deferred ObserveDuration. The
			// method and code values will be set before ObserveDuration is called
			// with the value returned from the response from the Firewall API request.
			timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
				fm.metrics.apiRequestDuration.With(prometheus.Labels{"method": method, "code": strconv.FormatInt(int64(code), 10)}).Observe(v)
			}))
			defer timer.ObserveDuration()
			fw, resp, err := fm.client.Firewalls.Get(ctx, fw.ID)
			if resp != nil {
				code = resp.StatusCode
				if resp.Request != nil {
					method = resp.Request.Method
				}
			}
			return fw, resp, err
		}()

		if err != nil && (resp == nil || resp.StatusCode != http.StatusNotFound) {
			return nil, fmt.Errorf("could not get firewall: %v", err)
		}
		if resp.StatusCode == http.StatusNotFound {
			klog.Warning("unable to retrieve firewall by ID because it no longer exists")
		}
		if fw != nil {
			return fw, nil
		}
	}

	// iterate through firewall API provided list and return the firewall with the matching firewall name.
	f := func(fw godo.Firewall) bool {
		return fw.Name == fm.workerFirewallName
	}
	klog.Info("filtering firewall list for the firewall that has the expected firewall name")
	fw, err := func() (*godo.Firewall, error) {
		var (
			code   int
			method string
		)
		// The ObserverFunc gets called by the deferred ObserveDuration. The
		// method and code values will be set before ObserveDuration is called
		// with the value returned from the response from the Firewall API request.
		timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
			fm.metrics.apiRequestDuration.With(prometheus.Labels{"method": method, "code": strconv.FormatInt(int64(code), 10)}).Observe(v)
		}))
		defer timer.ObserveDuration()
		fw, resp, err := filterFirewallList(ctx, fm.client, f)
		if resp != nil {
			code = resp.StatusCode
			if resp.Request != nil {
				method = resp.Request.Method
			}
		}
		return fw, err
	}()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve list of firewalls from DO API: %v", err)
	}
	if fw != nil {
		klog.Info("found firewall by listing")
	} else {
		klog.Info("could not find firewall by listing")
	}
	return fw, nil
}

// Set applies the given firewall request configuration to the public access firewall if there is a delta.
// Specifically Set will reconcile away any changes to the inbound rules, outbound rules, firewall name and/or tags.
func (fm *firewallManager) Set(ctx context.Context, fr *godo.FirewallRequest) error {
	targetFirewall := fm.fwCache.getCachedFirewall()

	if targetFirewall != nil {
		equal, unequalParts := isEqual(targetFirewall, fr)
		if equal {
			// A locally cached firewall with matching rules, correct name and tags means there is nothing to update.
			return nil
		}
		klog.Infof("firewall configuration mismatch: %v", unequalParts)

		// A locally cached firewall exists, but is does not match the expected
		// service inbound rules, outbound rules, name or tags. So we need to use the locally
		// cached firewall ID to attempt to update the firewall APIs representation of the
		// firewall with the new rules
		currentFirewall, resp, err := func() (*godo.Firewall, *godo.Response, error) {
			var (
				code   int
				method string
			)
			// The ObserverFunc gets called by the deferred ObserveDuration. The
			// method and code values will be set before ObserveDuration is called
			// with the value returned from the response from the Firewall API request.
			timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
				fm.metrics.apiRequestDuration.With(prometheus.Labels{"method": method, "code": strconv.FormatInt(int64(code), 10)}).Observe(v)
			}))
			defer timer.ObserveDuration()
			fw, resp, err := fm.client.Firewalls.Update(ctx, targetFirewall.ID, fr)
			if resp != nil {
				code = resp.StatusCode
				if resp.Request != nil {
					method = resp.Request.Method
				}
			}
			return fw, resp, err
		}()

		if err == nil {
			klog.Info("successfully updated firewall")
		} else {
			if resp == nil || resp.StatusCode != http.StatusNotFound {
				return fmt.Errorf("could not update firewall: %v", err)
			}
			currentFirewall, err = fm.createFirewall(ctx, fr)
			if err != nil {
				return fmt.Errorf("could not create firewall: %v", err)
			}
			klog.Info("successfully created firewall")
		}
		fm.fwCache.updateCache(currentFirewall)
		return nil
	}

	// Check if the target firewall ID exists. In the case that CCM first starts up and the
	// firewall ID does not exist yet, check the API and see if a firewall by the right name
	// already exists.
	if targetFirewall == nil {
		currentFirewall, err := fm.Get(ctx)
		if err != nil {
			return fmt.Errorf("failed to check if firewall already exists: %s", err)
		}
		if currentFirewall == nil {
			klog.Info("an existing firewall not found, we need to create one")
			currentFirewall, err = fm.createFirewall(ctx, fr)
			if err != nil {
				return err
			}
			klog.Info("successfully created firewall")
		} else {
			klog.Info("an existing firewall is found, we need to update it")
			currentFirewall, err = fm.updateFirewall(ctx, currentFirewall.ID, fr)
			if err != nil {
				return fmt.Errorf("could not update firewall: %v", err)
			}
			klog.Info("successfully updated firewall")
		}
		fm.fwCache.updateCache(currentFirewall)
	}
	return nil
}

func isEqual(targetFirewall *godo.Firewall, fr *godo.FirewallRequest) (bool, []string) {
	var unequalParts []string
	if targetFirewall.Name != fr.Name {
		unequalParts = append(unequalParts, "name")
	}
	if !cmp.Equal(targetFirewall.InboundRules, fr.InboundRules) {
		unequalParts = append(unequalParts, "inboundRules")
	}
	if !cmp.Equal(targetFirewall.OutboundRules, fr.OutboundRules) {
		unequalParts = append(unequalParts, "outboundRules")
	}
	if !cmp.Equal(targetFirewall.Tags, fr.Tags) {
		unequalParts = append(unequalParts, "tags")
	}
	return len(unequalParts) == 0, unequalParts
}

func (fm *firewallManager) createFirewall(ctx context.Context, fr *godo.FirewallRequest) (*godo.Firewall, error) {
	var (
		code   int
		method string
	)
	// The ObserverFunc gets called by the deferred ObserveDuration. The
	// method and code values will be set before ObserveDuration is called
	// with the value returned from the response from the Firewall API request.
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		fm.metrics.apiRequestDuration.With(prometheus.Labels{"method": method, "code": strconv.FormatInt(int64(code), 10)}).Observe(v)
	}))
	defer timer.ObserveDuration()

	currentFirewall, resp, err := fm.client.Firewalls.Create(ctx, fr)
	if resp != nil {
		code = resp.StatusCode
		if resp.Request != nil {
			method = resp.Request.Method
		}
	}

	return currentFirewall, err
}

func (fm *firewallManager) updateFirewall(ctx context.Context, fwID string, fr *godo.FirewallRequest) (*godo.Firewall, error) {
	var (
		code   int
		method string
	)
	// The ObserverFunc gets called by the deferred ObserveDuration. The
	// method and code values will be set before ObserveDuration is called
	// with the value returned from the response from the Firewall API request.
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		fm.metrics.apiRequestDuration.With(prometheus.Labels{"method": method, "code": strconv.FormatInt(int64(code), 10)}).Observe(v)
	}))
	defer timer.ObserveDuration()

	currentFirewall, resp, err := fm.client.Firewalls.Update(ctx, fwID, fr)
	if resp != nil {
		code = resp.StatusCode
		if resp.Request != nil {
			method = resp.Request.Method
		}
	}

	return currentFirewall, err
}

// createReconciledFirewallRequest creates a firewall request that has the correct rules, name and tag
func (fc *FirewallController) createReconciledFirewallRequest(serviceList []*v1.Service) *godo.FirewallRequest {
	var nodePortInboundRules []godo.InboundRule
	for _, svc := range serviceList {
		if svc.Spec.Type == v1.ServiceTypeNodePort {
			// this is a nodeport service so we should check for existing inbound rules on all ports.
			for _, servicePort := range svc.Spec.Ports {
				// In the odd case that a failure is asynchronous causing the NodePort to be set to zero.
				if servicePort.NodePort == 0 {
					klog.Warning("NodePort on the service is set to zero")
					continue
				}
				var protocol string
				switch servicePort.Protocol {
				case v1.ProtocolTCP:
					protocol = "tcp"
				case v1.ProtocolUDP:
					protocol = "udp"
				default:
					klog.Warningf("unsupported service protocol %v, skipping service port %v", servicePort.Protocol, servicePort.Name)
					continue
				}

				nodePortInboundRules = append(nodePortInboundRules,
					godo.InboundRule{
						Protocol:  protocol,
						PortRange: strconv.Itoa(int(servicePort.NodePort)),
						Sources: &godo.Sources{
							Addresses: []string{"0.0.0.0/0", "::/0"},
						},
					},
				)
			}
		}
	}
	return &godo.FirewallRequest{
		Name:          fc.workerFirewallName,
		InboundRules:  nodePortInboundRules,
		OutboundRules: allowAllOutboundRules,
		Tags:          fc.workerFirewallTags,
	}
}

func (fc *FirewallController) ensureReconciledFirewall(ctx context.Context) error {
	serviceList, err := fc.serviceLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list services: %v", err)
	}
	fr := fc.createReconciledFirewallRequest(serviceList)
	err = fc.fwManager.Set(ctx, fr)
	if err != nil {
		return fmt.Errorf("failed to set reconciled firewall: %v", err)
	}
	return nil
}

func (fc *firewallCache) getCachedFirewall() *godo.Firewall {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.firewall
}

func (fc *firewallCache) isEqual(fw *godo.Firewall) bool {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return cmp.Equal(fc.firewall, fw)
}

func (fc *firewallCache) updateCache(currentFirewall *godo.Firewall) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.firewall = currentFirewall
}

func (fc *FirewallController) observeReconcileDuration(ctx context.Context, origin string) error {
	labels := prometheus.Labels{"reconcile_type": origin}
	t := prometheus.NewTimer(fc.fwManager.metrics.reconcileDuration.With(labels))
	defer t.ObserveDuration()

	return fc.ensureReconciledFirewall(ctx)
}

func (fc *FirewallController) observeRunLoopDuration(ctx context.Context) error {
	labels := prometheus.Labels{"success": "false"}
	t := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		fc.fwManager.metrics.runLoopDuration.With(labels).Observe(v)
	}))
	defer t.ObserveDuration()

	err := fc.reconcileCloudFirewallChanges(ctx)
	if err == nil {
		labels["success"] = "true"
	}

	return err
}
