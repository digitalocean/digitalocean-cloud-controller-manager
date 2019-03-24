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
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/digitalocean/godo"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/kubernetes/pkg/cloudprovider"
)

func TestResources_DropletByID(t *testing.T) {
	droplet := newFakeDroplet()
	resources := &resources{
		dropletIDMap: map[int]*godo.Droplet{
			droplet.ID: droplet,
		},
	}

	foundDroplet, found, err := resources.DropletByID(droplet.ID)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if want, got := true, found; want != got {
		t.Errorf("incorrect found\nwant: %#v\n got: %#v", want, got)
	}
	if want, got := droplet, foundDroplet; !reflect.DeepEqual(want, got) {
		t.Errorf("incorrect droplet\nwant: %#v\n got: %#v", want, got)
	}

	_, found, err = resources.DropletByID(1000)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if want, got := false, found; want != got {
		t.Errorf("incorrect found\nwant: %#v\n got: %#v", want, got)
	}
}

func TestResources_DropletByName(t *testing.T) {
	droplet := newFakeDroplet()
	resources := &resources{
		dropletNameMap: map[string]*godo.Droplet{
			droplet.Name: droplet,
		},
	}

	foundDroplet, found, err := resources.DropletByName(droplet.Name)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if want, got := true, found; want != got {
		t.Errorf("incorrect found\nwant: %#v\n got: %#v", want, got)
	}
	if want, got := droplet, foundDroplet; !reflect.DeepEqual(want, got) {
		t.Errorf("incorrect droplet\nwant: %#v\n got: %#v", want, got)
	}

	_, found, err = resources.DropletByName("missing")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if want, got := false, found; want != got {
		t.Errorf("incorrect found\nwant: %#v\n got: %#v", want, got)
	}
}

func TestResources_Droplets(t *testing.T) {
	droplet := &godo.Droplet{ID: 1}
	resources := &resources{
		dropletIDMap: map[int]*godo.Droplet{
			droplet.ID: droplet,
		},
	}

	foundDroplets := resources.Droplets()
	if want, got := []*godo.Droplet{droplet}, foundDroplets; !reflect.DeepEqual(want, got) {
		t.Errorf("incorrect droplets\nwant: %#v\n got: %#v", want, got)
	}
}

func TestResources_LoadBalancerByID(t *testing.T) {
	lb := &godo.LoadBalancer{ID: "uuid"}
	resources := &resources{
		loadBalancerIDMap: map[string]*godo.LoadBalancer{
			lb.ID: lb,
		},
	}

	foundLB, found, err := resources.LoadBalancerByID(lb.ID)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if want, got := true, found; want != got {
		t.Errorf("incorrect found\nwant: %#v\n got: %#v", want, got)
	}
	if want, got := lb, foundLB; !reflect.DeepEqual(want, got) {
		t.Errorf("incorrect lb\nwant: %#v\n got: %#v", want, got)
	}

	_, found, err = resources.LoadBalancerByID("missing")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if want, got := false, found; want != got {
		t.Errorf("incorrect found\nwant: %#v\n got: %#v", want, got)
	}
}

func TestResources_LoadBalancers(t *testing.T) {
	lb := &godo.LoadBalancer{ID: "uuid"}
	resources := &resources{
		loadBalancerIDMap: map[string]*godo.LoadBalancer{
			lb.ID: lb,
		},
	}

	foundLBs := resources.LoadBalancers()
	if want, got := []*godo.LoadBalancer{lb}, foundLBs; !reflect.DeepEqual(want, got) {
		t.Errorf("incorrect lbs\nwant: %#v\n got: %#v", want, got)
	}
}

var (
	clusterID    = "0caf4c4e-e835-4a05-9ee8-5726bb66ab07"
	clusterIDTag = buildK8sTag(clusterID)
)

