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
	"time"

	"github.com/digitalocean/godo"
	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	v1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	v1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/kubernetes/pkg/cloudprovider"
)

const (
	controllerSyncPeriod     = 1 * time.Minute
	requestTimeout           = 10 * time.Second
	resourceTypeLoadBalancer = "load_balancer"
)

type tagMissingError struct {
	error
}

// ResourcesController ensures that DO resources are properly tagged.
type ResourcesController struct {
	clusterIDTag string
	kclient      kubernetes.Interface
	gclient      *godo.Client
	svcLister    v1lister.ServiceLister
}

// NewResourcesController returns a new services controller responsible for
// re-applying cluster IDs on DO load-balancers.
func NewResourcesController(clusterIDTag string, inf v1informers.ServiceInformer,
	k kubernetes.Interface, g *godo.Client) *ResourcesController {
	return &ResourcesController{
		clusterIDTag: clusterIDTag,
		kclient:      k,
		gclient:      g,
		svcLister:    inf.Lister(),
	}
}

// Run starts the resources controller. It watches over DigitalOcean resources
// making sure the right tags are set.
func (r *ResourcesController) Run(stopCh <-chan struct{}) {
	ticker := time.NewTicker(controllerSyncPeriod)
	defer ticker.Stop()
	// Do not wait for initial tick to pass; run immediately for reduced sync
	// latency.
	for tickerC := ticker.C; ; {
		if err := r.sync(); err != nil {
			glog.Errorf("failed to sync load-balancer tags: %s", err)
		}
		select {
		case <-tickerC:
			continue
		case <-stopCh:
			return
		}
	}
}

func (r *ResourcesController) sync() error {
	// Fetch all load-balancers.
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	lbs, _, err := r.gclient.LoadBalancers.List(ctx, &godo.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list load-balancers: %s", err)
	}

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

		name := cloudprovider.GetLoadBalancerName(svc)
		if id, ok := loadBalancers[name]; ok {
			res = append(res, godo.Resource{
				ID:   id,
				Type: godo.ResourceType(resourceTypeLoadBalancer),
			})
		}
	}

	if len(res) == 0 {
		return nil
	}

	// Tag collected resources with the cluster ID. If the tag does not exist
	// (for reasons outlined below), we will create it and retry tagging again.
	err = r.tagResources(res)
	if _, ok := err.(tagMissingError); ok {
		// Cluster ID tag has not been created yet. This should have happen
		// when we set the tag on LB creation. For LBs that have been created
		// prior to CCM using cluster IDs, however, we need to create the tag
		// explicitly.
		_, _, err = r.gclient.Tags.Create(ctx, &godo.TagCreateRequest{
			Name: r.clusterIDTag,
		})
		if err != nil {
			return fmt.Errorf("failed to create tag %q: %s", r.clusterIDTag, err)
		}

		// Try tagging again, which should not fail anymore due to a missing
		// tag.
		err = r.tagResources(res)
	}

	if err != nil {
		return fmt.Errorf("failed to tag LB resource(s) %v with tag %q: %s", res, r.clusterIDTag, err)
	}

	return nil
}

func (r *ResourcesController) tagResources(res []godo.Resource) error {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	tag := r.clusterIDTag
	resp, err := r.gclient.Tags.TagResources(ctx, tag, &godo.TagResourcesRequest{
		Resources: res,
	})

	if resp != nil && resp.StatusCode == http.StatusNotFound {
		return tagMissingError{fmt.Errorf("tag %q does not exist", tag)}
	}

	return err
}
