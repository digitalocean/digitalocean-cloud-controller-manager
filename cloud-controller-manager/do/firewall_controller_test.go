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
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

var (
	ctx = context.TODO()

	kclient kubernetes.Interface

	inf = informers.NewSharedInformerFactory(kclient, 0)

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
			Tags:       []string{"tag"},
			DropletIDs: []int{1},
		},
	}
	diffFakeInboundRule = godo.InboundRule{
		Protocol:  "tcp",
		PortRange: "32000",
		Sources: &godo.Sources{
			Tags:       []string{"tag"},
			DropletIDs: []int{1, 2},
		},
	}

	fwManager firewallManager
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
	return f.addDropletsFunc(ctx, fID)
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

func newFakeFirewall() *godo.Firewall {
	return &godo.Firewall{
		ID:            "123",
		Name:          testWorkerFWName,
		Tags:          testWorkerFWTags,
		InboundRules:  testInboundRules,
		OutboundRules: testOutboundRules,
	}
}

func newFakeFirewallWithDiffInboundRules() *godo.Firewall {
	return &godo.Firewall{
		ID:            "123",
		Name:          testWorkerFWName,
		Tags:          testWorkerFWTags,
		InboundRules:  testDiffInboundRules,
		OutboundRules: testOutboundRules,
	}
}

func newFakeFirewallCache() firewallCache {
	return firewallCache{
		mu:       new(sync.RWMutex),
		firewall: newFakeFirewall(),
	}
}

func newFakeFirewallCacheWithDiffInboundRules() firewallCache {
	return firewallCache{
		mu:       new(sync.RWMutex),
		firewall: newFakeFirewallWithDiffInboundRules(),
	}
}

func newFakeFirewallCacheEmpty() firewallCache {
	return firewallCache{
		mu: new(sync.RWMutex),
	}
}

func newFakeFirewallManager(client *godo.Client, cache firewallCache) firewallManager {
	return firewallManager{
		client:             client,
		fwCache:            cache,
		workerFirewallName: testWorkerFWName,
		workerFirewallTags: testWorkerFWTags,
	}
}

func newFakeGodoClient(fakeFirewall *fakeFirewallService) *godo.Client {
	return &godo.Client{
		Firewalls: fakeFirewall,
	}
}

