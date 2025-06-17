/*
Copyright 2024 DigitalOcean

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
	"net/http"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestHandle(t *testing.T) {
	testcases := []struct {
		name              string
		req               admission.Request
		givenGodoCreateFn func(ctx context.Context, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error)
		givenGodoUpdateFn func(ctx context.Context, lbID string, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error)
		expectedAllowed   bool
		expectedMessage   string
	}{
		{
			name:            "error if the admission request is not a proper service",
			req:             admission.Request{},
			expectedAllowed: false,
			expectedMessage: "failed to decode admission request: there is no content to decode",
		},
		{
			name: "allow if service type is not load balancer",
			req: fakeAdmissionRequest(&corev1.Service{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeNodePort,
				},
			}, nil),
			expectedAllowed: true,
			expectedMessage: "allowing service that is not a load balancer",
		},
		{
			name: "allow if service is being deleted",
			req: fakeAdmissionRequest(&corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					DeletionTimestamp: &metav1.Time{
						Time: time.Now().UTC(),
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
				},
			}, nil),
			expectedAllowed: true,
			expectedMessage: "allowing service that is being deleted",
		},
		{
			name: "allow create when godo answers with no error",
			req:  fakeAdmissionRequest(fakeService(), nil),
			givenGodoCreateFn: func(ctx context.Context, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
				return nil, &godo.Response{Response: &http.Response{StatusCode: http.StatusNoContent}}, nil
			},
			expectedAllowed: true,
			expectedMessage: "valid load balancer definition",
		},
		{
			name: "error create when building godo request fails",
			req: fakeAdmissionRequest(&corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annDOHealthCheckIntervalSeconds: "abc",
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
				},
			}, nil),
			expectedAllowed: false,
			expectedMessage: "failed to build DO API request: failed to build base load balancer request: failed to parse health check interval annotation \"service.beta.kubernetes.io/do-loadbalancer-healthcheck-check-interval-seconds\": strconv.Atoi: parsing \"abc\": invalid syntax",
		},
		{
			name: "error create when godo answers has no resp and error",
			req:  fakeAdmissionRequest(fakeService(), nil),
			givenGodoCreateFn: func(ctx context.Context, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
				return nil, nil, errors.New("random error")
			},
			expectedAllowed: false,
			expectedMessage: "failed to get a response from DO API: random error",
		},
		{
			name: "deny create when godo answers with StatusUnprocessableEntity",
			req:  fakeAdmissionRequest(fakeService(), nil),
			givenGodoCreateFn: func(ctx context.Context, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
				return nil, &godo.Response{Response: &http.Response{StatusCode: http.StatusUnprocessableEntity}}, errors.New("random error")
			},
			expectedAllowed: false,
			expectedMessage: "invalid load balancer definition: random error",
		},
		{
			name: "allow create when godo answers with a 500 error",
			req:  fakeAdmissionRequest(fakeService(), nil),
			givenGodoCreateFn: func(ctx context.Context, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
				return nil, &godo.Response{Response: &http.Response{StatusCode: http.StatusInternalServerError}}, errors.New("random error")
			},
			expectedAllowed: true,
			expectedMessage: "received unexpected status code (500) from DO API, allowing to prevent blocking: random error",
		},
		{
			name: "deny create when godo answers with a 404 error",
			req:  fakeAdmissionRequest(fakeService(), nil),
			givenGodoCreateFn: func(ctx context.Context, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
				return nil, &godo.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}, errors.New("random error")
			},
			expectedAllowed: false,
			expectedMessage: "invalid load balancer definition: random error",
		},
		{
			name: "update errors when old object is not a proper service",
			req: admission.Request{
				AdmissionRequest: v1.AdmissionRequest{
					Object:    runtime.RawExtension{Raw: mustMarshal(fakeService())},
					OldObject: runtime.RawExtension{Raw: make([]byte, 0)},
					Operation: admissionv1.Create,
				},
			},
			expectedAllowed: false,
			expectedMessage: "failed to decode old object: there is no content to decode",
		},
		{
			name:            "allow without calling do api when old and new are the same",
			req:             fakeAdmissionRequest(fakeService(), fakeService()),
			expectedAllowed: true,
			expectedMessage: "new service has irrelevant changes",
		},
		{
			name: "allow without calling do api when old and new are the same except for lb id",
			req: fakeAdmissionRequest(
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: nil,
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
						Ports: []corev1.ServicePort{
							{Protocol: "TCP", Port: 8080},
						},
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{IP: "1.1.1.1"},
							},
						},
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annDOLoadBalancerID: "lbid",
						},
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
						Ports: []corev1.ServicePort{
							{Protocol: "TCP", Port: 8080},
						},
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{IP: "1.1.1.1"},
							},
						},
					},
				},
			),
			expectedAllowed: true,
			expectedMessage: "new service has irrelevant changes",
		},
		{
			name: "allow create when old and new services do not have an lb id",
			req: fakeAdmissionRequest(
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: nil,
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
						Ports: []corev1.ServicePort{
							{Protocol: "TCP", Port: 8080},
						},
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{IP: "1.1.1.1"},
							},
						},
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: nil,
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
						Ports: []corev1.ServicePort{
							{Protocol: "TCP", Port: 8089},
						},
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{IP: "1.1.1.1"},
							},
						},
					},
				}),
			givenGodoCreateFn: func(ctx context.Context, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
				return nil, &godo.Response{Response: &http.Response{StatusCode: http.StatusNoContent}}, nil
			},
			expectedAllowed: true,
			expectedMessage: "valid load balancer definition",
		},
		{
			name: "allow update when old service has an lb id and new one does not",
			req: fakeAdmissionRequest(
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: nil,
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
						Ports: []corev1.ServicePort{
							{Protocol: "TCP", Port: 8080},
						},
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{IP: "1.1.1.1"},
							},
						},
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annDOLoadBalancerID: "lbid",
						},
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
						Ports: []corev1.ServicePort{
							{Protocol: "TCP", Port: 8089},
						},
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{IP: "1.1.1.1"},
							},
						},
					},
				}),
			givenGodoUpdateFn: func(ctx context.Context, lbID string, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
				return nil, &godo.Response{Response: &http.Response{StatusCode: http.StatusNoContent}}, nil
			},
			expectedAllowed: true,
			expectedMessage: "valid load balancer definition",
		},
		{
			name: "allow update when new service has an lb id and old one does not",
			req: fakeAdmissionRequest(
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annDOLoadBalancerID: "lbid",
						},
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
						Ports: []corev1.ServicePort{
							{Protocol: "TCP", Port: 8080},
						},
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{IP: "1.1.1.1"},
							},
						},
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: nil,
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
						Ports: []corev1.ServicePort{
							{Protocol: "TCP", Port: 8089},
						},
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{IP: "1.1.1.1"},
							},
						},
					},
				}),
			givenGodoUpdateFn: func(ctx context.Context, lbID string, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
				return nil, &godo.Response{Response: &http.Response{StatusCode: http.StatusNoContent}}, nil
			},
			expectedAllowed: true,
			expectedMessage: "valid load balancer definition",
		},
		{
			name: "allow update when new service and old service have different LB ids",
			req: fakeAdmissionRequest(
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annDOLoadBalancerID: "lbid",
						},
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
						Ports: []corev1.ServicePort{
							{Protocol: "TCP", Port: 8080},
						},
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{IP: "1.1.1.1"},
							},
						},
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annDOLoadBalancerID: "lbid2",
						},
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
						Ports: []corev1.ServicePort{
							{Protocol: "TCP", Port: 8089},
						},
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{IP: "1.1.1.1"},
							},
						},
					},
				}),
			givenGodoUpdateFn: func(ctx context.Context, lbID string, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
				return nil, &godo.Response{Response: &http.Response{StatusCode: http.StatusNoContent}}, nil
			},
			expectedAllowed: true,
			expectedMessage: "valid load balancer definition",
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			godoClient := godo.NewFromToken("")
			godoClient.LoadBalancers = &fakeLBService{
				createFn: tc.givenGodoCreateFn,
				updateFn: tc.givenGodoUpdateFn,
			}

			admissionHandler := NewLBServiceAdmissionHandler(&logr.Logger{}, godoClient, godo.LoadBalancerTypeRegionalNetwork)

			resp := admissionHandler.Handle(context.Background(), tc.req)
			if string(resp.Result.Message) != tc.expectedMessage {
				t.Fatalf("expected %s to equal %q, got %q", "message", tc.expectedMessage, string(resp.Result.Message))
			}
			if resp.Allowed != tc.expectedAllowed {
				t.Fatalf("expected %s to equal %v, got %v", "allowed", tc.expectedAllowed, resp.Allowed)
			}
		})
	}
}

func fakeAdmissionRequest(newSvc *corev1.Service, oldSvc *corev1.Service) admission.Request {
	var (
		m []byte
		p []byte
	)

	if newSvc != nil {
		m = mustMarshal(newSvc)
	}

	if oldSvc != nil {
		p = mustMarshal(oldSvc)
	}

	return admission.Request{
		AdmissionRequest: v1.AdmissionRequest{
			Object:    runtime.RawExtension{Raw: m},
			OldObject: runtime.RawExtension{Raw: p},
			Operation: admissionv1.Create,
		},
	}
}

func fakeService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				{
					Protocol: "TCP",
					Port:     8080,
				},
			},
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{
						IP: "1.2.3.4",
					},
				},
			},
		},
	}
}

func mustMarshal(svc *corev1.Service) []byte {
	m, err := json.Marshal(svc)
	if err != nil {
		panic(err.Error())
	}
	return m
}
