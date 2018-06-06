/*
Copyright 2017 DigitalOcean

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
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/cloudprovider"

	"github.com/digitalocean/godo"
)

type fakeDropletService struct {
	listFunc           func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error)
	listByTagFunc      func(ctx context.Context, tag string, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error)
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

func (f *fakeDropletService) ListByTag(ctx context.Context, tag string, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
	return f.listByTagFunc(ctx, tag, opt)
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

func (f *fakeDropletService) Actions(ctx context.Context, dropletID int, opt *godo.ListOptions) ([]godo.Action, *godo.Response, error) {
	return f.actionsFunc(ctx, dropletID, opt)
}

func (f *fakeDropletService) Neighbors(ctx context.Context, dropletID int) ([]godo.Droplet, *godo.Response, error) {
	return f.neighborsFunc(ctx, dropletID)
}

func newFakeClient(fake *fakeDropletService) *godo.Client {
	client := godo.NewClient(nil)
	client.Droplets = fake

	return client
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

func newFakeOKResponse() *godo.Response {
	return &godo.Response{
		Response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(bytes.NewBufferString("test")),
		},
	}
}

func newFakeNotOKResponse() *godo.Response {
	return &godo.Response{
		Response: &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       ioutil.NopCloser(bytes.NewBufferString("test")),
		},
	}
}

var _ cloudprovider.Instances = new(instances)

func TestNodeAddresses(t *testing.T) {
	fake := &fakeDropletService{}
	fake.listFunc = func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
		droplet := newFakeDroplet()
		droplets := []godo.Droplet{*droplet}

		resp := newFakeOKResponse()
		return droplets, resp, nil
	}

	client := newFakeClient(fake)
	instances := newInstances(client, "nyc1")

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

	addresses, err := instances.NodeAddresses(context.TODO(), "test-droplet")

	if !reflect.DeepEqual(addresses, expectedAddresses) {
		t.Errorf("unexpected node addresses. got: %v want: %v", addresses, expectedAddresses)
	}

	if err != nil {
		t.Errorf("unexpected err, expected nil. got: %v", err)
	}
}

func TestNodeAddressesByProviderID(t *testing.T) {
	fake := &fakeDropletService{}
	fake.getFunc = func(ctx context.Context, dropletID int) (*godo.Droplet, *godo.Response, error) {
		droplet := newFakeDroplet()
		resp := newFakeOKResponse()
		return droplet, resp, nil
	}
	client := newFakeClient(fake)
	instances := newInstances(client, "nyc1")

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

	addresses, err := instances.NodeAddressesByProviderID(context.TODO(), "digitalocean://123")

	if !reflect.DeepEqual(addresses, expectedAddresses) {
		t.Errorf("unexpected node addresses. got: %v want: %v", addresses, expectedAddresses)
	}

	if err != nil {
		t.Errorf("unexpected err, expected nil. got: %v", err)
	}
}

func TestInstanceID(t *testing.T) {
	fake := &fakeDropletService{}
	fake.listFunc = func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
		droplet := newFakeDroplet()
		droplets := []godo.Droplet{*droplet}

		resp := newFakeOKResponse()
		return droplets, resp, nil
	}

	client := newFakeClient(fake)
	instances := newInstances(client, "nyc1")

	id, err := instances.InstanceID(context.TODO(), "test-droplet")
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}

	if id != "123" {
		t.Errorf("expected id 123, got: %s", id)
	}
}

func TestInstanceType(t *testing.T) {
	fake := &fakeDropletService{}
	fake.listFunc = func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
		droplet := newFakeDroplet()
		droplets := []godo.Droplet{*droplet}

		resp := newFakeOKResponse()
		return droplets, resp, nil
	}

	client := newFakeClient(fake)
	instances := newInstances(client, "nyc1")

	instanceType, err := instances.InstanceType(context.TODO(), "test-droplet")
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}

	if instanceType != "2gb" {
		t.Errorf("expected type 2gb, got: %s", instanceType)
	}
}

func Test_InstanceShutdownByProviderID(t *testing.T) {
	fake := &fakeDropletService{}
	fake.getFunc = func(ctx context.Context, dropletID int) (*godo.Droplet, *godo.Response, error) {
		droplet := newFakeShutdownDroplet()
		resp := newFakeOKResponse()
		return droplet, resp, nil
	}

	client := newFakeClient(fake)
	instances := newInstances(client, "nyc1")

	shutdown, err := instances.InstanceShutdownByProviderID(context.TODO(), "digitalocean://123")
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	if !shutdown {
		t.Errorf("expected node to be shutdown, but it wasn't")
	}
}

func Test_dropletIDFromProviderID(t *testing.T) {
	testcases := []struct {
		name       string
		providerID string
		dropletID  string
		err        error
	}{
		{
			"valid providerID",
			"digitalocean://12345",
			"12345",
			nil,
		},
		{
			"invalid providerID - empty string",
			"",
			"",
			errors.New("providerID cannot be empty string"),
		},
		{
			"invalid providerID - wrong format",
			"digitalocean:/12345",
			"",
			errors.New("unexpected providerID format: digitalocean:/12345, format should be: digitalocean://12345"),
		},
		{
			"invalid providerID - wrong provider name",
			"do://12345",
			"",
			errors.New("provider name from providerID should be digitalocean: do://12345"),
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			dropletID, err := dropletIDFromProviderID(testcase.providerID)
			if dropletID != testcase.dropletID {
				t.Errorf("actual droplet ID: %s", dropletID)
				t.Errorf("expected droplet ID: %s", testcase.dropletID)
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