func TestResourcesController_SyncResources(t *testing.T) {
	tests := []struct {
		name              string
		dropletsSvc       godo.DropletsService
		lbsSvc            godo.LoadBalancersService
		err               error
		expectedResources *resources
	}{
		{
			name: "happy path",
			dropletsSvc: &fakeDropletService{
				listFunc: func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
					return []godo.Droplet{{ID: 2, Name: "two"}}, newFakeOKResponse(), nil
				},
			},
			lbsSvc: &fakeLBService{
				listFn: func(ctx context.Context, opt *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
					return []godo.LoadBalancer{{ID: "2"}}, newFakeOKResponse(), nil
				},
			},
			// both droplet and lb resources updated
			expectedResources: &resources{
				dropletIDMap:      map[int]*godo.Droplet{2: {ID: 2, Name: "two"}},
				dropletNameMap:    map[string]*godo.Droplet{"two": {ID: 2, Name: "two"}},
				loadBalancerIDMap: map[string]*godo.LoadBalancer{"two:": {ID: "two"}},
			},
		},
		{
			name: "droplets svc failure",
			dropletsSvc: &fakeDropletService{
				listFunc: func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
					return nil, newFakeNotOKResponse(), errors.New("droplets svc fail")
				},
			},
			lbsSvc: &fakeLBService{
				listFn: func(ctx context.Context, opt *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
					return []godo.LoadBalancer{{ID: "2"}}, newFakeOKResponse(), nil
				},
			},
			err: errors.New("droplets svc fail"),
			// only lb resources updated
			expectedResources: &resources{
				dropletIDMap:      map[int]*godo.Droplet{1: {ID: 1, Name: "one"}},
				dropletNameMap:    map[string]*godo.Droplet{"one": {ID: 1, Name: "one"}},
				loadBalancerIDMap: map[string]*godo.LoadBalancer{"two:": {ID: "two"}},
			},
		},
		{
			name: "lbs svc failure",
			dropletsSvc: &fakeDropletService{
				listFunc: func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
					return []godo.Droplet{{ID: 2, Name: "two"}}, newFakeOKResponse(), nil
				},
			},
			lbsSvc: &fakeLBService{
				listFn: func(ctx context.Context, opt *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
					return nil, newFakeNotOKResponse(), errors.New("lbs svc fail")
				},
			},
			err: errors.New("lbs svc fail"),
			// only droplet resources updated
			expectedResources: &resources{
				dropletIDMap:      map[int]*godo.Droplet{2: {ID: 2, Name: "two"}},
				dropletNameMap:    map[string]*godo.Droplet{"two": {ID: 2, Name: "two"}},
				loadBalancerIDMap: map[string]*godo.LoadBalancer{"one": {ID: "one"}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fakeResources := newResources()
			fakeResources.UpdateDroplets([]godo.Droplet{
				{ID: 1, Name: "one"},
			})
			fakeResources.UpdateLoadBalancers([]godo.LoadBalancer{
				{ID: "one"},
			})
			kclient := fake.NewSimpleClientset()
			inf := informers.NewSharedInformerFactory(kclient, 0)
			gclient := &godo.Client{
				Droplets:      test.dropletsSvc,
				LoadBalancers: test.lbsSvc,
			}
			res := NewResourcesController(clusterID, fakeResources, inf.Core().V1().Services(), kclient, gclient)

			err := res.syncResources()
			if test.err != nil && err == nil {
				t.Error("expected error but got none")
			}
			if test.err == nil && err != nil {
				t.Errorf("unexpected error: %s", err)
			}

			if want, got := test.expectedResources, res.resources; !reflect.DeepEqual(want, got) {
				t.Errorf("incorrect resources\nwant: %#v\n got: %#v", want, got)
			}
		})
	}
}

func lbName(idx int) string {
	svc := createSvc(idx, false)
	return cloudprovider.GetLoadBalancerName(svc)
}

func createSvc(idx int, isTypeLoadBalancer bool) *corev1.Service {
	rep := func(num int) string {
		return strings.Repeat(strconv.Itoa(idx), num)
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("service%d", idx),
			UID:  types.UID(fmt.Sprintf("%s-%s-%s-%s-%s", rep(7), rep(4), rep(4), rep(4), rep(12))),
		},
	}
	if isTypeLoadBalancer {
		svc.Spec.Type = corev1.ServiceTypeLoadBalancer
	}
	return svc
}

