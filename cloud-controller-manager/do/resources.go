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
	controllerSyncTagsPeriod = 15 * time.Minute
	syncTagsTimeout          = 1 * time.Minute
)

type tagMissingError struct {
	error
}

type resources struct {
	clusterID    string
	clusterVPCID string

	gclient *godo.Client
	kclient kubernetes.Interface

	mutex sync.RWMutex
}

// newResources initializes a new resources instance.
// kclient can only be set during the cloud. Initialize call since that is when
// the cloud provider framework provides us with a clientset. Fortunately, the
// initialization order guarantees that kclient won't be consumed prior to it
// being set.
func newResources(clusterID, clusterVPCID string, gclient *godo.Client) *resources {
	return &resources{
		clusterID:    clusterID,
		clusterVPCID: clusterVPCID,

		gclient: gclient,
	}
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
	svcLister v1lister.ServiceLister

	resources *resources
	syncer    syncer
}

// NewResourcesController returns a new resource controller.
func NewResourcesController(r *resources, inf v1informers.ServiceInformer, client kubernetes.Interface) *ResourcesController {
	r.kclient = client

	return &ResourcesController{
		resources: r,
		kclient:   client,
		svcLister: inf.Lister(),
		syncer:    &tickerSyncer{},
	}
}

// Run starts the resources controller loop.
func (r *ResourcesController) Run(stopCh <-chan struct{}) {
	if r.resources.clusterID == "" {
		klog.Info("No cluster ID configured -- skipping cluster dependent syncers.")
		return
	}
	go r.syncer.Sync("tags syncer", controllerSyncTagsPeriod, stopCh, r.syncTags)
}

// syncTags synchronizes tags. Currently, this is only needed to associate
// cluster ID tags with LoadBalancer resources.
func (r *ResourcesController) syncTags() error {
	ctx, cancel := context.WithTimeout(context.Background(), syncTagsTimeout)
	defer cancel()

	svcs, err := r.svcLister.List(labels.Everything())
	if err != err {
		return fmt.Errorf("failed to list services: %s", err)
	}

	var lbSvcs []*corev1.Service
	for _, svc := range svcs {
		if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
			lbSvcs = append(lbSvcs, svc)
		}
	}

	if len(lbSvcs) == 0 {
		klog.V(5).Info("No load-balancers to tag because no LoadBalancer-typed services exist")
		return nil
	}

	lbs, err := allLoadBalancerList(ctx, r.resources.gclient)
	if err != nil {
		return fmt.Errorf("failed to list load-balancers: %s", err)
	}

	// Collect tag resources for known load-balancers (i.e., services with
	// type=LoadBalancer that either have our own LB ID annotation set or go by
	// a matching name).
	var res []godo.Resource
	for _, svc := range lbSvcs {
		id, err := findLoadBalancerID(svc, lbs)
		if err != nil && err != errLBNotFound {
			return fmt.Errorf("error occurred when searching for a load balancer: %s", err)
		}

		// Load-balancers that have no LB ID set yet and were renamed directly
		// (e.g., via the cloud control panel) would still be missed, so check
		// again if we have found an ID.
		if id != "" {
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
		_, _, err = r.resources.gclient.Tags.Create(ctx, &godo.TagCreateRequest{
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
	resp, err := r.resources.gclient.Tags.TagResources(ctx, tag, &godo.TagResourcesRequest{
		Resources: res,
	})

	if resp != nil && resp.StatusCode == http.StatusNotFound {
		return tagMissingError{fmt.Errorf("tag %q does not exist", tag)}
	}

	return err
}
