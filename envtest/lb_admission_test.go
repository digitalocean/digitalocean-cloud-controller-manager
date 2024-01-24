//go:build integration
// +build integration

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

package envtest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/digitalocean/godo"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestLBAdmissionWebhook(t *testing.T) {
	testCases := []struct {
		name            string
		givenService    *corev1.Service
		givenLBResponse *stubLBResponse
		allow           bool
	}{
		{
			"allowed when service type is not LB",
			fakeService(nil, &corev1.ServiceSpec{
				Type: corev1.ServiceTypeNodePort,
				Ports: []corev1.ServicePort{
					{
						Name:       "http",
						Protocol:   "TCP",
						Port:       80,
						TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 80},
					},
				},
			}),
			nil,
			true,
		}, {
			"failed when annotation contains unallowed value",
			fakeService(map[string]string{
				"service.beta.kubernetes.io/do-loadbalancer-protocol": "udp",
			}, &corev1.ServiceSpec{
				Type: corev1.ServiceTypeLoadBalancer,
				Ports: []corev1.ServicePort{
					{
						Name:       "http",
						Protocol:   "TCP",
						Port:       80,
						TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 80},
					},
				},
			}),
			nil,
			false,
		}, {
			"allowed when godo returns a transiant error",
			fakeService(map[string]string{
				"service.beta.kubernetes.io/do-loadbalancer-protocol": "tcp",
			}, &corev1.ServiceSpec{
				Type: corev1.ServiceTypeLoadBalancer,
				Ports: []corev1.ServicePort{
					{
						Name:       "http",
						Protocol:   "TCP",
						Port:       80,
						TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 80},
					},
				},
			}),
			fakeGodoResp(http.StatusInternalServerError, errors.New("random error")),
			true,
		}, {
			"denied when godo returns a bad request error",
			fakeService(map[string]string{
				"service.beta.kubernetes.io/do-loadbalancer-protocol": "tcp",
			}, &corev1.ServiceSpec{
				Type: corev1.ServiceTypeLoadBalancer,
				Ports: []corev1.ServicePort{
					{
						Name:       "http",
						Protocol:   "TCP",
						Port:       80,
						TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 80},
					},
				},
			}),
			fakeGodoResp(http.StatusBadRequest, errors.New("random error")),
			false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			lbStub.setCreateResp(tc.givenService.Name, tc.givenLBResponse)
			err := k8sClient.Create(ctx, tc.givenService)
			if tc.allow && err != nil {
				t.Errorf("expected allowed admission request, got %q", err)
			}
			if !tc.allow && err == nil {
				t.Error("expected denied admission request, received no error")
			}
		})
	}
}

func TestLBAdmissionWebhook_Update(t *testing.T) {
	testCases := []struct {
		name                  string
		givenService          *corev1.Service
		givenUpdate           func(s *corev1.Service)
		givenLBUpdateResponse *stubLBResponse
		allow                 bool
	}{
		{
			"failed when annotation contains unallowed value",
			fakeService(map[string]string{
				"service.beta.kubernetes.io/do-loadbalancer-protocol": "tcp",
			}, &corev1.ServiceSpec{
				Type: corev1.ServiceTypeLoadBalancer,
				Ports: []corev1.ServicePort{
					{
						Name:       "http",
						Protocol:   "TCP",
						Port:       80,
						TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 80},
					},
				},
			}),
			func(s *corev1.Service) {
				s.Annotations["service.beta.kubernetes.io/do-loadbalancer-protocol"] = "udp"
			},
			nil,
			false,
		}, {
			"allowed when godo returns a transiant error",
			fakeService(map[string]string{
				"service.beta.kubernetes.io/do-loadbalancer-protocol": "tcp",
			}, &corev1.ServiceSpec{
				Type: corev1.ServiceTypeLoadBalancer,
				Ports: []corev1.ServicePort{
					{
						Name:       "http",
						Protocol:   "TCP",
						Port:       80,
						TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 80},
					},
				},
			}),
			func(s *corev1.Service) {
				s.Annotations["service.beta.kubernetes.io/do-loadbalancer-protocol"] = "http"
			},
			fakeGodoResp(http.StatusInternalServerError, errors.New("random error")),
			true,
		}, {
			"denied when godo returns a bad request error",
			fakeService(map[string]string{
				"service.beta.kubernetes.io/do-loadbalancer-protocol": "tcp",
			}, &corev1.ServiceSpec{
				Type: corev1.ServiceTypeLoadBalancer,
				Ports: []corev1.ServicePort{
					{
						Name:       "http",
						Protocol:   "TCP",
						Port:       80,
						TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 80},
					},
				},
			}),
			func(s *corev1.Service) {
				s.Annotations["service.beta.kubernetes.io/do-loadbalancer-protocol"] = "http"
			},
			fakeGodoResp(http.StatusBadRequest, errors.New("random error")),
			false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			svc := tc.givenService

			lbStub.setCreateResp(svc.Name, fakeGodoResp(http.StatusOK, nil))
			k8sClient.Create(ctx, svc)

			tc.givenUpdate(svc)
			lbStub.setUpdateResp(svc.Name, tc.givenLBUpdateResponse)
			err := k8sClient.Update(ctx, svc)
			if tc.allow && err != nil {
				t.Errorf("expected allowed admission request, got %q", err)
			}
			if !tc.allow && err == nil {
				t.Error("expected denied admission request, received no error")
			}
		})
	}
}

func fakeService(annotations map[string]string, spec *corev1.ServiceSpec) *corev1.Service {
	lbID := uuid.NewString()
	serviceName := fmt.Sprintf("svc-%s", lbID)
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["kubernetes.digitalocean.com/load-balancer-id"] = lbID
	annotations["service.beta.kubernetes.io/do-loadbalancer-name"] = serviceName
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        serviceName,
			Namespace:   "default",
			Annotations: annotations,
		},
		Spec: *spec,
	}
}

func fakeGodoResp(status int, err error) *stubLBResponse {
	return &stubLBResponse{
		lb:  nil,
		r:   &godo.Response{Response: &http.Response{StatusCode: status}},
		err: err,
	}
}
