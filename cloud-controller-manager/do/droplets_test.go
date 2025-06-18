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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cloudprovider "k8s.io/cloud-provider"

	"github.com/digitalocean/godo"
)

type fakeDropletService struct {
	listFunc           func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error)
	listWithGPUsFunc   func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error)
	listByTagFunc      func(ctx context.Context, tag string, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error)
	listByNameFunc     func(ctx context.Context, name string, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error)
	getFunc            func(ctx context.Context, dropletID int) (*godo.Droplet, *godo.Response, error)
	createFunc         func(ctx context.Context, createRequest *godo.DropletCreateRequest) (*godo.Droplet, *godo.Response, error)
	createMultipleFunc func(ctx context.Context, createRequest *godo.DropletMultiCreateRequest) ([]godo.Droplet, *godo.Response, error)
	deleteFunc         func(ctx context.Context, dropletID int) (*godo.Response, error)
	deleteByTagFunc    func(ctx context.Context, tag string) (*godo.Response, error)
	kernelsFunc        func(ctx context.Context, dropletID int, opt *godo.ListOptions) ([]godo.Kernel, *godo.Response, error)
	snapshotsFunc      func(ctx context.Context, dropletID int, opt *godo.ListOptions) ([]godo.Image, *godo.Response, error)
	backupsFunc        func(ctx context.Context, dropletID int, opt *godo.ListOptions) ([]godo.Image, *godo.Response, error)
	actionsFunc        func(ctx context.Context, dropletID int, opt *godo.ListOptions) ([]godo.Action, *godo.Response, error)
	neighborsFunc      func(cxt context.Context, dropletID int) ([]godo.Droplet, *godo.Response, error)
}

func (f *fakeDropletService) List(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
	return f.listFunc(ctx, opt)
}

func (f *fakeDropletService) ListWithGPUs(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
	return f.listWithGPUsFunc(ctx, opt)
}

func (f *fakeDropletService) ListByTag(ctx context.Context, tag string, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
	return f.listByTagFunc(ctx, tag, opt)
}

func (f *fakeDropletService) ListByName(ctx context.Context, name string, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
	return f.listByNameFunc(ctx, name, opt)
}

func (f *fakeDropletService) Get(ctx context.Context, dropletID int) (*godo.Droplet, *godo.Response, error) {
	return f.getFunc(ctx, dropletID)
}

func (f *fakeDropletService) Create(ctx context.Context, createRequest *godo.DropletCreateRequest) (*godo.Droplet, *godo.Response, error) {
	return f.createFunc(ctx, createRequest)
}

func (f *fakeDropletService) CreateMultiple(ctx context.Context, createRequest *godo.DropletMultiCreateRequest) ([]godo.Droplet, *godo.Response, error) {
	return f.createMultipleFunc(ctx, createRequest)
}

func (f *fakeDropletService) Delete(ctx context.Context, dropletID int) (*godo.Response, error) {
	return f.deleteFunc(ctx, dropletID)
}

func (f *fakeDropletService) DeleteByTag(ctx context.Context, tag string) (*godo.Response, error) {
	return f.deleteByTagFunc(ctx, tag)
}

func (f *fakeDropletService) Kernels(ctx context.Context, dropletID int, opt *godo.ListOptions) ([]godo.Kernel, *godo.Response, error) {
	return f.kernelsFunc(ctx, dropletID, opt)
}

func (f *fakeDropletService) Snapshots(ctx context.Context, dropletID int, opt *godo.ListOptions) ([]godo.Image, *godo.Response, error) {
	return f.snapshotsFunc(ctx, dropletID, opt)
}

func (f *fakeDropletService) Backups(ctx context.Context, dropletID int, opt *godo.ListOptions) ([]godo.Image, *godo.Response, error) {
	return f.backupsFunc(ctx, dropletID, opt)
}

func (f *fakeDropletService) GetBackupPolicy(ctx context.Context, dropletID int) (*godo.DropletBackupPolicy, *godo.Response, error) {
	return nil, nil, fmt.Errorf("not necessary to implement")
}

func (f *fakeDropletService) ListBackupPolicies(ctx context.Context, opt *godo.ListOptions) (map[int]*godo.DropletBackupPolicy, *godo.Response, error) {
	return nil, nil, fmt.Errorf("not necessary to implement")
}

func (f *fakeDropletService) ListSupportedBackupPolicies(ctx context.Context) ([]*godo.SupportedBackupPolicy, *godo.Response, error) {
	return nil, nil, fmt.Errorf("not necessary to implement")
}

func (f *fakeDropletService) Actions(ctx context.Context, dropletID int, opt *godo.ListOptions) ([]godo.Action, *godo.Response, error) {
	return f.actionsFunc(ctx, dropletID, opt)
}

