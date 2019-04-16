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
	"sync"
	"testing"
	"time"

	"github.com/digitalocean/godo"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

func TestResources_DropletByID(t *testing.T) {
	tests := []struct {
		name         string
		droplets     []godo.Droplet
		findID       int
		foundDroplet *godo.Droplet
		found        bool
	}{
		{
			name:         "existing droplet",
			droplets:     []godo.Droplet{{ID: 1, Name: "one"}},
			findID:       1,
			foundDroplet: &godo.Droplet{ID: 1, Name: "one"},
			found:        true,
		},
		{
			name:         "missing droplet",
			droplets:     []godo.Droplet{{ID: 1, Name: "one"}},
			findID:       2,
			foundDroplet: nil,
			found:        false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			resources := newResources("", "")
			resources.UpdateDroplets(test.droplets)

			droplet, found := resources.DropletByID(test.findID)
			if want, got := test.found, found; want != got {
				t.Errorf("incorrect found\nwant: %#v\n got: %#v", want, got)
			}
			if want, got := test.foundDroplet, droplet; !reflect.DeepEqual(want, got) {
				t.Errorf("incorrect droplet\nwant: %#v\n got: %#v", want, got)
			}
		})
	}
}

func TestResources_DropletByName(t *testing.T) {
	tests := []struct {
		name         string
		droplets     []godo.Droplet
		findName     string
		foundDroplet *godo.Droplet
		found        bool
	}{
		{
			name:         "existing droplet",
			droplets:     []godo.Droplet{{ID: 1, Name: "one"}},
			findName:     "one",
			foundDroplet: &godo.Droplet{ID: 1, Name: "one"},
			found:        true,
		},
		{
			name:         "missing droplet",
			droplets:     []godo.Droplet{{ID: 1, Name: "one"}},
			findName:     "two",
			foundDroplet: nil,
			found:        false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			resources := newResources("", "")
			resources.UpdateDroplets(test.droplets)

			droplet, found := resources.DropletByName(test.findName)
			if want, got := test.found, found; want != got {
				t.Errorf("incorrect found\nwant: %#v\n got: %#v", want, got)
			}
			if want, got := test.foundDroplet, droplet; !reflect.DeepEqual(want, got) {
				t.Errorf("incorrect droplet\nwant: %#v\n got: %#v", want, got)
			}
		})
	}
}

func TestResources_Droplets(t *testing.T) {
	droplets := []*godo.Droplet{
		{ID: 1}, {ID: 2},
	}
	resources := &resources{
		dropletIDMap: map[int]*godo.Droplet{
			droplets[0].ID: droplets[0],
			droplets[1].ID: droplets[1],
		},
	}

	foundDroplets := resources.Droplets()
	// order found droplets by id so we can compare
	sort.Slice(foundDroplets, func(a, b int) bool { return foundDroplets[a].ID < foundDroplets[b].ID })
	if want, got := droplets, foundDroplets; !reflect.DeepEqual(want, got) {
		t.Errorf("incorrect droplets\nwant: %#v\n got: %#v", want, got)
	}
}

func TestResources_LoadBalancerByID(t *testing.T) {
	tests := []struct {
		name    string
		lbs     []godo.LoadBalancer
		findID  string
		foundLB *godo.LoadBalancer
		found   bool
	}{
		{
			name:    "existing lb",
			lbs:     []godo.LoadBalancer{{ID: "1", Name: "one"}},
			findID:  "1",
			foundLB: &godo.LoadBalancer{ID: "1", Name: "one"},
			found:   true,
		},
		{
			name:    "missing lb",
			lbs:     []godo.LoadBalancer{{ID: "1", Name: "one"}},
			findID:  "two",
			foundLB: nil,
			found:   false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			resources := newResources("", "")
			resources.UpdateLoadBalancers(test.lbs)

			lb, found := resources.LoadBalancerByID(test.findID)
			if want, got := test.found, found; want != got {
				t.Errorf("incorrect found\nwant: %#v\n got: %#v", want, got)
			}
			if want, got := test.foundLB, lb; !reflect.DeepEqual(want, got) {
				t.Errorf("incorrect lb\nwant: %#v\n got: %#v", want, got)
			}
		})
	}
}

