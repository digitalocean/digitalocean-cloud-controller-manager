/*
Copyright 2024 DigitalOcean

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
	"sort"
	"strconv"
	"time"

	"github.com/digitalocean/godo"
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	// Frequency at which the firewall controller runs.
	firewallReconcileFrequency = 5 * time.Minute
	// Timeout value for processing worker items taken from the queue.
	processWorkerItemTimeout = 30 * time.Second
	queueKey                 = "service"

	// How long to wait before retrying the processing of a firewall change.
	minRetryDelay = 1 * time.Second
	maxRetryDelay = 5 * time.Minute
)

const (
	// annotationDOFirewallManaged is the annotation specifying if the given Service
	// should be managed with regards to public firewall access.
	annotationDOFirewallManaged = "kubernetes.digitalocean.com/firewall-managed"
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

// firewallManager manages the interaction with the DO Firewalls API.
type firewallManager struct {
	client             *godo.Client
	fwCache            *firewallCache
	workerFirewallName string
	workerFirewallTags []string
	metrics            metrics
	defaultLBType      string
}

// FirewallController helps to keep cloud provider service firewalls in sync.
type FirewallController struct {
	kubeClient         clientset.Interface
	client             *godo.Client
	workerFirewallTags []string
	workerFirewallName string
	serviceLister      corelisters.ServiceLister
	fwManager          *firewallManager
	queue              workqueue.RateLimitingInterface
}

// NewFirewallController returns a new firewall controller to reconcile public access firewall state.
func NewFirewallController(kubeClient clientset.Interface, client *godo.Client, serviceInformer coreinformers.ServiceInformer, fwManager *firewallManager) *FirewallController {
	fc := &FirewallController{
		kubeClient: kubeClient,
		client:     client,
		fwManager:  fwManager,
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(minRetryDelay, maxRetryDelay), "firewall"),
	}

	serviceInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(cur interface{}) {
				fc.queue.Add(queueKey)
			},
			UpdateFunc: func(old, cur interface{}) {
				fc.queue.Add(queueKey)
			},
			DeleteFunc: func(cur interface{}) {
				fc.queue.Add(queueKey)
			},
		},
		0,
	)
	fc.serviceLister = serviceInformer.Lister()

	return fc
}

// Run starts the firewall controller loop.
func (fc *FirewallController) Run(ctx context.Context, stopCh <-chan struct{}, fwReconcileFrequency time.Duration) {
	// Use PollUntil instead of Until to wait one fwReconcileFrequency interval
	// before syncing the cloud firewall: when the firewall controller starts
	// up, the event handler is triggered as the cache gets populated and runs
	// through all services already. There is no need to for us to do so again
	// from here.
	err := wait.PollUntil(fwReconcileFrequency, func() (done bool, err error) {
		klog.V(6).Info("running cloud firewall sync loop")
		runErr := fc.syncResource(ctx)
		if runErr != nil && ctx.Err() == nil {
			klog.Errorf("failed to run firewall reconcile loop: %v", runErr)
		}
		return false, nil
	}, stopCh)
	if err != nil {
		klog.Errorf("run loop should never error but did: %s", err)
	}
	fc.queue.ShutDown()
}

func (fc *FirewallController) runWorker() {
	for fc.processNextItem() {
	}
}

func (fc *FirewallController) processNextItem() bool {
	// Wait until there is a new item in the working queue
	key, quit := fc.queue.Get()
	if quit {
		return false
	}
	// Tell the queue that we are done with processing this key to unblock the
	// key for other workers. This allows safe parallel processing because items
	// with the same key are never processed in parallel.
	defer fc.queue.Done(key)

	ctx, cancel := context.WithTimeout(context.Background(), processWorkerItemTimeout)
	defer cancel()
	err := fc.ensureReconciledFirewallInstrumented(ctx)
	if err != nil {
		klog.Errorf("failed to process worker item: %v", err)
		fc.queue.AddRateLimited(key)
	} else {
		fc.queue.Forget(key)
	}
	return true
}

// GetPreferFromCache returns the public access firewall representation from the
// cache if available, and otherwise retrieves it from the API and updates the
// cache afterwards.
func (fm *firewallManager) GetPreferFromCache(ctx context.Context) (fw *godo.Firewall, err error) {
	if fw, isSet := fm.fwCache.getCachedFirewall(); isSet {
		return fw, nil
	}

	return fm.Get(ctx)
}

// Get returns the current public access firewall representation.
// On success, the cache is updated.
func (fm *firewallManager) Get(ctx context.Context) (fw *godo.Firewall, err error) {
	defer func() {
		if err == nil {
			fm.fwCache.updateCache(fw)
		}
	}()

	// check cache and query the API firewall service to get firewall by ID, if
	// it exists. Return it. If not, continue.
	fw, _ = fm.fwCache.getCachedFirewall()
	if fw != nil {
		fw, resp, err := fm.executeInstrumentedFirewallOperationGetByID(ctx, fw.ID)
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

	klog.V(6).Infof("filtering firewall list for firewall name %q", fm.workerFirewallName)
	fw, err = fm.executeInstrumentedFirewallOperationGetByList(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve list of firewalls from DO API: %v", err)
	}
	if fw != nil {
		klog.V(6).Infof("found firewall %q by listing", fm.workerFirewallName)
	} else {
		klog.V(6).Infof("could not find firewall %q by listing", fm.workerFirewallName)
	}
	return fw, nil
}

// Set applies the given firewall request configuration to the public access
// firewall to reconcile away any changes to the inbound rules, outbound rules,
// firewall name, and/or tags. The given firewall ID is non-empty if a firewall
// already exists.
// On success, the cache is updated.
func (fm *firewallManager) Set(ctx context.Context, fwID string, fr *godo.FirewallRequest) (err error) {
	var currentFirewall *godo.Firewall
	defer func() {
		if err == nil {
			fm.fwCache.updateCache(currentFirewall)
		}
	}()

	if fwID != "" {
		var resp *godo.Response
		currentFirewall, resp, err = fm.updateFirewall(ctx, fwID, fr)
		if err == nil {
			klog.Info("successfully updated firewall")
			return nil
		}
		if resp == nil || resp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("failed to update firewall: %v", err)
		}
	}

	// We either did not have a firewall ID (i.e., the firewall has not been
	// created yet) or we failed to update the firewall (which could happen if
	// the firewall was deleted directly). Either way, we need to (re-)create
	// it.
	currentFirewall, err = fm.createFirewall(ctx, fr)
	if err != nil {
		return fmt.Errorf("failed to create firewall: %v", err)
	}
	klog.Info("successfully created firewall")
	return nil
}

func (fm *firewallManager) createFirewall(ctx context.Context, fr *godo.FirewallRequest) (*godo.Firewall, error) {
	return fm.executeInstrumentedFirewallOperationCreate(ctx, fr)
}

func (fm *firewallManager) updateFirewall(ctx context.Context, fwID string, fr *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
	return fm.executeInstrumentedFirewallOperationUpdate(ctx, fwID, fr)
}

type portProtocol struct {
	port     int
	protocol string
	sources  *godo.Sources
}

// createReconciledFirewallRequest creates a firewall request that has the correct rules, name and tag
func (fm *firewallManager) createReconciledFirewallRequest(serviceList []*v1.Service) (*godo.FirewallRequest, error) {
	var nodePortInboundRules []godo.InboundRule
	loadBalancerPorts := make(map[portProtocol]struct{})
	for _, svc := range serviceList {
		if svc.Spec.Type == v1.ServiceTypeNodePort {
			managed, err := isManaged(svc)
			if err != nil {
				klog.Warningf("managing service %s/%s for which no correct management flag setting could be detected: %s", svc.Namespace, svc.Name, err)
				managed = true
			}
			if !managed {
				continue
			}
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
		} else if svc.Spec.Type == v1.ServiceTypeLoadBalancer {
			lbType, err := getType(svc, fm.defaultLBType)
			if err != nil {
				return nil, fmt.Errorf("failed to get load balancer type for service %s/%s: %v", svc.Namespace, svc.Name, err)
			}
			lbNetwork, err := getNetwork(svc)
			if err != nil {
				return nil, fmt.Errorf("failed to get load balancer network for service %s/%s: %v", svc.Namespace, svc.Name, err)
			}
			if lbType == godo.LoadBalancerTypeRegionalNetwork && lbNetwork == godo.LoadBalancerNetworkTypeExternal {
				// Add the health check port
				_, hcp := healthCheckPathAndPort(svc)
				if hcp == 0 {
					continue
				}
				pp := portProtocol{
					protocol: "tcp",
					port:     hcp,
				}

				if svc.Annotations[annDOLoadBalancerID] != "" {
					pp.sources = &godo.Sources{
						LoadBalancerUIDs: []string{
							svc.Annotations[annDOLoadBalancerID],
						},
					}
				}

				loadBalancerPorts[pp] = struct{}{}
				// Add the services (port, protocol)
				var protocol string
				for _, servicePort := range svc.Spec.Ports {
					switch servicePort.Protocol {
					case v1.ProtocolTCP:
						protocol = "tcp"
					case v1.ProtocolUDP:
						protocol = "udp"
					default:
						klog.Warningf("unsupported service protocol %v, skipping service port %v", servicePort.Protocol, servicePort.Name)
						continue
					}
					loadBalancerPorts[portProtocol{protocol: protocol, port: int(servicePort.Port)}] = struct{}{}
				}
			}
		}
	}
	for p := range loadBalancerPorts {
		ir := godo.InboundRule{
			Protocol:  p.protocol,
			PortRange: strconv.Itoa(p.port),
			Sources: &godo.Sources{
				Addresses: []string{"0.0.0.0/0", "::/0"},
			},
		}
		if p.sources != nil {
			ir.Sources = p.sources
		}
		nodePortInboundRules = append(nodePortInboundRules, ir)
	}
	// Sort for deterministic output
	sort.SliceStable(nodePortInboundRules, func(i, j int) bool {
		if nodePortInboundRules[i].Protocol == nodePortInboundRules[j].Protocol {
			return nodePortInboundRules[i].PortRange < nodePortInboundRules[j].PortRange
		}
		return nodePortInboundRules[i].Protocol < nodePortInboundRules[j].Protocol
	})
	return &godo.FirewallRequest{
		Name:          fm.workerFirewallName,
		InboundRules:  nodePortInboundRules,
		OutboundRules: allowAllOutboundRules,
		Tags:          fm.workerFirewallTags,
	}, nil
}

// isManaged returns if the given Service should be firewall-managed based on the
// configuration annotation. An omitted annotation applies the default behavior
// of managing firewall rules for the Service.
func isManaged(service *v1.Service) (bool, error) {
	val, found, err := getBool(service.Annotations, annotationDOFirewallManaged)
	if err != nil {
		return false, err
	}

	return !found || val, nil
}

func (fm *firewallManager) executeInstrumentedFirewallOperationGetByID(ctx context.Context, fwID string) (*godo.Firewall, *godo.Response, error) {
	return fm.executeInstrumentedFirewallOperation(ctx, firewallOperationGetByID, func(ctx context.Context) (*godo.Firewall, *godo.Response, error) {
		return fm.client.Firewalls.Get(ctx, fwID)
	})
}

func (fm *firewallManager) executeInstrumentedFirewallOperationGetByList(ctx context.Context) (*godo.Firewall, error) {
	fw, _, err := fm.executeInstrumentedFirewallOperation(ctx, firewallOperationGetByList, func(ctx context.Context) (*godo.Firewall, *godo.Response, error) {
		// iterate through firewall API provided list and return the firewall
		// with the matching firewall name.
		f := func(fw godo.Firewall) bool {
			return fw.Name == fm.workerFirewallName
		}
		return filterFirewallList(ctx, fm.client, f)
	})
	return fw, err
}

func (fm *firewallManager) executeInstrumentedFirewallOperationCreate(ctx context.Context, fr *godo.FirewallRequest) (*godo.Firewall, error) {
	fw, _, err := fm.executeInstrumentedFirewallOperation(ctx, firewallOperationCreate, func(ctx context.Context) (*godo.Firewall, *godo.Response, error) {
		return fm.client.Firewalls.Create(ctx, fr)
	})
	return fw, err
}

func (fm *firewallManager) executeInstrumentedFirewallOperationUpdate(ctx context.Context, fwID string, fr *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
	return fm.executeInstrumentedFirewallOperation(ctx, firewallOperationUpdate, func(ctx context.Context) (*godo.Firewall, *godo.Response, error) {
		return fm.client.Firewalls.Update(ctx, fwID, fr)
	})
}

func (fm *firewallManager) executeInstrumentedFirewallOperation(ctx context.Context, fwOp firewallOperation, f func(ctx context.Context) (*godo.Firewall, *godo.Response, error)) (*godo.Firewall, *godo.Response, error) {
	promLabels := prometheus.Labels{
		"operation":          string(fwOp),
		"result":             "succeeded",
		"http_response_code": "",
	}

	// The ObserverFunc gets called by the deferred ObserveDuration. The label
	// values can be updated before ObserveDuration is called with the value
	// returned from the response from the Firewall API request.
	timer := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		fm.metrics.apiOperationDuration.With(promLabels).Observe(v)
	}))
	defer func() {
		timer.ObserveDuration()
		fm.metrics.apiOperationsTotal.With(promLabels).Inc()
	}()

	fw, resp, err := f(ctx)
	if err != nil {
		promLabels["result"] = "failed"
	}
	if resp != nil {
		promLabels["http_response_code"] = strconv.Itoa(resp.StatusCode)
	}

	return fw, resp, err
}

func (fc *FirewallController) ensureReconciledFirewall(ctx context.Context) (skipped bool, err error) {
	serviceList, err := fc.serviceLister.List(labels.Everything())
	if err != nil {
		return false, fmt.Errorf("failed to list services: %v", err)
	}
	fr, err := fc.fwManager.createReconciledFirewallRequest(serviceList)
	if err != nil {
		return false, fmt.Errorf("failed to create reconciled firewall request: %v", err)
	}

	fw, err := fc.fwManager.GetPreferFromCache(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get firewall (preferred from cache): %v", err)
	}

	isEqual, diff := firewallRequestEqual(fw, fr)
	if isEqual {
		klog.V(6).Info("skipping firewall reconcile because target and cached firewall match")
		return true, nil
	}

	var fwID string
	if fw == nil {
		klog.Infof("creating firewall: %s", printRelevantFirewallRequestParts(fr))
	} else {
		fwID = fw.ID
		if diff != "" {
			diff = fmt.Sprintf("\ndiff:\n%s", diff)
		}
		klog.Infof("updating firewall\nfrom: %s\nto:   %s%s", printRelevantFirewallParts(fw), printRelevantFirewallRequestParts(fr), diff)
	}

	err = fc.fwManager.Set(ctx, fwID, fr)
	if err != nil {
		return false, fmt.Errorf("failed to set firewall: %v", err)
	}
	return false, nil
}

func (fc *FirewallController) ensureReconciledFirewallInstrumented(ctx context.Context) error {
	labels := prometheus.Labels{
		"result":     "reconciled",
		"error_type": "",
	}
	t := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		fc.fwManager.metrics.reconcileDuration.With(labels).Observe(v)
	}))
	defer func() {
		t.ObserveDuration()
		fc.fwManager.metrics.reconcilesTotal.With(labels).Inc()
	}()

	skipped, err := fc.ensureReconciledFirewall(ctx)
	if err != nil {
		labels["result"] = "failed"
		labels["error_type"] = "generic"
		if ctx.Err() != nil {
			labels["error_type"] = "timeout"
		}
		return err
	}

	if skipped {
		labels["result"] = "skipped"
	}

	return nil
}

func (fc *FirewallController) syncResource(ctx context.Context) error {
	labels := prometheus.Labels{"result": "updated"}
	t := prometheus.NewTimer(prometheus.ObserverFunc(func(v float64) {
		fc.fwManager.metrics.resourceSyncDuration.With(labels).Observe(v)
	}))
	defer func() {
		t.ObserveDuration()
		fc.fwManager.metrics.resourceSyncsTotal.With(labels).Inc()
	}()

	// Ignore Get() result since we only care about the cache getting updated.
	_, err := fc.fwManager.Get(ctx)
	if err != nil {
		labels["result"] = "failed"
		if ctx.Err() != nil {
			labels["result"] = "canceled"
		}

		return err
	}

	klog.V(6).Info("issuing firewall reconcile")
	fc.queue.Add(queueKey)
	return nil
}