func (f *fakeDropletService) Neighbors(ctx context.Context, dropletID int) ([]godo.Droplet, *godo.Response, error) {
	return f.neighborsFunc(ctx, dropletID)
}

func newFakeDropletClient(fakeDroplet *fakeDropletService) *godo.Client {
	return newFakeClient(fakeDroplet, nil, nil)
}

func newFakeDroplet() *godo.Droplet {
	return &godo.Droplet{
		ID:       123,
		Name:     "test-droplet",
		SizeSlug: "2gb",
		Networks: &godo.Networks{
			V4: []godo.NetworkV4{
				{
					IPAddress: "10.0.0.0",
					Type:      "private",
				},
				{
					IPAddress: "99.99.99.99",
					Type:      "public",
				},
			},
		},
		Region: &godo.Region{
			Name: "test-region",
			Slug: "test1",
		},
	}
}

func newFakeShutdownDroplet() *godo.Droplet {
	return &godo.Droplet{
		ID:       123,
		Name:     "test-droplet",
		SizeSlug: "2gb",
		Status:   "off",
		Networks: &godo.Networks{
			V4: []godo.NetworkV4{
				{
					IPAddress: "10.0.0.0",
					Type:      "private",
				},
				{
					IPAddress: "99.99.99.99",
					Type:      "public",
				},
			},
		},
		Region: &godo.Region{
			Name: "test-region",
			Slug: "test1",
		},
	}
}

var _ cloudprovider.InstancesV2 = new(instances)

func TestInstanceExists(t *testing.T) {
	tests := []struct {
		name        string
		fake        *fakeDropletService
		expected    bool
		expectedErr string
	}{
		{
			name: "instance exists",
			fake: &fakeDropletService{
				getFunc: func(ctx context.Context, dropletID int) (*godo.Droplet, *godo.Response, error) {
					droplet := newFakeDroplet()
					resp := newFakeOKResponse()
					return droplet, resp, nil
				},
			},
			expected: true,
		},
		{
			name: "instance not found",
			fake: &fakeDropletService{
				getFunc: func(ctx context.Context, dropletID int) (*godo.Droplet, *godo.Response, error) {
					droplet := newFakeDroplet()
					resp := newFakeNotFoundResponse()
					return droplet, resp, newFakeNotFoundErrorResponse()
				},
			},
			expected:    false,
			expectedErr: "",
		},
		{
			name: "error getting instance",
			fake: &fakeDropletService{
				getFunc: func(ctx context.Context, dropletID int) (*godo.Droplet, *godo.Response, error) {
					resp := newFakeNotOKResponse()
					return nil, resp, &godo.ErrorResponse{
						Response: &http.Response{
							Request: &http.Request{
								Method: "GET",
								URL:    &url.URL{},
							},
							StatusCode: http.StatusInternalServerError,
							Body:       io.NopCloser(bytes.NewBufferString("boom")),
						},
					}
				},
			},
			expected:    false,
			expectedErr: "error checking if instance exists: GET : 500 ",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res := &resources{gclient: newFakeDropletClient(tc.fake)}
			instances := newInstances(res, "nyc1")
			exists, err := instances.InstanceExists(context.TODO(), &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-droplet",
				},
				Spec: v1.NodeSpec{
					ProviderID: "digitalocean://1234",
				},
			})
			if tc.expectedErr != "" {
				if !strings.Contains(err.Error(), tc.expectedErr) {
					t.Fatalf("got error %v, expected %s", err, tc.expectedErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("got error %v, expected nil", err)
			}
			if tc.expected != exists {
				t.Fatalf("got %v, expected %v", exists, tc.expected)
			}
		})
	}
}

func TestInstanceShutdown(t *testing.T) {
	fake := &fakeDropletService{}
	fake.getFunc = func(ctx context.Context, dropletID int) (*godo.Droplet, *godo.Response, error) {
		droplet := newFakeShutdownDroplet()
		resp := newFakeOKResponse()
		return droplet, resp, nil
	}

	res := &resources{gclient: newFakeDropletClient(fake)}
	instances := newInstances(res, "nyc1")

	shutdown, err := instances.InstanceShutdown(context.TODO(), &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-droplet",
		},
		Spec: v1.NodeSpec{
			ProviderID: "digitalocean://123",
		},
	})
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if !shutdown {
		t.Errorf("expected node to be shutdown, but it wasn't")
	}
}

