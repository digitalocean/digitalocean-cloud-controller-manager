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
	"reflect"
	"testing"

	"github.com/digitalocean/godo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type kvCertService struct {
	store         map[string]*godo.Certificate
	reflectorMode bool
}

func (f *kvCertService) Get(ctx context.Context, certID string) (*godo.Certificate, *godo.Response, error) {
	if f.reflectorMode {
		return &godo.Certificate{
			ID:   certID,
			Type: certTypeLetsEncrypt,
		}, newFakeOKResponse(), nil
	}
	cert, ok := f.store[certID]
	if ok {
		return cert, newFakeOKResponse(), nil
	}
	return nil, newFakeNotFoundResponse(), newFakeNotFoundErrorResponse()
}

func (f *kvCertService) List(ctx context.Context, listOpts *godo.ListOptions) ([]godo.Certificate, *godo.Response, error) {
	panic("not implemented")
}

func (f *kvCertService) Create(ctx context.Context, crtr *godo.CertificateRequest) (*godo.Certificate, *godo.Response, error) {
	panic("not implemented")
}

func (f *kvCertService) Delete(ctx context.Context, certID string) (*godo.Response, error) {
	panic("not implemented")
}

func (f *kvCertService) ListByName(ctx context.Context, name string, opt *godo.ListOptions) ([]godo.Certificate, *godo.Response, error) {
	if f.reflectorMode {
		return []godo.Certificate{{
			ID:   name,
			Type: certTypeLetsEncrypt,
		}}, newFakeOKResponse(), nil
	}
	cert, ok := f.store[name]
	if ok {
		return []godo.Certificate{*cert}, newFakeOKResponse(), nil
	}
	return nil, newFakeNotFoundResponse(), newFakeNotFoundErrorResponse()
}

func newKVCertService(store map[string]*godo.Certificate, reflectorMode bool) kvCertService {
	return kvCertService{
		store:         store,
		reflectorMode: reflectorMode,
	}
}

func createServiceAndCert(lbID, certID, certType string) (*v1.Service, *godo.Certificate) {
	c := &godo.Certificate{
		ID:   certID,
		Type: certType,
	}
	s := createServiceWithCert(lbID, certID)
	return s, c
}

func createCertWithName(certID string, certName string, certType string) *godo.Certificate {
	return &godo.Certificate{
		ID:   certID,
		Name: certName,
		Type: certType,
	}
}

func createServiceWithCert(lbID, certID string) *v1.Service {
	s := createService(lbID)
	s.Annotations[annDOCertificateID] = certID
	return s
}

func createServiceAndCertName(lbID, certID string, certName, certType string) (*v1.Service, *godo.Certificate) {
	c := &godo.Certificate{
		ID:   certID,
		Name: certName,
		Type: certType,
	}
	s := createServiceWithCertName(lbID, certName)
	return s, c
}

func createServiceWithCertName(lbID, certName string) *v1.Service {
	s := createService(lbID)
	s.Annotations[annDOCertificateName] = certName
	return s
}

func createService(lbID string) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			UID:  "foobar123",
			Annotations: map[string]string{
				annDOProtocol:       "http",
				annDOLoadBalancerID: lbID,
				annDOType:           godo.LoadBalancerTypeRegional,
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
	}
}

