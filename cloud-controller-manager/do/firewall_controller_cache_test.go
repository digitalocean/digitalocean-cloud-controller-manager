/*
Copyright 2020 DigitalOcean

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

func TestUpdateCache(t *testing.T) {
	tests := []struct {
		name           string
		cachedFirewall *godo.Firewall
		curFirewall    *godo.Firewall
	}{
		{
			name:           "no cached firewall",
			cachedFirewall: nil,
			curFirewall:    testFirewall,
		},
		{
			name:           "non-matching cached firewall",
			cachedFirewall: testFirewall,
			curFirewall:    testDiffFirewall,
		},
		{
			name:           "matching cached firewall",
			cachedFirewall: testFirewall,
			curFirewall:    testFirewall,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cache := firewallCache{
				firewall: test.cachedFirewall,
			}

			cache.updateCache(test.curFirewall)
			if !cache.isSet {
				t.Error("cache is not set")
			}
			if cache.firewall != test.curFirewall {
				t.Errorf("got firewall %s, want %s", printRelevantFirewallParts(cache.firewall), printRelevantFirewallParts(test.curFirewall))
			}
		})
	}
}
