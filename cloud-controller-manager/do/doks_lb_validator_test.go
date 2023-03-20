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

package do

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/digitalocean/godo"

	//"github.com/go-logr/logr"
	v1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlruntimelog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	//"sigs.k8s.io/controller-runtime/pkg/client"
	//ctrlruntimelog "sigs.k8s.io/controller-runtime/pkg/log"
	//"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	//"k8s.io/client-go/kubernetes/fake"
	admissionv1 "k8s.io/api/admission/v1"
	//"github.com/digitalocean/digitalocean-cloud-controller-manager/test/clients"
)

var (
	scheme = runtime.NewScheme()
)

type fakeKubernetesService struct{}

func (f *fakeKubernetesService) Create(ctx context.Context, request *godo.KubernetesClusterCreateRequest) (*godo.KubernetesCluster, *godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) Get(ctx context.Context, s string) (*godo.KubernetesCluster, *godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) GetUser(ctx context.Context, s string) (*godo.KubernetesClusterUser, *godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) GetUpgrades(ctx context.Context, s string) ([]*godo.KubernetesVersion, *godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) GetKubeConfig(ctx context.Context, s string) (*godo.KubernetesClusterConfig, *godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) GetKubeConfigWithExpiry(ctx context.Context, s string, i int64) (*godo.KubernetesClusterConfig, *godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) GetCredentials(ctx context.Context, s string, request *godo.KubernetesClusterCredentialsGetRequest) (*godo.KubernetesClusterCredentials, *godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) List(ctx context.Context, options *godo.ListOptions) ([]*godo.KubernetesCluster, *godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) Update(ctx context.Context, s string, request *godo.KubernetesClusterUpdateRequest) (*godo.KubernetesCluster, *godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) Upgrade(ctx context.Context, s string, request *godo.KubernetesClusterUpgradeRequest) (*godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) Delete(ctx context.Context, s string) (*godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) DeleteSelective(ctx context.Context, s string, request *godo.KubernetesClusterDeleteSelectiveRequest) (*godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) DeleteDangerous(ctx context.Context, s string) (*godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) ListAssociatedResourcesForDeletion(ctx context.Context, s string) (*godo.KubernetesAssociatedResources, *godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) CreateNodePool(ctx context.Context, clusterID string, req *godo.KubernetesNodePoolCreateRequest) (*godo.KubernetesNodePool, *godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) GetNodePool(ctx context.Context, clusterID, poolID string) (*godo.KubernetesNodePool, *godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) UpdateNodePool(ctx context.Context, clusterID, poolID string, req *godo.KubernetesNodePoolUpdateRequest) (*godo.KubernetesNodePool, *godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) RecycleNodePoolNodes(ctx context.Context, clusterID, poolID string, req *godo.KubernetesNodePoolRecycleNodesRequest) (*godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) DeleteNodePool(ctx context.Context, clusterID, poolID string) (*godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) DeleteNode(ctx context.Context, clusterID, poolID, nodeID string, req *godo.KubernetesNodeDeleteRequest) (*godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) GetOptions(ctx2 context.Context) (*godo.KubernetesOptions, *godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) AddRegistry(ctx context.Context, req *godo.KubernetesClusterRegistryRequest) (*godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) RemoveRegistry(ctx context.Context, req *godo.KubernetesClusterRegistryRequest) (*godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) RunClusterlint(ctx context.Context, clusterID string, req *godo.KubernetesRunClusterlintRequest) (string, *godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) GetClusterlintResults(ctx context.Context, clusterID string, req *godo.KubernetesGetClusterlintRequest) ([]*godo.ClusterlintDiagnostic, *godo.Response, error) {
	panic("not supported")
}

func (f *fakeKubernetesService) ListNodePools(ctx context.Context, clusterID string, opts *godo.ListOptions) ([]*godo.KubernetesNodePool, *godo.Response, error) {
	cluster := fakeCluster()
	return cluster.NodePools, newFakeOKResponse(), nil
}

