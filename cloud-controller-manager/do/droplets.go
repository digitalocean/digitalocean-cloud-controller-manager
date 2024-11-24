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
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/digitalocean/godo"
	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
)

const (
	dropletShutdownStatus = "off"
)

// instances implements the InstancesV2() interface
type instances struct {
	region    string
	resources *resources
}

func newInstances(resources *resources, region string) cloudprovider.InstancesV2 {
	return &instances{
		resources: resources,
		region:    region,
	}
}

// cloudprovider.InstancesV2 methods
// InstancesV2 require ProviderID to be present, so the interface methods all use providerID to get droplet.

func (i *instances) InstanceExists(ctx context.Context, node *v1.Node) (bool, error) {
	dropletID, err := dropletIDFromProviderID(node.Spec.ProviderID)
	if err != nil {
		return false, fmt.Errorf("determining droplet ID from providerID: %s", err.Error())
	}

	// NOTE: when false is returned with no error, the instance will be
	// immediately deleted by the cloud controller manager.

	_, err = dropletByID(ctx, i.resources.gclient, dropletID)
	if err == nil {
		return true, nil
	}

	godoErr, ok := err.(*godo.ErrorResponse)
	if !ok {
		return false, fmt.Errorf("unexpected error type %T from godo: %s", err, err)
	}

	if godoErr.Response.StatusCode != http.StatusNotFound {
		return false, fmt.Errorf("error checking if instance exists: %s", err)
	}

	return false, nil
}

func (i *instances) InstanceShutdown(ctx context.Context, node *v1.Node) (bool, error) {
	dropletID, err := dropletIDFromProviderID(node.Spec.ProviderID)
	if err != nil {
		return false, fmt.Errorf("determining droplet ID from providerID: %s", err.Error())
	}

	droplet, err := dropletByID(ctx, i.resources.gclient, dropletID)
	if err != nil {
		return false, fmt.Errorf("getting droplet by ID: %s: ", err.Error())
	}
	if droplet == nil {
		return false, fmt.Errorf("droplet %d for node %s does not exist", dropletID, node.Name)
	}

	return droplet.Status == dropletShutdownStatus, nil
}

func (i *instances) InstanceMetadata(ctx context.Context, node *v1.Node) (*cloudprovider.InstanceMetadata, error) {
	dropletID, err := dropletIDFromProviderID(node.Spec.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("determining droplet ID from providerID: %s", err.Error())
	}

	droplet, err := dropletByID(ctx, i.resources.gclient, dropletID)
	if err != nil {
		return nil, fmt.Errorf("getting droplet by ID: %s: ", err.Error())
	}
	if droplet == nil {
		return nil, fmt.Errorf("droplet %d for node %s does not exist", dropletID, node.Name)
	}
	nodeAddrs, err := nodeAddresses(droplet)
	if err != nil {
		return nil, fmt.Errorf("getting node addresses of droplet %d for node %s: %s", dropletID, node.Name, err.Error())
	}
	return &cloudprovider.InstanceMetadata{
		ProviderID:    node.Spec.ProviderID, // the providerID may or may not be present according to the interface doc. However, we set this from kubelet.
		InstanceType:  droplet.SizeSlug,
		Region:        droplet.Region.Slug,
		NodeAddresses: nodeAddrs,
	}, nil
}

// dropletByID returns a *godo.Droplet value for the droplet identified by id.
func dropletByID(ctx context.Context, client *godo.Client, id int) (*godo.Droplet, error) {
	droplet, _, err := client.Droplets.Get(ctx, id)
	return droplet, err
}

// dropletIDFromProviderID returns a droplet's ID from providerID.
//
// The providerID spec should be retrievable from the Kubernetes
// node object. The expected format is: digitalocean://droplet-id
func dropletIDFromProviderID(providerID string) (int, error) {
	if providerID == "" {
		return 0, errors.New("provider ID cannot be empty")
	}

	const prefix = "digitalocean://"

	if !strings.HasPrefix(providerID, prefix) {
		return 0, fmt.Errorf("provider ID %q is missing prefix %q", providerID, prefix)
	}

	provIDNum := strings.TrimPrefix(providerID, prefix)
	if provIDNum == "" {
		return 0, errors.New("provider ID number cannot be empty")
	}

	dropletID, err := strconv.Atoi(provIDNum)
	if err != nil {
		return 0, fmt.Errorf("failed to convert provider ID number %q: %s", provIDNum, err)
	}

	return dropletID, nil
}
