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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/digitalocean/godo"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes/fake"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog"
)

var _ cloudprovider.LoadBalancer = new(loadBalancers)

type fakeLBService struct {
	store                   map[string]*godo.LoadBalancer
	getFn                   func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error)
	listFn                  func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error)
	createFn                func(context.Context, *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error)
	updateFn                func(ctx context.Context, lbID string, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error)
	deleteFn                func(ctx context.Context, lbID string) (*godo.Response, error)
	addDropletsFn           func(ctx context.Context, lbID string, dropletIDs ...int) (*godo.Response, error)
	removeDropletsFn        func(ctx context.Context, lbID string, dropletIDs ...int) (*godo.Response, error)
	addForwardingRulesFn    func(ctx context.Context, lbID string, rules ...godo.ForwardingRule) (*godo.Response, error)
	removeForwardingRulesFn func(ctx context.Context, lbID string, rules ...godo.ForwardingRule) (*godo.Response, error)
}

func (f *fakeLBService) Get(ctx context.Context, lbID string) (*godo.LoadBalancer, *godo.Response, error) {
	return f.getFn(ctx, lbID)
}

func (f *fakeLBService) List(ctx context.Context, listOpts *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
	return f.listFn(ctx, listOpts)
}

func (f *fakeLBService) Create(ctx context.Context, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
	return f.createFn(ctx, lbr)
}

func (f *fakeLBService) Update(ctx context.Context, lbID string, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
	return f.updateFn(ctx, lbID, lbr)
}

func (f *fakeLBService) Delete(ctx context.Context, lbID string) (*godo.Response, error) {
	return f.deleteFn(ctx, lbID)
}

func (f *fakeLBService) AddDroplets(ctx context.Context, lbID string, dropletIDs ...int) (*godo.Response, error) {
	return f.addDropletsFn(ctx, lbID, dropletIDs...)
}

func (f *fakeLBService) RemoveDroplets(ctx context.Context, lbID string, dropletIDs ...int) (*godo.Response, error) {
	return f.removeDropletsFn(ctx, lbID, dropletIDs...)
}
func (f *fakeLBService) AddForwardingRules(ctx context.Context, lbID string, rules ...godo.ForwardingRule) (*godo.Response, error) {
	return f.addForwardingRulesFn(ctx, lbID, rules...)
}

func (f *fakeLBService) RemoveForwardingRules(ctx context.Context, lbID string, rules ...godo.ForwardingRule) (*godo.Response, error) {
	return f.removeForwardingRulesFn(ctx, lbID, rules...)
}

func newKVLBService(store map[string]*godo.LoadBalancer) fakeLBService {
	return fakeLBService{
		store: store,
		getFn: func(ctx context.Context, lbID string) (*godo.LoadBalancer, *godo.Response, error) {
			lb, ok := store[lbID]
			if ok {
				return lb, newFakeOKResponse(), nil
			}
			return nil, newFakeNotFoundResponse(), newFakeNotFoundErrorResponse()
		},
		updateFn: func(ctx context.Context, lbID string, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
			lb, ok := store[lbID]
			if !ok {
				return nil, newFakeNotFoundResponse(), newFakeNotFoundErrorResponse()
			}

			lb.ForwardingRules = lbr.ForwardingRules
			lb.RedirectHttpToHttps = lbr.RedirectHttpToHttps
			lb.StickySessions = lbr.StickySessions
			lb.HealthCheck = lbr.HealthCheck
			lb.EnableProxyProtocol = lbr.EnableProxyProtocol
			lb.Name = lbr.Name
			lb.Tags = lbr.Tags
			lb.Algorithm = lbr.Algorithm

			return lb, newFakeOKResponse(), nil
		},
	}
}

func newFakeLBClient(fakeLB *fakeLBService) *godo.Client {
	return newFakeClient(nil, fakeLB, nil)
}

func createLB() *godo.LoadBalancer {
	return &godo.LoadBalancer{
		// loadbalancer names are a + service.UID
		// see cloudprovider.DefaultLoadBalancerName
		ID:     "load-balancer-id",
		Name:   "afoobar123",
		IP:     "10.0.0.1",
		Status: lbStatusActive,
	}
}

func createHTTPSLB(lbID, certID, certType string) (*godo.LoadBalancer, *godo.Certificate) {
	lb := &godo.LoadBalancer{
		// loadbalancer names are a + service.UID
		// see cloudprovider.DefaultLoadBalancerName
		ID:     lbID,
		Name:   "afoobar123",
		IP:     "10.0.0.1",
		Status: lbStatusActive,
		ForwardingRules: []godo.ForwardingRule{
			{
				EntryProtocol:  protocolHTTPS,
				EntryPort:      443,
				TargetProtocol: protocolHTTP,
				TargetPort:     30000,
				CertificateID:  certID,
			},
		},
	}
	cert := &godo.Certificate{
		ID:   certID,
		Type: certType,
	}
	return lb, cert
}

func defaultHealthCheck(proto string, port int, path string) *godo.HealthCheck {
	svc := &v1.Service{}
	is, _ := healthCheckIntervalSeconds(svc)
	rts, _ := healthCheckResponseTimeoutSeconds(svc)
	ut, _ := healthCheckUnhealthyThreshold(svc)
	ht, _ := healthCheckHealthyThreshold(svc)

	return &godo.HealthCheck{
		Protocol:               proto,
		Port:                   port,
		Path:                   path,
		CheckIntervalSeconds:   is,
		ResponseTimeoutSeconds: rts,
		UnhealthyThreshold:     ut,
		HealthyThreshold:       ht,
	}
}

func Test_getAlgorithm(t *testing.T) {
	testcases := []struct {
		name      string
		service   *v1.Service
		algorithm string
	}{
		{
			"algorithm should be least_connection",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOAlgorithm: "least_connections",
					},
				},
			},
			"least_connections",
		},
		{
			"algorithm should be round_robin",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOAlgorithm: "round_robin",
					},
				},
			},
			"round_robin",
		},
		{
			"invalid algorithm should default to round_robin",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOAlgorithm: "invalid",
					},
				},
			},
			"round_robin",
		},
		{
			"no algorithm specified should default to round_robin",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
				},
			},
			"round_robin",
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			algorithm := getAlgorithm(test.service)
			if algorithm != test.algorithm {
				t.Error("unexpected algoritmh")
				t.Logf("expected: %q", test.algorithm)
				t.Logf("actual: %q", algorithm)
			}
		})
	}
}

func Test_getTLSPassThrough(t *testing.T) {
	testcases := []struct {
		name           string
		service        *v1.Service
		tlsPassThrough bool
	}{
		{
			"TLS pass through true",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOTLSPassThrough: "true",
					},
				},
			},
			true,
		},
		{
			"TLS pass through false",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOTLSPassThrough: "false",
					},
				},
			},
			false,
		},
		{
			"TLS pass through not defined",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					UID:         "abc123",
					Annotations: map[string]string{},
				},
			},
			false,
		},
		{
			"Service annotations nil",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
				},
			},
			false,
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			tlsPassThrough := getTLSPassThrough(test.service)
			if tlsPassThrough != test.tlsPassThrough {
				t.Error("unexpected TLS passthrough")
				t.Logf("expected: %t", test.tlsPassThrough)
				t.Logf("actual: %t", tlsPassThrough)
			}
		})
	}

}

func Test_getCertificateID(t *testing.T) {
	testcases := []struct {
		name          string
		service       *v1.Service
		certificateID string
	}{
		{
			"certificate ID set",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOCertificateID: "test-certificate",
					},
				},
			},
			"test-certificate",
		},
		{
			"certificate ID not set",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					UID:         "abc123",
					Annotations: map[string]string{},
				},
			},
			"",
		},
		{
			"service annotation nil",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
				},
			},
			"",
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			certificateID := getCertificateID(test.service)
			if certificateID != test.certificateID {
				t.Error("unexpected certificate ID")
				t.Logf("expected %q", test.certificateID)
				t.Logf("actual: %q", certificateID)
			}
		})
	}
}

func Test_getPorts(t *testing.T) {
	tests := []struct {
		name      string
		service   *v1.Service
		wantPorts []int
		wantErr   bool
	}{
		{
			name: "single port specified",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOTLSPorts: "443",
					},
				},
			},
			wantPorts: []int{443},
			wantErr:   false,
		},
		{
			name: "multiple ports specified",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOTLSPorts: "443,8443",
					},
				},
			},
			wantPorts: []int{443, 8443},
			wantErr:   false,
		},
		{
			name: "wrong port specification",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOTLSPorts: "443,eight-four-four-three",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotPorts, err := getPorts(test.service, annDOTLSPorts)
			isErr := err != nil
			if isErr != test.wantErr {
				t.Fatalf("got error %q, want error: %t", err, test.wantErr)
			}

			if !reflect.DeepEqual(gotPorts, test.wantPorts) {
				t.Errorf("got ports %v, want %v", gotPorts, test.wantPorts)
			}
		})
	}
}

