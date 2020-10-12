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
	"context"
	"errors"
	"sync"

	"github.com/digitalocean/godo"
	"github.com/mitchellh/copystructure"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

var (
	ctx = context.TODO()

	kclient kubernetes.Interface

	inf = informers.NewSharedInformerFactory(kclient, 0)

	testFirewall = &godo.Firewall{
		ID:            "id",
		Name:          testWorkerFWName,
		InboundRules:  testInboundRules,
		OutboundRules: testOutboundRules,
		Tags:          testWorkerFWTags,
	}
	testDiffFirewall = &godo.Firewall{
		ID:            "other-id",
		Name:          "other-name",
		InboundRules:  testInboundRules,
		OutboundRules: testOutboundRules,
		Tags:          []string{"othertag1", "othertag2"},
	}

	testWorkerFWName  = "k8s-test-firewall"
	testWorkerFWTags  = []string{"tag1", "tag2"}
	testOutboundRules = []godo.OutboundRule{
		{
			Protocol:  "tcp",
			PortRange: "all",
			Destinations: &godo.Destinations{
				Addresses: []string{"0.0.0.0/0", "::/0"},
			},
		},
		{
			Protocol:  "udp",
			PortRange: "all",
			Destinations: &godo.Destinations{
				Addresses: []string{"0.0.0.0/0", "::/0"},
			},
		},
		{
			Protocol: "icmp",
			Destinations: &godo.Destinations{
				Addresses: []string{"0.0.0.0/0", "::/0"},
			},
		},
	}
	testDiffOutboundRules = []godo.OutboundRule{
		{
			Protocol:  "udp",
			PortRange: "all",
			Destinations: &godo.Destinations{
				Addresses: []string{"0.0.0.0/0", "::/0"},
			},
		},
	}
	testInboundRules     = []godo.InboundRule{fakeInboundRule}
	testDiffInboundRules = []godo.InboundRule{diffFakeInboundRule}

	fakeInboundRule = godo.InboundRule{
		Protocol:  "tcp",
		PortRange: "31000",
		Sources: &godo.Sources{
			Addresses: []string{"0.0.0.0/0", "::/0"},
		},
	}
	diffFakeInboundRule = godo.InboundRule{
		Protocol:  "tcp",
		PortRange: "32000",
		Sources: &godo.Sources{
			Addresses: []string{"0.0.0.0/0", "::/0"},
		},
	}
)

// fakeFirewallService satisfies the FirewallsService interface.
type fakeFirewallService struct {
	getFunc            func(context.Context, string) (*godo.Firewall, *godo.Response, error)
	createFunc         func(context.Context, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error)
	updateFunc         func(context.Context, string, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error)
	deleteFunc         func(context.Context, string) (*godo.Response, error)
	listFunc           func(context.Context, *godo.ListOptions) ([]godo.Firewall, *godo.Response, error)
	listByDropletFunc  func(context.Context, int, *godo.ListOptions) ([]godo.Firewall, *godo.Response, error)
	addDropletsFunc    func(context.Context, string, ...int) (*godo.Response, error)
	removeDropletsFunc func(context.Context, string, ...int) (*godo.Response, error)
	addTagsFunc        func(context.Context, string, ...string) (*godo.Response, error)
	removeTagsFunc     func(context.Context, string, ...string) (*godo.Response, error)
	addRulesFunc       func(context.Context, string, *godo.FirewallRulesRequest) (*godo.Response, error)
	removeRulesFunc    func(context.Context, string, *godo.FirewallRulesRequest) (*godo.Response, error)
}

// Get an existing Firewall by its identifier.
func (f *fakeFirewallService) Get(ctx context.Context, fID string) (*godo.Firewall, *godo.Response, error) {
	return f.getFunc(ctx, fID)
}

// Create a new Firewall with a given configuration.
func (f *fakeFirewallService) Create(ctx context.Context, fr *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
	return f.createFunc(ctx, fr)
}

// Update an existing Firewall with new configuration.
func (f *fakeFirewallService) Update(ctx context.Context, fID string, fr *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
	return f.updateFunc(ctx, fID, fr)
}

// Delete a Firewall by its identifier.
func (f *fakeFirewallService) Delete(ctx context.Context, fID string) (*godo.Response, error) {
	return f.deleteFunc(ctx, fID)
}

// List Firewalls.
func (f *fakeFirewallService) List(ctx context.Context, opt *godo.ListOptions) ([]godo.Firewall, *godo.Response, error) {
	return f.listFunc(ctx, opt)
}

// ListByDroplet Firewalls.
func (f *fakeFirewallService) ListByDroplet(ctx context.Context, dID int, opt *godo.ListOptions) ([]godo.Firewall, *godo.Response, error) {
	return f.listByDropletFunc(ctx, dID, opt)
}

// AddDroplets to a Firewall.
func (f *fakeFirewallService) AddDroplets(ctx context.Context, fID string, dropletIDs ...int) (*godo.Response, error) {
	return f.addDropletsFunc(ctx, fID, dropletIDs...)
}

