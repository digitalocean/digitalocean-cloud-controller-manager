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

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/cloudprovider"
)

var _ cloudprovider.LoadBalancer = new(loadbalancers)

type fakeLBService struct {
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

func newFakeLBClient(fakeLB *fakeLBService, fakeDroplet *fakeDropletService) *godo.Client {
	client := godo.NewClient(nil)
	client.Droplets = fakeDroplet
	client.LoadBalancers = fakeLB

	return client
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
			"Service annotatiosn nil",
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

func Test_getTLSPorts(t *testing.T) {
	testcases := []struct {
		name     string
		service  *v1.Service
		tlsPorts []int
		err      error
	}{
		{
			"tls port specified",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOTLSPorts: "443",
					},
				},
			},
			[]int{443},
			nil,
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			tlsPorts, err := getTLSPorts(test.service)
			if !reflect.DeepEqual(tlsPorts, test.tlsPorts) {
				t.Error("unexpected TLS ports")
				t.Logf("expected %v", test.tlsPorts)
				t.Logf("actual: %v", tlsPorts)
			}

			if !reflect.DeepEqual(err, test.err) {
				t.Error("unexpected error")
				t.Logf("expected: %v", test.err)
				t.Logf("actual: %v", err)
			}
		})
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
			errors.New("either certificate id should be set or tls pass through enabled, not both"),
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
			errors.New("must set certificate id or enable tls pass through"),
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
		name        string
		service     *v1.Service
		healthcheck *godo.HealthCheck
		err         error
	}{
		{
			"tcp health check",
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
			&godo.HealthCheck{
				Protocol:               "tcp",
				Port:                   30000,
				CheckIntervalSeconds:   3,
				ResponseTimeoutSeconds: 5,
				HealthyThreshold:       5,
				UnhealthyThreshold:     3,
			},
			nil,
		},
		{
			"http health check",
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
			&godo.HealthCheck{
				Protocol:               "http",
				Port:                   30000,
				CheckIntervalSeconds:   3,
				ResponseTimeoutSeconds: 5,
				HealthyThreshold:       5,
				UnhealthyThreshold:     3,
			},
			nil,
		},
		{
			"invalid protocol health check",
			&v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "abc123",
					Annotations: map[string]string{
						annDOProtocol: "invalid",
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
			nil,
			fmt.Errorf("invalid protocol: %q specified in annotation: %q", "invalid", annDOProtocol),
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			healthcheck, err := buildHealthCheck(test.service)
			if !reflect.DeepEqual(healthcheck, test.healthcheck) {
				t.Error("unexpected health check")
				t.Logf("expected: %v", test.healthcheck)
				t.Logf("actual: %v", healthcheck)
			}

			if !reflect.DeepEqual(err, test.err) {
				t.Error("unexpected error")
				t.Logf("expected: %v", test.err)
				t.Logf("actual: %v", err)
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

func Test_buildLoadBalancerRequest(t *testing.T) {
	testcases := []struct {
		name          string
		dropletListFn func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error)
		service       *v1.Service
		nodes         []*v1.Node
		lbr           *godo.LoadBalancerRequest
		err           error
	}{
		{
			"successful load balancer request",
			func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
				return []godo.Droplet{
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
				}, newFakeOKResponse(), nil
			},
			&v1.Service{
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
				HealthCheck: &godo.HealthCheck{
					Protocol:               "http",
					Port:                   30000,
					CheckIntervalSeconds:   3,
					ResponseTimeoutSeconds: 5,
					HealthyThreshold:       5,
					UnhealthyThreshold:     3,
				},
				Algorithm: "round_robin",
				StickySessions: &godo.StickySessions{
					Type: "none",
				},
			},
			nil,
		},
		{
			"successful load balancer request using least_connections algorithm",
			func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
				return []godo.Droplet{
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
				}, newFakeOKResponse(), nil
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
				HealthCheck: &godo.HealthCheck{
					Protocol:               "http",
					Port:                   30000,
					CheckIntervalSeconds:   3,
					ResponseTimeoutSeconds: 5,
					HealthyThreshold:       5,
					UnhealthyThreshold:     3,
				},
				Algorithm: "least_connections",
				StickySessions: &godo.StickySessions{
					Type: "none",
				},
			},
			nil,
		},
		{
			"successful load balancer request with cookies sticky sessions.",
			func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
				return []godo.Droplet{
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
				}, newFakeOKResponse(), nil
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
				HealthCheck: &godo.HealthCheck{
					Protocol:               "http",
					Port:                   30000,
					CheckIntervalSeconds:   3,
					ResponseTimeoutSeconds: 5,
					HealthyThreshold:       5,
					UnhealthyThreshold:     3,
				},
				Algorithm: "round_robin",
				StickySessions: &godo.StickySessions{
					Type:             "cookies",
					CookieName:       "DO-CCM",
					CookieTtlSeconds: 300,
				},
			},
			nil,
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			fakeDroplet := &fakeDropletService{}
			fakeDroplet.listFunc = test.dropletListFn
			fakeClient := newFakeLBClient(&fakeLBService{}, fakeDroplet)

			lb := &loadbalancers{fakeClient, "nyc3", 2, 1}

			lbr, err := lb.buildLoadBalancerRequest(test.service, test.nodes)

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

func Test_nodeToDropletIDs(t *testing.T) {
	testcases := []struct {
		name          string
		nodes         []*v1.Node
		dropletListFn func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error)
		dropletIDs    []int
		err           error
	}{
		{
			"node to droplet ids",
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
			func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
				return []godo.Droplet{
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
				}, newFakeOKResponse(), nil
			},
			[]int{100, 101, 102},
			nil,
		},
		{
			"error from DO API",
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
			func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
				return nil, nil, errors.New("badness")
			},
			nil,
			errors.New("badness"),
		},
		{
			"node to droplet ID with droplets not in cluster",
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
			func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
				return []godo.Droplet{
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
				}, newFakeOKResponse(), nil
			},
			[]int{100, 101, 102},
			nil,
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			fakeDroplet := &fakeDropletService{}
			fakeDroplet.listFunc = test.dropletListFn
			fakeClient := newFakeLBClient(&fakeLBService{}, fakeDroplet)

			lb := &loadbalancers{fakeClient, "nyc1", 2, 1}
			dropletIDs, err := lb.nodesToDropletIDs(test.nodes)
			if !reflect.DeepEqual(dropletIDs, test.dropletIDs) {
				t.Error("unexpected droplet IDs")
				t.Logf("expected: %v", test.dropletIDs)
				t.Logf("actual: %v", dropletIDs)
			}

			if !reflect.DeepEqual(err, test.err) {
				t.Error("unexpected error")
				t.Logf("expected: %v", test.err)
				t.Logf("actual: %v", err)
			}

		})
	}
}

