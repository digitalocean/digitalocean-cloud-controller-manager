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
	"fmt"

	"k8s.io/kubernetes/pkg/cloudprovider"
)

type region struct{}

func newZones() cloudprovider.Zones {
	return region{}
}

// GetZone returns a cloudprovider.Zone by fetching the droplet
// metadata API for the currently running region. GetZone
// will only fill the Region field of cloudprovider.Zone since
// there's no DO related data to fill it with.
func (r region) GetZone() (cloudprovider.Zone, error) {
	region, err := dropletRegion()
	if err != nil {
		return cloudprovider.Zone{}, fmt.Errorf("failed to get droplet region: %v", err)
	}

	return cloudprovider.Zone{Region: region}, nil
}