func TestResourcesController_SyncTags(t *testing.T) {
	testcases := []struct {
		name        string
		services    []*corev1.Service
		lbs         []*godo.LoadBalancer
		tagSvc      *fakeTagsService
		errMsg      string
		tagRequests []*godo.TagResourcesRequest
	}{
		{
			name:     "no matching services",
			services: []*corev1.Service{createSvc(1, true)},
			lbs: []*godo.LoadBalancer{
				{ID: "1", Name: lbName(2)},
			},
		},
		{
			name:     "service without LoadBalancer type",
			services: []*corev1.Service{createSvc(1, false)},
			lbs: []*godo.LoadBalancer{
				{ID: "1", Name: lbName(1)},
			},
		},
		{
			name:     "unrecoverable resource tagging error",
			services: []*corev1.Service{createSvc(1, true)},
			lbs: []*godo.LoadBalancer{
				{ID: "1", Name: lbName(1)},
			},
			tagSvc: newFakeTagsServiceWithFailure(0, errors.New("no tagging for you")),
			errMsg: "no tagging for you",
		},
		{
			name:     "unrecoverable resource creation error",
			services: []*corev1.Service{createSvc(1, true)},
			lbs: []*godo.LoadBalancer{
				{ID: "1", Name: lbName(1)},
			},
			tagSvc: newFakeTagsServiceWithFailure(1, errors.New("no tag creating for you")),
			errMsg: "no tag creating for you",
		},
		{
			name: "success on first resource tagging",
			services: []*corev1.Service{
				createSvc(1, true),
			},
			lbs: []*godo.LoadBalancer{
				{Name: lbName(1)},
			},
			tagSvc: newFakeTagsService(clusterIDTag),
		},
		{
			name: "multiple tags",
			services: []*corev1.Service{
				createSvc(1, true),
				createSvc(2, true),
			},
			lbs: []*godo.LoadBalancer{
				{ID: "1", Name: lbName(1)},
				{ID: "2", Name: lbName(2)},
			},
			tagSvc: newFakeTagsService(clusterIDTag),
			tagRequests: []*godo.TagResourcesRequest{
				{
					Resources: []godo.Resource{
						{
							ID:   "1",
							Type: godo.LoadBalancerResourceType,
						},
						{
							ID:   "2",
							Type: godo.LoadBalancerResourceType,
						},
					},
				},
			},
		},
		{
			name: "success on second resource tagging",
			services: []*corev1.Service{
				createSvc(1, true),
			},
			lbs: []*godo.LoadBalancer{
				{ID: "1", Name: lbName(1)},
			},
			tagSvc: newFakeTagsService(),
		},
	}

	for _, test := range testcases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fakeResources := newResources()
			for _, lb := range test.lbs {
				lb := lb
				fakeResources.loadBalancerIDMap[lb.ID] = lb
			}
			fakeTagsService := test.tagSvc
			if fakeTagsService == nil {
				fakeTagsService = newFakeTagsServiceWithFailure(0, errors.New("tags service not configured, should probably not have been called"))
			}

			gclient := godo.NewClient(nil)
			gclient.Tags = fakeTagsService
			kclient := fake.NewSimpleClientset()

			for _, svc := range test.services {
				_, err := kclient.CoreV1().Services(corev1.NamespaceDefault).Create(svc)
				if err != nil {
					t.Fatalf("failed to create service: %s", err)
				}
			}

			sharedInformer := informers.NewSharedInformerFactory(kclient, 0)
			res := NewResourcesController(clusterID, fakeResources, sharedInformer.Core().V1().Services(), kclient, gclient)
			sharedInformer.Start(nil)
			sharedInformer.WaitForCacheSync(nil)

			wantErr := test.errMsg != ""
			err := res.syncTags()
			if wantErr != (err != nil) {
				t.Fatalf("got error %q, want error: %t", err, wantErr)
			}

			if wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("error message %q does not contain %q", err.Error(), test.errMsg)
			}

			if test.tagRequests != nil {
				// We need to sort request resources for reliable test
				// assertions as informer's List() ordering is indeterministic.
				for _, tagReq := range fakeTagsService.tagRequests {
					sort.SliceStable(tagReq.Resources, func(i, j int) bool {
						return tagReq.Resources[i].ID < tagReq.Resources[j].ID
					})
				}

				if !reflect.DeepEqual(test.tagRequests, fakeTagsService.tagRequests) {
					want, _ := json.Marshal(test.tagRequests)
					got, _ := json.Marshal(fakeTagsService.tagRequests)
					t.Errorf("want tagRequests %s, got %s", want, got)
				}
			}
		})
	}
}
