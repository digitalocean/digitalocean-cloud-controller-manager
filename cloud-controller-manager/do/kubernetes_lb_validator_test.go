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
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
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

type annotations map[string]string

type fakeRegionsService struct {
	listFn func(context.Context, *godo.ListOptions) ([]godo.Region, *godo.Response, error)
}

func (f *fakeRegionsService) List(ctx context.Context, listOpts *godo.ListOptions) ([]godo.Region, *godo.Response, error) {
	return f.listFn(ctx, listOpts)
}

func newFakeUnprocessableResponse() *godo.Response {
	return newFakeResponse(http.StatusUnprocessableEntity)
}

func newFakeUnprocessableErrorResponse() *godo.ErrorResponse {
	return &godo.ErrorResponse{
		Response: &http.Response{
			Request: &http.Request{
				Method: "FAKE",
				URL:    &url.URL{},
			},
			StatusCode: http.StatusUnprocessableEntity,
			Body:       ioutil.NopCloser(bytes.NewBufferString("test")),
		},
	}
}

func Test_Handle(t *testing.T) {
	os.Setenv(regionEnv, "nyc3")

	testcases := []struct {
		name            string
		req             admission.Request
		gCLient         *godo.Client
		expectedAllowed bool
		resp            *godo.Response
		err             error
		expectedMessage string
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
			expectedMessage: "ignoring the service because it is not a load balancer",
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
			expectedAllowed: true,
			expectedMessage: "ignoring the service because it's being deleted",
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
			expectedMessage: "valid lb create request",
		},
		{
			name: "Deny CREATE invalid configuration",
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
			resp:            newFakeUnprocessableResponse(),
			err:             newFakeUnprocessableErrorResponse(),
			expectedMessage: "invalid LB update configuration: " + newFakeUnprocessableErrorResponse().Error(),
		},
		{
			name: "Deny create validation error",
			req: admission.Request{AdmissionRequest: fakeAdmissionRequest(
				fakeService("new-test", annotations{}), fakeService("old-service", annotations{}))},
			expectedAllowed: false,
			resp:            newFakeNotFoundResponse(),
			err:             newFakeNotFoundErrorResponse(),
			expectedMessage: "failed to validate lb create: " + newFakeNotFoundErrorResponse().Error(),
		},
		{
			name: "Allow Update happy path",
			req: admission.Request{AdmissionRequest: fakeAdmissionRequest(
				fakeService("new-test", annotations{}), fakeService("old-service", annotations{annDOLoadBalancerID: "test"}))},
			expectedAllowed: true,
			expectedMessage: "valid update request",
		},
		{
			name: "Allow Update with existing lb IP",
			req: admission.Request{AdmissionRequest: fakeAdmissionRequest(
				fakeService("new-test", annotations{}), fakeServiceWithStatus())},
			expectedAllowed: true,
			expectedMessage: "valid update request",
		},
		{
			name: "Deny Update invalid configuration",
			req: admission.Request{AdmissionRequest: fakeAdmissionRequest(
				fakeService("new-test", annotations{}), fakeService("old-service", annotations{annDOLoadBalancerID: "test"}))},
			expectedAllowed: false,
			resp:            newFakeUnprocessableResponse(),
			err:             newFakeUnprocessableErrorResponse(),
			expectedMessage: "invalid LB update configuration: " + newFakeUnprocessableErrorResponse().Error(),
		},
		{
			name: "Deny Update validation error",
			req: admission.Request{AdmissionRequest: fakeAdmissionRequest(
				fakeService("new-test", annotations{}), fakeService("old-service", annotations{annDOLoadBalancerID: "test"}))},
			expectedAllowed: false,
			resp:            newFakeNotFoundResponse(),
			err:             newFakeNotFoundErrorResponse(),
			expectedMessage: "failed to validate lb update: " + newFakeNotFoundErrorResponse().Error(),
		},
	}

	for _, test := range testcases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// setup client
			gClient := godo.NewFromToken("")
			gClient.LoadBalancers = &fakeLBService{
				createFn: func(context.Context, *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
					return &godo.LoadBalancer{ID: "2", Name: "two"}, test.resp, test.err
				},
				updateFn: func(context.Context, string, *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
					return &godo.LoadBalancer{ID: "2", Name: "two"}, test.resp, test.err
				},
			}
			gClient.Regions = &fakeRegionsService{
				listFn: func(context.Context, *godo.ListOptions) ([]godo.Region, *godo.Response, error) {
					return []godo.Region{{Name: "nyc3", Slug: "nyc3"}}, newFakeOKResponse(), nil
				}}

			decoder, err := admission.NewDecoder(scheme)
			if err != nil {
				t.Fatalf("failed to initialize decoder %s", err)
			}

			ll := zap.New().WithName("webhook-validation-server")
			ctrlruntimelog.SetLogger(ll)

			validator := &KubernetesLBServiceValidator{
				Log:     ll,
				decoder: decoder,
				GClient: gClient,
			}

			res := validator.Handle(context.TODO(), test.req)
			if res.Allowed != test.expectedAllowed {
				t.Fatalf("got allowed %v, want %v", res.Allowed, test.expectedAllowed)
			}
			if res.Result.Reason != "" && string(res.Result.Reason) != test.expectedMessage {
				t.Fatalf("got reason %v, want %v", res.Result.Reason, test.expectedMessage)
			}
			if res.Result.Message != "" && res.Result.Message != test.expectedMessage {
				t.Fatalf("got message %v, want %v", res.Result.Message, test.expectedMessage)
			}
		})
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

func fakeService(name string, annotations map[string]string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   "test",
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
		},
	}
}

func fakeServiceWithStatus() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{
						IP: "1.2.3.4",
					},
				},
			}},
	}
}
