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

	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
)

type zones struct {
	resources *resources
	region    string
}

func newZones(resources *resources, region string) cloudprovider.Zones {
	return zones{
		resources: resources,
		region:    region,
	}
}

// GetZone returns a cloudprovider.Zone from the region of z. GetZone only sets
// the Region field of the returned cloudprovider.Zone.
//
// Kuberenetes uses this method to get the region that the program is running in.
func (z zones) GetZone(ctx context.Context) (cloudprovider.Zone, error) {
	return cloudprovider.Zone{Region: z.region}, nil
}

// GetZoneByProviderID returns a cloudprovider.Zone from the droplet identified
// by providerID. GetZoneByProviderID only sets the Region field of the
// returned cloudprovider.Zone.
func (z zones) GetZoneByProviderID(ctx context.Context, providerID string) (cloudprovider.Zone, error) {
	id, err := dropletIDFromProviderID(providerID)
	if err != nil {
		return cloudprovider.Zone{}, err
	}

	d, found := z.resources.DropletByID(id)
	if !found {
		return cloudprovider.Zone{}, cloudprovider.InstanceNotFound
	}

	return cloudprovider.Zone{Region: d.Region.Slug}, nil
}

// GetZoneByNodeName returns a cloudprovider.Zone from the droplet identified
// by nodeName. GetZoneByNodeName only sets the Region field of the returned
// cloudprovider.Zone.
func (z zones) GetZoneByNodeName(ctx context.Context, nodeName types.NodeName) (cloudprovider.Zone, error) {
	d, found := z.resources.DropletByName(string(nodeName))
	if !found {
		return cloudprovider.Zone{}, cloudprovider.InstanceNotFound
	}

	return cloudprovider.Zone{Region: d.Region.Slug}, nil
}
