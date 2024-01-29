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
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"github.com/google/go-cmp/cmp"
	"golang.org/x/sync/errgroup"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"
)

func TestFirewallController_Get(t *testing.T) {
	testcases := []struct {
		name                         string
		fwCache                      *firewallCache
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
			fwManager := newFakeFirewallManager(gclient, test.fwCache)

			fw, err := fwManager.Get(ctx)
			if (err != nil && test.expectedError == nil) || (err == nil && test.expectedError != nil) {
				t.Errorf("incorrect firewall config\nwant: %#v\n got: %#v", test.expectedError, err)
			}

			if diff := cmp.Diff(test.expectedFirewall, fw); diff != "" {
				t.Errorf("Get() mismatch (-want +got):\n%s", diff)
			}

			if test.expectedError == nil {
				if _, isSet := fwManager.fwCache.getCachedFirewall(); !isSet {
					t.Error("firewall cache is unset")
				}
			}
		})
	}
}

func TestFirewallController_Set(t *testing.T) {
	testcases := []struct {
		name                           string
		fwID                           string
		fakeOverrides                  fakeFirewallService
		expectedGodoFirewallCreateResp func(context.Context, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error)
		expectedGodoFirewallUpdateResp func(context.Context, string, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error)
		expectedError                  error
	}{
		{
			name: "update firewall when firewall ID is present",
			fwID: "id",
			fakeOverrides: fakeFirewallService{
				updateFunc: func(context.Context, string, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
					return newFakeFirewall(), newFakeOKResponse(), nil
				},
			},
		},
		{
			name: "create firewall when firewall ID is absent",
			fwID: "",
			fakeOverrides: fakeFirewallService{
				createFunc: func(context.Context, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
					return newFakeFirewall(), newFakeOKResponse(), nil
				},
			},
		},
		{
			name: "create firewall when firewall ID is present",
			fwID: "id",
			fakeOverrides: fakeFirewallService{
				updateFunc: func(context.Context, string, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
					return nil, newFakeNotFoundResponse(), errors.New("not found")
				},
				createFunc: func(context.Context, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
					return newFakeFirewall(), newFakeOKResponse(), nil
				},
			},
		},
		{
			name: "error on update firewall",
			fwID: "id",
			fakeOverrides: fakeFirewallService{
				updateFunc: func(context.Context, string, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
					return nil, newFakeNotOKResponse(), errors.New("unexpected error")
				},
			},
			expectedError: errors.New("failed to update firewall: unexpected error"),
		},
		{
			name: "error on create firewall",
			fwID: "",
			fakeOverrides: fakeFirewallService{
				createFunc: func(context.Context, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
					return nil, newFakeNotOKResponse(), errors.New("unexpected error")
				},
			},
			expectedError: errors.New("failed to create firewall: unexpected error"),
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			fake := createFakeFirewallService(test.fakeOverrides)

			gclient := newFakeGodoClient(fake)

			fwManager := newFakeFirewallManager(gclient, newFakeFirewallCache())
			fc := NewFirewallController(kclient, gclient, inf.Core().V1().Services(), fwManager)

			firewallRequest := &godo.FirewallRequest{
				Name:          testWorkerFWName,
				InboundRules:  testInboundRules,
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			}
			err := fc.fwManager.Set(ctx, test.fwID, firewallRequest)

			if (err != nil && test.expectedError == nil) || (err == nil && test.expectedError != nil) {
				t.Fatalf("expected error %q, got %q", test.expectedError, err)
			}
			if test.expectedError != nil {
				if err.Error() != test.expectedError.Error() {
					t.Errorf("expected error %q does not match %q", test.expectedError, err)
				}
			}
			if test.expectedError == nil {
				if _, isSet := fc.fwManager.fwCache.getCachedFirewall(); !isSet {
					t.Error("firewall cache is unset")
				}
			}
		})
	}
}