func Test_getHTTPSPorts(t *testing.T) {
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			UID:  "abc123",
			Annotations: map[string]string{
				annDOTLSPorts: "443",
			},
		},
	}

	gotPorts, err := getHTTPSPorts(svc)
	if err != nil {
		t.Fatalf("got error %q", err)
	}

	wantPorts := []int{443}
	if !reflect.DeepEqual(gotPorts, wantPorts) {
		t.Errorf("got ports %v, want %v", gotPorts, wantPorts)
	}
}

func Test_getHTTP2Ports(t *testing.T) {
	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			UID:  "abc123",
			Annotations: map[string]string{
				annDOHTTP2Ports: "443",
			},
		},
	}

	gotPorts, err := getHTTP2Ports(svc)
	if err != nil {
		t.Fatalf("got error %q", err)
	}

	wantPorts := []int{443}
	if !reflect.DeepEqual(gotPorts, wantPorts) {
		t.Errorf("got ports %v, want %v", gotPorts, wantPorts)
	}
}

func Test_getProtocol(t *testing.T) {
	testcases := []struct {
		name     string
		service  *v1.Service
		protocol string
		err      error
	}{
		{
			"no protocol specified",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
				},
			},
			"tcp",
			nil,
		},
		{
			"tcp protocol specified",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol: "http",
					},
				},
			},
			"http",
			nil,
		},
		{
			"https protocol specified",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol: "https",
					},
				},
			},
			"https",
			nil,
		},
		{
			"http2 protocol specified",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol: "http2",
					},
				},
			},
			"http2",
			nil,
		},
		{
			"invalid protocol",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol: "invalid",
					},
				},
			},
			"",
			fmt.Errorf("invalid protocol: %q specified in annotation: %q", "invalid", annDOProtocol),
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			protocol, err := getProtocol(test.service)
			if protocol != test.protocol {
				t.Error("unexpected protocol")
				t.Logf("expected: %q", test.protocol)
				t.Logf("actual: %q", protocol)
			}

			if !reflect.DeepEqual(err, test.err) {
				t.Error("unexpected error")
				t.Logf("expected: %q", test.err)
				t.Logf("actual: %q", err)
			}
		})
	}
}

func Test_getStickySessionsType(t *testing.T) {
	testcases := []struct {
		name    string
		service *v1.Service
		ssType  string
		err     error
	}{
		{
			"sticky sessions type cookies",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annDOStickySessionsType: "cookies",
					},
				},
			},
			"cookies",
			nil,
		},
		{
			"sticky sessions type none",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annDOStickySessionsType: "none",
					},
				},
			},
			"none",
			nil,
		},
		{
			"sticky sessions type not defined",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			"none",
			nil,
		},
		{
			"sticky sessions type incorrect",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annDOStickySessionsType: "incorrect",
					},
				},
			},
			"none",
			nil,
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			ssType := getStickySessionsType(test.service)
			if ssType != test.ssType {
				t.Error("unexpected sticky sessions type")
				t.Logf("expected: %q", test.ssType)
				t.Logf("actual: %q", ssType)
			}
		})
	}
}

func Test_getStickySessionsCookieName(t *testing.T) {
	testcases := []struct {
		name    string
		service *v1.Service
		cName   string
		err     error
	}{
		{
			"sticky sessions cookies name DO-CCM",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annDOStickySessionsType:       "cookies",
						annDOStickySessionsCookieName: "DO-CCM",
					},
				},
			},
			"DO-CCM",
			nil,
		},
		{
			"sticky sessions cookies name empty",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annDOStickySessionsType:       "cookies",
						annDOStickySessionsCookieName: "",
					},
				},
			},
			"",
			fmt.Errorf("sticky session cookie name not specified, but required"),
		},
		{
			"sticky sessions cookie name not defined",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annDOStickySessionsType: "cookies",
					},
				},
			},
			"",
			fmt.Errorf("sticky session cookie name not specified, but required"),
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			cName, err := getStickySessionsCookieName(test.service)
			if cName != test.cName {
				t.Error("unexpected sticky sessions cookie name")
				t.Logf("expected: %q", test.cName)
				t.Logf("actual: %q", cName)
			}

			if !reflect.DeepEqual(err, test.err) {
				t.Error("unexpected error")
				t.Logf("expected: %v", test.err)
				t.Logf("actual: %v", err)
			}
		})
	}
}

func Test_getStickySessionsCookieTTL(t *testing.T) {
	testcases := []struct {
		name    string
		service *v1.Service
		ttl     int
		err     error
	}{
		{
			"sticky sessions cookies ttl",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annDOStickySessionsType:      "cookies",
						annDOStickySessionsCookieTTL: "300",
					},
				},
			},
			300,
			nil,
		},
		{
			"sticky sessions cookie ttl empty",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annDOStickySessionsType:      "cookies",
						annDOStickySessionsCookieTTL: "",
					},
				},
			},
			0,
			fmt.Errorf("sticky session cookie ttl not specified, but required"),
		},
		{
			"sticky sessions cookie ttl not defined",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annDOStickySessionsType: "cookies",
					},
				},
			},
			0,
			fmt.Errorf("sticky session cookie ttl not specified, but required"),
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			ttl, err := getStickySessionsCookieTTL(test.service)
			if ttl != test.ttl {
				t.Error("unexpected sticky sessions cookie ttl")
				t.Logf("expected: %q", test.ttl)
				t.Logf("actual: %q", ttl)
			}

			if !reflect.DeepEqual(err, test.err) {
				t.Error("unexpected error")
				t.Logf("expected: %v", test.err)
				t.Logf("actual: %v", err)
			}
		})
	}
}