func TestResources_AddLoadBalancer(t *testing.T) {
	tests := []struct {
		name            string
		lbs             []godo.LoadBalancer
		newLB           godo.LoadBalancer
		expectedIDMap   map[string]*godo.LoadBalancer
		expectedNameMap map[string]*godo.LoadBalancer
	}{
		{
			name:  "update existing",
			lbs:   []godo.LoadBalancer{{ID: "1", Name: "one"}},
			newLB: godo.LoadBalancer{ID: "1", Name: "new"},
			expectedIDMap: map[string]*godo.LoadBalancer{
				"1": {ID: "1", Name: "new"},
			},
			expectedNameMap: map[string]*godo.LoadBalancer{
				"new": {ID: "1", Name: "new"},
			},
		},
		{
			name:  "update new",
			lbs:   []godo.LoadBalancer{{ID: "1", Name: "one"}},
			newLB: godo.LoadBalancer{ID: "2", Name: "two"},
			expectedIDMap: map[string]*godo.LoadBalancer{
				"1": {ID: "1", Name: "one"},
				"2": {ID: "2", Name: "two"},
			},
			expectedNameMap: map[string]*godo.LoadBalancer{
				"one": {ID: "1", Name: "one"},
				"two": {ID: "2", Name: "two"},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			resources := newResources("", "")
			resources.UpdateLoadBalancers(test.lbs)

			resources.AddLoadBalancer(test.newLB)

			if want, got := test.expectedIDMap, resources.loadBalancerIDMap; !reflect.DeepEqual(want, got) {
				t.Errorf("incorrect id map\nwant :%#v\n got: %#v", want, got)
			}
			if want, got := test.expectedNameMap, resources.loadBalancerNameMap; !reflect.DeepEqual(want, got) {
				t.Errorf("incorrect name map\nwant :%#v\n got: %#v", want, got)
			}
		})
	}
}

func TestResources_LoadBalancers(t *testing.T) {
	lbs := []*godo.LoadBalancer{{ID: "1"}, {ID: "2"}}
	resources := &resources{
		loadBalancerIDMap: map[string]*godo.LoadBalancer{
			lbs[0].ID: lbs[0],
			lbs[1].ID: lbs[1],
		},
	}

	foundLBs := resources.LoadBalancers()
	// order found lbs by id so we can compare
	sort.Slice(foundLBs, func(a, b int) bool { return foundLBs[a].ID < foundLBs[b].ID })
	if want, got := lbs, foundLBs; !reflect.DeepEqual(want, got) {
		t.Errorf("incorrect lbs\nwant: %#v\n got: %#v", want, got)
	}
}

type recordingSyncer struct {
	*tickerSyncer

	synced map[string]int
	mutex  sync.Mutex
	stopOn int
	stopCh chan struct{}
}

func newRecordingSyncer(stopOn int, stopCh chan struct{}) *recordingSyncer {
	return &recordingSyncer{
		tickerSyncer: &tickerSyncer{},
		synced:       make(map[string]int),
		stopOn:       stopOn,
		stopCh:       stopCh,
	}
}

func (s *recordingSyncer) Sync(name string, period time.Duration, stopCh <-chan struct{}, fn func() error) {
	recordingFn := func() error {
		s.mutex.Lock()
		defer s.mutex.Unlock()

		count, _ := s.synced[name]
		s.synced[name] = count + 1

		if len(s.synced) == s.stopOn {
			close(s.stopCh)
		}

		return fn()
	}

	s.tickerSyncer.Sync(name, period, stopCh, recordingFn)
}

var (
	clusterID    = "0caf4c4e-e835-4a05-9ee8-5726bb66ab07"
	clusterIDTag = buildK8sTag(clusterID)
)