func TestFirewallController_Get(t *testing.T) {
	testcases := []struct {
		name                         string
		fwCache                      firewallCache
		expectedGodoFirewallGetResp  func(context.Context, string) (*godo.Firewall, *godo.Response, error)
		expectedGodoFirewallListResp func(context.Context, *godo.ListOptions) ([]godo.Firewall, *godo.Response, error)
		expectedError                error
		expectedFirewall             *godo.Firewall
	}{
		{
			name:    "return error when error on GET firewall by ID",
			fwCache: newFakeFirewallCache(),
			expectedGodoFirewallGetResp: func(context.Context, string) (*godo.Firewall, *godo.Response, error) {
				return nil, newFakeNotOKResponse(), errors.New("failed to retrieve firewall by ID")
			},
			expectedError: errors.New("failed to retrieve firewall by ID"),
		},
		{
			name:    "return error when error on List firewalls",
			fwCache: newFakeFirewallCacheEmpty(),
			expectedGodoFirewallListResp: func(context.Context, *godo.ListOptions) ([]godo.Firewall, *godo.Response, error) {
				return nil, newFakeNotOKResponse(), errors.New("failed to retrieve list of firewalls from DO API")
			},
			expectedError: errors.New("failed to retrieve list of firewalls from DO API"),
		},
		{
			name:    "firewall does not exist in API",
			fwCache: newFakeFirewallCache(),
			expectedGodoFirewallGetResp: func(context.Context, string) (*godo.Firewall, *godo.Response, error) {
				return nil, newFakeNotFoundResponse(), errors.New("not found")
			},
			expectedGodoFirewallListResp: func(context.Context, *godo.ListOptions) ([]godo.Firewall, *godo.Response, error) {
				return nil, newFakeOKResponse(), nil
			},
		},
		{
			name:    "handle 404 response code from GET firewall by ID and instead return firewall from List",
			fwCache: newFakeFirewallCache(),
			expectedGodoFirewallGetResp: func(context.Context, string) (*godo.Firewall, *godo.Response, error) {
				return nil, newFakeNotFoundResponse(), errors.New("not found")
			},
			expectedGodoFirewallListResp: func(context.Context, *godo.ListOptions) ([]godo.Firewall, *godo.Response, error) {
				return []godo.Firewall{*newFakeFirewall()}, newFakeOKResponse(), nil
			},
			expectedFirewall: newFakeFirewall(),
		},
		{
			name:    "get firewall from API with cached firewall ID",
			fwCache: newFakeFirewallCache(),
			expectedGodoFirewallGetResp: func(context.Context, string) (*godo.Firewall, *godo.Response, error) {
				return newFakeFirewall(), newFakeOKResponse(), nil
			},
			expectedFirewall: newFakeFirewall(),
		},
		{
			name:    "get firewall from API without cached firewall",
			fwCache: newFakeFirewallCacheEmpty(),
			expectedGodoFirewallListResp: func(context.Context, *godo.ListOptions) ([]godo.Firewall, *godo.Response, error) {
				return nil, newFakeOKResponse(), nil
			},
		},
	}
	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			gclient := newFakeGodoClient(
				&fakeFirewallService{
					getFunc:  test.expectedGodoFirewallGetResp,
					listFunc: test.expectedGodoFirewallListResp,
				},
			)
			fwManager = newFakeFirewallManager(gclient, test.fwCache)

			fw, err := fwManager.Get(ctx)
			if (err != nil && test.expectedError == nil) || (err == nil && test.expectedError != nil) {
				t.Errorf("incorrect firewall config\nwant: %#v\n got: %#v", test.expectedError, err)
			}

			if diff := cmp.Diff(test.expectedFirewall, fw); diff != "" {
				t.Errorf("Get() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFirewallController_Set(t *testing.T) {
	testcases := []struct {
		name                           string
		fwCache                        firewallCache
		firewallRequest                *godo.FirewallRequest
		expectedGodoFirewallCreateResp func(context.Context, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error)
		expectedGodoFirewallGetResp    func(context.Context, string) (*godo.Firewall, *godo.Response, error)
		expectedGodoFirewallListResp   func(context.Context, *godo.ListOptions) ([]godo.Firewall, *godo.Response, error)
		expectedGodoFirewallUpdateResp func(context.Context, string, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error)
		expectedError                  error
		expectedFirewall               *godo.Firewall
	}{
		{
			name:    "do nothing when firewall is already properly configured",
			fwCache: newFakeFirewallCache(),
			firewallRequest: &godo.FirewallRequest{
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
		},
		{
			name:    "update firewall when rule parts are improperly configured",
			fwCache: newFakeFirewallCache(),
			firewallRequest: &godo.FirewallRequest{
				Name:          "wrong name",
				InboundRules:  testDiffInboundRules,
				OutboundRules: testDiffOutboundRules,
				Tags:          []string{"wrong-tag"},
			},
			expectedGodoFirewallGetResp: func(context.Context, string) (*godo.Firewall, *godo.Response, error) {
				return newFakeFirewall(), newFakeOKResponse(), nil
			},
			expectedGodoFirewallUpdateResp: func(context.Context, string, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
				return newFakeFirewall(), newFakeOKResponse(), nil
			},
		},
		{
			name:    "create firewall when cache does not exist (i.e. initial startup)",
			fwCache: newFakeFirewallCacheEmpty(),
			firewallRequest: &godo.FirewallRequest{
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			expectedGodoFirewallListResp: func(context.Context, *godo.ListOptions) ([]godo.Firewall, *godo.Response, error) {
				return []godo.Firewall{}, newFakeOKResponse(), nil
			},
			expectedGodoFirewallCreateResp: func(context.Context, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
				return newFakeFirewall(), newFakeOKResponse(), nil
			},
		},
		{
			name:    "failing to create the firewall because of an unexpected error",
			fwCache: newFakeFirewallCacheWithDiffInboundRules(),
			firewallRequest: &godo.FirewallRequest{
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			expectedGodoFirewallUpdateResp: func(context.Context, string, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
				return nil, newFakeNotFoundResponse(), errors.New("not found")
			},
			expectedGodoFirewallCreateResp: func(context.Context, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
				return nil, newFakeNotOKResponse(), errors.New("unexpected error")
			},
			expectedError: errors.New("failed to create firewall"),
		},
		{
			name:    "failing to get the firewall",
			fwCache: newFakeFirewallCacheEmpty(),
			firewallRequest: &godo.FirewallRequest{
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			expectedGodoFirewallListResp: func(context.Context, *godo.ListOptions) ([]godo.Firewall, *godo.Response, error) {
				return nil, newFakeNotOKResponse(), errors.New("unexpected error")
			},
			expectedError: errors.New("failed to create firewall"),
		},
		{
			name:    "failing to update the firewall because of an unexpected error",
			fwCache: newFakeFirewallCacheWithDiffInboundRules(),
			firewallRequest: &godo.FirewallRequest{
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			expectedGodoFirewallUpdateResp: func(context.Context, string, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
				return nil, newFakeNotOKResponse(), errors.New("unexpected error")
			},
			expectedError: errors.New("unexpected error"),
		},
		{
			name:    "when the firewall cache is nil return existing firewall from API then update cache",
			fwCache: newFakeFirewallCacheEmpty(),
			firewallRequest: &godo.FirewallRequest{
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			expectedGodoFirewallListResp: func(context.Context, *godo.ListOptions) ([]godo.Firewall, *godo.Response, error) {
				return []godo.Firewall{*newFakeFirewall()}, newFakeOKResponse(), nil
			},
			expectedGodoFirewallUpdateResp: func(context.Context, string, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
				return newFakeFirewall(), newFakeOKResponse(), nil
			},
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			gclient := newFakeGodoClient(
				&fakeFirewallService{
					listFunc:   test.expectedGodoFirewallListResp,
					updateFunc: test.expectedGodoFirewallUpdateResp,
					createFunc: test.expectedGodoFirewallCreateResp,
					getFunc:    test.expectedGodoFirewallGetResp,
				},
			)
			fwManager = newFakeFirewallManager(gclient, test.fwCache)
			fc := NewFirewallController(ctx, kclient, gclient, inf.Core().V1().Services(), fwManager, testWorkerFWTags, testWorkerFWName)

			err := fc.fwManager.Set(ctx, test.firewallRequest)
			if (err != nil && test.expectedError == nil) || (err == nil && test.expectedError != nil) {
				t.Errorf("incorrect firewall config\nwant: %#v\n got: %#v", test.expectedError, err)
			}
		})
	}
}

func TestFirewallController_NoDataRace(t *testing.T) {
	// setup
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var wg sync.WaitGroup

	gclient := newFakeGodoClient(
		&fakeFirewallService{
			listFunc: func(context.Context, *godo.ListOptions) ([]godo.Firewall, *godo.Response, error) {
				return nil, newFakeOKResponse(), nil
			},
			updateFunc: func(context.Context, string, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
				return newFakeFirewall(), newFakeOKResponse(), nil
			},
			createFunc: func(context.Context, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
				return nil, newFakeOKResponse(), nil
			},
			getFunc: func(context.Context, string) (*godo.Firewall, *godo.Response, error) {
				return newFakeFirewall(), newFakeOKResponse(), nil
			},
		},
	)
	fwManager := newFakeFirewallManager(gclient, newFakeFirewallCache())
	fc := NewFirewallController(ctx, kclient, gclient, inf.Core().V1().Services(), fwManager, testWorkerFWTags, testWorkerFWName)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for ctx.Err() == nil { // context has not been terminated
			if err := fc.ensureReconciledFirewall(ctx); err != nil && err != context.Canceled {
				t.Errorf("ensureReconciledFirewall failed: %s", err)
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		fc.Run(ctx.Done(), time.Duration(0))
	}()

	wg.Wait()
	// We do not assert on anything because the goal of this test is to catch data races.
}

func TestFirewallController_actualRun(t *testing.T) {
	testcases := []struct {
		name                           string
		fwCache                        firewallCache
		expectedGodoFirewallCreateResp func(context.Context, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error)
		expectedGodoFirewallGetResp    func(context.Context, string) (*godo.Firewall, *godo.Response, error)
		expectedGodoFirewallListResp   func(context.Context, *godo.ListOptions) ([]godo.Firewall, *godo.Response, error)
		expectedGodoFirewallUpdateResp func(context.Context, string, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error)
		expectedError                  error
	}{
		{
			name:    "error returns when we fail to get firewall from firewall api",
			fwCache: newFakeFirewallCacheEmpty(),
			expectedGodoFirewallListResp: func(context.Context, *godo.ListOptions) ([]godo.Firewall, *godo.Response, error) {
				return nil, newFakeNotOKResponse(), errors.New("unexpected error")
			},
			expectedError: errors.New("failed to get worker firewall: failed to retrieve list of firewalls from DO API"),
		},
		{
			name:    "when firewall matches what we expect there is nothing to reconcile",
			fwCache: newFakeFirewallCache(),
			expectedGodoFirewallGetResp: func(context.Context, string) (*godo.Firewall, *godo.Response, error) {
				return newFakeFirewall(), newFakeOKResponse(), nil
			},
		},
		{
			name:    "when firewall needs to be reconciled an update call is made",
			fwCache: newFakeFirewallCacheWithDiffInboundRules(),
			expectedGodoFirewallGetResp: func(context.Context, string) (*godo.Firewall, *godo.Response, error) {
				return newFakeFirewall(), newFakeOKResponse(), nil
			},
			expectedGodoFirewallUpdateResp: func(context.Context, string, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
				return newFakeFirewallWithDiffInboundRules(), newFakeOKResponse(), nil
			},
		},
		{
			name:    "error is returned when firewall cache exists but needs to be reconciled and update call fails",
			fwCache: newFakeFirewallCacheWithDiffInboundRules(),
			expectedGodoFirewallGetResp: func(context.Context, string) (*godo.Firewall, *godo.Response, error) {
				return newFakeFirewall(), newFakeOKResponse(), nil
			},
			expectedGodoFirewallUpdateResp: func(context.Context, string, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
				return nil, newFakeNotOKResponse(), errors.New("unexpected error")
			},
			expectedError: errors.New("failed to reconcile worker firewall: failed to set reconciled firewall: could not update firewall: unexpected error"),
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			// setup
			gclient := newFakeGodoClient(
				&fakeFirewallService{
					listFunc:   test.expectedGodoFirewallListResp,
					updateFunc: test.expectedGodoFirewallUpdateResp,
					createFunc: test.expectedGodoFirewallCreateResp,
					getFunc:    test.expectedGodoFirewallGetResp,
				},
			)
			fwManager := newFakeFirewallManager(gclient, test.fwCache)
			fc := NewFirewallController(ctx, kclient, gclient, inf.Core().V1().Services(), fwManager, testWorkerFWTags, testWorkerFWName)

			err := fc.actualRun(ctx.Done(), time.Duration(0))
			if (err != nil && test.expectedError == nil) || (err == nil && test.expectedError != nil) {
				t.Errorf("error with firewall controller run\nwant: %#v\n got: %#v", test.expectedError, err)
			}
		})
	}
}

func TestFirewallController_isEqualCheckForFirewallRequest(t *testing.T) {
	testcases := []struct {
		name            string
		firewall        *godo.Firewall
		firewallRequest *godo.FirewallRequest
		unequalParts    []string
		equal           bool
	}{
		{
			name: "detects when firewall request fields and firewall fields are equal",
			firewall: &godo.Firewall{
				ID:            "123",
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			firewallRequest: &godo.FirewallRequest{
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			equal: true,
		},
		{
			name: "detects when name is not equal",
			firewall: &godo.Firewall{
				ID:            "123",
				Name:          "diff firewall name",
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			firewallRequest: &godo.FirewallRequest{
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			unequalParts: []string{"name"},
			equal:        false,
		},
		{
			name: "detects when inboundRules are not equal",
			firewall: &godo.Firewall{
				ID:            "123",
				Name:          testWorkerFWName,
				InboundRules:  testDiffInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			firewallRequest: &godo.FirewallRequest{
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			unequalParts: []string{"inboundRules"},
			equal:        false,
		},
		{
			name: "detects when outboundRules are not equal",
			firewall: &godo.Firewall{
				ID:            "123",
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testDiffOutboundRules,
				Tags:          testWorkerFWTags,
			},
			firewallRequest: &godo.FirewallRequest{
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			unequalParts: []string{"outboundRules"},
			equal:        false,
		},
		{
			name: "detects when tags are not equal",
			firewall: &godo.Firewall{
				ID:            "123",
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          []string{"rick", "and", "morty"},
			},
			firewallRequest: &godo.FirewallRequest{
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			unequalParts: []string{"tags"},
			equal:        false,
		},
		{
			name: "detects when more then one property is not equal",
			firewall: &godo.Firewall{
				ID:            "123",
				Name:          testWorkerFWName,
				InboundRules:  testDiffInboundRules,
				OutboundRules: testDiffOutboundRules,
				Tags:          testWorkerFWTags,
			},
			firewallRequest: &godo.FirewallRequest{
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			unequalParts: []string{"inboundRules", "outboundRules"},
			equal:        false,
		},
		{
			name: "detects when all properties are not equal",
			firewall: &godo.Firewall{
				Name:          "different name",
				InboundRules:  testDiffInboundRules,
				OutboundRules: testDiffOutboundRules,
				Tags:          []string{"rick", "and", "morty"},
			},
			firewallRequest: &godo.FirewallRequest{
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			unequalParts: []string{"name", "inboundRules", "outboundRules", "tags"},
			equal:        false,
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			equal, unequalParts := isEqual(test.firewall, test.firewallRequest)
			if test.equal != equal {
				t.Errorf("isEqual() returned unexpected equal bool value (want: %v, got: %v)", test.equal, equal)
			}
			if diff := cmp.Diff(test.unequalParts, unequalParts); diff != "" {
				t.Errorf("isEqual() returned unexpected unequalParts value (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFirewallController_createReconciledFirewallRequest(t *testing.T) {
	testcases := []struct {
		name               string
		firewallRequest    *godo.FirewallRequest
		firewallController FirewallController
		serviceList        []*v1.Service
	}{
		{
			name: "nothing to reconcile when there are no changes",
			firewallRequest: &godo.FirewallRequest{
				Name:          testWorkerFWName,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
		},
		{
			name: "reconcile firewall when nodeport service has sctp",
			firewallRequest: &godo.FirewallRequest{
				Name: testWorkerFWName,
				InboundRules: []godo.InboundRule{
					{
						Protocol:  "tcp",
						PortRange: "31000",
						Sources: &godo.Sources{
							Tags: testWorkerFWTags,
						},
					},
				},
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			serviceList: []*v1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
						UID:  "abc123",
					},
					Spec: v1.ServiceSpec{
						Type: v1.ServiceTypeNodePort,
						Ports: []v1.ServicePort{
							{
								Name:     "port1",
								Protocol: v1.ProtocolTCP,
								NodePort: int32(31000),
							},
							{
								Name:     "port2",
								Protocol: v1.ProtocolSCTP,
								NodePort: int32(31000),
							},
						},
					},
				},
			},
		},
		{
			name: "nothing to reconcile when service type is not of type NodePort",
			firewallRequest: &godo.FirewallRequest{
				Name:          testWorkerFWName,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			serviceList: []*v1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "lbServiceType",
						UID:  "abc12345",
					},
					Spec: v1.ServiceSpec{
						Type: v1.ServiceTypeLoadBalancer,
						Ports: []v1.ServicePort{
							{
								Name:     "port",
								Protocol: v1.ProtocolTCP,
								Port:     int32(80),
							},
						},
					},
				},
			},
		},
		{
			name: "reconcile firewall based on added nodeport service",
			firewallRequest: &godo.FirewallRequest{
				Name: testWorkerFWName,
				InboundRules: []godo.InboundRule{
					{
						Protocol:  "tcp",
						PortRange: "31000",
						Sources: &godo.Sources{
							Tags: testWorkerFWTags,
						},
					},
					{
						Protocol:  "udp",
						PortRange: "31000",
						Sources: &godo.Sources{
							Tags: testWorkerFWTags,
						},
					},
				},
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			serviceList: []*v1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
						UID:  "abc123",
					},
					Spec: v1.ServiceSpec{
						Type: v1.ServiceTypeNodePort,
						Ports: []v1.ServicePort{
							{
								Name:     "port1",
								Protocol: v1.ProtocolTCP,
								NodePort: int32(31000),
							},
							{
								Name:     "port2",
								Protocol: v1.ProtocolUDP,
								NodePort: int32(31000),
							},
						},
					},
				},
			},
		},
		{
			name: "reconcile firewall based on multiple added services",
			firewallRequest: &godo.FirewallRequest{
				Name: testWorkerFWName,
				InboundRules: []godo.InboundRule{
					{
						Protocol:  "tcp",
						PortRange: "31000",
						Sources: &godo.Sources{
							Tags: testWorkerFWTags,
						},
					},
					{
						Protocol:  "udp",
						PortRange: "31000",
						Sources: &godo.Sources{
							Tags: testWorkerFWTags,
						},
					},
					{
						Protocol:  "tcp",
						PortRange: "30000",
						Sources: &godo.Sources{
							Tags: testWorkerFWTags,
						},
					},
					{
						Protocol:  "udp",
						PortRange: "30000",
						Sources: &godo.Sources{
							Tags: testWorkerFWTags,
						},
					},
					{
						Protocol:  "tcp",
						PortRange: "32727",
						Sources: &godo.Sources{
							Tags: testWorkerFWTags,
						},
					},
					{
						Protocol:  "udp",
						PortRange: "32727",
						Sources: &godo.Sources{
							Tags: testWorkerFWTags,
						},
					},
				},
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			serviceList: []*v1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "singleNodePort",
						UID:  "abc123",
					},
					Spec: v1.ServiceSpec{
						Type: v1.ServiceTypeNodePort,
						Ports: []v1.ServicePort{
							{
								Name:     "port1",
								Protocol: v1.ProtocolTCP,
								NodePort: int32(31000),
							},
							{
								Name:     "port2",
								Protocol: v1.ProtocolUDP,
								NodePort: int32(31000),
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "multipleNodePorts",
						UID:  "abc1234",
					},
					Spec: v1.ServiceSpec{
						Type: v1.ServiceTypeNodePort,
						Ports: []v1.ServicePort{
							{
								Name:     "port1",
								Protocol: v1.ProtocolTCP,
								NodePort: int32(30000),
							},
							{
								Name:     "port2",
								Protocol: v1.ProtocolUDP,
								NodePort: int32(30000),
							},
							{
								Name:     "port3",
								Protocol: v1.ProtocolTCP,
								NodePort: int32(32727),
							},
							{
								Name:     "port4",
								Protocol: v1.ProtocolUDP,
								NodePort: int32(32727),
							},
						},
					},
				},
			},
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			fc := FirewallController{
				workerFirewallTags: testWorkerFWTags,
				workerFirewallName: testWorkerFWName,
			}
			fwReq := fc.createReconciledFirewallRequest(test.serviceList)
			if diff := cmp.Diff(test.firewallRequest, fwReq); diff != "" {
				t.Errorf("createReconciledFirewallRequest() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