func Test_buildForwardingRules(t *testing.T) {
	testcases := []struct {
		name            string
		service         *v1.Service
		forwardingRules []godo.ForwardingRule
		err             error
	}{
		{
			"default forwarding rules",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
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
			[]godo.ForwardingRule{
				{
					EntryProtocol:  "tcp",
					EntryPort:      80,
					TargetProtocol: "tcp",
					TargetPort:     30000,
					CertificateID:  "",
					TlsPassthrough: false,
				},
			},
			nil,
		},
		{
			"http forwarding rules",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol: "http",
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
			[]godo.ForwardingRule{
				{
					EntryProtocol:  "http",
					EntryPort:      80,
					TargetProtocol: "http",
					TargetPort:     30000,
					CertificateID:  "",
					TlsPassthrough: false,
				},
			},
			nil,
		},
		{
			"http2 forwarding rules with certificate ID",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol:      "http2",
						annDOCertificateID: "test-certificate",
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
			[]godo.ForwardingRule{
				{
					EntryProtocol:  "http2",
					EntryPort:      80,
					TargetProtocol: "http",
					TargetPort:     30000,
					CertificateID:  "test-certificate",
					TlsPassthrough: false,
				},
			},
			nil,
		},
		{
			"http2 forwarding rules with TLS passthrough",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol:       "http2",
						annDOTLSPassThrough: "true",
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
			[]godo.ForwardingRule{
				{
					EntryProtocol:  "http2",
					EntryPort:      80,
					TargetProtocol: "http2",
					TargetPort:     30000,
					CertificateID:  "",
					TlsPassthrough: true,
				},
			},
			nil,
		},
		{
			"unset forwarding rules on 443",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol: "tcp",
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
			[]godo.ForwardingRule{
				{
					EntryProtocol:  "tcp",
					EntryPort:      443,
					TargetProtocol: "tcp",
					TargetPort:     30000,
					CertificateID:  "",
					TlsPassthrough: false,
				},
			},
			nil,
		},
		{
			"http forwarding rules on 443",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol: "http",
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
			[]godo.ForwardingRule{
				{
					EntryProtocol:  "http",
					EntryPort:      443,
					TargetProtocol: "http",
					TargetPort:     30000,
					CertificateID:  "",
					TlsPassthrough: false,
				},
			},
			nil,
		},
		{
			"tcp forwarding rules on 443",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol: "tcp",
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
			[]godo.ForwardingRule{
				{
					EntryProtocol:  "tcp",
					EntryPort:      443,
					TargetProtocol: "tcp",
					TargetPort:     30000,
					CertificateID:  "",
					TlsPassthrough: false,
				},
			},
			nil,
		},
		{
			"tls forwarding rules",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol:      "http",
						annDOTLSPorts:      "443",
						annDOCertificateID: "test-certificate",
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
						{
							Name:     "test-https",
							Protocol: "TCP",
							Port:     int32(443),
							NodePort: int32(30000),
						},
					},
				},
			},
			[]godo.ForwardingRule{
				{
					EntryProtocol:  "http",
					EntryPort:      80,
					TargetProtocol: "http",
					TargetPort:     30000,
					CertificateID:  "",
					TlsPassthrough: false,
				},
				{
					EntryProtocol:  "https",
					EntryPort:      443,
					TargetProtocol: "http",
					TargetPort:     30000,
					CertificateID:  "test-certificate",
					TlsPassthrough: false,
				},
			},
			nil,
		},
		{
			"tls forwarding rules with tls pass through",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol:       "http",
						annDOTLSPorts:       "443",
						annDOTLSPassThrough: "true",
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
						{
							Name:     "test-https",
							Protocol: "TCP",
							Port:     int32(443),
							NodePort: int32(30000),
						},
					},
				},
			},
			[]godo.ForwardingRule{
				{
					EntryProtocol:  "http",
					EntryPort:      80,
					TargetProtocol: "http",
					TargetPort:     30000,
					CertificateID:  "",
					TlsPassthrough: false,
				},
				{
					EntryProtocol:  "https",
					EntryPort:      443,
					TargetProtocol: "https",
					TargetPort:     30000,
					CertificateID:  "",
					TlsPassthrough: true,
				},
			},
			nil,
		},
		{
			"tls forwarding rules with certificate and 443 port",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOCertificateID: "test-certificate",
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
						{
							Name:     "test-https",
							Protocol: "TCP",
							Port:     int32(443),
							NodePort: int32(30000),
						},
					},
				},
			},
			[]godo.ForwardingRule{
				{
					EntryProtocol:  "tcp",
					EntryPort:      80,
					TargetProtocol: "tcp",
					TargetPort:     30000,
					CertificateID:  "",
					TlsPassthrough: false,
				},
				{
					EntryProtocol:  "https",
					EntryPort:      443,
					TargetProtocol: "http",
					TargetPort:     30000,
					CertificateID:  "test-certificate",
					TlsPassthrough: false,
				},
			},
			nil,
		},
		{
			"tls forwarding rules with tls path through and 443 port",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOTLSPassThrough: "true",
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
						{
							Name:     "test-https",
							Protocol: "TCP",
							Port:     int32(443),
							NodePort: int32(30000),
						},
					},
				},
			},
			[]godo.ForwardingRule{
				{
					EntryProtocol:  "tcp",
					EntryPort:      80,
					TargetProtocol: "tcp",
					TargetPort:     30000,
					CertificateID:  "",
					TlsPassthrough: false,
				},
				{
					EntryProtocol:  "https",
					EntryPort:      443,
					TargetProtocol: "https",
					TargetPort:     30000,
					CertificateID:  "",
					TlsPassthrough: true,
				},
			},
			nil,
		},
		{
			"HTTP2 port 443 specified",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOTLSPassThrough: "true",
						annDOHTTP2Ports:     "443",
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
						{
							Name:     "test-http2",
							Protocol: "TCP",
							Port:     int32(443),
							NodePort: int32(40000),
						},
					},
				},
			},
			[]godo.ForwardingRule{
				{
					EntryProtocol:  "tcp",
					EntryPort:      80,
					TargetProtocol: "tcp",
					TargetPort:     30000,
					CertificateID:  "",
					TlsPassthrough: false,
				},
				{
					EntryProtocol:  "http2",
					EntryPort:      443,
					TargetProtocol: "http2",
					TargetPort:     40000,
					CertificateID:  "",
					TlsPassthrough: true,
				},
			},
			nil,
		},
		{
			"all HTTP* protocols used simultaneously",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol:       "http",
						annDOTLSPassThrough: "true",
						annDOHTTP2Ports:     "886",
					},
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name:     "test-http",
							Protocol: "TCP",
							Port:     int32(80),
							NodePort: int32(30000),
						},
						{
							Name:     "test-https",
							Protocol: "TCP",
							Port:     int32(443),
							NodePort: int32(40000),
						},
						{
							Name:     "test-http2",
							Protocol: "TCP",
							Port:     int32(886),
							NodePort: int32(50000),
						},
					},
				},
			},
			[]godo.ForwardingRule{
				{
					EntryProtocol:  "http",
					EntryPort:      80,
					TargetProtocol: "http",
					TargetPort:     30000,
					CertificateID:  "",
					TlsPassthrough: false,
				},
				{
					EntryProtocol:  "https",
					EntryPort:      443,
					TargetProtocol: "https",
					TargetPort:     40000,
					CertificateID:  "",
					TlsPassthrough: true,
				},
				{
					EntryProtocol:  "http2",
					EntryPort:      886,
					TargetProtocol: "http2",
					TargetPort:     50000,
					CertificateID:  "",
					TlsPassthrough: true,
				},
			},
			nil,
		},
		{
			"default protocol is maintained",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol:       "http",
						annDOTLSPassThrough: "true",
						annDOHTTP2Ports:     "443",
					},
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name:     "test-http2",
							Protocol: "TCP",
							Port:     int32(443),
							NodePort: int32(40000),
						},
						{
							Name:     "test-http",
							Protocol: "TCP",
							Port:     int32(80),
							NodePort: int32(30000),
						},
					},
				},
			},
			[]godo.ForwardingRule{
				{
					EntryProtocol:  "http2",
					EntryPort:      443,
					TargetProtocol: "http2",
					TargetPort:     40000,
					CertificateID:  "",
					TlsPassthrough: true,
				},
				{
					EntryProtocol:  "http",
					EntryPort:      80,
					TargetProtocol: "http",
					TargetPort:     30000,
					CertificateID:  "",
					TlsPassthrough: false,
				},
			},
			nil,
		},
		{
			"default forwarding rules with sticky sessions no protocol specified",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOStickySessionsType:       "cookies",
						annDOStickySessionsCookieName: "DO-CCM",
						annDOStickySessionsCookieTTL:  "300",
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
			[]godo.ForwardingRule{
				{
					EntryProtocol:  "tcp",
					EntryPort:      80,
					TargetProtocol: "tcp",
					TargetPort:     30000,
					CertificateID:  "",
					TlsPassthrough: false,
				},
			},
			nil,
		},
		{
			"http forwarding rules with sticky sessions and http protocol",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol:                 "http",
						annDOStickySessionsType:       "cookies",
						annDOStickySessionsCookieName: "DO-CCM",
						annDOStickySessionsCookieTTL:  "300",
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
			[]godo.ForwardingRule{
				{
					EntryProtocol:  "http",
					EntryPort:      80,
					TargetProtocol: "http",
					TargetPort:     30000,
					CertificateID:  "",
					TlsPassthrough: false,
				},
			},
			nil,
		},
		{
			"tls forwarding rules with sticky sessions",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol:                 "http",
						annDOTLSPorts:                 "443",
						annDOCertificateID:            "test-certificate",
						annDOStickySessionsType:       "cookies",
						annDOStickySessionsCookieName: "DO-CCM",
						annDOStickySessionsCookieTTL:  "300",
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
						{
							Name:     "test-https",
							Protocol: "TCP",
							Port:     int32(443),
							NodePort: int32(30000),
						},
					},
				},
			},
			[]godo.ForwardingRule{
				{
					EntryProtocol:  "http",
					EntryPort:      80,
					TargetProtocol: "http",
					TargetPort:     30000,
					CertificateID:  "",
					TlsPassthrough: false,
				},
				{
					EntryProtocol:  "https",
					EntryPort:      443,
					TargetProtocol: "http",
					TargetPort:     30000,
					CertificateID:  "test-certificate",
					TlsPassthrough: false,
				},
			},
			nil,
		},
		{
			"invalid service protocol",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol:       "http",
						annDOTLSPorts:       "443",
						annDOTLSPassThrough: "true",
					},
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name:     "test",
							Protocol: "UDP",
							Port:     int32(80),
							NodePort: int32(30000),
						},
					},
				},
			},
			nil,
			fmt.Errorf("only TCP protocol is supported, got: %q", "UDP"),
		},
		{
			"invalid TLS config, set both certificate id and tls pass through",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol:       "http",
						annDOTLSPorts:       "443",
						annDOCertificateID:  "test-certificate",
						annDOTLSPassThrough: "true",
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
						{
							Name:     "test-https",
							Protocol: "TCP",
							Port:     int32(443),
							NodePort: int32(30000),
						},
					},
				},
			},
			nil,
			errors.New("failed to build TLS part(s) of forwarding rule: either certificate id should be set or tls pass through enabled, not both"),
		},
		{
			"invalid TLS config, neither certificate ID is set or tls pass through enabled",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol: "http",
						annDOTLSPorts: "443",
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
						{
							Name:     "test-https",
							Protocol: "TCP",
							Port:     int32(443),
							NodePort: int32(30000),
						},
					},
				},
			},
			nil,
			errors.New("failed to build TLS part(s) of forwarding rule: must set certificate id or enable tls pass through"),
		},
		{
			"invalid HTTP2 config, neither certificate ID is set or tls pass through enabled",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol: "http2",
						annDOTLSPorts: "443",
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
						{
							Name:     "test-https",
							Protocol: "TCP",
							Port:     int32(443),
							NodePort: int32(30000),
						},
					},
				},
			},
			nil,
			errors.New("failed to build TLS part(s) of forwarding rule: must set certificate id or enable tls pass through"),
		},
		{
			"secure ports shared",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOTLSPorts:       "443,8443",
						annDOHTTP2Ports:     "4443,443",
						annDOTLSPassThrough: "true",
					},
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name:     "test-https",
							Protocol: "TCP",
							Port:     int32(443),
							NodePort: int32(30000),
						},
						{
							Name:     "test-http2",
							Protocol: "TCP",
							Port:     int32(4443),
							NodePort: int32(40000),
						},
					},
				},
			},
			nil,
			fmt.Errorf("%q and %q cannot share values but found: 443", annDOTLSPorts, annDOHTTP2Ports),
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			forwardingRules, err := buildForwardingRules(test.service)
			if !reflect.DeepEqual(forwardingRules, test.forwardingRules) {
				t.Error("unexpected forwarding rules")
				t.Logf("expected: %v", test.forwardingRules)
				t.Logf("actual: %v", forwardingRules)
			}

			if !reflect.DeepEqual(err, test.err) {
				t.Error("unexpected error")
				t.Logf("expected: %v", test.err)
				t.Logf("actual: %v", err)
			}
		})
	}
}