func TestResourcesController_Run(t *testing.T) {
	fakeResources := newResources(clusterID, "")
	kclient := fake.NewSimpleClientset()
	inf := informers.NewSharedInformerFactory(kclient, 0)
	gclient := &godo.Client{
		Droplets: &fakeDropletService{
			listFunc: func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
				return []godo.Droplet{{ID: 2, Name: "two"}}, newFakeOKResponse(), nil
			},
		},
		LoadBalancers: &fakeLBService{
			listFn: func(ctx context.Context, opt *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{{ID: "2", Name: "two"}}, newFakeOKResponse(), nil
			},
		},
	}

	res := NewResourcesController(fakeResources, inf.Core().V1().Services(), kclient, gclient)
	stop := make(chan struct{})
	syncer := newRecordingSyncer(2, stop)
	res.syncer = syncer

	res.Run(stop)

	select {
	case <-stop:
		// No-op: test succeeded
	case <-time.After(3 * time.Second):
		// Terminate goroutines just in case.
		close(stop)
		t.Errorf("resources calls: %d tags calls: %d", syncer.synced["resources syncer"], syncer.synced["tags syncer"])
	}
}

func TestResourcesController_SyncResources(t *testing.T) {
	tests := []struct {
		name              string
		dropletsSvc       godo.DropletsService
		lbsSvc            godo.LoadBalancersService
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
					return []godo.LoadBalancer{{ID: "2", Name: "two"}}, newFakeOKResponse(), nil
				},
			},
			// both droplet and lb resources updated
			expectedResources: &resources{
				dropletIDMap:        map[int]*godo.Droplet{2: {ID: 2, Name: "two"}},
				dropletNameMap:      map[string]*godo.Droplet{"two": {ID: 2, Name: "two"}},
				loadBalancerIDMap:   map[string]*godo.LoadBalancer{"2": {ID: "2", Name: "two"}},
				loadBalancerNameMap: map[string]*godo.LoadBalancer{"two": {ID: "2", Name: "two"}},
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
					return []godo.LoadBalancer{{ID: "2", Name: "two"}}, newFakeOKResponse(), nil
				},
			},
			// only lb resources updated
			expectedResources: &resources{
				dropletIDMap:        map[int]*godo.Droplet{1: {ID: 1, Name: "one"}},
				dropletNameMap:      map[string]*godo.Droplet{"one": {ID: 1, Name: "one"}},
				loadBalancerIDMap:   map[string]*godo.LoadBalancer{"2": {ID: "2", Name: "two"}},
				loadBalancerNameMap: map[string]*godo.LoadBalancer{"two": {ID: "2", Name: "two"}},
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
			// only droplet resources updated
			expectedResources: &resources{
				dropletIDMap:        map[int]*godo.Droplet{2: {ID: 2, Name: "two"}},
				dropletNameMap:      map[string]*godo.Droplet{"two": {ID: 2, Name: "two"}},
				loadBalancerIDMap:   map[string]*godo.LoadBalancer{"1": {ID: "1", Name: "one"}},
				loadBalancerNameMap: map[string]*godo.LoadBalancer{"one": {ID: "1", Name: "one"}},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fakeResources := newResources("", "")
			fakeResources.UpdateDroplets([]godo.Droplet{
				{ID: 1, Name: "one"},
			})
			fakeResources.UpdateLoadBalancers([]godo.LoadBalancer{
				{ID: "1", Name: "one"},
			})
			kclient := fake.NewSimpleClientset()
			inf := informers.NewSharedInformerFactory(kclient, 0)
			gclient := &godo.Client{
				Droplets:      test.dropletsSvc,
				LoadBalancers: test.lbsSvc,
			}
			res := NewResourcesController(fakeResources, inf.Core().V1().Services(), kclient, gclient)
			res.syncResources()
			if want, got := test.expectedResources, res.resources; !reflect.DeepEqual(want, got) {
				t.Errorf("incorrect resources\nwant: %#v\n got: %#v", want, got)
			}
		})
	}
}

func lbName(idx int) string {
	svc := createSvc(idx, false)
	return getDefaultLoadBalancerName(svc)
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

			fakeResources := newResources("", "")
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
			res := NewResourcesController(fakeResources, sharedInformer.Core().V1().Services(), kclient, gclient)
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
