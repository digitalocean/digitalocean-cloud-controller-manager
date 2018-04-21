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
	"github.com/digitalocean/godo"
	"testing"
)

func Test_checkIfPortInRange(t *testing.T) {
	testcases := []struct {
		name      string
		portRange string
		port      int
		result    bool
	}{
		{
			"Port in range",
			"100-200",
			150,
			true,
		},
		{
			"Port not in range",
			"100-200",
			250,
			false,
		},
		{
			"Return true if all specified",
			"all",
			250,
			true,
		},
		{
			"Return true if single port match",
			"250",
			250,
			true,
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			result := checkIfPortInRange(testcase.port, testcase.portRange)
			if result != testcase.result {
				t.Errorf("actual result of check port %d in range %s is: %t", testcase.port, testcase.portRange, result)
				t.Errorf("expected result: %t", testcase.result)
				t.Error("unexpected check if port is in range result")
			}
		})
	}

}

func Test_checkIfPortRequiresDeletion(t *testing.T) {
	testcases := []struct {
		name      string
		portRange string
		port      int
		result    bool
	}{
		{
			"Port is in a range",
			"100-200",
			150,
			false,
		},
		{
			"Port not in range",
			"100-200",
			250,
			false,
		},
		{
			"Return false if all specified",
			"all",
			250,
			false,
		},
		{
			"Single port match",
			"250",
			250,
			true,
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			result := checkIfPortRequiresDeletion(testcase.port, testcase.portRange)
			if result != testcase.result {
				t.Errorf("actual result of check port %d requires deletion %s is: %t", testcase.port, testcase.portRange, result)
				t.Errorf("expected result: %t", testcase.result)
				t.Error("unexpected check if port requires deletion result")
			}
		})
	}

}