func TestFirewallController_ConcurrentUsage(t *testing.T) {
	// setup
	var (
		stFake          statefulFake
		numCreateCalled int
	)
	gclient := newFakeGodoClient(
		createFakeFirewallService(fakeFirewallService{
			listFunc: stFake.listFunc,
			getFunc:  stFake.getFunc,
			createFunc: func(ctx context.Context, fr *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
				numCreateCalled++
				if numCreateCalled > 1 {
					return nil, nil, errors.New("create should not be invoked more than once")
				}

				return stFake.createFunc(ctx, fr)
			},
		}),
	)
	fwManager := newFakeFirewallManager(gclient, newFakeFirewallCacheEmpty())
	fc := NewFirewallController(kclient, gclient, inf.Core().V1().Services(), fwManager)

	doneCtx, doneCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer doneCancel()
	eg, ctx := errgroup.WithContext(doneCtx)

	// loopCreator takes a function and runs it until doneCtx is due or an error
	// occurs. It returns the logic as a function for easy consumption in errgroup.
	loopCreator := func(f func() error) func() error {
		return func() error {
			for {
				select {
				case <-doneCtx.Done():
					return nil
				default:
					if err := f(); err != nil {
						if doneCtx.Err() != nil {
							// The error is due to doneCtx having finished normally.
							return nil
						}
						return err
					}
				}
			}
		}
	}

	eg.Go(loopCreator(func() error {
		_, err := fc.ensureReconciledFirewall(ctx)
		if err != nil {
			return fmt.Errorf("ensureReconciledFirewall failed: %s", err)
		}
		return nil
	}))

	eg.Go(loopCreator(func() error {
		err := fc.syncResource(ctx)
		if err != nil {
			return fmt.Errorf("run loop failed: %s", err)
		}
		return nil
	}))

	err := eg.Wait()
	if err != nil {
		t.Error(err)
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
							Addresses: []string{"0.0.0.0/0", "::/0"},
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
							Addresses: []string{"0.0.0.0/0", "::/0"},
						},
					},
					{
						Protocol:  "udp",
						PortRange: "31000",
						Sources: &godo.Sources{
							Addresses: []string{"0.0.0.0/0", "::/0"},
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
							Addresses: []string{"0.0.0.0/0", "::/0"},
						},
					},
					{
						Protocol:  "udp",
						PortRange: "31000",
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
					{
						Protocol:  "udp",
						PortRange: "30000",
						Sources: &godo.Sources{
							Addresses: []string{"0.0.0.0/0", "::/0"},
						},
					},
					{
						Protocol:  "tcp",
						PortRange: "32727",
						Sources: &godo.Sources{
							Addresses: []string{"0.0.0.0/0", "::/0"},
						},
					},
					{
						Protocol:  "udp",
						PortRange: "32727",
						Sources: &godo.Sources{
							Addresses: []string{"0.0.0.0/0", "::/0"},
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
		{
			name: "reconcile firewall with management flag",
			firewallRequest: &godo.FirewallRequest{
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
						PortRange: "31000",
						Sources: &godo.Sources{
							Addresses: []string{"0.0.0.0/0", "::/0"},
						},
					},
					{
						Protocol:  "tcp",
						PortRange: "32000",
						Sources: &godo.Sources{
							Addresses: []string{"0.0.0.0/0", "::/0"},
						},
					},
				},
				OutboundRules: testOutboundRules,
				Tags:          testWorkerFWTags,
			},
			serviceList: []*v1.Service{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "noManagementFlag",
						UID:  "uid1",
					},
					Spec: v1.ServiceSpec{
						Type: v1.ServiceTypeNodePort,
						Ports: []v1.ServicePort{
							{
								Name:     "port",
								Protocol: v1.ProtocolTCP,
								NodePort: int32(30000),
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "managementEnabled",
						UID:  "uid2",
						Annotations: map[string]string{
							annotationDOFirewallManaged: "true",
						},
					},
					Spec: v1.ServiceSpec{
						Type: v1.ServiceTypeNodePort,
						Ports: []v1.ServicePort{
							{
								Name:     "port",
								Protocol: v1.ProtocolTCP,
								NodePort: int32(31000),
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "managementFlagInvalid",
						UID:  "uid4",
						Annotations: map[string]string{
							annotationDOFirewallManaged: "undecided",
						},
					},
					Spec: v1.ServiceSpec{
						Type: v1.ServiceTypeNodePort,
						Ports: []v1.ServicePort{
							{
								Name:     "port",
								Protocol: v1.ProtocolTCP,
								NodePort: int32(32000),
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "managementDisabled",
						UID:  "uid3",
						Annotations: map[string]string{
							annotationDOFirewallManaged: "false",
						},
					},
					Spec: v1.ServiceSpec{
						Type: v1.ServiceTypeNodePort,
						Ports: []v1.ServicePort{
							{
								Name:     "port",
								Protocol: v1.ProtocolTCP,
								NodePort: int32(33000),
							},
						},
					},
				},
			},
		},
	}

	for _, test := range testcases {
		t.Run(test.name, func(t *testing.T) {
			fm := firewallManager{
				workerFirewallTags: testWorkerFWTags,
				workerFirewallName: testWorkerFWName,
			}
			fwReq := fm.createReconciledFirewallRequest(test.serviceList)
			if diff := cmp.Diff(test.firewallRequest, fwReq); diff != "" {
				t.Errorf("createReconciledFirewallRequest() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFirewallController_ensureReconciledFirewall(t *testing.T) {
	// setOp defines a Set() operation outcome.
	type setOp string
	const (
		setOpNone   = "none"
		setOpCreate = "create"
		setOpUpdate = "update"
	)

	nodePortToService := func(nodePort int32) *v1.Service {
		return &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "svc",
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{
						Name:       "tcp",
						Protocol:   v1.ProtocolTCP,
						Port:       80,
						TargetPort: intstr.FromInt(80),
						NodePort:   nodePort,
					},
				},
				Type: v1.ServiceTypeNodePort,
			},
		}
	}

	serviceToFirewall := func(fm *firewallManager, svc *v1.Service) *godo.Firewall {
		fr := fm.createReconciledFirewallRequest([]*v1.Service{svc})
		return &godo.Firewall{
			ID:            "id",
			Name:          fr.Name,
			InboundRules:  fr.InboundRules,
			OutboundRules: fr.OutboundRules,
			Tags:          fr.Tags,
		}
	}

	tests := []struct {
		name                      string
		nodePortForCachedFirewall *int32
		nodePortForService        *int32
		wantSetOp                 setOp
	}{
		{
			name:                      "create firewall on empty cache and empty NodePort Services",
			nodePortForCachedFirewall: nil,
			nodePortForService:        nil,
			wantSetOp:                 setOpCreate,
		},
		{
			name:                      "create firewall on empty cache and non-empty NodePort Services",
			nodePortForCachedFirewall: nil,
			nodePortForService:        pointer.Int32Ptr(31337),
			wantSetOp:                 setOpCreate,
		},
		{
			name:                      "update firewall on non-empty cache and empty NodePort Services",
			nodePortForCachedFirewall: pointer.Int32Ptr(30000),
			nodePortForService:        nil,
			wantSetOp:                 setOpUpdate,
		},
		{
			name:                      "update firewall on mismatching cache and NodePort Services",
			nodePortForCachedFirewall: pointer.Int32Ptr(30000),
			nodePortForService:        pointer.Int32Ptr(31337),
			wantSetOp:                 setOpUpdate,
		},
		{
			name:                      "skip reconcile on matching cache and NodePort Services",
			nodePortForCachedFirewall: pointer.Int32Ptr(30000),
			nodePortForService:        pointer.Int32Ptr(30000),
			wantSetOp:                 setOpNone,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeOverrides := fakeFirewallService{
				// Never return a result from the DO API since the test already configures the cache as needed.
				listFunc: func(context.Context, *godo.ListOptions) ([]godo.Firewall, *godo.Response, error) {
					return nil, newFakeOKResponse(), nil
				},
			}

			wantSet := test.wantSetOp != setOpNone
			wantSkipped := test.wantSetOp == setOpNone

			// Set up our expectations for the create and update DO API calls depending on how we expect Set() to be
			// invoked.
			var gotSet bool
			switch test.wantSetOp {
			case setOpCreate:
				fakeOverrides.createFunc = func(context.Context, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
					gotSet = true
					return &godo.Firewall{}, newFakeOKResponse(), nil
				}
			case setOpUpdate:
				fakeOverrides.updateFunc = func(context.Context, string, *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
					gotSet = true
					return &godo.Firewall{}, newFakeOKResponse(), nil
				}
			}
			fake := createFakeFirewallService(fakeOverrides)
			gclient := newFakeGodoClient(fake)
			fwManager := newFakeFirewallManager(gclient, newFakeFirewallCacheEmpty())

			// Populate the firewall cache.
			if test.nodePortForCachedFirewall != nil {
				fw := serviceToFirewall(fwManager, nodePortToService(*test.nodePortForCachedFirewall))
				fwManager.fwCache.updateCache(fw)
			}

			kube := k8sfake.NewSimpleClientset()
			factory := informers.NewSharedInformerFactory(kube, 0)
			svcInformer := factory.Core().V1().Services()
			informer := svcInformer.Informer()

			// Populate informer.
			if test.nodePortForService != nil {
				svc := nodePortToService(*test.nodePortForService)
				_, err := kube.CoreV1().Services(v1.NamespaceDefault).Create(ctx, svc, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create service: %s", err)
				}
			}

			// Wait for the informer to be synchronized.
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			factory.Start(ctx.Done())
			ok := cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)
			if !ok {
				t.Fatal("informer cache did not sync")
			}

			// Run the test.
			fc := NewFirewallController(kclient, gclient, svcInformer, fwManager)

			gotSkipped, err := fc.ensureReconciledFirewall(ctx)
			if err != nil {
				t.Fatalf("got error %s", err)
			}
			if gotSkipped != wantSkipped {
				t.Errorf("got skipped %t, want %t", gotSkipped, wantSkipped)
			}
			if gotSet != wantSet {
				t.Errorf("got set %t, want %t", gotSet, wantSet)
			}
		})
	}
}
