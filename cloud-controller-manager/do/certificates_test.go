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
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/digitalocean/godo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type fakeCertService struct {
	getFn    func(context.Context, string) (*godo.Certificate, *godo.Response, error)
	listFn   func(context.Context, *godo.ListOptions) ([]godo.Certificate, *godo.Response, error)
	createFn func(context.Context, *godo.CertificateRequest) (*godo.Certificate, *godo.Response, error)
	deleteFn func(ctx context.Context, lbID string) (*godo.Response, error)
}

func (f *fakeCertService) Get(ctx context.Context, certID string) (*godo.Certificate, *godo.Response, error) {
	return f.getFn(ctx, certID)
}

func (f *fakeCertService) List(ctx context.Context, listOpts *godo.ListOptions) ([]godo.Certificate, *godo.Response, error) {
	return f.listFn(ctx, listOpts)
}

func (f *fakeCertService) Create(ctx context.Context, crtr *godo.CertificateRequest) (*godo.Certificate, *godo.Response, error) {
	return f.createFn(ctx, crtr)
}

func (f *fakeCertService) Delete(ctx context.Context, certID string) (*godo.Response, error) {
	return f.deleteFn(ctx, certID)
}

func Test_LBaaSCertificateScenarios(t *testing.T) {
	defaultLBService := fakeLBService{
		createFn: func(context.Context, *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
			return nil, newFakeNotOKResponse(), errors.New("create should not have been invoked")
		},
		getFn: func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error) {
			return createLB(), newFakeOKResponse(), nil
		},
		listFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
			return []godo.LoadBalancer{*createLB()}, newFakeOKResponse(), nil
		},
		updateFn: func(ctx context.Context, lbID string, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
			lb := updateLB(lbr)
			return lb, newFakeOKResponse(), nil
		},
	}

	defaultCertService := fakeCertService{
		getFn: func(ctx context.Context, certID string) (*godo.Certificate, *godo.Response, error) {
			return &godo.Certificate{
				ID:   certID,
				Type: certTypeLetsEncrypt,
			}, nil, nil
		},
	}

	testcases := []struct {
		name                  string
		droplets              []godo.Droplet
		fakeLBServiceFn       func() fakeLBService
		fakeCertServiceFn     func() fakeCertService
		service               *v1.Service
		expectedServiceCertID string
		expectedLBCertID      string
		err                   error
	}{
		{
			name: "default test values, tls not enabled",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "foobar123",
					Annotations: map[string]string{
						annoDOLoadBalancerID: "load-balancer-id",
						annDOProtocol:        "http",
					},
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name:     "test",
							Protocol: "TCP",
							Port:     int32(80),
							NodePort: int32(30000),
						},
					},
				},
			},
		},
		{
			name: "[letsencrypt] LB cert ID and service cert ID match ",
			fakeLBServiceFn: func() fakeLBService {
				s := defaultLBService
				s.getFn = func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error) {
					return createHTTPSLB(443, 30000, "test-lb-id", "test-cert-id"), newFakeOKResponse(), nil
				}
				return s
			},
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "foobar123",
					Annotations: map[string]string{
						annDOProtocol:        "http",
						annoDOLoadBalancerID: "test-lb-id",
						annDOCertificateID:   "test-cert-id",
					},
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name:     "test",
							Protocol: "TCP",
							Port:     int32(443),
							NodePort: int32(30000),
						},
					},
				},
			},
			expectedServiceCertID: "test-cert-id",
			expectedLBCertID:      "test-cert-id",
		},
		{
			name: "[letsencrypt] LB cert ID and service cert ID match and correspond to non-existent cert",
			fakeLBServiceFn: func() fakeLBService {
				s := defaultLBService
				s.getFn = func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error) {
					return createHTTPSLB(443, 30000, "test-lb-id", "test-cert-id"), newFakeOKResponse(), nil
				}
				return s
			},
			fakeCertServiceFn: func() fakeCertService {
				s := defaultCertService
				s.getFn = func(ctx context.Context, certID string) (*godo.Certificate, *godo.Response, error) {
					return nil, nil, newFakeNotFoundErrorResponse()
				}
				return s
			},
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "foobar123",
					Annotations: map[string]string{
						annDOProtocol:        "http",
						annoDOLoadBalancerID: "test-lb-id",
						annDOCertificateID:   "test-cert-id",
					},
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name:     "test",
							Protocol: "TCP",
							Port:     int32(443),
							NodePort: int32(30000),
						},
					},
				},
			},
			expectedLBCertID:      "test-cert-id",
			expectedServiceCertID: "test-cert-id",
			err:                   fmt.Errorf("the %q service annotation refers to nonexistent DO Certificate %q", annDOCertificateID, "test-cert-id"),
		},
		{
			name: "[letsencrypt] LB cert ID and service cert ID differ and both certs exist",
			fakeLBServiceFn: func() fakeLBService {
				s := defaultLBService
				s.getFn = func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error) {
					return createHTTPSLB(443, 30000, "test-lb-id", "test-cert-id"), newFakeOKResponse(), nil
				}
				return s
			},
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "foobar123",
					Annotations: map[string]string{
						annDOProtocol:        "http",
						annoDOLoadBalancerID: "test-lb-id",
						annDOCertificateID:   "service-cert",
					},
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name:     "test",
							Protocol: "TCP",
							Port:     int32(443),
							NodePort: int32(30000),
						},
					},
				},
			},
			expectedServiceCertID: "test-cert-id",
			expectedLBCertID:      "test-cert-id",
		},
		{
			name: "[letsencrypt] LB cert ID exists and service cert ID does not",
			fakeLBServiceFn: func() fakeLBService {
				s := defaultLBService
				s.getFn = func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error) {
					return createHTTPSLB(443, 30000, "test-lb-id", "test-cert-id"), newFakeOKResponse(), nil
				}
				return s
			},
			fakeCertServiceFn: func() fakeCertService {
				s := defaultCertService
				s.getFn = func(ctx context.Context, certID string) (*godo.Certificate, *godo.Response, error) {
					if certID == "service-cert" {
						return nil, nil, newFakeNotFoundErrorResponse()
					}
					return &godo.Certificate{
						ID:   certID,
						Type: certTypeLetsEncrypt,
					}, nil, nil
				}
				return s
			},
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "foobar123",
					Annotations: map[string]string{
						annDOProtocol:        "http",
						annoDOLoadBalancerID: "test-lb-id",
						annDOCertificateID:   "service-cert",
					},
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name:     "test",
							Protocol: "TCP",
							Port:     int32(443),
							NodePort: int32(30000),
						},
					},
				},
			},
			expectedServiceCertID: "test-cert-id",
			expectedLBCertID:      "test-cert-id",
		},
	}

	nodes := []*v1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-1",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-2",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node-3",
			},
		},
	}
	droplets := []godo.Droplet{
		{
			ID:   100,
			Name: "node-1",
		},
		{
			ID:   101,
			Name: "node-2",
		},
		{
			ID:   102,
			Name: "node-3",
		},
	}
	fakeDroplet := fakeDropletService{
		listFunc: func(context.Context, *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
			return droplets, newFakeOKResponse(), nil
		},
	}

	for _, tc := range testcases {
		tc := tc
		for _, methodName := range []string{"EnsureLoadBalancer", "UpdateLoadBalancer"} {
			t.Run(tc.name+"_"+methodName, func(t *testing.T) {
				lbService := defaultLBService
				certService := defaultCertService

				if tc.fakeLBServiceFn != nil {
					lbService = tc.fakeLBServiceFn()
				}

				if tc.fakeCertServiceFn != nil {
					certService = tc.fakeCertServiceFn()
				}

				fakeClient := newFakeClient(&fakeDroplet, &lbService, &certService)
				fakeResources := newResources("", "", fakeClient)
				fakeResources.kclient = fake.NewSimpleClientset()
				if _, err := fakeResources.kclient.CoreV1().Services(tc.service.Namespace).Create(tc.service); err != nil {
					t.Fatalf("failed to add service to fake client: %s", err)
				}

				lb := &loadBalancers{
					resources:         fakeResources,
					region:            "nyc1",
					lbActiveTimeout:   2,
					lbActiveCheckTick: 1,
				}

				var err error
				switch methodName {
				case "EnsureLoadBalancer":
					_, err = lb.EnsureLoadBalancer(context.TODO(), "test", tc.service, nodes)
				case "UpdateLoadBalancer":
					err = lb.UpdateLoadBalancer(context.TODO(), "test", tc.service, nodes)
				default:
					t.Errorf("unsupported loadbalancer method: %s", methodName)
				}

				if !reflect.DeepEqual(err, tc.err) {
					t.Error("error does not match test case expectation")
					t.Logf("expected: %v", tc.err)
					t.Logf("actual: %v", err)
				}

				service, err := fakeResources.kclient.CoreV1().Services(tc.service.Namespace).Get(tc.service.Name, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("failed to get service from fake client: %s", err)
				}

				if tc.expectedServiceCertID != getCertificateID(service) {
					t.Error("unexpected service certificate ID annotation")
					t.Logf("expected: %s", tc.expectedServiceCertID)
					t.Logf("actual: %s", getCertificateID(service))
				}

				godoLoadBalancer, _, err := lbService.Get(context.TODO(), getLoadBalancerID(service))
				if err != nil {
					t.Fatalf("failed to get loadbalancer %q from fake client: %s", getLoadBalancerID(service), err)
				}
				lbCertID := getCertificateIDFromLB(godoLoadBalancer)
				if tc.expectedLBCertID != lbCertID {
					t.Error("unexpected loadbalancer certificate ID")
					t.Logf("expected: %s", tc.expectedLBCertID)
					t.Logf("actual: %s", lbCertID)
				}
			})
		}
	}
}