func TestInstanceMetadata(t *testing.T) {
	fake := &fakeDropletService{}
	fake.getFunc = func(ctx context.Context, dropletID int) (*godo.Droplet, *godo.Response, error) {
		droplet := newFakeDroplet()
		resp := newFakeOKResponse()
		return droplet, resp, nil
	}
	res := &resources{gclient: newFakeDropletClient(fake)}
	instances := newInstances(res, "nyc1")

	expectedAddresses := []v1.NodeAddress{
		{
			Type:    v1.NodeHostName,
			Address: "test-droplet",
		},
		{
			Type:    v1.NodeInternalIP,
			Address: "10.0.0.0",
		},
		{
			Type:    v1.NodeExternalIP,
			Address: "99.99.99.99",
		},
	}

	metadata, err := instances.InstanceMetadata(context.TODO(), &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-droplet",
		},
		Spec: v1.NodeSpec{
			ProviderID: "digitalocean://123",
		},
	})
	if err != nil {
		t.Errorf("unexpected err, expected nil. got: %v", err)
	}

	if !reflect.DeepEqual(metadata.NodeAddresses, expectedAddresses) {
		t.Errorf("unexpected node addresses. got: %v want: %v", metadata.NodeAddresses, expectedAddresses)
	}

	if metadata.ProviderID != "digitalocean://123" {
		t.Errorf("unexpected node providerID. got: %v, want: %v", metadata.ProviderID, "digitalocean://123")
	}

	if metadata.InstanceType != "2gb" {
		t.Errorf("unexpected node instance type. got: %v want: %v", metadata.InstanceType, newFakeDroplet().SizeSlug)
	}
	if metadata.Region != "test1" {
		t.Errorf("unexpected node region. got: %v want: %v", metadata.Region, newFakeDroplet().Region)
	}
}

func TestInstanceMetadataWithoutProviderID(t *testing.T) {
	fake := &fakeDropletService{}
	fake.listByNameFunc = func(ctx context.Context, name string, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
		droplet := newFakeDroplet()
		resp := newFakeOKResponse()
		return []godo.Droplet{*droplet}, resp, nil
	}
	res := &resources{gclient: newFakeDropletClient(fake)}
	instances := newInstances(res, "nyc1")

	expectedAddresses := []v1.NodeAddress{
		{
			Type:    v1.NodeHostName,
			Address: "test-droplet",
		},
		{
			Type:    v1.NodeInternalIP,
			Address: "10.0.0.0",
		},
		{
			Type:    v1.NodeExternalIP,
			Address: "99.99.99.99",
		},
	}

	metadata, err := instances.InstanceMetadata(context.TODO(), &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-droplet",
		},
		Spec: v1.NodeSpec{},
	})
	if err != nil {
		t.Errorf("unexpected err, expected nil. got: %v", err)
	}

	if !reflect.DeepEqual(metadata.NodeAddresses, expectedAddresses) {
		t.Errorf("unexpected node addresses. got: %v want: %v", metadata.NodeAddresses, expectedAddresses)
	}

	if metadata.ProviderID != "digitalocean://123" {
		t.Errorf("unexpected node providerID. got: %v, want: %v", metadata.ProviderID, "digitalocean://123")
	}

	if metadata.InstanceType != "2gb" {
		t.Errorf("unexpected node instance type. got: %v want: %v", metadata.InstanceType, newFakeDroplet().SizeSlug)
	}
	if metadata.Region != "test1" {
		t.Errorf("unexpected node region. got: %v want: %v", metadata.Region, newFakeDroplet().Region)
	}
}

func Test_dropletIDFromProviderID(t *testing.T) {
	testcases := []struct {
		name       string
		providerID string
		dropletID  int
		err        error
	}{
		{
			name:       "valid providerID",
			providerID: "digitalocean://12345",
			dropletID:  12345,
			err:        nil,
		},
		{
			name:       "invalid providerID - empty string",
			providerID: "",
			dropletID:  0,
			err:        errors.New("provider ID cannot be empty"),
		},
		{
			name:       "invalid providerID - empty number",
			providerID: "digitalocean://",
			dropletID:  0,
			err:        errors.New("provider ID number cannot be empty"),
		},
		{
			name:       "invalid providerID - wrong prefix",
			providerID: "digitalocean:/12345",
			dropletID:  0,
			err:        errors.New("provider ID \"digitalocean:/12345\" is missing prefix \"digitalocean://\""),
		},
		{
			name:       "invalid providerID - extra cruft",
			providerID: "digitalocean://12345cruft",
			dropletID:  0,
			err:        errors.New("failed to convert provider ID number \"12345cruft\": strconv.Atoi: parsing \"12345cruft\": invalid syntax"),
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			dropletID, err := dropletIDFromProviderID(testcase.providerID)
			if dropletID != testcase.dropletID {
				t.Errorf("actual droplet ID: %d", dropletID)
				t.Errorf("expected droplet ID: %d", testcase.dropletID)
				t.Error("unexpected droplet ID")
			}

			if !reflect.DeepEqual(err, testcase.err) {
				t.Errorf("actual err: %v", err)
				t.Errorf("expected err: %v", testcase.err)
				t.Error("unexpected err")
			}
		})
	}
}