func Test_buildHealthCheck(t *testing.T) {
	testcases := []struct {
		name         string
		service      *v1.Service
		healthcheck  *godo.HealthCheck
		errMsgPrefix string
	}{
		{
			name: "tcp health check",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
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
			healthcheck: defaultHealthCheck("tcp", 30000, ""),
		},
		{
			name: "default health check with http service protocol",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol: "http",
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
			healthcheck: defaultHealthCheck("tcp", 30000, ""),
		},
		{
			name: "default health check with https service protocol",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol:      "https",
						annDOCertificateID: "test-certificate",
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
			healthcheck: defaultHealthCheck("tcp", 30000, ""),
		},
		{
			name: "default health check with TLS passthrough",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol:       "https",
						annDOTLSPassThrough: "true",
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
			healthcheck: defaultHealthCheck("tcp", 30000, ""),
		},
		{
			name: "https health check",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol:      "https",
						annDOCertificateID: "test-certificate",
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
			healthcheck: defaultHealthCheck("tcp", 30000, ""),
		},
		{
			name: "http2 health check",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol:      "http2",
						annDOCertificateID: "test-certificate",
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
			healthcheck: defaultHealthCheck("tcp", 30000, ""),
		},
		{
			name: "https health check with TLS passthrough",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol:       "https",
						annDOTLSPassThrough: "true",
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
			healthcheck: defaultHealthCheck("tcp", 30000, ""),
		},
		{
			name: "http2 health check with TLS passthrough",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol:       "http2",
						annDOTLSPassThrough: "true",
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
			healthcheck: defaultHealthCheck("tcp", 30000, ""),
		},
		{
			name: "http health check using protocol override",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol:            "tcp",
						annDOHealthCheckProtocol: "http",
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
			healthcheck: defaultHealthCheck("http", 30000, ""),
		},
		{
			name: "https health check using protocol override",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol:            "https",
						annDOCertificateID:       "test-certificate",
						annDOHealthCheckProtocol: "http",
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
			healthcheck: defaultHealthCheck("http", 30000, ""),
		},
		{
			name: "http2 health check using protocol override",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol:            "http2",
						annDOCertificateID:       "test-certificate",
						annDOHealthCheckProtocol: "http",
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
			healthcheck: defaultHealthCheck("http", 30000, ""),
		},
		{
			name: "http health check with https and certificate",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol:            "https",
						annDOCertificateID:       "test-certificate",
						annDOHealthCheckProtocol: "http",
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
			healthcheck: defaultHealthCheck("http", 30000, ""),
		},
		{
			name: "http health check with path",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol:        "http",
						annDOHealthCheckPath: "/health",
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
			healthcheck: defaultHealthCheck("http", 30000, "/health"),
		},
		{
			name: "invalid health check using protocol override",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOHealthCheckProtocol: "invalid",
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
			errMsgPrefix: fmt.Sprintf("invalid protocol: %q specified in annotation: %q", "invalid", annDOProtocol),
		},
		{
			name: "health check with custom port",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol:        "http",
						annDOHealthCheckPath: "/health",
						annDOHealthCheckPort: "636",
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
						{
							Name:     "test",
							Protocol: "TCP",
							Port:     int32(636),
							NodePort: int32(32000),
						},
					},
				},
			},
			healthcheck: defaultHealthCheck("http", 32000, "/health"),
		},
		{
			name: "invalid health check using port override with non-existent port",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
					UID:       "abc123",
					Annotations: map[string]string{
						annDOHealthCheckPort: "9999",
					},
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name:     "test",
							Protocol: "TCP",
							Port:     int32(636),
							NodePort: int32(30000),
						},
						{
							Name:     "test",
							Protocol: "TCP",
							Port:     int32(332),
							NodePort: int32(32000),
						},
					},
				},
			},
			errMsgPrefix: fmt.Sprintf("specified health check port %d does not exist on service default/test", 9999),
		},
		{
			name: "invalid health check using port override with non-numeric port",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOHealthCheckPort: "invalid",
					},
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name:     "test",
							Protocol: "TCP",
							Port:     int32(636),
							NodePort: int32(30000),
						},
						{
							Name:     "test",
							Protocol: "TCP",
							Port:     int32(332),
							NodePort: int32(32000),
						},
					},
				},
			},
			errMsgPrefix: "failed to get health check port: strconv.Atoi: parsing \"invalid\": invalid syntax",
		},
		{
			name: "invalid health check using port override with multiple ports",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOHealthCheckPort: "636,332",
					},
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name:     "test",
							Protocol: "TCP",
							Port:     int32(636),
							NodePort: int32(30000),
						},
						{
							Name:     "test",
							Protocol: "TCP",
							Port:     int32(332),
							NodePort: int32(32000),
						},
					},
				},
			},
			errMsgPrefix: fmt.Sprintf("annotation %s only supports a single port, but found multiple: [636 332]", annDOHealthCheckPort),
		},
		{
			name: "default numeric parameters",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
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
			healthcheck: &godo.HealthCheck{
				Protocol:               "tcp",
				Port:                   30000,
				CheckIntervalSeconds:   3,
				ResponseTimeoutSeconds: 5,
				UnhealthyThreshold:     3,
				HealthyThreshold:       5,
			},
		},
		{
			name: "custom numeric parameters",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOHealthCheckIntervalSeconds:        "1",
						annDOHealthCheckResponseTimeoutSeconds: "3",
						annDOHealthCheckUnhealthyThreshold:     "1",
						annDOHealthCheckHealthyThreshold:       "2",
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
			healthcheck: &godo.HealthCheck{
				Protocol:               "tcp",
				Port:                   30000,
				CheckIntervalSeconds:   1,
				ResponseTimeoutSeconds: 3,
				UnhealthyThreshold:     1,
				HealthyThreshold:       2,
			},
		},
		{
			name: "invalid check interval",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOHealthCheckIntervalSeconds: "invalid",
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
			errMsgPrefix: fmt.Sprintf("failed to parse health check interval annotation %q:", annDOHealthCheckIntervalSeconds),
		},
		{
			name: "invalid response timeout",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOHealthCheckResponseTimeoutSeconds: "invalid",
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
			errMsgPrefix: fmt.Sprintf("failed to parse health check response timeout annotation %q:", annDOHealthCheckResponseTimeoutSeconds),
		},
		{
			name: "invalid unhealthy threshold",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOHealthCheckUnhealthyThreshold: "invalid",
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
			errMsgPrefix: fmt.Sprintf("failed to parse health check unhealthy threshold annotation %q:", annDOHealthCheckUnhealthyThreshold),
		},
		{
			name: "invalid healthy threshold",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOHealthCheckHealthyThreshold: "invalid",
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
			errMsgPrefix: fmt.Sprintf("failed to parse health check healthy threshold annotation %q:", annDOHealthCheckHealthyThreshold),
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			healthcheck, err := buildHealthCheck(test.service)
			if !reflect.DeepEqual(healthcheck, test.healthcheck) {
				t.Fatalf("got health check:\n\n%v\n\nwant:\n\n%s\n", healthcheck, test.healthcheck)
			}

			wantErr := test.errMsgPrefix != ""
			if wantErr && !strings.HasPrefix(err.Error(), test.errMsgPrefix) {
				t.Fatalf("got error:\n\n%s\n\nwant prefix:\n\n%s\n", err, test.errMsgPrefix)
			}
		})
	}

}

