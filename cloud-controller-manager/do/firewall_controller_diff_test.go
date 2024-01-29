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
	"testing"

	"github.com/digitalocean/godo"
)

func TestCompFirewallsEqual(t *testing.T) {
	tests := []struct {
		name      string
		cf1       *comparableFirewall
		cf2       *comparableFirewall
		wantEqual bool
		wantDiff  bool
	}{
		{
			name:      "missing cf1 and cf2",
			cf1:       nil,
			cf2:       nil,
			wantEqual: true,
			wantDiff:  false,
		},
		{
			name:      "missing cf1",
			cf1:       nil,
			cf2:       &comparableFirewall{},
			wantEqual: false,
			wantDiff:  false,
		},
		{
			name:      "missing cf2",
			cf1:       &comparableFirewall{},
			cf2:       nil,
			wantEqual: false,
			wantDiff:  false,
		},
		{
			name: "simple equal",
			cf1: &comparableFirewall{
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			cf2: &comparableFirewall{
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			wantEqual: true,
			wantDiff:  false,
		},
		{
			name: "name mismatch",
			cf1: &comparableFirewall{
				Name:          "name1",
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			cf2: &comparableFirewall{
				Name:          "name2",
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			wantEqual: false,
			wantDiff:  true,
		},
		{
			name: "inbound rules mismatch",
			cf1: &comparableFirewall{
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			cf2: &comparableFirewall{
				Name:          testWorkerFWName,
				InboundRules:  testDiffInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			wantEqual: false,
			wantDiff:  true,
		},
		{
			name: "outbound rules mismatch",
			cf1: &comparableFirewall{
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			cf2: &comparableFirewall{
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testDiffOutboundRules,
				Tags:          testWorkerFWTags,
			},
			wantEqual: false,
			wantDiff:  true,
		},
		{
			name: "tags mismatch",
			cf1: &comparableFirewall{
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          []string{"tag1"},
			},
			cf2: &comparableFirewall{
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          []string{"tag2"},
			},
			wantEqual: false,
			wantDiff:  true,
		},
		{
			name: "ignore inbound rule order",
			cf1: &comparableFirewall{
				Name: testWorkerFWName,
				InboundRules: []godo.InboundRule{
					{
						Protocol:  "tcp",
						PortRange: "30000",
						Sources: &godo.Sources{
							Addresses: []string{"0.0.0.0/0", "::/0"},
						},
					},
					{
						Protocol:  "tcp",
						PortRange: "31337",
						Sources: &godo.Sources{
							Addresses: []string{"0.0.0.0/0", "::/0"},
						},
					},
				},
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			cf2: &comparableFirewall{
				Name: testWorkerFWName,
				InboundRules: []godo.InboundRule{
					{
						Protocol:  "tcp",
						PortRange: "31337",
						Sources: &godo.Sources{
							Addresses: []string{"0.0.0.0/0", "::/0"},
						},
					},
					{
						Protocol:  "tcp",
						PortRange: "30000",
						Sources: &godo.Sources{
							Addresses: []string{"0.0.0.0/0", "::/0"},
						},
					},
				},
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			wantEqual: true,
			wantDiff:  false,
		},
		{
			name: "ignore outbound rule order",
			cf1: &comparableFirewall{
				Name:         testWorkerFWName,
				InboundRules: testInboundRules,
				OutboundRules: []godo.OutboundRule{
					{
						Protocol:  "tcp",
						PortRange: "all",
						Destinations: &godo.Destinations{
							Addresses: []string{"0.0.0.0/0"},
						},
					},
					{
						Protocol:  "udp",
						PortRange: "all",
						Destinations: &godo.Destinations{
							Addresses: []string{"0.0.0.0/0"},
						},
					},
				},
				Tags: testWorkerFWTags,
			},
			cf2: &comparableFirewall{
				Name:         testWorkerFWName,
				InboundRules: testInboundRules,
				OutboundRules: []godo.OutboundRule{
					{
						Protocol:  "udp",
						PortRange: "all",
						Destinations: &godo.Destinations{
							Addresses: []string{"0.0.0.0/0"},
						},
					},
					{
						Protocol:  "tcp",
						PortRange: "all",
						Destinations: &godo.Destinations{
							Addresses: []string{"0.0.0.0/0"},
						},
					},
				},
				Tags: testWorkerFWTags,
			},
			wantEqual: true,
			wantDiff:  false,
		},
		{
			name: "ignore emptiness flavors",
			cf1: &comparableFirewall{
				Name:          testWorkerFWName,
				InboundRules:  nil,
				OutboundRules: nil,
				Tags:          nil,
			},
			cf2: &comparableFirewall{
				Name:          testWorkerFWName,
				InboundRules:  []godo.InboundRule{},
				OutboundRules: []godo.OutboundRule{},
				Tags:          []string{},
			},
			wantEqual: true,
			wantDiff:  false,
		},
		{
			name: "PortRange sanitization",
			cf1: &comparableFirewall{
				Name:         testWorkerFWName,
				InboundRules: testInboundRules,
				OutboundRules: []godo.OutboundRule{
					{
						Protocol:  "tcp",
						PortRange: "all",
						Destinations: &godo.Destinations{
							Addresses: []string{"0.0.0.0/0"},
						},
					},
					{
						Protocol:  "udp",
						PortRange: "0",
						Destinations: &godo.Destinations{
							Addresses: []string{"0.0.0.0/0"},
						},
					},
					{
						Protocol:  "icmp",
						PortRange: "",
						Destinations: &godo.Destinations{
							Addresses: []string{"0.0.0.0/0"},
						},
					},
				},
				Tags: testWorkerFWTags,
			},
			cf2: &comparableFirewall{
				Name:         testWorkerFWName,
				InboundRules: testInboundRules,
				OutboundRules: []godo.OutboundRule{
					{
						Protocol:  "udp",
						PortRange: "0",
						Destinations: &godo.Destinations{
							Addresses: []string{"0.0.0.0/0"},
						},
					},
					{
						Protocol:  "tcp",
						PortRange: "0",
						Destinations: &godo.Destinations{
							Addresses: []string{"0.0.0.0/0"},
						},
					},
					{
						Protocol:  "icmp",
						PortRange: "0",
						Destinations: &godo.Destinations{
							Addresses: []string{"0.0.0.0/0"},
						},
					},
				},
				Tags: testWorkerFWTags,
			},
			wantEqual: true,
			wantDiff:  false,
		},
		{
			name: "ignore non-addresses part of outbound rule sources and destinations",
			cf1: &comparableFirewall{
				Name: testWorkerFWName,
				InboundRules: []godo.InboundRule{
					{
						Protocol:  "tcp",
						PortRange: "31000",
						Sources: &godo.Sources{
							Addresses:  []string{"0.0.0.0/0", "::/0"},
							DropletIDs: []int{23, 42},
						},
					},
				},
				OutboundRules: []godo.OutboundRule{
					{
						Protocol:  "tcp",
						PortRange: "all",
						Destinations: &godo.Destinations{
							Addresses: []string{"0.0.0.0/0"},
							Tags:      []string{"tag1", "tag2"},
						},
					},
				},
				Tags: testWorkerFWTags,
			},
			cf2: &comparableFirewall{
				Name: testWorkerFWName,
				InboundRules: []godo.InboundRule{
					{
						Protocol:  "tcp",
						PortRange: "31000",
						Sources: &godo.Sources{
							Addresses:  []string{"0.0.0.0/0", "::/0"},
							DropletIDs: []int{1337},
						},
					},
				},
				OutboundRules: []godo.OutboundRule{
					{
						Protocol:  "tcp",
						PortRange: "all",
						Destinations: &godo.Destinations{
							Addresses:        []string{"0.0.0.0/0"},
							LoadBalancerUIDs: []string{"lb1", "lb2"},
						},
					},
				},
				Tags: testWorkerFWTags,
			},
			wantEqual: true,
			wantDiff:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotEqual, diff := compFirewallsEqual(test.cf1, test.cf2)
			if gotEqual != test.wantEqual {
				t.Errorf("got equal %v, want %v", gotEqual, test.wantEqual)
			}

			gotDiff := diff != ""
			if gotDiff != test.wantDiff {
				t.Errorf("got diff %q, want one: %v", diff, test.wantDiff)
			}
		})
	}
}
