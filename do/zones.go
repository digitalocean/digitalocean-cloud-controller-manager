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