func Test_lbfwRuleExists(t *testing.T) {
	testcases := []struct {
		name           string
		lbID           string
		forwardingRule godo.ForwardingRule
		firewall       []godo.Firewall
		checkFunc      func(port int, fwPortRange string) bool
		result         bool
	}{
		{
			"Firewall rule does not exist for load balancer forward port",
			"4de7ac8b-495b-4884-9a69-1050c6793cd6",
			godo.ForwardingRule{
				EntryProtocol:  "tcp",
				EntryPort:      80,
				TargetProtocol: "tcp",
				//A forwarding rule exists for port 30000
				TargetPort:     30000,
				CertificateID:  "",
				TlsPassthrough: false,
			},
			[]godo.Firewall{
				{
					ID: "fb6045f1-cf1d-4ca3-bfac-18832663025b",
					InboundRules: []godo.InboundRule{
						{
							Protocol: "tcp",
							//An existing rule exists allowing port 123 through
							PortRange: "123",
							Sources: &godo.Sources{
								LoadBalancerUIDs: []string{
									"4de7ac8b-495b-4884-9a69-1050c6793cd6",
								},
							},
						},
					},
				},
			},
			checkIfPortInRange,
			//A rule does not exist for port 30000 so we expect a result of false
			false,
		},
		{
			"Firewall rule exists for load balancer forward port",
			"4de7ac8b-495b-4884-9a69-1050c6793cd6",
			godo.ForwardingRule{
				EntryProtocol:  "tcp",
				EntryPort:      80,
				TargetProtocol: "tcp",
				//A forwarding rule exists for port 30000
				TargetPort:     30000,
				CertificateID:  "",
				TlsPassthrough: false,
			},
			[]godo.Firewall{
				{
					ID: "fb6045f1-cf1d-4ca3-bfac-18832663025b",
					InboundRules: []godo.InboundRule{
						{
							Protocol: "tcp",
							//An existing rule exists allowing port 30000 through
							PortRange: "30000",
							Sources: &godo.Sources{
								LoadBalancerUIDs: []string{
									"4de7ac8b-495b-4884-9a69-1050c6793cd6",
								},
							},
						},
					},
				},
			},
			checkIfPortInRange,
			//A rule does exist for port 30000 so we expect a result of true
			true,
		},
		{
			"Firewall rule is covered by a port range for load balancer forward port",
			"4de7ac8b-495b-4884-9a69-1050c6793cd6",
			godo.ForwardingRule{
				EntryProtocol:  "tcp",
				EntryPort:      80,
				TargetProtocol: "tcp",
				//A forwarding rule exists for port 30000
				TargetPort:     30000,
				CertificateID:  "",
				TlsPassthrough: false,
			},
			[]godo.Firewall{
				{
					ID: "fb6045f1-cf1d-4ca3-bfac-18832663025b",
					InboundRules: []godo.InboundRule{
						{
							Protocol: "tcp",
							//An existing rule exists allowing a range of ports through
							PortRange: "30000-30050",
							Sources: &godo.Sources{
								LoadBalancerUIDs: []string{
									"4de7ac8b-495b-4884-9a69-1050c6793cd6",
								},
							},
						},
					},
				},
			},
			checkIfPortInRange,
			//A rule exists within a range allowing the port through
			true,
		},
		{
			"Firewall rule does not require deletion for load balancer forward port",
			"4de7ac8b-495b-4884-9a69-1050c6793cd6",
			godo.ForwardingRule{
				EntryProtocol:  "tcp",
				EntryPort:      80,
				TargetProtocol: "tcp",
				//A forwarding rule exists for port 30000
				TargetPort:     30000,
				CertificateID:  "",
				TlsPassthrough: false,
			},
			[]godo.Firewall{
				{
					ID: "fb6045f1-cf1d-4ca3-bfac-18832663025b",
					InboundRules: []godo.InboundRule{
						{
							Protocol: "tcp",
							//An existing rule exists allowing port 123 through
							PortRange: "123",
							Sources: &godo.Sources{
								LoadBalancerUIDs: []string{
									"4de7ac8b-495b-4884-9a69-1050c6793cd6",
								},
							},
						},
					},
				},
			},
			checkIfPortRequiresDeletion,
			//A rule does not exist for port 30000 and so no rule requires deletion
			false,
		},
		{
			"Firewall rule requires deletion for load balancer forward port",
			"4de7ac8b-495b-4884-9a69-1050c6793cd6",
			godo.ForwardingRule{
				EntryProtocol:  "tcp",
				EntryPort:      80,
				TargetProtocol: "tcp",
				//A forwarding rule exists for port 30000
				TargetPort:     30000,
				CertificateID:  "",
				TlsPassthrough: false,
			},
			[]godo.Firewall{
				{
					ID: "fb6045f1-cf1d-4ca3-bfac-18832663025b",
					InboundRules: []godo.InboundRule{
						{
							Protocol: "tcp",
							//An  rule exists allowing port 30000 through
							PortRange: "30000",
							Sources: &godo.Sources{
								LoadBalancerUIDs: []string{
									"4de7ac8b-495b-4884-9a69-1050c6793cd6",
								},
							},
						},
					},
				},
			},
			checkIfPortRequiresDeletion,
			//A rule does exist for port 30000 which requires deletion so we expect a result of true
			true,
		},
		{
			"Firewall rule exists for range no deletion required for load balancer forward port",
			"4de7ac8b-495b-4884-9a69-1050c6793cd6",
			godo.ForwardingRule{
				EntryProtocol:  "tcp",
				EntryPort:      80,
				TargetProtocol: "tcp",
				//A forwarding rule exists for port 30000
				TargetPort:     30000,
				CertificateID:  "",
				TlsPassthrough: false,
			},
			[]godo.Firewall{
				{
					ID: "fb6045f1-cf1d-4ca3-bfac-18832663025b",
					InboundRules: []godo.InboundRule{
						{
							Protocol: "tcp",
							//An  rule exists allowing port 30000 through
							PortRange: "30000-30050",
							Sources: &godo.Sources{
								LoadBalancerUIDs: []string{
									"4de7ac8b-495b-4884-9a69-1050c6793cd6",
								},
							},
						},
					},
				},
			},
			checkIfPortRequiresDeletion,
			//A rule does exist for port 30000 but it lies within a range
			//therefore no rule requres deletion
			false,
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			result := lbfwRuleExists(testcase.lbID, testcase.forwardingRule, testcase.firewall, testcase.checkFunc)
			if result != testcase.result {
				t.Errorf("actual result of check firewall rule required is: %t", result)
				t.Errorf("expected result: %t", testcase.result)
				t.Error("unexpected check if rule is required for forwarding rule result")
			}
		})
	}

}