func Test_buildStickySessions(t *testing.T) {
	testcases := []struct {
		name          string
		service       *v1.Service
		stickysession *godo.StickySessions
		err           error
	}{
		{
			"sticky sessions type none",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annDOStickySessionsType: "none",
					},
				},
			},
			&godo.StickySessions{
				Type: "none",
			},
			nil,
		},
		{
			"sticky sessions type not provided",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			&godo.StickySessions{
				Type: "none",
			},
			nil,
		},
		{
			"sticky sessions type cookies",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annDOStickySessionsType:       "cookies",
						annDOStickySessionsCookieName: "DO-CCM",
						annDOStickySessionsCookieTTL:  "300",
					},
				},
			},
			&godo.StickySessions{
				Type:             "cookies",
				CookieName:       "DO-CCM",
				CookieTtlSeconds: 300,
			},
			nil,
		},
		{
			"sticky sessions type cookies without ttl",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annDOStickySessionsType:       "cookies",
						annDOStickySessionsCookieName: "DO-CCM",
					},
				},
			},
			nil,
			fmt.Errorf("sticky session cookie ttl not specified, but required"),
		},
		{
			"sticky sessions type cookies without name",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annDOStickySessionsType:      "cookies",
						annDOStickySessionsCookieTTL: "300",
					},
				},
			},
			nil,
			fmt.Errorf("sticky session cookie name not specified, but required"),
		},
		{
			"sticky sessions type cookies without name and ttl",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						annDOStickySessionsType: "cookies",
					},
				},
			},
			nil,
			fmt.Errorf("sticky session cookie name not specified, but required"),
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			stickysession, err := buildStickySessions(test.service)
			if !reflect.DeepEqual(stickysession, test.stickysession) {
				t.Error("unexpected health check")
				t.Logf("expected: %v", test.stickysession)
				t.Logf("actual: %v", stickysession)
			}

			if !reflect.DeepEqual(err, test.err) {
				t.Error("unexpected error")
				t.Logf("expected: %v", test.err)
				t.Logf("actual: %v", err)
			}
		})
	}
}

func Test_getRedirectHTTPToHTTPS(t *testing.T) {
	testcases := []struct {
		name                string
		service             *v1.Service
		redirectHTTPToHTTPS bool
	}{
		{
			"Redirect Http to Https true",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDORedirectHTTPToHTTPS: "true",
					},
				},
			},
			true,
		},
		{
			"Redirect Http to Https false",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDORedirectHTTPToHTTPS: "false",
					},
				},
			},
			false,
		},
		{
			"Redirect Http to Https not defined",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					UID:         "abc123",
					Annotations: map[string]string{},
				},
			},
			false,
		},
		{
			"Service annotations nil",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
				},
			},
			false,
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			redirectHTTPToHTTPS := getRedirectHTTPToHTTPS(test.service)
			if redirectHTTPToHTTPS != test.redirectHTTPToHTTPS {
				t.Error("unexpected redirect Http to Https")
				t.Logf("expected: %t", test.redirectHTTPToHTTPS)
				t.Logf("actual: %t", redirectHTTPToHTTPS)
			}
		})
	}

}

func Test_getEnableProxyProtocol(t *testing.T) {
	testcases := []struct {
		name                    string
		service                 *v1.Service
		wantErr                 bool
		wantEnableProxyProtocol bool
	}{
		{
			name: "enabled",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOEnableProxyProtocol: "true",
					},
				},
			},
			wantErr:                 false,
			wantEnableProxyProtocol: true,
		},
		{
			name: "disabled",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOEnableProxyProtocol: "false",
					},
				},
			},
			wantErr:                 false,
			wantEnableProxyProtocol: false,
		},
		{
			name: "annotation missing",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
				},
			},
			wantErr:                 false,
			wantEnableProxyProtocol: false,
		},
		{
			name: "illegal value",
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOEnableProxyProtocol: "42",
					},
				},
			},
			wantErr:                 true,
			wantEnableProxyProtocol: false,
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			gotEnabledProxyProtocol, err := getEnableProxyProtocol(test.service)
			if test.wantErr != (err != nil) {
				t.Errorf("got error %q, want error: %t", err, test.wantErr)
			}

			if gotEnabledProxyProtocol != test.wantEnableProxyProtocol {
				t.Fatalf("got enabled proxy protocol %t, want %t", gotEnabledProxyProtocol, test.wantEnableProxyProtocol)
			}
		})
	}
}

