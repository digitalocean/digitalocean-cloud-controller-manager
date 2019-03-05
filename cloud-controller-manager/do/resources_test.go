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

var clusterIDTag = buildK8sTag("0caf4c4e-e835-4a05-9ee8-5726bb66ab07")

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

func TestTagsSync(t *testing.T) {
	testcases := []struct {
		name        string
		services    []*corev1.Service
		lbListFn    func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error)
		tagSvc      *fakeTagsService
		errMsg      string
		tagRequests []*godo.TagResourcesRequest
	}{
		{
			name: "LB service fails",
			lbListFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return nil, nil, errors.New("no LBs for you")
			},
			errMsg: "no LBs for you",
		},
		{
			name:     "no matching services",
			services: []*corev1.Service{createSvc(1, true)},
			lbListFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{
					{Name: lbName(2)},
				}, newFakeOKResponse(), nil
			},
		},
		{
			name:     "service without LoadBalancer type",
			services: []*corev1.Service{createSvc(1, false)},
			lbListFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{
					{Name: lbName(1)},
				}, newFakeOKResponse(), nil
			},
		},
		{
			name:     "unrecoverable resource tagging error",
			services: []*corev1.Service{createSvc(1, true)},
			lbListFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{
					{Name: lbName(1)},
				}, newFakeOKResponse(), nil
			},
			tagSvc: newFakeTagsServiceWithFailure(0, errors.New("no tagging for you")),
			errMsg: "no tagging for you",
		},
		{
			name:     "unrecoverable resource creation error",
			services: []*corev1.Service{createSvc(1, true)},
			lbListFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{
					{Name: lbName(1)},
				}, newFakeOKResponse(), nil
			},
			tagSvc: newFakeTagsServiceWithFailure(1, errors.New("no tag creating for you")),
			errMsg: "no tag creating for you",
		},
		{
			name: "success on first resource tagging",
			services: []*corev1.Service{
				createSvc(1, true),
			},
			lbListFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{
					{Name: lbName(1)},
				}, newFakeOKResponse(), nil
			},
			tagSvc: newFakeTagsService(clusterIDTag),
		},
		{
			name: "multiple tags",
			services: []*corev1.Service{
				createSvc(1, true),
				createSvc(2, true),
			},
			lbListFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{
					{
						Name: lbName(1),
						ID:   "1",
					},
					{
						Name: lbName(2),
						ID:   "2",
					},
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
				createSvc(1, true),
			},
			lbListFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{
					{Name: lbName(1)},
				}, newFakeOKResponse(), nil
			},
			tagSvc: newFakeTagsService(),
		},
	}

	for _, test := range testcases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			fakeLBService := &fakeLBService{
				listFn: test.lbListFn,
			}
			fakeTagsService := test.tagSvc
			if fakeTagsService == nil {
				fakeTagsService = newFakeTagsServiceWithFailure(0, errors.New("tags service not configured, should probably not have been called"))
			}

			gclient := godo.NewClient(nil)
			gclient.LoadBalancers = fakeLBService
			gclient.Tags = fakeTagsService
			kclient := fake.NewSimpleClientset()

			for _, svc := range test.services {
				_, err := kclient.CoreV1().Services(corev1.NamespaceDefault).Create(svc)
				if err != nil {
					t.Fatalf("failed to create service: %s", err)
				}
			}

			sharedInformer := informers.NewSharedInformerFactory(kclient, 0)
			res := NewResourcesController(clusterIDTag, sharedInformer.Core().V1().Services(), kclient, gclient)
			sharedInformer.Start(nil)
			sharedInformer.WaitForCacheSync(nil)

			wantErr := test.errMsg != ""
			err := res.sync()
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
