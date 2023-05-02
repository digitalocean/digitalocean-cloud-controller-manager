/*
Copyright 2023 DigitalOcean

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
	"os"
	"testing"
	"time"

	"github.com/digitalocean/godo"

	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlruntimelog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	admissionv1 "k8s.io/api/admission/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	scheme = runtime.NewScheme()
)

type fakeRegionsService struct {
	listFn func(context.Context, *godo.ListOptions) ([]godo.Region, *godo.Response, error)
}

func (f *fakeRegionsService) List(ctx context.Context, listOpts *godo.ListOptions) ([]godo.Region, *godo.Response, error) {
	return f.listFn(ctx, listOpts)
}

func Test_Handle(t *testing.T) {
	//cluster := fakeCluster()
	testcases := []struct {
		name            string
		req             admission.Request
		oldObject       runtime.RawExtension
		gCLient         *godo.Client
		lbRequest       *godo.LoadBalancerRequest
		expectedAllowed bool
		expectedReason  string
		resp            *godo.Response
	}{
		{
			name: "Allow if service type is not load balancer",
			req: admission.Request{AdmissionRequest: fakeAdmissionRequest(
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeNodePort,
					},
				},
				nil,
			)},
			expectedAllowed: true,
		},
		{
			name: "Allow if request is of type DELETE",
			req: admission.Request{AdmissionRequest: fakeAdmissionRequest(
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
						DeletionTimestamp: &metav1.Time{
							Time: time.Now().UTC(),
						},
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
					},
				},
				nil,
			)},

			lbRequest:       fakeValidLBRequest(),
			expectedAllowed: true,
		},
		{
			name: "Allow CREATE happy path",
			req: admission.Request{AdmissionRequest: fakeAdmissionRequest(
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
					},
				},
				nil,
			)},
			expectedAllowed: true,
		},
		{
			name: "Deny CREATE happy path",
			req: admission.Request{AdmissionRequest: fakeAdmissionRequest(
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
					},
				},
				nil,
			)},
			expectedAllowed: false,
		},
		{
			name: "Allow Update happy path",
			req: admission.Request{AdmissionRequest: fakeAdmissionRequest(
				fakeService("test2"), fakeService("old-service"))},
			expectedAllowed: true,
		},
		{
			name: "Deny Update happy path",
			req: admission.Request{AdmissionRequest: fakeAdmissionRequest(
				fakeService("test2"), fakeService("old-service"))},
			expectedAllowed: false,
		},
	}

	for _, test := range testcases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// setup client
			gClient := godo.NewFromToken("test-token")
			gClient.LoadBalancers = &fakeLBService{
				listFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
					return []godo.LoadBalancer{{ID: "2", Name: "two"}}, newFakeOKResponse(), nil
				},
				createFn: func(context.Context, *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
					return &godo.LoadBalancer{ID: "2", Name: "two"}, newFakeOKResponse(), nil
				},
				updateFn: func(context.Context, string, *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
					return &godo.LoadBalancer{ID: "2", Name: "two"}, newFakeOKResponse(), nil
				},
			}
			gClient.Regions = &fakeRegionsService{
				listFn: func(context.Context, *godo.ListOptions) ([]godo.Region, *godo.Response, error) {
					return []godo.Region{{Name: "nyc3", Slug: "nyc3"}}, newFakeOKResponse(), nil
				}}

			decoder, err := admission.NewDecoder(scheme)
			if err != nil {
				t.Error("failed to initialize decoder", err)
			}

			os.Setenv(regionEnv, "nyc3")

			var logOpts []zap.Opts
			ll := zap.New(logOpts...).WithName("webhook-validation-server")
			ctrlruntimelog.SetLogger(ll)

			validator := &KubernetesLBServiceValidator{
				Log:     ll,
				decoder: decoder,
				GClient: gClient,
			}

			res := validator.Handle(context.TODO(), test.req)
			// if test.allowed & res.allowed are not equal
			if test.expectedAllowed && res.Allowed == false {
				t.Error("Expected is not equal to actual", res)
			}
			if !test.expectedAllowed {
				if test.expectedReason != res.Result.Message {
					t.Error("Expected reason is not the actual reason", res.Result.Message)
				}
			}
		})
	}
}

func fakeValidLBRequest() *godo.LoadBalancerRequest {
	return &godo.LoadBalancerRequest{
		Name:       "test",
		Region:     "nyc3",
		DropletIDs: []int{1},
		ForwardingRules: []godo.ForwardingRule{
			{
				EntryProtocol:  "http",
				EntryPort:      80,
				TargetProtocol: "http",
				TargetPort:     80,
				CertificateID:  "",
				TlsPassthrough: false,
			},
		},
		ValidateOnly: true,
	}
}

func fakeAdmissionRequest(newSvc *corev1.Service, oldSvc *corev1.Service) v1.AdmissionRequest {
	var (
		m   []byte
		p   []byte
		err error
	)

	if newSvc != nil {
		m, err = json.Marshal(*newSvc)
		if err != nil {
			panic(err.Error())
		}
	}

	if oldSvc != nil {
		p, err = json.Marshal(*oldSvc)
		if err != nil {
			panic(err.Error())
		}
	}

	return v1.AdmissionRequest{
		UID:       "test",
		Name:      "test",
		Namespace: "test",
		Object:    runtime.RawExtension{Raw: m},
		OldObject: runtime.RawExtension{Raw: p},
		Operation: admissionv1.Create,
		Kind: metav1.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Service",
		},
		Resource: metav1.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "service",
		},
	}
}

func fakeService(name string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "test",
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
		},
	}
}