func Test_buildLoadBalancerRequest(t *testing.T) {
	testcases := []struct {
		name     string
		droplets []godo.Droplet
		service  *v1.Service
		nodes    []*v1.Node
		lbr      *godo.LoadBalancerRequest
		err      error
	}{
		{
			"successful load balancer request",
			[]godo.Droplet{
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
			},
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "foobar123",
					Annotations: map[string]string{
						annDOProtocol:        "http",
						annDOHealthCheckPath: "/health",
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
			[]*v1.Node{
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
			},
			&godo.LoadBalancerRequest{
				// cloudprovider.GetLoadBalancer name uses 'a' + service.UID
				// as loadbalancer name
				Name:       "afoobar123",
				DropletIDs: []int{100, 101, 102},
				Region:     "nyc3",
				ForwardingRules: []godo.ForwardingRule{
					{
						EntryProtocol:  "http",
						EntryPort:      80,
						TargetProtocol: "http",
						TargetPort:     30000,
						CertificateID:  "",
						TlsPassthrough: false,
					},
				},
				HealthCheck: defaultHealthCheck("http", 30000, "/health"),
				Algorithm:   "round_robin",
				StickySessions: &godo.StickySessions{
					Type: "none",
				},
			},
			nil,
		},
		{
			"successful load balancer request using http2",
			[]godo.Droplet{
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
			},
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "foobar123",
					Annotations: map[string]string{
						annDOProtocol:        "http2",
						annDOCertificateID:   "test-certificate",
						annDOHealthCheckPath: "/health",
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
			[]*v1.Node{
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
			},
			&godo.LoadBalancerRequest{
				// cloudprovider.GetLoadBalancer name uses 'a' + service.UID
				// as loadbalancer name
				Name:       "afoobar123",
				DropletIDs: []int{100, 101, 102},
				Region:     "nyc3",
				ForwardingRules: []godo.ForwardingRule{
					{
						EntryProtocol:  "http2",
						EntryPort:      80,
						TargetProtocol: "http",
						TargetPort:     30000,
						CertificateID:  "test-certificate",
						TlsPassthrough: false,
					},
				},
				HealthCheck: defaultHealthCheck("http", 30000, "/health"),
				Algorithm:   "round_robin",
				StickySessions: &godo.StickySessions{
					Type: "none",
				},
			},
			nil,
		},
		{
			"successful load balancer request with custom health checks",
			[]godo.Droplet{
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
			},
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "foobar123",
					Annotations: map[string]string{
						annDOProtocol:            "tcp",
						annDOHealthCheckPath:     "/health",
						annDOHealthCheckProtocol: "http",
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
			[]*v1.Node{
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
			},
			&godo.LoadBalancerRequest{
				// cloudprovider.GetLoadBalancer name uses 'a' + service.UID
				// as loadbalancer name
				Name:       "afoobar123",
				DropletIDs: []int{100, 101, 102},
				Region:     "nyc3",
				ForwardingRules: []godo.ForwardingRule{
					{
						EntryProtocol:  "tcp",
						EntryPort:      80,
						TargetProtocol: "tcp",
						TargetPort:     30000,
						CertificateID:  "",
						TlsPassthrough: false,
					},
				},
				HealthCheck: defaultHealthCheck("http", 30000, "/health"),
				Algorithm:   "round_robin",
				StickySessions: &godo.StickySessions{
					Type: "none",
				},
			},
			nil,
		},
		{
			"successful load balancer request using least_connections algorithm",
			[]godo.Droplet{
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
			},
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "foobar123",
					Annotations: map[string]string{
						annDOProtocol:  "http",
						annDOAlgorithm: "least_connections",
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
			[]*v1.Node{
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
			},
			&godo.LoadBalancerRequest{
				// cloudprovider.GetLoadBalancer name uses 'a' + service.UID
				// as loadbalancer name
				Name:       "afoobar123",
				DropletIDs: []int{100, 101, 102},
				Region:     "nyc3",
				ForwardingRules: []godo.ForwardingRule{
					{
						EntryProtocol:  "http",
						EntryPort:      80,
						TargetProtocol: "http",
						TargetPort:     30000,
						CertificateID:  "",
						TlsPassthrough: false,
					},
				},
				HealthCheck: defaultHealthCheck("tcp", 30000, ""),
				Algorithm:   "least_connections",
				StickySessions: &godo.StickySessions{
					Type: "none",
				},
			},
			nil,
		},
		{
			"successful load balancer request with cookies sticky sessions.",
			[]godo.Droplet{
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
			},
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "foobar123",
					Annotations: map[string]string{
						annDOProtocol:                 "http",
						annDOStickySessionsType:       "cookies",
						annDOStickySessionsCookieName: "DO-CCM",
						annDOStickySessionsCookieTTL:  "300",
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
			[]*v1.Node{
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
			},
			&godo.LoadBalancerRequest{
				// cloudprovider.GetLoadBalancer name uses 'a' + service.UID
				// as loadbalancer name
				Name:       "afoobar123",
				DropletIDs: []int{100, 101, 102},
				Region:     "nyc3",
				ForwardingRules: []godo.ForwardingRule{
					{
						EntryProtocol:  "http",
						EntryPort:      80,
						TargetProtocol: "http",
						TargetPort:     30000,
						CertificateID:  "",
						TlsPassthrough: false,
					},
				},
				HealthCheck: defaultHealthCheck("tcp", 30000, ""),
				Algorithm:   "round_robin",
				StickySessions: &godo.StickySessions{
					Type:             "cookies",
					CookieName:       "DO-CCM",
					CookieTtlSeconds: 300,
				},
			},
			nil,
		},
		{
			"successful load balancer request with cookies sticky sessions using https",
			[]godo.Droplet{
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
			},
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "foobar123",
					Annotations: map[string]string{
						annDOProtocol:                 "https",
						annDOCertificateID:            "test-certificate",
						annDOHealthCheckProtocol:      "http",
						annDOStickySessionsType:       "cookies",
						annDOStickySessionsCookieName: "DO-CCM",
						annDOStickySessionsCookieTTL:  "300",
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
			[]*v1.Node{
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
			},
			&godo.LoadBalancerRequest{
				// cloudprovider.GetLoadBalancer name uses 'a' + service.UID
				// as loadbalancer name
				Name:       "afoobar123",
				DropletIDs: []int{100, 101, 102},
				Region:     "nyc3",
				ForwardingRules: []godo.ForwardingRule{
					{
						EntryProtocol:  "https",
						EntryPort:      443,
						TargetProtocol: "http",
						TargetPort:     30000,
						CertificateID:  "test-certificate",
						TlsPassthrough: false,
					},
				},
				HealthCheck: defaultHealthCheck("http", 30000, ""),
				Algorithm:   "round_robin",
				StickySessions: &godo.StickySessions{
					Type:             "cookies",
					CookieName:       "DO-CCM",
					CookieTtlSeconds: 300,
				},
			},
			nil,
		},
		{
			"successful load balancer request with redirect_http_to_https",
			[]godo.Droplet{
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
			},
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "foobar123",
					Annotations: map[string]string{
						annDOProtocol:            "http",
						annDOAlgorithm:           "round_robin",
						annDORedirectHTTPToHTTPS: "true",
						annDOTLSPorts:            "443",
						annDOCertificateID:       "test-certificate",
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
						{
							Name:     "test",
							Protocol: "TCP",
							Port:     int32(443),
							NodePort: int32(30000),
						},
					},
				},
			},
			[]*v1.Node{
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
			},
			&godo.LoadBalancerRequest{
				// cloudprovider.GetLoadBalancer name uses 'a' + service.UID
				// as loadbalancer name
				Name:       "afoobar123",
				DropletIDs: []int{100, 101, 102},
				Region:     "nyc3",
				ForwardingRules: []godo.ForwardingRule{
					{
						EntryProtocol:  "http",
						EntryPort:      80,
						TargetProtocol: "http",
						TargetPort:     30000,
					},
					{
						EntryProtocol:  "https",
						EntryPort:      443,
						TargetProtocol: "http",
						TargetPort:     30000,
						CertificateID:  "test-certificate",
					},
				},
				HealthCheck:         defaultHealthCheck("tcp", 30000, ""),
				Algorithm:           "round_robin",
				RedirectHttpToHttps: true,
				StickySessions: &godo.StickySessions{
					Type: "none",
				},
			},
			nil,
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := newFakeDropletClient(
				&fakeDropletService{
					listFunc: func(context.Context, *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
						return test.droplets, newFakeOKResponse(), nil
					},
				},
			)
			fakeResources := newResources("", "", fakeClient)

			lb := &loadBalancers{
				resources:         fakeResources,
				region:            "nyc3",
				lbActiveTimeout:   2,
				lbActiveCheckTick: 1,
			}

			lbr, err := lb.buildLoadBalancerRequest(context.Background(), test.service, test.nodes)

			if !reflect.DeepEqual(lbr, test.lbr) {
				t.Error("unexpected load balancer request")
				t.Logf("expected: %v", test.lbr)
				t.Logf("actual: %v", lbr)
			}

			if !reflect.DeepEqual(err, test.err) {
				t.Error("unexpected error")
				t.Logf("expected: %v", test.err)
				t.Logf("actual: %v", err)
			}

		})
	}
}

func Test_buildLoadBalancerRequestWithClusterID(t *testing.T) {
	tests := []struct {
		name      string
		clusterID string
		vpcID     string
		err       error
	}{
		{
			name:      "happy path",
			clusterID: clusterID,
			vpcID:     "vpc_uuid",
		},
		{
			name:      "missing cluster id",
			clusterID: "",
			vpcID:     "vpc_uuid",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
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
			}
			nodes := []*v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
					},
				},
			}
			fakeClient := newFakeDropletClient(
				&fakeDropletService{
					listFunc: func(context.Context, *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
						droplets := []godo.Droplet{
							{
								ID:   100,
								Name: "node-1",
							},
						}
						return droplets, newFakeOKResponse(), nil
					},
				},
			)
			fakeResources := newResources(test.clusterID, test.vpcID, fakeClient)
			fakeResources.clusterVPCID = test.vpcID

			lb := &loadBalancers{
				resources: fakeResources,
				region:    "nyc3",
				clusterID: clusterID,
			}

			lbr, err := lb.buildLoadBalancerRequest(context.Background(), service, nodes)
			if test.err != nil {
				if err == nil {
					t.Fatal("expected error but got none")
				}

				if want, got := test.err, err; !reflect.DeepEqual(want, got) {
					t.Errorf("incorrect err\nwant: %#v\n got: %#v", want, got)
				}
				return
			}
			if err != nil {
				t.Errorf("got error: %s", err)
			}

			var wantTags []string
			if test.clusterID != "" {
				wantTags = []string{buildK8sTag(clusterID)}
			}
			if !reflect.DeepEqual(lbr.Tags, wantTags) {
				t.Errorf("got tags %q, want %q", lbr.Tags, wantTags)
			}

			if want, got := "vpc_uuid", lbr.VPCUUID; want != got {
				t.Errorf("incorrect vpc uuid\nwant: %#v\n got: %#v", want, got)
			}
		})
	}
}

func Test_nodeToDropletIDs(t *testing.T) {
	testcases := []struct {
		name         string
		nodes        []*v1.Node
		droplets     []godo.Droplet
		dropletIDs   []int
		missingNames []string
	}{
		{
			name: "node to droplet ids",
			nodes: []*v1.Node{
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
			},
			droplets: []godo.Droplet{
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
			},
			dropletIDs: []int{100, 101, 102},
		},
		{
			name: "node to droplet ID with droplets not in cluster",
			nodes: []*v1.Node{
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
			},
			droplets: []godo.Droplet{
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
				{
					ID:   201,
					Name: "node-10",
				},
				{
					ID:   202,
					Name: "node-11",
				},
			},
			dropletIDs: []int{100, 101, 102},
		},
		{
			name: "droplet IDs returned from provider ID and API",
			nodes: []*v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
					},
					Spec: v1.NodeSpec{
						ProviderID: "digitalocean://100",
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
			},
			droplets: []godo.Droplet{
				{
					ID:   101,
					Name: "node-2",
				},
				{
					ID:   102,
					Name: "node-3",
				},
			},
			dropletIDs: []int{100, 101, 102},
		},
		{
			name: "missing droplets",
			nodes: []*v1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-1",
					},
					Spec: v1.NodeSpec{
						ProviderID: "digitalocean://100",
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
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node-4",
					},
				},
			},
			droplets: []godo.Droplet{
				{
					ID:   101,
					Name: "node-2",
				},
			},
			dropletIDs:   []int{100, 101},
			missingNames: []string{"node-3", "node-4"},
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := newFakeDropletClient(
				&fakeDropletService{
					listFunc: func(context.Context, *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
						return test.droplets, newFakeOKResponse(), nil
					},
				},
			)
			fakeResources := newResources("", "", fakeClient)

			lb := &loadBalancers{
				resources:         fakeResources,
				region:            "nyc1",
				lbActiveTimeout:   2,
				lbActiveCheckTick: 1,
			}

			var logBuf bytes.Buffer
			if len(test.missingNames) > 0 {
				klog.SetOutput(ioutil.Discard)
				klog.SetOutputBySeverity("ERROR", &logBuf)
			}

			dropletIDs, err := lb.nodesToDropletIDs(context.Background(), test.nodes)
			if !reflect.DeepEqual(dropletIDs, test.dropletIDs) {
				t.Error("unexpected droplet IDs")
				t.Logf("expected: %v", test.dropletIDs)
				t.Logf("actual: %v", dropletIDs)
			}

			if err != nil {
				t.Errorf("got error: %s", err)
			}

			if len(test.missingNames) > 0 {
				klog.Flush()
				wantErrMsg := fmt.Sprintf("Failed to find droplets for nodes %s", strings.Join(test.missingNames, " "))
				gotErrMsg := logBuf.String()
				if !strings.Contains(gotErrMsg, wantErrMsg) {
					t.Errorf("got missing nodes error message %q, missing %q", gotErrMsg, wantErrMsg)
				}
			}
		})
	}
}