func Test_LBaaSCertificateScenarios(t *testing.T) {
	tests := []struct {
		name                    string
		setupFn                 func(fakeLBService, kvCertService) *v1.Service
		expectedServiceCertID   string
		expectedServiceCertName string
		expectedLBCertID        string
		expectedLBCertName      string
		err                     error
	}{
		// lets_encrypt test cases
		{
			name: "[letsencrypt] LB cert ID and service cert ID match",
			setupFn: func(lbService fakeLBService, certService kvCertService) *v1.Service {
				lb, cert := createHTTPSLB("test-lb-id", "lb-cert-id", certTypeLetsEncrypt)
				lbService.store[lb.ID] = lb
				certService.store[cert.ID] = cert
				return createServiceWithCert(lb.ID, cert.ID)
			},
			expectedServiceCertID: "lb-cert-id",
			expectedLBCertID:      "lb-cert-id",
		},
		{
			name: "[letsencrypt] LB cert ID and service cert ID differ and both certs exist",
			setupFn: func(lbService fakeLBService, certService kvCertService) *v1.Service {
				lb, cert := createHTTPSLB("test-lb-id", "lb-cert-id", certTypeLetsEncrypt)
				lbService.store[lb.ID] = lb
				certService.store[cert.ID] = cert

				service, cert := createServiceAndCert(lb.ID, "service-cert-id", certTypeLetsEncrypt)
				certService.store[cert.ID] = cert
				return service
			},
			expectedServiceCertID: "lb-cert-id",
			expectedLBCertID:      "lb-cert-id",
		},
		{
			name: "[letsencrypt] LB cert ID exists and service cert ID does not",
			setupFn: func(lbService fakeLBService, certService kvCertService) *v1.Service {
				lb, cert := createHTTPSLB("test-lb-id", "lb-cert-id", certTypeLetsEncrypt)
				lbService.store[lb.ID] = lb
				certService.store[cert.ID] = cert
				service, _ := createServiceAndCert(lb.ID, "meow", certTypeLetsEncrypt)
				return service
			},
			expectedServiceCertID: "lb-cert-id",
			expectedLBCertID:      "lb-cert-id",
		},
		{
			name: "[lets_encrypt] LB cert ID does not exit and service cert ID does",
			setupFn: func(lbService fakeLBService, certService kvCertService) *v1.Service {
				lb, _ := createHTTPSLB("test-lb-id", "lb-cert-id", certTypeLetsEncrypt)
				lbService.store[lb.ID] = lb

				service, cert := createServiceAndCert(lb.ID, "service-cert-id", certTypeLetsEncrypt)
				certService.store[cert.ID] = cert
				return service
			},
			expectedServiceCertID: "service-cert-id",
			expectedLBCertID:      "service-cert-id",
		},
		{
			name: "[lets_encrypt] LB cert ID differs from service cert ID and both exist, different name",
			setupFn: func(lbService fakeLBService, certService kvCertService) *v1.Service {
				lb, cert := createHTTPSLB("test-lb-id", "lb-cert-id", certTypeLetsEncrypt)
				lbService.store[lb.ID] = lb
				cert.Name = "meow1"
				certService.store[cert.ID] = cert

				svcert := createCertWithName("service-cert-id", "meow2", certTypeLetsEncrypt)

				service := createServiceWithCert(lb.ID, "service-cert-id")
				certService.store[svcert.ID] = svcert
				return service
			},
			expectedServiceCertID: "service-cert-id",
			expectedLBCertID:      "service-cert-id",
			expectedLBCertName:    "meow2",
		},
		{
			name: "[lets_encrypt] LB cert ID differs from service cert ID and both exist, same name",
			setupFn: func(lbService fakeLBService, certService kvCertService) *v1.Service {
				lb, cert := createHTTPSLB("test-lb-id", "lb-cert-id", certTypeLetsEncrypt)
				lbService.store[lb.ID] = lb
				cert.Name = "meow1"
				certService.store[cert.ID] = cert

				svcert := createCertWithName("service-cert-id", "meow1", certTypeLetsEncrypt)

				service := createServiceWithCert(lb.ID, "service-cert-id")
				certService.store[svcert.ID] = svcert
				return service
			},
			expectedServiceCertID: "lb-cert-id",
			expectedLBCertID:      "lb-cert-id",
			expectedLBCertName:    "meow1",
		},
		{
			name: "[lets_encrypt] LB cert ID exists, service cert annotation nonexistant",
			setupFn: func(lbService fakeLBService, certService kvCertService) *v1.Service {
				lb, cert := createHTTPSLB("test-lb-id", "lb-cert-id", certTypeLetsEncrypt)
				lbService.store[lb.ID] = lb
				cert.Name = "meow1"
				certService.store[cert.ID] = cert

				svcert := createCertWithName("service-cert-id", "meow1", certTypeLetsEncrypt)

				service := createService(lb.ID)
				certService.store[svcert.ID] = svcert
				return service
			},
			expectedServiceCertID: "",
			expectedLBCertID:      "",
			expectedLBCertName:    "",
		},
		// custom test cases
		{
			name: "[custom] LB cert ID and service cert ID match",
			setupFn: func(lbService fakeLBService, certService kvCertService) *v1.Service {
				lb, cert := createHTTPSLB("test-lb-id", "lb-cert-id", certTypeCustom)
				lbService.store[lb.ID] = lb
				certService.store[cert.ID] = cert
				return createServiceWithCert(lb.ID, cert.ID)
			},
			expectedServiceCertID: "lb-cert-id",
			expectedLBCertID:      "lb-cert-id",
		},
		{
			name: "[custom] LB cert ID and service cert ID differ and both certs exist",
			setupFn: func(lbService fakeLBService, certService kvCertService) *v1.Service {
				lb, cert := createHTTPSLB("test-lb-id", "lb-cert-id", certTypeCustom)
				lbService.store[lb.ID] = lb
				certService.store[cert.ID] = cert

				service, cert := createServiceAndCert(lb.ID, "service-cert-id", certTypeCustom)
				certService.store[cert.ID] = cert
				return service
			},
			expectedServiceCertID: "service-cert-id",
			expectedLBCertID:      "service-cert-id",
		},
		{
			name: "[custom] LB cert ID does not exit and service cert ID does",
			setupFn: func(lbService fakeLBService, certService kvCertService) *v1.Service {
				lb, _ := createHTTPSLB("test-lb-id", "lb-cert-id", certTypeCustom)
				lbService.store[lb.ID] = lb

				service, cert := createServiceAndCert(lb.ID, "service-cert-id", certTypeCustom)
				certService.store[cert.ID] = cert
				return service
			},
			expectedServiceCertID: "service-cert-id",
			expectedLBCertID:      "service-cert-id",
		},
		{
			name: "use certificate name",
			setupFn: func(lbService fakeLBService, certService kvCertService) *v1.Service {
				lb, _ := createHTTPSLB("test-lb-id", "lb-cert-id", certTypeCustom)
				lbService.store[lb.ID] = lb

				service, cert := createServiceAndCertName(lb.ID, "lb-cert-id", "my-cert", certTypeCustom)
				certService.store[cert.Name] = cert
				return service
			},
			expectedServiceCertName: "my-cert",
			expectedLBCertID:        "lb-cert-id",
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

	for _, test := range tests {
		test := test
		for _, methodName := range []string{"EnsureLoadBalancer", "UpdateLoadBalancer"} {
			t.Run(test.name+"_"+methodName, func(t *testing.T) {
				lbStore := make(map[string]*godo.LoadBalancer)
				lbService := newKVLBService(lbStore)
				certStore := make(map[string]*godo.Certificate)
				certService := newKVCertService(certStore, false)
				service := test.setupFn(lbService, certService)

				fakeClient := newFakeClient(&fakeDroplet, &lbService, &certService)
				fakeResources := newResources("", "", publicAccessFirewall{}, fakeClient)
				fakeResources.kclient = fake.NewSimpleClientset()
				if _, err := fakeResources.kclient.CoreV1().Services(service.Namespace).Create(context.Background(), service, metav1.CreateOptions{}); err != nil {
					t.Fatalf("failed to add service to fake client: %s", err)
				}

				lb := &loadBalancers{
					resources:         fakeResources,
					region:            "nyc1",
					lbActiveTimeout:   2,
					lbActiveCheckTick: 1,
					defaultLBType:     godo.LoadBalancerTypeRegionalNetwork,
				}

				var err error
				switch methodName {
				case "EnsureLoadBalancer":
					_, err = lb.EnsureLoadBalancer(context.Background(), "test", service, nodes)
				case "UpdateLoadBalancer":
					err = lb.UpdateLoadBalancer(context.Background(), "test", service, nodes)
				default:
					t.Fatalf("unsupported loadbalancer method: %s", methodName)
				}

				if !reflect.DeepEqual(err, test.err) {
					t.Errorf("got error %q, want: %q", err, test.err)
				}

				service, err = fakeResources.kclient.CoreV1().Services(service.Namespace).Get(context.Background(), service.Name, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("failed to get service from fake client: %s", err)
				}

				serviceCertID := getCertificateID(service)
				if test.expectedServiceCertID != serviceCertID {
					t.Errorf("got service certificate ID: %s, want: %s", test.expectedServiceCertID, serviceCertID)
				}
				serviceCertName := getCertificateName(service)
				if test.expectedServiceCertName != serviceCertName {
					t.Errorf("got service certificate name: %s, want: %s", test.expectedServiceCertName, serviceCertName)
				}

				godoLoadBalancer, _, err := lbService.Get(context.Background(), getLoadBalancerID(service))
				if err != nil {
					t.Fatalf("failed to get loadbalancer %q from fake client: %s", getLoadBalancerID(service), err)
				}
				lbCertID := getCertificateIDFromLB(godoLoadBalancer)
				if test.expectedLBCertID != lbCertID {
					t.Errorf("got load-balancer certificate ID: %s, want: %s", lbCertID, test.expectedLBCertID)
				}

				if test.expectedLBCertName != "" {

					lbCert, _, err := fakeResources.gclient.Certificates.Get(context.Background(), lbCertID)
					if err != nil {
						t.Fatalf("failed to get certificate %q from fake client: %s", lbCertID, err)
					}
					if test.expectedLBCertName != lbCert.Name {
						t.Errorf("got lb certificate name: %s, want: %s", test.expectedLBCertName, lbCert.Name)
					}
				}
			})
		}
	}
}
