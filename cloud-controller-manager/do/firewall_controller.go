/*
Copyright 2020 The Kubernetes Authors.

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
	"sync"
	"time"

	"github.com/digitalocean/godo"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/component-base/metrics/prometheus/ratelimiter"
	"k8s.io/klog"
)

const (
	// Interval of synchronizing service status from apiserver
	serviceSyncPeriod = 30 * time.Second

	// How long to wait before retrying the processing of a service change.
	// If this changes, the sleep in hack/jenkins/e2e.sh before downing a cluster
	// should be changed appropriately.
	minRetryDelay = 5 * time.Second
	maxRetryDelay = 300 * time.Second

	// The format we should expect for ccm worker firewall names.
	firewallWorkerCCMNameFormat = "k8s-%s-ccm"
)

type cachedService struct {
	// The cached state of the service
	state *v1.Service
}

type serviceCache struct {
	mu         sync.RWMutex // protects serviceMap
	serviceMap map[string]*cachedService
}

type cachedFirewall struct {
	// The cached state of the firewall
	state *godo.Firewall
}

type firewallCache struct {
	mu          sync.RWMutex // protects firewallMap
	firewallMap map[string]*cachedFirewall
}

// Controller keeps cloud provider service firewalls in sync.
type Controller struct {
	kubeClient      clientset.Interface
	svcCache        *serviceCache
	fwCache         *firewallCache
	serviceLister   corelisters.ServiceLister
	queue           workqueue.RateLimitingInterface
	firewallService *godo.FirewallsServiceOp
}

// NewFirewallController returns a new service controller to reconcile CCM worker firewall state
func NewFirewallController(
	workerFirewallName string,
	kubeClient clientset.Interface,
	firewallService *godo.FirewallsService,
	serviceInformer coreinformers.ServiceInformer,
) (*Controller, error) {
	if kubeClient != nil && kubeClient.CoreV1().RESTClient().GetRateLimiter() != nil {
		if err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage("firewall_controller", kubeClient.CoreV1().RESTClient().GetRateLimiter()); err != nil {
			return nil, err
		}
	}

	fc := &Controller{
		kubeClient: kubeClient,
		svcCache:   &serviceCache{serviceMap: make(map[string]*cachedService)},
		fwCache:    &firewallCache{firewallMap: make(map[string]*cachedFirewall)},
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(minRetryDelay, maxRetryDelay), "firewall"),
	}

	serviceInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(cur interface{}) {
				fc.checkForServiceChanges(context.TODO(), workerFirewallName)
			},
			UpdateFunc: func(old, cur interface{}) {
				fc.checkForServiceChanges(context.TODO(), workerFirewallName)
			},
			DeleteFunc: func(obj interface{}) {
				fc.checkForServiceChanges(context.TODO(), workerFirewallName)
			},
		},
		serviceSyncPeriod,
	)
	fc.serviceLister = serviceInformer.Lister()

	return fc, nil
}

func (fc *Controller) checkForServiceChanges(ctx context.Context, workerFirewallName string) error {
	if workerFirewallName == "" {
		// CCM worker firewall name flag was not set, no need to continue
		return nil
	}

	svcs, err := fc.serviceLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list services: %s", err)
	}
	// make sure that service cache is updated
	for _, s := range svcs {
		err := fc.syncService(s.Name)
		if err != nil {
			return fmt.Errorf("failed to sync service: %s", err)
		}
	}
	// reconcile firewall state of cached services
	cachedSvcs := fc.svcCache.allServices()
	for _, svc := range cachedSvcs {
		err := fc.reconcileFirewallState(ctx, workerFirewallName, svc)
		if err != nil {
			return fmt.Errorf("failed to reconcile firewall state: %s", err)
		}
	}

	return nil
}

func (fc *Controller) reconcileFirewallState(ctx context.Context, workerFirewallName string, service *v1.Service) error {
	if isNodePort(service) {
		// change it's worker firewall to accept all traffic for all protocols and sources, to NodePorts
		return fc.processFirewallDelete(service.Name, workerFirewallName, service)
	}
	// nodeport service was deleted or changed from nodeport service to some other type of service, a.k.a. it does not have type NodePort
	if !isNodePort(service) {
		// change the worker firewall to deny all traffic to NodePorts
		return fc.processFirewallCreateOrUpdate(ctx, service.Name, workerFirewallName)
	}

	return nil
}

// syncService will sync the Service with the given key if it has had its expectations fulfilled,
// meaning it did not expect to see any more of its firewalls created or deleted. This function is not meant to be
// invoked concurrently with the same key.
func (fc *Controller) syncService(key string) error {
	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished syncing service %q (%v)", key, time.Since(startTime))
	}()

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	// service holds the latest service info from apiserver
	service, err := fc.serviceLister.Services(namespace).Get(name)
	switch {
	case errors.IsNotFound(err):
		// cleanup firewall and service cache
		fc.svcCache.delete(key)
		fc.fwCache.delete(key)
	case err != nil:
		runtime.HandleError(fmt.Errorf("Unable to retrieve service %v from store: %v", key, err))
	default:
		err = fc.processServiceCreateOrUpdate(service, key)
	}

	return err
}

// processServiceCreateOrUpdate modifies (or not) the firewall for the incoming service accordingly.
// Returns an error if processing the service update failed.
func (fc *Controller) processServiceCreateOrUpdate(service *v1.Service, key string) error {
	cachedService := fc.svcCache.getOrCreate(key)
	if cachedService.state != nil && cachedService.state.UID != service.UID {
		// This happens only when a service is deleted and re-created
		// in a short period, which is only possible when it doesn't
		// contain finalizer.
		if err := fc.processFirewallDelete(key, "", cachedService.state); err != nil {
			return err
		}
	}
	// Always cache the service, we need the info for firewall modification.
	cachedService.state = service

	return nil
}

func (fc *Controller) processFirewallCreateOrUpdate(ctx context.Context, key, workerFirewallName string) error {
	cachedFirewall := fc.fwCache.getOrCreate(key)
	// We do not care about other firewalls
	if cachedFirewall.state.Name != workerFirewallName {
		return nil
	}

	// Inbound rules firewall request setup for NodePort range.
	inboundNodePortRules := &godo.InboundRule{
		Protocol:  "tcp",
		PortRange: "30000-32767",
	}
	rr := &godo.FirewallRulesRequest{
		InboundRules: []godo.InboundRule{*inboundNodePortRules},
	}
	fr := &godo.FirewallRequest{
		Name:         workerFirewallName,
		InboundRules: []godo.InboundRule{*inboundNodePortRules},
	}

	// Create the firewall if it does not exist, update it if it does.
	fw, _, _ := fc.firewallService.Get(ctx, cachedFirewall.state.ID)
	if fw == nil {
		// Create a new firewall with the worker firewall name and inbound rules for NodePort range.
		fw, err := fc.create(ctx, workerFirewallName, fr, rr)
		if err != nil {
			return fmt.Errorf("Failed to create firewall: %s", err)
		}
		cachedFirewall.state = fw
		return nil
	} else if fw.Name == workerFirewallName {
		// Update the existing firewall to include inbound rules for NodePort range.
		fw, err := fc.update(ctx, fw, rr)
		if err != nil {
			return fmt.Errorf("Failed to update firewall: %s", err)
		}
		cachedFirewall.state = fw
		return nil
	}
	// Always cache the firewall, we need the info for future firewall modification.
	cachedFirewall.state = fw

	return nil
}

func (fc *Controller) processFirewallDelete(key, workerFirewallname string, service *v1.Service) error {
	cachedFirewall := fc.fwCache.getOrCreate(service.Name)
	// We do not care about other firewalls
	if cachedFirewall.state.Name != workerFirewallName {
		return nil
	}
	_, err := fc.firewallService.Delete(context.TODO(), cachedFirewall.state.ID)
	if err != nil {
		return err
	}

	return nil
}

func (fc *Controller) processServiceDeletion(key string) {
	fc.svcCache.delete(key)
	klog.V(2).Infof("Service %v has been deleted. Attempting to cleanup firewall resources", key)
	fc.fwCache.delete(key)
}

func (fc *Controller) create(ctx context.Context, workerFirewallName string, fr *godo.FirewallRequest, rr *godo.FirewallRulesRequest) (*godo.Firewall, error) {
	// Create the firewall.
	fw, _, err := fc.firewallService.Create(ctx, fr)
	if err != nil {
		return nil, fmt.Errorf("failed to create firewall: %s", err)
	}
	// Add the inbound rules to the firewall.
	fc.firewallService.AddRules(ctx, fw.ID, rr)
	return fw, nil
}

func (fc *Controller) update(ctx context.Context, fw *godo.Firewall, rr *godo.FirewallRulesRequest) (*godo.Firewall, error) {
	// Add the inbound rules to the firewall.
	fc.firewallService.AddRules(ctx, fw.ID, rr)
	return fw, nil
}

func isNodePort(service *v1.Service) bool {
	return service.Spec.Type == v1.ServiceTypeNodePort
}

func workerserviceName(clusterUUID string) string {
	return fmt.Sprintf(firewallWorkerCCMNameFormat, clusterUUID)
}

// Run starts the firewall controller loop.
func (fc *Controller) Run(stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	defer fc.queue.ShutDown()

	klog.Info("Starting service controller")
	defer klog.Info("Shutting down service controller")

	<-stopCh
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (fc *Controller) worker() {
	for fc.processNextWorkItem() {
	}
}

func (fc *Controller) processNextWorkItem() bool {
	key, quit := fc.queue.Get()
	if quit {
		return false
	}
	defer fc.queue.Done(key)

	err := errors.NewBadRequest("place holder")
	runtime.HandleError(fmt.Errorf("error processing service %v (will retry): %v", key, err))
	fc.queue.AddRateLimited(key)
	return true
}

// ListKeys implements the interface required by DeltaFIFO to list the keys we
// already know about.
func (s *serviceCache) ListKeys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.serviceMap))
	for k := range s.serviceMap {
		keys = append(keys, k)
	}
	return keys
}

// GetByKey returns the value stored in the serviceMap under the given key
func (s *serviceCache) GetByKey(key string) (interface{}, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if v, ok := s.serviceMap[key]; ok {
		return v, true, nil
	}
	return nil, false, nil
}

// ListKeys implements the interface required by DeltaFIFO to list the keys we
// already know about.
func (s *serviceCache) allServices() []*v1.Service {
	s.mu.RLock()
	defer s.mu.RUnlock()
	services := make([]*v1.Service, 0, len(s.serviceMap))
	for _, v := range s.serviceMap {
		services = append(services, v.state)
	}
	return services
}

func (s *serviceCache) get(serviceName string) (*cachedService, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	service, ok := s.serviceMap[serviceName]
	return service, ok
}

func (s *serviceCache) getOrCreate(serviceName string) *cachedService {
	s.mu.Lock()
	defer s.mu.Unlock()
	service, ok := s.serviceMap[serviceName]
	if !ok {
		service = &cachedService{}
		s.serviceMap[serviceName] = service
	}
	return service
}

func (s *serviceCache) set(serviceName string, service *cachedService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.serviceMap[serviceName] = service
}

func (s *serviceCache) delete(serviceName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.serviceMap, serviceName)
}

// ListKeys implements the interface required by DeltaFIFO to list the keys we
// already know about.
func (f *firewallCache) ListKeys() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	keys := make([]string, 0, len(f.firewallMap))
	for k := range f.firewallMap {
		keys = append(keys, k)
	}
	return keys
}

// GetByKey returns the value stored in the firewallMap under the given key
func (f *firewallCache) GetByKey(key string) (interface{}, bool, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if v, ok := f.firewallMap[key]; ok {
		return v, true, nil
	}
	return nil, false, nil
}

func (f *firewallCache) get(serviceName string) (*cachedFirewall, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	firewall, ok := f.firewallMap[serviceName]
	return firewall, ok
}

func (f *firewallCache) getOrCreate(serviceName string) *cachedFirewall {
	f.mu.Lock()
	defer f.mu.Unlock()
	firewall, ok := f.firewallMap[serviceName]
	if !ok {
		firewall = &cachedFirewall{}
		f.firewallMap[serviceName] = firewall
	}
	return firewall
}

func (f *firewallCache) set(serviceName string, firewall *cachedFirewall) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.firewallMap[serviceName] = firewall
}

func (f *firewallCache) delete(serviceName string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.firewallMap, serviceName)
}