func Test_GetLoadBalancer(t *testing.T) {
	testcases := []struct {
		name     string
		getFn    func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error)
		listFn   func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error)
		service  *v1.Service
		lbStatus *v1.LoadBalancerStatus
		exists   bool
		err      error
	}{
		{
			name: "got loadbalancer by name",
			listFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{
					{
						// loadbalancer names are a + service.UID
						// see cloudprovider.DefaultLoadBalancerName
						ID:     "load-balancer-id",
						Name:   "afoobar123",
						IP:     "10.0.0.1",
						Status: lbStatusActive,
					},
				}, newFakeOKResponse(), nil
			},
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: v1.NamespaceDefault,
					UID:       "foobar123",
					Annotations: map[string]string{
						annDOProtocol: "http",
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
			lbStatus: &v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{
						IP: "10.0.0.1",
					},
				},
			},
			exists: true,
			err:    nil,
		},
		{
			name: "got loadbalancer by annotation name",
			listFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{
					{
						ID:     "load-balancer-id",
						Name:   "my-awesome-load-balancer",
						IP:     "10.0.0.1",
						Status: lbStatusActive,
					},
				}, newFakeOKResponse(), nil
			},
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: v1.NamespaceDefault,
					UID:       "foobar123",
					Annotations: map[string]string{
						annDOProtocol:          "http",
						annoDOLoadBalancerName: "my awesome/load-balancer",
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
			lbStatus: &v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{
						IP: "10.0.0.1",
					},
				},
			},
			exists: true,
			err:    nil,
		},
		{
			name: "got loadbalancer by ID",
			getFn: func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error) {
				return &godo.LoadBalancer{
					// loadbalancer names are a + service.UID
					// see cloudprovider.DefaultLoadBalancerName
					ID:     "load-balancer-id",
					Name:   "afoobar123",
					IP:     "10.0.0.1",
					Status: lbStatusActive,
				}, newFakeOKResponse(), nil
			},
			listFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return nil, newFakeNotOKResponse(), errors.New("list should not have been invoked")
			},
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: v1.NamespaceDefault,
					UID:       "foobar123",
					Annotations: map[string]string{
						annDOProtocol:        "http",
						annoDOLoadBalancerID: "load-balancer-id",
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
			lbStatus: &v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{
						IP: "10.0.0.1",
					},
				},
			},
			exists: true,
			err:    nil,
		},
		{
			name: "loadbalancer not found",
			listFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{}, newFakeOKResponse(), nil
			},
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "foobar123",
					Annotations: map[string]string{
						annDOProtocol: "http",
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
			lbStatus: nil,
			exists:   false,
			err:      nil,
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			fakeLB := &fakeLBService{
				getFn:  test.getFn,
				listFn: test.listFn,
			}
			fakeClient := newFakeLBClient(fakeLB)
			fakeResources := newResources("", "", fakeClient)
			fakeResources.kclient = fake.NewSimpleClientset()
			if _, err := fakeResources.kclient.CoreV1().Services(test.service.Namespace).Create(test.service); err != nil {
				t.Fatalf("failed to add service to fake client: %s", err)
			}

			lb := &loadBalancers{
				resources:         fakeResources,
				region:            "nyc1",
				lbActiveTimeout:   2,
				lbActiveCheckTick: 1,
			}

			// we don't actually use clusterName param in GetLoadBalancer
			lbStatus, exists, err := lb.GetLoadBalancer(context.TODO(), "test", test.service)
			if !reflect.DeepEqual(lbStatus, test.lbStatus) {
				t.Error("unexpected LB status")
				t.Logf("expected: %v", test.lbStatus)
				t.Logf("actual: %v", lbStatus)
			}

			if exists != test.exists {
				t.Error("unexpected LB existence")
				t.Logf("expected: %t", test.exists)
				t.Logf("actual: %t", exists)
			}

			if !reflect.DeepEqual(err, test.err) {
				t.Error("unexpected error")
				t.Logf("expected: %v", test.err)
				t.Logf("actual: %v", err)
			}

			if test.exists {
				svc, err := fakeResources.kclient.CoreV1().Services(test.service.Namespace).Get(test.service.Name, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("failed to get service from kube client: %s", err)
				}

				gotLoadBalancerID := svc.Annotations[annoDOLoadBalancerID]
				wantLoadBalancerID := "load-balancer-id"
				if gotLoadBalancerID != wantLoadBalancerID {
					t.Errorf("got load-balancer ID %q, want %q", gotLoadBalancerID, wantLoadBalancerID)
				}
			}
		})
	}
}

