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
	"github.com/digitalocean/godo"
	"github.com/digitalocean/godo/context"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/cloudprovider"
)

type zones struct {
	client *godo.Client
	region string
}

func newZones(client *godo.Client, region string) cloudprovider.Zones {
	return zones{client, region}
}

// GetZone returns a cloudprovider.Zone for the currently running node.
// GetZone will only fill the Region field of cloudprovider.Zone for DO.
func (z zones) GetZone() (cloudprovider.Zone, error) {
	return cloudprovider.Zone{Region: z.region}, nil
}

// GetZoneByProviderID returns the Zone containing the current zone and
// locality region of the node specified by providerId. GetZoneByProviderID
// will only fill the Region field of cloudprovider.Zone for DO.
func (z zones) GetZoneByProviderID(providerID string) (cloudprovider.Zone, error) {
	d, err := dropletByID(context.Background(), z.client, providerID)
	if err != nil {
		return cloudprovider.Zone{}, err
	}

	return cloudprovider.Zone{Region: d.Region.Name}, nil
}

// GetZoneByNodeName returns the Zone containing the current zone and locality
// region of the node specified by node name. GetZoneByNodeName will only fill
// the Region field of cloudprovider.Zone for DO.
func (z zones) GetZoneByNodeName(nodeName types.NodeName) (cloudprovider.Zone, error) {
	d, err := dropletByName(context.Background(), z.client, nodeName)
	if err != nil {
		return cloudprovider.Zone{}, err
	}

	return cloudprovider.Zone{Region: d.Region.Name}, nil
}
