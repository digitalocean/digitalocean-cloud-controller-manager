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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

type serviceBuilder struct {
	idx                int
	isTypeLoadBalancer bool
	loadBalancerID     string
	certificateID      string
	certificateType    string
}

func newSvcBuilder(idx int) *serviceBuilder {
	return &serviceBuilder{
		idx: idx,
	}
}

func (sb *serviceBuilder) setTypeLoadBalancer(isTypeLoadBalancer bool) *serviceBuilder {
	sb.isTypeLoadBalancer = isTypeLoadBalancer
	return sb
}

func (sb *serviceBuilder) setLoadBalancerID(id string) *serviceBuilder {
	sb.loadBalancerID = id
	return sb
}

func (sb *serviceBuilder) setCertificateIDAndType(id, certType string) *serviceBuilder {
	sb.certificateID = id
	sb.certificateType = certType
	return sb
}

func (sb *serviceBuilder) build() *v1.Service {
	rep := func(num int) string {
		return strings.Repeat(strconv.Itoa(sb.idx), num)
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("service%d", sb.idx),
			Namespace: corev1.NamespaceDefault,
			UID:       types.UID(fmt.Sprintf("%s-%s-%s-%s-%s", rep(7), rep(4), rep(4), rep(4), rep(12))),
		},
	}
	if sb.isTypeLoadBalancer {
		svc.Spec.Type = corev1.ServiceTypeLoadBalancer
	}
	if sb.loadBalancerID != "" {
		if svc.Annotations == nil {
			svc.Annotations = map[string]string{}
		}
		svc.Annotations[annoDOLoadBalancerID] = sb.loadBalancerID
	}

	return svc
}

func lbName(idx int) string {
	svc := newSvcBuilder(idx).build()
	return getDefaultLoadBalancerName(svc)
}

func createLBSvc(idx int) *corev1.Service {
	return newSvcBuilder(idx).setTypeLoadBalancer(true).build()
}

type recordingSyncer struct {
	*tickerSyncer

	synced int
	mutex  sync.Mutex
	stopOn int
	stopCh chan struct{}
}

func newRecordingSyncer(stopOn int, stopCh chan struct{}) *recordingSyncer {
	return &recordingSyncer{
		tickerSyncer: &tickerSyncer{},
		stopOn:       stopOn,
		stopCh:       stopCh,
	}
}

func (s *recordingSyncer) Sync(name string, period time.Duration, stopCh <-chan struct{}, fn func() error) {
	recordingFn := func() error {
		s.mutex.Lock()
		defer s.mutex.Unlock()

		s.synced++

		if s.synced == s.stopOn {
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
	gclient := newFakeClient(
		&fakeDropletService{
			listFunc: func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
				return []godo.Droplet{{ID: 2, Name: "two"}}, newFakeOKResponse(), nil
			},
		},
		&fakeLBService{
			listFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{{ID: "2", Name: "two"}}, newFakeOKResponse(), nil
			},
		},
		nil,
	)
	fakeResources := newResources(clusterID, "", gclient)
	kclient := fake.NewSimpleClientset()
	inf := informers.NewSharedInformerFactory(kclient, 0)

	res := NewResourcesController(fakeResources, inf.Core().V1().Services(), kclient)
	stop := make(chan struct{})
	wantSyncs := 1
	syncer := newRecordingSyncer(wantSyncs, stop)
	res.syncer = syncer

	res.Run(stop)

	select {
	case <-stop:
		// No-op: test succeeded
	case <-time.After(3 * time.Second):
		// Terminate goroutines just in case.
		close(stop)
		t.Errorf("got %d distinct sync(s) within timeout, want %d", syncer.synced, wantSyncs)
	}
}

func TestResourcesController_SyncTags(t *testing.T) {
	testcases := []struct {
		name        string
		services    []*corev1.Service
		lbSvcListFn func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error)
		tagSvc      *fakeTagsService
		errMsg      string
		tagRequests []*godo.TagResourcesRequest
	}{
		{
			name:     "no matching services",
			services: []*corev1.Service{createLBSvc(1)},
			lbSvcListFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{
					{ID: "1", Name: lbName(2)},
				}, newFakeOKResponse(), nil
			},
		},
		{
			name: "service without LoadBalancer type",
			services: []*corev1.Service{
				newSvcBuilder(1).setTypeLoadBalancer(false).build(),
			},
			lbSvcListFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return nil, newFakeNotOKResponse(), errors.New("list should not have been invoked")
			},
		},
		{
			name:     "unrecoverable resource tagging error",
			services: []*corev1.Service{createLBSvc(1)},
			lbSvcListFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{
					{ID: "1", Name: lbName(1)},
				}, newFakeOKResponse(), nil
			},
			tagSvc: newFakeTagsServiceWithFailure(0, errors.New("no tagging for you")),
			errMsg: "no tagging for you",
		},
		{
			name:     "unrecoverable resource creation error",
			services: []*corev1.Service{createLBSvc(1)},
			lbSvcListFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{
					{ID: "1", Name: lbName(1)},
				}, newFakeOKResponse(), nil
			},
			tagSvc: newFakeTagsServiceWithFailure(1, errors.New("no tag creating for you")),
			errMsg: "no tag creating for you",
		},
		{
			name: "success on first resource tagging",
			services: []*corev1.Service{
				createLBSvc(1),
			},
			lbSvcListFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{
					{Name: lbName(1)},
				}, newFakeOKResponse(), nil
			},
			tagSvc: newFakeTagsService(clusterIDTag),
		},
		{
			name: "multiple tags",
			services: []*corev1.Service{
				createLBSvc(1),
				createLBSvc(2),
			},
			lbSvcListFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{
					{ID: "1", Name: lbName(1)},
					{ID: "2", Name: lbName(2)},
				}, newFakeOKResponse(), nil
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
				createLBSvc(1),
			},
			lbSvcListFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{
					{ID: "1", Name: lbName(1)},
				}, newFakeOKResponse(), nil
			},
			tagSvc: newFakeTagsService(),
		},
		{
			name: "found LB resource by ID annotation",
			services: []*corev1.Service{
				newSvcBuilder(1).setTypeLoadBalancer(true).setLoadBalancerID("f7968b52-4ed9-4a16-af8b-304253f04e20").build(),
			},
			lbSvcListFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{
					{Name: "renamed-lb"},
				}, newFakeOKResponse(), nil
			},
			tagSvc: newFakeTagsService(clusterIDTag),
			tagRequests: []*godo.TagResourcesRequest{
				{
					Resources: []godo.Resource{
						{
							ID:   "f7968b52-4ed9-4a16-af8b-304253f04e20",
							Type: godo.LoadBalancerResourceType,
						},
					},
				},
			},
		},
	}

	for _, test := range testcases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			gclient := newFakeLBClient(
				&fakeLBService{
					listFn: test.lbSvcListFn,
				},
			)

			fakeResources := newResources("", "", gclient)
			fakeTagsService := test.tagSvc
			if fakeTagsService == nil {
				fakeTagsService = newFakeTagsServiceWithFailure(0, errors.New("tags service not configured, should probably not have been called"))
			}

			gclient.Tags = fakeTagsService
			kclient := fake.NewSimpleClientset()

			for _, svc := range test.services {
				_, err := kclient.CoreV1().Services(corev1.NamespaceDefault).Create(svc)
				if err != nil {
					t.Fatalf("failed to create service: %s", err)
				}
			}

			sharedInformer := informers.NewSharedInformerFactory(kclient, 0)
			res := NewResourcesController(fakeResources, sharedInformer.Core().V1().Services(), kclient)
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