func Test_EnsureLoadBalancer(t *testing.T) {
	testcases := []struct {
		name              string
		droplets          []godo.Droplet
		getFn             func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error)
		listFn            func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error)
		createFn          func(context.Context, *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error)
		updateFn          func(ctx context.Context, lbID string, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error)
		service           *v1.Service
		newLoadBalancerID string
		nodes             []*v1.Node
		lbStatus          *v1.LoadBalancerStatus
		err               error
	}{
		{
			name: "successfully ensured loadbalancer by name, already exists",
			droplets: []godo.Droplet{
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
			},
			getFn: func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error) {
				return nil, newFakeNotOKResponse(), errors.New("get should not have been invoked")
			},
			listFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{*createLB()}, newFakeOKResponse(), nil
			},
			createFn: func(context.Context, *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
				return nil, newFakeNotOKResponse(), errors.New("create should not have been invoked")
			},
			updateFn: func(ctx context.Context, lbID string, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
				return createLB(), newFakeOKResponse(), nil
			},
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "foobar123",
					Annotations: map[string]string{
						annDOProtocol: "http",
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
			nodes: []*v1.Node{
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
			},
			lbStatus: &v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{
						IP: "10.0.0.1",
					},
				},
			},
			err: nil,
		},
		{
			name: "successfully ensured loadbalancer by ID, already exists",
			droplets: []godo.Droplet{
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
			},
			getFn: func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error) {
				return createLB(), newFakeOKResponse(), nil
			},
			listFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return nil, newFakeNotOKResponse(), errors.New("list should not have been invoked")
			},
			createFn: func(context.Context, *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
				return nil, newFakeNotOKResponse(), errors.New("create should not have been invoked")
			},
			updateFn: func(ctx context.Context, lbID string, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
				return createLB(), newFakeOKResponse(), nil
			},
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "foobar123",
					Annotations: map[string]string{
						annDOProtocol:        "http",
						annoDOLoadBalancerID: "load-balancer-id",
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
			nodes: []*v1.Node{
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
			},
			lbStatus: &v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{
						IP: "10.0.0.1",
					},
				},
			},
			err: nil,
		},
		{
			name: "successfully ensured loadbalancer by name that didn't exist",
			droplets: []godo.Droplet{
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
			},
			getFn: func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error) {
				return nil, newFakeNotOKResponse(), errors.New("get should not have been invoked")
			},
			listFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{}, newFakeOKResponse(), nil
			},
			createFn: func(context.Context, *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
				return createLB(), newFakeOKResponse(), nil
			},
			updateFn: func(ctx context.Context, lbID string, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
				return nil, newFakeNotOKResponse(), errors.New("update should not have been invoked")
			},
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "foobar123",
					Annotations: map[string]string{
						annDOProtocol: "http",
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
			nodes: []*v1.Node{
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
			},
			lbStatus: &v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{
						IP: "10.0.0.1",
					},
				},
			},
			err: nil,
		},
		{
			name: "successfully ensured loadbalancer by ID that didn't exist",
			droplets: []godo.Droplet{
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
			},
			getFn: func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error) {
				return nil, newFakeResponse(http.StatusNotFound), errors.New("LB not found")
			},
			listFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return nil, newFakeNotOKResponse(), errors.New("list should not have been invoked")
			},
			createFn: func(context.Context, *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
				lb := createLB()
				lb.ID = "other-load-balancer-id"
				return lb, newFakeOKResponse(), nil
			},
			updateFn: func(ctx context.Context, lbID string, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
				return nil, newFakeNotOKResponse(), errors.New("update should not have been invoked")
			},
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "foobar123",
					Annotations: map[string]string{
						annDOProtocol:        "http",
						annoDOLoadBalancerID: "load-balancer-id",
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
			newLoadBalancerID: "other-load-balancer-id",
			nodes: []*v1.Node{
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
			},
			lbStatus: &v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{
						IP: "10.0.0.1",
					},
				},
			},
			err: nil,
		},
		{
			name:     "failed to ensure existing load-balancer, state is non-active",
			droplets: []godo.Droplet{},
			listFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{*createLB()}, newFakeOKResponse(), nil
			},
			createFn: func(context.Context, *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
				return nil, newFakeNotOKResponse(), errors.New("create should not have been invoked")
			},
			updateFn: func(ctx context.Context, lbID string, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
				lb := createLB()
				lb.Status = lbStatusNew
				return lb, newFakeOKResponse(), nil
			},
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "foobar123",
					Annotations: map[string]string{
						annDOProtocol: "http",
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
			nodes: []*v1.Node{
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
			},
			lbStatus: nil,
			err:      utilerrors.NewAggregate([]error{fmt.Errorf("load-balancer is not yet active (current status: %s)", lbStatusNew)}),
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			fakeDroplet := &fakeDropletService{
				listFunc: func(context.Context, *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
					return test.droplets, newFakeOKResponse(), nil
				},
			}
			fakeLB := &fakeLBService{
				getFn:    test.getFn,
				listFn:   test.listFn,
				createFn: test.createFn,
				updateFn: test.updateFn,
			}
			certStore := make(map[string]*godo.Certificate)
			fakeCert := newKVCertService(certStore, true)
			fakeClient := newFakeClient(fakeDroplet, fakeLB, &fakeCert)
			fakeResources := newResources("", "", fakeClient)
			fakeResources.kclient = fake.NewSimpleClientset()
			if _, err := fakeResources.kclient.CoreV1().Services(test.service.Namespace).Create(test.service); err != nil {
				t.Fatalf("failed to add service to fake client: %s", err)
			}

			lb := &loadBalancers{
				resources:         fakeResources,
				region:            "nyc1",
				lbActiveTimeout:   2,
				lbActiveCheckTick: 1,
			}

			// clusterName param in EnsureLoadBalancer currently not used
			lbStatus, err := lb.EnsureLoadBalancer(context.TODO(), "test", test.service, test.nodes)
			if !reflect.DeepEqual(lbStatus, test.lbStatus) {
				t.Error("unexpected LB status")
				t.Logf("expected: %v", test.lbStatus)
				t.Logf("actual: %v", lbStatus)
			}

			if !reflect.DeepEqual(err, test.err) {
				t.Error("unexpected error")
				t.Logf("expected: %v", test.err)
				t.Logf("actual: %v", err)
			}

			if test.err == nil {
				svc, err := fakeResources.kclient.CoreV1().Services(test.service.Namespace).Get(test.service.Name, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("failed to get service from kube client: %s", err)
				}

				gotLoadBalancerID := svc.Annotations[annoDOLoadBalancerID]
				wantLoadBalancerID := "load-balancer-id"
				if test.newLoadBalancerID != "" {
					wantLoadBalancerID = test.newLoadBalancerID
				}
				if gotLoadBalancerID != wantLoadBalancerID {
					t.Errorf("got load-balancer ID %q, want %q", gotLoadBalancerID, wantLoadBalancerID)
				}
			}
		})
	}
}

func Test_EnsureLoadBalancerDeleted(t *testing.T) {
	lbName := "afoobar123"
	tests := []struct {
		name     string
		listFn   func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error)
		deleteFn func(ctx context.Context, lbID string) (*godo.Response, error)
		service  *v1.Service
		err      error
	}{
		{
			name: "retrieval failed",
			listFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return nil, newFakeNotOKResponse(), errors.New("API failed")
			},
			deleteFn: func(ctx context.Context, lbID string) (*godo.Response, error) {
				return newFakeNotOKResponse(), errors.New("delete should not have been invoked")
			},
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "foobar123",
				},
			},
			err: errors.New("API failed"),
		},
		{
			name: "load-balancer resource not found",
			listFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{}, newFakeOKResponse(), nil
			},
			deleteFn: func(ctx context.Context, lbID string) (*godo.Response, error) {
				return newFakeNotOKResponse(), errors.New("delete should not have been called")
			},
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "foobar123",
				},
			},
			err: nil,
		},
		{
			name: "delete failed",
			listFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{
					{
						// loadbalancer names are a + service.UID
						// see cloudprovider.DefaultLoadBalancerName
						Name:   lbName,
						IP:     "10.0.0.1",
						Status: lbStatusActive,
					},
				}, newFakeOKResponse(), nil
			},
			deleteFn: func(ctx context.Context, lbID string) (*godo.Response, error) {
				return newFakeNotOKResponse(), errors.New("API failed")
			},
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "foobar123",
				},
			},
			err: errors.New("failed to delete load-balancer: API failed"),
		},
		{
			name: "delete succeeded",
			listFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{
					{
						// loadbalancer names are a + service.UID
						// see cloudprovider.DefaultLoadBalancerName
						Name:   lbName,
						IP:     "10.0.0.1",
						Status: lbStatusActive,
					},
				}, newFakeOKResponse(), nil
			},
			deleteFn: func(ctx context.Context, lbID string) (*godo.Response, error) {
				return newFakeOKResponse(), nil
			},
			service: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "foobar123",
				},
			},
			err: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeLB := &fakeLBService{
				listFn:   test.listFn,
				deleteFn: test.deleteFn,
			}
			fakeClient := newFakeLBClient(fakeLB)
			fakeResources := newResources("", "", fakeClient)

			lb := &loadBalancers{
				resources:         fakeResources,
				region:            "nyc1",
				lbActiveTimeout:   2,
				lbActiveCheckTick: 1,
			}

			// clusterName param in EnsureLoadBalancer currently not used
			err := lb.EnsureLoadBalancerDeleted(context.Background(), "clusterName", test.service)
			if !reflect.DeepEqual(err, test.err) {
				t.Errorf("got error %q, want %q", err, test.err)
			}
		})
	}
}

func TestEnsureLoadBalancerIDAnnotation(t *testing.T) {
	tests := []struct {
		name string
		sut  func(l *loadBalancers, svc *v1.Service) error
	}{
		{
			name: "GetLoadBalancer",
			sut: func(l *loadBalancers, svc *v1.Service) error {
				_, _, err := l.GetLoadBalancer(context.Background(), "clusterName", svc)
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			svc := createLBSvc(1)
			lb := godo.LoadBalancer{
				// loadbalancer names are a + service.UID
				// see cloudprovider.DefaultLoadBalancerName
				ID:   "f7968b52-4ed9-4a16-af8b-304253f04e20",
				Name: getDefaultLoadBalancerName(svc),
				IP:   "10.0.0.1",
				// Status: lbStatusActive,
			}
			fakeLB := &fakeLBService{
				listFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
					return []godo.LoadBalancer{lb}, newFakeOKResponse(), nil
				},
			}
			fakeClient := newFakeLBClient(fakeLB)
			fakeResources := newResources("", "", fakeClient)
			// fakeResources.kclient = fake.NewSimpleClientset(svc)
			fakeResources.kclient = fake.NewSimpleClientset()
			if _, err := fakeResources.kclient.CoreV1().Services(v1.NamespaceDefault).Create(svc); err != nil {
				t.Fatalf("failed to add service to fake client: %s", err)
			}

			l := &loadBalancers{
				resources:         fakeResources,
				region:            "nyc1",
				lbActiveTimeout:   2,
				lbActiveCheckTick: 1,
			}

			err := test.sut(l, svc)
			if err != nil {
				t.Fatal(err)
			}

			svc, err = fakeResources.kclient.CoreV1().Services(v1.NamespaceDefault).Get(svc.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("failed to get service from kube client: %s", err)
			}

			gotLoadBalancerID := svc.Annotations[annoDOLoadBalancerID]
			wantLoadBalancerID := lb.ID
			if gotLoadBalancerID != wantLoadBalancerID {
				t.Errorf("got load-balancer ID %q, want %q", gotLoadBalancerID, wantLoadBalancerID)
			}
		})
	}
}
