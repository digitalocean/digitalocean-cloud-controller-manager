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
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/digitalocean/godo"
)

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

func TestMemResources_DropletByID(t *testing.T) {
	droplet := newFakeDroplet()
	resources := &memResources{
		dropletIDMap: map[int]*godo.Droplet{
			droplet.ID: droplet,
		},
	}

	foundDroplet, found, err := resources.DropletByID(context.Background(), droplet.ID)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if want, got := true, found; want != got {
		t.Errorf("incorrect found\nwant: %#v\n got: %#v", want, got)
	}
	if want, got := droplet, foundDroplet; !reflect.DeepEqual(want, got) {
		t.Errorf("incorrect droplet\nwant: %#v\n got: %#v", want, got)
	}

	_, found, err = resources.DropletByID(context.Background(), 1000)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if want, got := false, found; want != got {
		t.Errorf("incorrect found\nwant: %#v\n got: %#v", want, got)
	}
}

func TestMemResources_DropletByName(t *testing.T) {
	droplet := newFakeDroplet()
	resources := &memResources{
		dropletNameMap: map[string]*godo.Droplet{
			droplet.Name: droplet,
		},
	}

	foundDroplet, found, err := resources.DropletByName(context.Background(), droplet.Name)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if want, got := true, found; want != got {
		t.Errorf("incorrect found\nwant: %#v\n got: %#v", want, got)
	}
	if want, got := droplet, foundDroplet; !reflect.DeepEqual(want, got) {
		t.Errorf("incorrect droplet\nwant: %#v\n got: %#v", want, got)
	}

	_, found, err = resources.DropletByName(context.Background(), "missing")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if want, got := false, found; want != got {
		t.Errorf("incorrect found\nwant: %#v\n got: %#v", want, got)
	}
}

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

func TestCacheAPI_Reload(t *testing.T) {
	expired := false
	ttl := 1 * time.Minute
	fake := &fakeDropletService{}
	fake.listFunc = func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
		var droplet godo.Droplet
		if !expired {
			droplet.ID = 1
			droplet.Name = "one"
		} else {
			droplet.ID = 2
			droplet.Name = "two"
		}

		droplets := []godo.Droplet{droplet}

		resp := newFakeOKResponse()
		return droplets, resp, nil
	}
	fakeClient := newFakeClient(fake)
	resources := newCachedAPI(ttl, fakeClient)
	resources.now = func() time.Time {
		if !expired {
			return time.Unix(0, 0)
		}

		return time.Unix(0, 0).Add(ttl).Add(1 * time.Second)
	}

	err := resources.Reload(context.Background())
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	expectedResources := &memResources{
		dropletIDMap: map[int]*godo.Droplet{
			1: {
				ID:   1,
				Name: "one",
			},
		},
		dropletNameMap: map[string]*godo.Droplet{
			"one": {
				ID:   1,
				Name: "one",
			},
		},
	}
	if want, got := expectedResources, resources.memResources; !reflect.DeepEqual(want, got) {
		t.Errorf("incorrect resources\nwant: %#v\n got: %#v", want, got)
	}
	expectedExpiration := time.Unix(0, 0).Add(ttl)
	if want, got := expectedExpiration, resources.expiration; want != got {
		t.Errorf("incorrect expiration\nwant: %#v\n got: %#v", want, got)
	}

	expired = true

	err = resources.Reload(context.Background())
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	expectedResources = &memResources{
		dropletIDMap: map[int]*godo.Droplet{
			2: {
				ID:   2,
				Name: "two",
			},
		},
		dropletNameMap: map[string]*godo.Droplet{
			"two": {
				ID:   2,
				Name: "two",
			},
		},
	}
	if want, got := expectedResources, resources.memResources; !reflect.DeepEqual(want, got) {
		t.Errorf("incorrect resources\nwant: %#v\n got: %#v", want, got)
	}
}
