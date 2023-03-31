/*
Copyright 2023 DigitalOcean

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

package clients

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	v1meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/config/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var _ client.Client = &ControllerRuntimeTestClient{}

// ControllerRuntimeTestClient is a controller-runtime client usable for
// unit-testing.
// It can delegate to custom API methods and the controller-runtime fake (giving
// precedence to the former if defined), and if none defined returns an error.
// The latter can be used to ensure that a certain API method is not called,
// thereby allowing a (lightweight) mock-like usage.
type ControllerRuntimeTestClient struct {
	Fake client.WithWatch
	ControllerRuntimeStubFuncs
	ssaPatchedObjects []runtime.Object
}

// ControllerRuntimeStubFuncs are controller-runtime functions that can be
// stubbed out.
type ControllerRuntimeStubFuncs struct {
	Get         func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
	List        func(ctx context.Context, obj client.ObjectList, opts ...client.ListOption) error
	Create      func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
	Delete      func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error
	Update      func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error
	Patch       func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error
	DeleteAllOf func(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error
	RestMapper  func() meta.RESTMapper
}

// NewControllerRuntimeFakeClient returns a ControllerRuntimeTestClient that uses
// controller-runtime's fake client.
func NewControllerRuntimeFakeClient(scheme *runtime.Scheme, objs ...runtime.Object) *ControllerRuntimeTestClient {
	return &ControllerRuntimeTestClient{
		Fake: fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build(),
	}
}

// NewControllerRuntimeFakeClientWithStubs is like NewControllerRuntimeFakeClient
// but allows to override specific fake methods with stub calls. This can be
// useful when fake semantics are mostly desirable except for a few cases.
func NewControllerRuntimeFakeClientWithStubs(scheme *runtime.Scheme, funcs ControllerRuntimeStubFuncs, objs ...runtime.Object) *ControllerRuntimeTestClient {
	cl := NewControllerRuntimeFakeClient(scheme, objs...)
	cl.ControllerRuntimeStubFuncs = funcs
	return cl
}

// Get behaves like client.Get.
func (c ControllerRuntimeTestClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if c.ControllerRuntimeStubFuncs.Get != nil {
		return c.ControllerRuntimeStubFuncs.Get(ctx, key, obj)
	}

	if c.Fake != nil {
		return c.Fake.Get(ctx, key, obj, opts...)
	}

	return errors.New("get should not have been invoked")
}

// List behaves like client.List
func (c ControllerRuntimeTestClient) List(ctx context.Context, obj client.ObjectList, opts ...client.ListOption) error {
	if c.ControllerRuntimeStubFuncs.List != nil {
		return c.ControllerRuntimeStubFuncs.List(ctx, obj, opts...)
	}

	if c.Fake != nil {
		return c.Fake.List(ctx, obj, opts...)
	}

	return errors.New("list should not have been invoked")
}

// Create behaves like client.Create.
func (c ControllerRuntimeTestClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	obj.SetCreationTimestamp(v1meta.NewTime(time.Now().UTC()))

	if c.ControllerRuntimeStubFuncs.Create != nil {
		return c.ControllerRuntimeStubFuncs.Create(ctx, obj, opts...)
	}

	if c.Fake != nil {
		return c.Fake.Create(ctx, obj, opts...)
	}

	return errors.New("create should not have been invoked")
}

// Delete behaves like client.Delete.
func (c ControllerRuntimeTestClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	panic("not supported")
}

// Update behaves like client.Update.
func (c ControllerRuntimeTestClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if c.ControllerRuntimeStubFuncs.Update != nil {
		return c.ControllerRuntimeStubFuncs.Update(ctx, obj, opts...)
	}

	if c.Fake != nil {
		return c.Fake.Update(ctx, obj, opts...)
	}

	return errors.New("update should not have been invoked")
}

// Patch behaves like client.Patch.
func (c ControllerRuntimeTestClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if c.ControllerRuntimeStubFuncs.Patch != nil {
		return c.ControllerRuntimeStubFuncs.Patch(ctx, obj, patch, opts...)
	}

	if patch.Type() != types.ApplyPatchType && c.Fake != nil {
		return c.Fake.Patch(ctx, obj, patch, opts...)
	}
	// The built-in fake does not support SSA yet, so populate a
	// would-be-SSA-patched list of objects instead.
	c.ssaPatchedObjects = append(c.ssaPatchedObjects, obj)
	return nil
}

// DeleteAllOf behaves like client.DeleteAllOf.
func (c ControllerRuntimeTestClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	panic("not supported")
}

// Status behaves likes client.Status.
func (c ControllerRuntimeTestClient) Status() client.StatusWriter {
	panic("not supported")
}

// Scheme behaves likes client.Scheme.
func (c ControllerRuntimeTestClient) Scheme() *runtime.Scheme {
	panic("not supported")
}

// RESTMapper behaves likes client.RESTMapper.
func (c ControllerRuntimeTestClient) RESTMapper() meta.RESTMapper {
	if c.ControllerRuntimeStubFuncs.RestMapper != nil {
		return c.ControllerRuntimeStubFuncs.RestMapper()
	}

	if c.Fake != nil {
		return c.Fake.RESTMapper()
	}

	return nil
}

func (c ControllerRuntimeTestClient) SubResource(subResource string) client.SubResourceClient {
	panic("not supported")
}

// CRClusterStub implementes controller-runtime's cluster.Cluster
type CRClusterStub struct {
	Client client.Client
}

// GetClient ...
func (e *CRClusterStub) GetClient() client.Client {
	return e.Client
}

// SetFields ...
func (e *CRClusterStub) SetFields(i interface{}) error {
	return nil
}

// GetConfig ...
func (e *CRClusterStub) GetConfig() *rest.Config {
	return nil
}

// GetScheme ...
func (e *CRClusterStub) GetScheme() *runtime.Scheme {
	return nil
}

// GetFieldIndexer ...
func (e *CRClusterStub) GetFieldIndexer() client.FieldIndexer {
	return nil
}

// GetCache ...
func (e *CRClusterStub) GetCache() cache.Cache {
	return nil
}

// GetEventRecorderFor ...
func (e *CRClusterStub) GetEventRecorderFor(name string) record.EventRecorder {
	return nil
}

// GetRESTMapper ...
func (e *CRClusterStub) GetRESTMapper() meta.RESTMapper {
	return e.Client.RESTMapper()
}

// GetAPIReader ...
func (e *CRClusterStub) GetAPIReader() client.Reader {
	return nil
}

// Start ...
func (e *CRClusterStub) Start(ctx context.Context) error {
	return nil
}

// ManagerStubFuncs are controller-runtime's manager functions that can be
// stubbed out.
type ManagerStubFuncs struct {
	Add func(runnable manager.Runnable) error
}

// FakeManager implement's controller runtime's manager interface
type FakeManager struct {
	CRClusterStub
	ManagerStubs ManagerStubFuncs
}

// Add ...
func (f *FakeManager) Add(r manager.Runnable) error {
	if f.ManagerStubs.Add != nil {
		return f.ManagerStubs.Add(r)
	}
	return nil
}

// Elected ...
func (f *FakeManager) Elected() <-chan struct{} {
	panic("not implemented")
}

// AddMetricsExtraHandler ...
func (f *FakeManager) AddMetricsExtraHandler(path string, handler http.Handler) error {
	panic("not implemented")
}

// AddHealthzCheck ...
func (f *FakeManager) AddHealthzCheck(name string, check healthz.Checker) error {
	panic("not implemented")
}

// AddReadyzCheck ...
func (f *FakeManager) AddReadyzCheck(name string, check healthz.Checker) error {
	panic("not implemented")
}

// Start ...
func (f *FakeManager) Start(ctx context.Context) error {
	panic("not implemented")
}

// GetWebhookServer ...
func (f *FakeManager) GetWebhookServer() *webhook.Server {
	panic("not implemented")
}

// GetLogger ...
func (f *FakeManager) GetLogger() logr.Logger {
	panic("not implemented")
}

// GetControllerOptions ...
func (f *FakeManager) GetControllerOptions() v1alpha1.ControllerConfigurationSpec {
	panic("not implemented")
}
