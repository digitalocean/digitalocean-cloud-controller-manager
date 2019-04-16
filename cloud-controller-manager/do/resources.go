/*
Copyright 2017 DigitalOcean

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
	"sync"
	"time"

	"github.com/digitalocean/godo"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	v1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	v1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog"
)

const (
	controllerSyncTagsPeriod      = 1 * time.Minute
	controllerSyncResourcesPeriod = 1 * time.Minute
	syncTagsTimeout               = 1 * time.Minute
	syncResourcesTimeout          = 3 * time.Minute
)

type tagMissingError struct {
	error
}

type resources struct {
	clusterID    string
	clusterVPCID string

	dropletIDMap        map[int]*godo.Droplet
	dropletNameMap      map[string]*godo.Droplet
	loadBalancerIDMap   map[string]*godo.LoadBalancer
	loadBalancerNameMap map[string]*godo.LoadBalancer

	mutex sync.RWMutex
}

func newResources(clusterID, clusterVPCID string) *resources {
	return &resources{
		clusterID:    clusterID,
		clusterVPCID: clusterVPCID,

		dropletIDMap:        make(map[int]*godo.Droplet),
		dropletNameMap:      make(map[string]*godo.Droplet),
		loadBalancerIDMap:   make(map[string]*godo.LoadBalancer),
		loadBalancerNameMap: make(map[string]*godo.LoadBalancer),
	}
}

func (c *resources) DropletByID(id int) (droplet *godo.Droplet, found bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	droplet, found = c.dropletIDMap[id]
	return droplet, found
}

func (c *resources) DropletByName(name string) (droplet *godo.Droplet, found bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	droplet, found = c.dropletNameMap[name]
	return droplet, found
}

func (c *resources) Droplets() []*godo.Droplet {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	var droplets []*godo.Droplet
	for _, droplet := range c.dropletIDMap {
		droplet := droplet
		droplets = append(droplets, droplet)
	}

	return droplets
}

func (c *resources) LoadBalancerByID(id string) (droplet *godo.LoadBalancer, found bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	lb, found := c.loadBalancerIDMap[id]
	return lb, found
}

func (c *resources) LoadBalancerByName(name string) (droplet *godo.LoadBalancer, found bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	lb, found := c.loadBalancerNameMap[name]
	return lb, found
}

func (c *resources) LoadBalancers() []*godo.LoadBalancer {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	var lbs []*godo.LoadBalancer
	for _, lb := range c.loadBalancerIDMap {
		lb := lb
		lbs = append(lbs, lb)
	}

	return lbs
}

func (c *resources) UpdateDroplets(droplets []godo.Droplet) {
	newIDMap := make(map[int]*godo.Droplet)
	newNameMap := make(map[string]*godo.Droplet)

	for _, droplet := range droplets {
		droplet := droplet
		newIDMap[droplet.ID] = &droplet
		newNameMap[droplet.Name] = &droplet
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.dropletIDMap = newIDMap
	c.dropletNameMap = newNameMap
}

func (c *resources) AddLoadBalancer(lb godo.LoadBalancer) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	existingLB, found := c.loadBalancerIDMap[lb.ID]
	if found {
		delete(c.loadBalancerIDMap, existingLB.ID)
		delete(c.loadBalancerNameMap, existingLB.Name)
	}
	c.loadBalancerIDMap[lb.ID] = &lb
	c.loadBalancerNameMap[lb.Name] = &lb
}

func (c *resources) UpdateLoadBalancers(lbs []godo.LoadBalancer) {
	newIDMap := make(map[string]*godo.LoadBalancer)
	newNameMap := make(map[string]*godo.LoadBalancer)

	for _, lb := range lbs {
		lb := lb
		newIDMap[lb.ID] = &lb
		newNameMap[lb.Name] = &lb
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.loadBalancerIDMap = newIDMap
	c.loadBalancerNameMap = newNameMap
}

type syncer interface {
	Sync(name string, period time.Duration, stopCh <-chan struct{}, fn func() error)
}

type tickerSyncer struct{}

func (s *tickerSyncer) Sync(name string, period time.Duration, stopCh <-chan struct{}, fn func() error) {
	ticker := time.NewTicker(period)
	defer ticker.Stop()

	// manually call to avoid initial tick delay
	if err := fn(); err != nil {
		klog.Errorf("%s failed: %s", name, err)
	}

	for {
		select {
		case <-ticker.C:
			if err := fn(); err != nil {
				klog.Errorf("%s failed: %s", name, err)
			}
		case <-stopCh:
			return
		}
	}
}

// ResourcesController is responsible for managing DigitalOcean cloud
// resources. It maintains a local state of the resources and
// synchronizes when needed.
type ResourcesController struct {
	kclient   kubernetes.Interface
	gclient   *godo.Client
	svcLister v1lister.ServiceLister

	resources *resources
	syncer    syncer
}

// NewResourcesController returns a new resource controller.
func NewResourcesController(
	r *resources,
	inf v1informers.ServiceInformer,
	k kubernetes.Interface,
	g *godo.Client,
) *ResourcesController {
	return &ResourcesController{
		resources: r,
		kclient:   k,
		gclient:   g,
		svcLister: inf.Lister(),
		syncer:    &tickerSyncer{},
	}
}

// Run starts the resources controller loop.
func (r *ResourcesController) Run(stopCh <-chan struct{}) {
	go r.syncer.Sync("resources syncer", controllerSyncResourcesPeriod, stopCh, r.syncResources)

	if r.resources.clusterID == "" {
		klog.Info("No cluster ID configured -- skipping cluster dependent syncers.")
		return
	}
	go r.syncer.Sync("tags syncer", controllerSyncTagsPeriod, stopCh, r.syncTags)
}

// syncResources updates the local resources representation from the
// DigitalOcean API.
func (r *ResourcesController) syncResources() error {
	ctx, cancel := context.WithTimeout(context.Background(), syncResourcesTimeout)
	defer cancel()

	klog.V(2).Info("syncing droplet resources.")
	droplets, err := allDropletList(ctx, r.gclient)
	if err != nil {
		klog.Errorf("failed to sync droplet resources: %s.", err)
	} else {
		r.resources.UpdateDroplets(droplets)
		klog.V(2).Info("synced droplet resources.")
	}

	klog.V(2).Info("syncing load-balancer resources.")
	lbs, err := allLoadBalancerList(ctx, r.gclient)
	if err != nil {
		klog.Errorf("failed to sync load-balancer resources: %s.", err)
	} else {
		r.resources.UpdateLoadBalancers(lbs)
		klog.V(2).Info("synced load-balancer resources.")
	}

	return nil
}

// syncTags synchronizes tags. Currently, this is only needed to associate
// cluster ID tags with LoadBalancer resources.
func (r *ResourcesController) syncTags() error {
	ctx, cancel := context.WithTimeout(context.Background(), syncTagsTimeout)
	defer cancel()

	lbs := r.resources.LoadBalancers()

	// Collect tag resources for known load balancer names (i.e., services
	// with type=LoadBalancer.)
	svcs, err := r.svcLister.List(labels.Everything())
	if err != err {
		return fmt.Errorf("failed to list services: %s", err)
	}

	loadBalancers := make(map[string]string, len(lbs))
	for _, lb := range lbs {
		loadBalancers[lb.Name] = lb.ID
	}

	var res []godo.Resource
	for _, svc := range svcs {
		if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
			continue
		}

		name := getDefaultLoadBalancerName(svc)
		if id, ok := loadBalancers[name]; ok {
			res = append(res, godo.Resource{
				ID:   id,
				Type: godo.ResourceType(godo.LoadBalancerResourceType),
			})
		}
	}

	if len(res) == 0 {
		return nil
	}

	tag := buildK8sTag(r.resources.clusterID)
	// Tag collected resources with the cluster ID. If the tag does not exist
	// (for reasons outlined below), we will create it and retry tagging again.
	err = r.tagResources(res)
	if _, ok := err.(tagMissingError); ok {
		// Cluster ID tag has not been created yet. This should have happen
		// when we set the tag on LB creation. For LBs that have been created
		// prior to CCM using cluster IDs, however, we need to create the tag
		// explicitly.
		_, _, err = r.gclient.Tags.Create(ctx, &godo.TagCreateRequest{
			Name: tag,
		})
		if err != nil {
			return fmt.Errorf("failed to create tag %q: %s", tag, err)
		}

		// Try tagging again, which should not fail anymore due to a missing
		// tag.
		err = r.tagResources(res)
	}

	if err != nil {
		return fmt.Errorf("failed to tag LB resource(s) %v with tag %q: %s", res, tag, err)
	}

	return nil
}

func (r *ResourcesController) tagResources(res []godo.Resource) error {
	ctx, cancel := context.WithTimeout(context.Background(), syncTagsTimeout)
	defer cancel()
	tag := buildK8sTag(r.resources.clusterID)
	resp, err := r.gclient.Tags.TagResources(ctx, tag, &godo.TagResourcesRequest{
		Resources: res,
	})

	if resp != nil && resp.StatusCode == http.StatusNotFound {
		return tagMissingError{fmt.Errorf("tag %q does not exist", tag)}
	}

	return err
}
