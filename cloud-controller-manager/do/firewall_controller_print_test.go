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

func TestPrint(t *testing.T) {
	in := &godo.Firewall{
		ID:   "id",
		Name: "firewall",
		InboundRules: []godo.InboundRule{
			{
				Protocol:  "tcp",
				PortRange: "30000",
				Sources: &godo.Sources{
					Addresses:        []string{"0.0.0.0"},
					LoadBalancerUIDs: []string{"irrelevant-lb1"},
				},
			},
		},
		OutboundRules: []godo.OutboundRule{
			{
				Protocol:  "tcp",
				PortRange: "all",
				Destinations: &godo.Destinations{
					Addresses:        []string{"0.0.0.0"},
					LoadBalancerUIDs: []string{"irrelevant-lb2"},
				},
			},
			{
				Protocol:  "udp",
				PortRange: "all",
				Destinations: &godo.Destinations{
					Addresses:        []string{"0.0.0.0"},
					LoadBalancerUIDs: []string{"irrelevant-lb3"},
				},
			},
		},
		Tags: []string{"tag1", "tag2"},
	}

	want := "Name:firewall inRules:[<Proto:tcp PortRange:30000 AddrSources:[0.0.0.0]>] outRules:[<Proto:tcp PortRange:0 AddrDestinations:[0.0.0.0]> <Proto:udp PortRange:0 AddrDestinations:[0.0.0.0]>] Tags:[tag1 tag2]"

	if got := printRelevantFirewallParts(in); got != want {
		t.Errorf("\ngot:  %q\nwant: %q", got, want)
	}
}