// RemoveDroplets from a Firewall.
func (f *fakeFirewallService) RemoveDroplets(ctx context.Context, fID string, dropletIDs ...int) (*godo.Response, error) {
	return f.removeDropletsFunc(ctx, fID, dropletIDs...)
}

// AddTags to a Firewall.
func (f *fakeFirewallService) AddTags(ctx context.Context, fID string, tags ...string) (*godo.Response, error) {
	return f.addTagsFunc(ctx, fID, tags...)
}

// RemoveTags from a Firewall.
func (f *fakeFirewallService) RemoveTags(ctx context.Context, fID string, tags ...string) (*godo.Response, error) {
	return f.removeTagsFunc(ctx, fID, tags...)
}

// AddRules to a Firewall.
func (f *fakeFirewallService) AddRules(ctx context.Context, fID string, rr *godo.FirewallRulesRequest) (*godo.Response, error) {
	return f.addRulesFunc(ctx, fID, rr)
}

// RemoveRules from a Firewall.
func (f *fakeFirewallService) RemoveRules(ctx context.Context, fID string, rr *godo.FirewallRulesRequest) (*godo.Response, error) {
	return f.removeRulesFunc(ctx, fID, rr)
}

// createFakeFirewallService returns a fake firewall service with all CRUD
// operations not expected to be called and returning an error. The given
// override defines customizations.
func createFakeFirewallService(override fakeFirewallService) *fakeFirewallService {
	fake := &fakeFirewallService{
		getFunc: func(context.Context, string) (*godo.Firewall, *godo.Response, error) {
			return nil, newFakeNotOKResponse(), errors.New("get should not have been invoked")
		},
		listFunc: func(context.Context, *godo.ListOptions) ([]godo.Firewall, *godo.Response, error) {
			return nil, newFakeNotOKResponse(), errors.New("list should not have been invoked")
		},
		createFunc: func(context.Context, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
			return nil, newFakeNotOKResponse(), errors.New("create should not have been invoked")
		},
		updateFunc: func(context.Context, string, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
			return nil, newFakeNotOKResponse(), errors.New("update should not have been invoked")
		},
	}
	if override.getFunc != nil {
		fake.getFunc = override.getFunc
	}
	if override.listFunc != nil {
		fake.listFunc = override.listFunc
	}
	if override.createFunc != nil {
		fake.createFunc = override.createFunc
	}
	if override.updateFunc != nil {
		fake.updateFunc = override.updateFunc
	}
	return fake
}

type statefulFake struct {
	mu              sync.RWMutex
	createdFirewall *godo.Firewall
}

func (sf *statefulFake) listFunc(context.Context, *godo.ListOptions) ([]godo.Firewall, *godo.Response, error) {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	if sf.createdFirewall != nil {
		return []godo.Firewall{*sf.createdFirewall}, newFakeOKResponse(), nil
	}
	return nil, newFakeOKResponse(), nil
}

func (sf *statefulFake) getFunc(context.Context, string) (*godo.Firewall, *godo.Response, error) {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	return sf.createdFirewall, newFakeOKResponse(), nil
}

func (sf *statefulFake) createFunc(_ context.Context, fr *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	if sf.createdFirewall == nil {
		sf.createdFirewall = &godo.Firewall{
			ID:            "id",
			Name:          fr.Name,
			InboundRules:  fr.InboundRules,
			OutboundRules: fr.OutboundRules,
			Tags:          fr.Tags,
		}
	}
	return sf.createdFirewall, newFakeOKResponse(), nil
}

func newFakeFirewall() *godo.Firewall {
	return mustCopy(
		&godo.Firewall{
			ID:            "123",
			Name:          testWorkerFWName,
			InboundRules:  testInboundRules,
			OutboundRules: testOutboundRules,
			Tags:          testWorkerFWTags,
		},
	).(*godo.Firewall)
}

func newFakeFirewallCache() *firewallCache {
	return &firewallCache{
		firewall: newFakeFirewall(),
	}
}

func newFakeFirewallCacheEmpty() *firewallCache {
	return &firewallCache{}
}

func newFakeFirewallManager(client *godo.Client, cache *firewallCache) *firewallManager {
	return &firewallManager{
		client:             client,
		fwCache:            cache,
		workerFirewallName: testWorkerFWName,
		workerFirewallTags: testWorkerFWTags,
		metrics: metrics{
			host:               "localhost:8080",
			apiRequestDuration: apiRequestDuration,
			runLoopDuration:    runLoopDuration,
			reconcileDuration:  reconcileDuration,
		},
	}
}

func newFakeGodoClient(fakeFirewallSvc *fakeFirewallService) *godo.Client {
	return &godo.Client{
		Firewalls: fakeFirewallSvc,
	}
}

func mustCopy(v interface{}) interface{} {
	w, err := copystructure.Copy(v)
	return copystructure.Must(w, err)
}
