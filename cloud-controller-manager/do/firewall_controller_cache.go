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
	"sync"

	"github.com/digitalocean/godo"
	"k8s.io/klog/v2"
)

// firewallCache stores a cached firewall and mutex to handle concurrent access.
type firewallCache struct {
	sync.RWMutex
	firewall *godo.Firewall
	isSet    bool
}

func (fc *firewallCache) getCachedFirewall() (*godo.Firewall, bool) {
	fc.RLock()
	defer fc.RUnlock()
	return fc.firewall, fc.isSet
}

func (fc *firewallCache) updateCache(currentFirewall *godo.Firewall) {
	fc.Lock()
	fc.isSet = true
	if isEqual, _ := firewallsEqual(fc.firewall, currentFirewall); !isEqual {
		klog.V(5).Infof("updated firewall cache to: %s", printRelevantFirewallParts(currentFirewall))
		fc.firewall = currentFirewall
	} else {
		klog.V(6).Infof("not updating firewall cache which already contains: %s", printRelevantFirewallParts(currentFirewall))
	}
	fc.Unlock()
}