func Test_lbByName(t *testing.T) {
	testcases := []struct {
		name         string
		lbName       string
		loadbalancer *godo.LoadBalancer
		listFn       func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error)
		err          error
	}{
		{
			"load balancer found",
			"lb-0",
			&godo.LoadBalancer{
				Name: "lb-0",
			},
			func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{
					{
						Name: "lb-0",
					},
				}, nil, nil
			},
			nil,
		},
		{
			"DO API returns error",
			"lb-0",
			nil,
			func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return nil, nil, errors.New("badness")
			},
			errors.New("badness"),
		},
		{
			"Loadbalancer not found",
			"lb-0",
			nil,
			func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{
					{
						Name: "lb-1",
					},
				}, nil, nil

			},
			errLBNotFound,
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			fakeLB := &fakeLBService{}
			fakeLB.listFn = test.listFn
			fakeClient := newFakeLBClient(fakeLB, &fakeDropletService{})

			lb := &loadbalancers{fakeClient, "nyc1", 2, 1}
			loadbalancer, err := lb.lbByName(context.TODO(), test.lbName)

			if !reflect.DeepEqual(loadbalancer, test.loadbalancer) {
				t.Error("unexpected DO loadbalancer")
				t.Logf("expected: %v", test.loadbalancer)
				t.Logf("actual: %v", loadbalancer)
			}

			if !reflect.DeepEqual(err, test.err) {
				t.Error("unexpected error")
				t.Logf("expected: %v", test.err)
				t.Logf("actual: %v", err)
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
			"got loadbalancer",
			func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error) {
				return nil, nil, nil
			},
			func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{
					{
						// loadbalancer names are a + service.UID
						// see cloudprovider.GetLoadBalancerName
						Name:   "afoobar123",
						IP:     "10.0.0.1",
						Status: lbStatusActive,
					},
				}, nil, nil
			},
			&v1.Service{
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
			&v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{
						IP: "10.0.0.1",
					},
				},
			},
			true,
			nil,
		},
		{
			"loadbalancer not found",
			func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error) {
				return nil, nil, nil
			},
			func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{
					{
						// loadbalancer names are a + service.UID
						// see cloudprovider.GetLoadBalancerName
						Name:   "anotherLB",
						IP:     "10.0.0.1",
						Status: lbStatusActive,
					},
				}, nil, nil
			},
			&v1.Service{
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
			nil,
			false,
			nil,
		},
		{
			"DO API returned error",
			func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error) {
				return nil, nil, nil
			},
			func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return nil, nil, errors.New("badness")
			},
			&v1.Service{
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
			nil,
			false,
			errors.New("badness"),
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			fakeLB := &fakeLBService{}
			fakeLB.listFn = test.listFn
			fakeClient := newFakeLBClient(fakeLB, &fakeDropletService{})

			lb := &loadbalancers{fakeClient, "nyc1", 2, 1}

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

		})
	}
}