func Test_Handle(t *testing.T) {
	//cluster := fakeCluster()
	testcases := []struct {
		name            string
		req             admission.Request
		oldObject       runtime.RawExtension
		gCLient         *godo.Client
		lbRequest       *godo.LoadBalancerRequest
		expectedAllowed bool
		expectedReason  string
	}{
		{
			name: "Allow if service type is not load balancer",
			req: admission.Request{fakeAdmissionRequest(
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeNodePort,
					},
				},
				nil,
			)},
			expectedAllowed: true,
		},
		{
			name: "Allow if request is of type DELETE",
			req: admission.Request{fakeAdmissionRequest(
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
						DeletionTimestamp: &metav1.Time{
							Time: time.Now().UTC(),
						},
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
					},
				},
				nil,
			)},

			lbRequest:       fakeValidLBRequest(),
			expectedAllowed: true,
		},
		{
			name: "Allow CREATE happy path",
			req: admission.Request{fakeAdmissionRequest(
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
					},
				},
				nil,
			)},
			expectedAllowed: true,
		},
		{
			name:            "Allow Update happy path",
			req:             admission.Request{fakeAdmissionRequest(fakeService("test2"), fakeService("old-service"))},
			expectedAllowed: true,
		},
	}

	for _, test := range testcases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			// setup client
			var testKubernetesService godo.KubernetesService
			testKubernetesService = &fakeKubernetesService{}

			gClient := godo.NewFromToken("test-token")
			gClient.Kubernetes = testKubernetesService
			gClient.LoadBalancers = &fakeLBService{
				listFn: func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
					return []godo.LoadBalancer{{ID: "2", Name: "two"}}, newFakeOKResponse(), nil
				},
				createFn: func(context.Context, *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
					return &godo.LoadBalancer{ID: "2", Name: "two"}, newFakeOKResponse(), nil
				},
				updateFn: func(context.Context, string, *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
					return &godo.LoadBalancer{ID: "2", Name: "two"}, newFakeOKResponse(), nil
				},
			}

			decoder, err := admission.NewDecoder(scheme)
			if err != nil {
				t.Error("failed to initialize decoder", err)
			}

			os.Setenv("DO_CLUSTER_ID", fakeCluster().ID)

			var logOpts []zap.Opts
			ll := zap.New(logOpts...).WithName("webhook-validation-server")
			ctrlruntimelog.SetLogger(ll)

			validator := &DOKSLBServiceValidator{
				Log:     ll,
				decoder: decoder,
				gClient: gClient,
			}

			res := validator.Handle(context.TODO(), test.req)
			// if test.allowed & res.allowed are not equal
			if test.expectedAllowed && res.Allowed == false {
				t.Error("Expected is not equal to actual")
			}
			if !test.expectedAllowed {
				if test.expectedReason != res.Result.Message {
					t.Error("Expected reason is not the actual reason")
				}
			}
		})
	}
}

func fakeCluster() *godo.KubernetesCluster {
	return &godo.KubernetesCluster{
		ID:         "1",
		Name:       "test",
		RegionSlug: "nyc3",
		NodePools: []*godo.KubernetesNodePool{
			{
				Name: "test",
				ID:   "1",
				Nodes: []*godo.KubernetesNode{
					{
						ID:        "1",
						Name:      "test",
						DropletID: "1",
					},
				},
			},
		},
	}
}

func fakeValidLBRequest() *godo.LoadBalancerRequest {
	return &godo.LoadBalancerRequest{
		Name:       "test",
		Region:     "nyc3",
		DropletIDs: []int{1},
		ForwardingRules: []godo.ForwardingRule{
			{
				EntryProtocol:  "http",
				EntryPort:      80,
				TargetProtocol: "http",
				TargetPort:     80,
				CertificateID:  "",
				TlsPassthrough: false,
			},
		},
		ValidateOnly: true,
	}
}

func fakeAdmissionRequest(newSvc *corev1.Service, oldSvc *corev1.Service) v1.AdmissionRequest {
	var (
		m   []byte
		p   []byte
		err error
	)

	if newSvc != nil {
		m, err = json.Marshal(*newSvc)
		if err != nil {
			panic(err.Error())
		}
	}

	if oldSvc != nil {
		p, err = json.Marshal(*oldSvc)
		if err != nil {
			panic(err.Error())
		}
	}

	return v1.AdmissionRequest{
		UID:       "test",
		Name:      "test",
		Namespace: "test",
		Object:    runtime.RawExtension{Raw: m},
		OldObject: runtime.RawExtension{Raw: p},
		Operation: admissionv1.Create,
		Kind: metav1.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Service",
		},
		Resource: metav1.GroupVersionResource{
			Group:    "",
			Version:  "v1",
			Resource: "service",
		},
	}
}

func fakeService(name string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "test",
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
		},
	}
}
