package do

import (
	"reflect"
	"testing"

	"github.com/digitalocean/godo"
	"github.com/digitalocean/godo/context"
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

	expected := cloudprovider.Zone{Region: "test-region"}

	actual, err := zones.GetZoneByNodeName("test-droplet")

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("unexpected region. got: %v want: %v", actual, expected)
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

	expected := cloudprovider.Zone{Region: "test-region"}

	actual, err := zones.GetZoneByProviderID("123")

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("unexpected region. got: %v want: %v", actual, expected)
	}

	if err != nil {
		t.Errorf("unexpected err, expected nil. got: %v", err)
	}
}