func Test_EnsureLoadBalancer(t *testing.T) {
	testcases := []struct {
		name          string
		getFn         func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error)
		dropletListFn func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error)
		listFn        func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error)
		createFn      func(context.Context, *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error)
		updateFn      func(ctx context.Context, lbID string, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error)
		service       *v1.Service
		nodes         []*v1.Node
		lbStatus      *v1.LoadBalancerStatus
		err           error
	}{
		{
			"successfully ensured loadbalancer, already exists",
			func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error) {
				return &godo.LoadBalancer{
					// loadbalancer names are a + service.UID
					// see cloudprovider.GetLoadBalancerName
					Name:   "afoobar123",
					IP:     "10.0.0.1",
					Status: lbStatusActive,
				}, newFakeOKResponse(), nil
			},
			func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
				return []godo.Droplet{
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
				}, newFakeOKResponse(), nil
			},
			func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{
					{
						// loadbalancer names are a + service.UID
						// see cloudprovider.GetLoadBalancerName
						Name:   "afoobar123",
						IP:     "10.0.0.1",
						Status: lbStatusActive,
					},
				}, newFakeOKResponse(), nil
			},
			func(context.Context, *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
				// shouldn't be run in this test case
				return nil, nil, nil
			},
			func(ctx context.Context, lbID string, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
				return &godo.LoadBalancer{
					// loadbalancer names are a + service.UID
					// see cloudprovider.GetLoadBalancerName
					Name:   "afoobar123",
					IP:     "10.0.0.1",
					Status: lbStatusActive,
				}, newFakeOKResponse(), nil
			},
			&v1.Service{
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
			&v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{
						IP: "10.0.0.1",
					},
				},
			},
			nil,
		},
		{
			"successfully ensured loadbalancer that didn't exist",
			func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error) {
				return &godo.LoadBalancer{
					Name:   "afoobar123",
					IP:     "10.0.0.1",
					Status: lbStatusActive,
				}, newFakeOKResponse(), nil
			},
			func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
				return []godo.Droplet{
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
				}, newFakeOKResponse(), nil
			},
			func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				return []godo.LoadBalancer{}, nil, nil
			},
			func(context.Context, *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
				return &godo.LoadBalancer{
					Name:   "afoobar123",
					IP:     "10.0.0.1",
					Status: lbStatusActive,
				}, newFakeOKResponse(), nil
			},
			func(ctx context.Context, lbID string, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
				// should not be run in this test case
				return nil, nil, nil
			},
			&v1.Service{
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
			&v1.LoadBalancerStatus{
				Ingress: []v1.LoadBalancerIngress{
					{
						IP: "10.0.0.1",
					},
				},
			},
			nil,
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			fakeLB := &fakeLBService{
				listFn:   test.listFn,
				createFn: test.createFn,
				updateFn: test.updateFn,
				getFn:    test.getFn,
			}
			fakeDroplet := &fakeDropletService{
				listFunc: test.dropletListFn,
			}
			fakeClient := newFakeLBClient(fakeLB, fakeDroplet)

			lb := &loadbalancers{fakeClient, "nyc1", 2, 1}

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
		})
	}
}

func Test_waitActive(t *testing.T) {
	testcases := []struct {
		name     string
		getFn    func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error)
		lbStatus *godo.LoadBalancer
		err      error
	}{
		{
			"balancer active",
			func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error) {
				return &godo.LoadBalancer{
					Status: lbStatusActive,
				}, nil, nil
			},
			&godo.LoadBalancer{
				Status: lbStatusActive,
			},
			nil,
		},
		{
			"balancer error",
			func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error) {
				return &godo.LoadBalancer{
					ID:     "lb1",
					Status: lbStatusErrored,
				}, newFakeOKResponse(), nil
			},
			nil,
			errors.New("error creating DigitalOcean balancer: \"lb1\""),
		},
		{
			"balancer retrieve error",
			func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error) {
				return nil, nil, errors.New("balancer retrieve error")
			},
			nil,
			errors.New("balancer retrieve error"),
		},
		{
			"balancer timeout error",
			func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error) {
				return &godo.LoadBalancer{
					ID:     "lb1",
					Status: lbStatusNew,
				}, newFakeOKResponse(), nil
			},
			nil,
			errors.New("load balancer creation for \"lb1\" timed out"),
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			fakeLB := &fakeLBService{
				getFn: test.getFn,
			}

			fakeClient := newFakeLBClient(fakeLB, nil)

			lb := &loadbalancers{fakeClient, "nyc1", 2, 1}

			lbStatus, err := lb.waitActive("lb1")
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
		})
	}
}
