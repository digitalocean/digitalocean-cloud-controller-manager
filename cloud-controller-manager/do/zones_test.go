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

	"github.com/digitalocean/godo"
	"k8s.io/kubernetes/pkg/cloudprovider"
)

var _ cloudprovider.Zones = new(zones)

func TestZones_GetZoneByNodeName(t *testing.T) {
	fake := &fakeDropletService{}
	fake.listFunc = func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
		droplet := newFakeDroplet()
		droplets := []godo.Droplet{*droplet}

		resp := newFakeOKResponse()
		return droplets, resp, nil
	}

	client := newFakeClient(fake)
	zones := newZones(client, "nyc1")

	expected := cloudprovider.Zone{Region: "test1"}

	actual, err := zones.GetZoneByNodeName(context.TODO(), "test-droplet")

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("unexpected region. got: %+v want: %+v", actual, expected)
	}

	if err != nil {
		t.Errorf("unexpected err, expected nil. got: %v", err)
	}
}

func TestZones_GetZoneByProviderID(t *testing.T) {
	fake := &fakeDropletService{}

	fake.getFunc = func(ctx context.Context, dropletID int) (*godo.Droplet, *godo.Response, error) {
		droplet := newFakeDroplet()
		resp := newFakeOKResponse()
		return droplet, resp, nil
	}
	client := newFakeClient(fake)
	zones := newZones(client, "nyc1")

	expected := cloudprovider.Zone{Region: "test1"}

	actual, err := zones.GetZoneByProviderID(context.TODO(), "digitalocean://123")

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("unexpected region. got: %+v want: %+v", actual, expected)
	}

	if err != nil {
		t.Errorf("unexpected err, expected nil. got: %v", err)
	}
}
