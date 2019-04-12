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
	"errors"
	"fmt"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
)

const (
	dropletShutdownStatus = "off"
)

type instances struct {
	region    string
	resources *resources
}

func newInstances(resources *resources, region string) cloudprovider.Instances {
	return &instances{
		resources: resources,
		region:    region,
	}
}

// NodeAddresses returns all the valid addresses of the droplet identified by
// nodeName. Only the public/private IPv4 addresses are considered for now.
//
// When nodeName identifies more than one droplet, only the first will be
// considered.
func (i *instances) NodeAddresses(_ context.Context, nodeName types.NodeName) ([]v1.NodeAddress, error) {
	droplet, found := i.resources.DropletByName(string(nodeName))
	if !found {
		return nil, cloudprovider.InstanceNotFound
	}

	return nodeAddresses(droplet)
}

// NodeAddressesByProviderID returns all the valid addresses of the droplet
// identified by providerID. Only the public/private IPv4 addresses will be
// considered for now.
func (i *instances) NodeAddressesByProviderID(_ context.Context, providerID string) ([]v1.NodeAddress, error) {
	id, err := dropletIDFromProviderID(providerID)
	if err != nil {
		return nil, err
	}

	droplet, found := i.resources.DropletByID(id)
	if !found {
		return nil, cloudprovider.InstanceNotFound
	}

	return nodeAddresses(droplet)
}

// ExternalID returns the cloud provider ID of the droplet identified by
// nodeName. If the droplet does not exist or is no longer running, the
// returned error will be cloudprovider.InstanceNotFound.
//
// When nodeName identifies more than one droplet, only the first will be
// considered.
func (i *instances) ExternalID(ctx context.Context, nodeName types.NodeName) (string, error) {
	return i.InstanceID(ctx, nodeName)
}

// InstanceID returns the cloud provider ID of the droplet identified by nodeName.
func (i *instances) InstanceID(_ context.Context, nodeName types.NodeName) (string, error) {
	droplet, found := i.resources.DropletByName(string(nodeName))
	if !found {
		return "", cloudprovider.InstanceNotFound
	}

	return strconv.Itoa(droplet.ID), nil
}

// InstanceType returns the type of the droplet identified by name.
func (i *instances) InstanceType(_ context.Context, nodeName types.NodeName) (string, error) {
	droplet, found := i.resources.DropletByName(string(nodeName))
	if !found {
		return "", cloudprovider.InstanceNotFound
	}

	return droplet.SizeSlug, nil
}

// InstanceTypeByProviderID returns the type of the droplet identified by providerID.
func (i *instances) InstanceTypeByProviderID(_ context.Context, providerID string) (string, error) {
	id, err := dropletIDFromProviderID(providerID)
	if err != nil {
		return "", err
	}

	droplet, found := i.resources.DropletByID(id)
	if !found {
		return "", cloudprovider.InstanceNotFound
	}

	return droplet.SizeSlug, err
}

// AddSSHKeyToAllInstances is not implemented; it always returns an error.
func (i *instances) AddSSHKeyToAllInstances(_ context.Context, _ string, _ []byte) error {
	return errors.New("not implemented")
}

// CurrentNodeName returns hostname as a NodeName value.
func (i *instances) CurrentNodeName(_ context.Context, hostname string) (types.NodeName, error) {
	return types.NodeName(hostname), nil
}

// InstanceExistsByProviderID returns true if the droplet identified by
// providerID is running.
func (i *instances) InstanceExistsByProviderID(_ context.Context, providerID string) (bool, error) {
	// NOTE: when false is returned with no error, the instance will be
	// immediately deleted by the cloud controller manager.

	id, err := dropletIDFromProviderID(providerID)
	if err != nil {
		return false, err
	}

	_, found := i.resources.DropletByID(id)
	if !found {
		// this is the case where we know the droplet is gone so we return false
		// with no err to delete it
		return false, nil
	}

	return true, nil
}

// InstanceShutdownByProviderID returns true if the droplet is turned off
func (i *instances) InstanceShutdownByProviderID(_ context.Context, providerID string) (bool, error) {
	id, err := dropletIDFromProviderID(providerID)
	if err != nil {
		return false, fmt.Errorf("error getting droplet ID from provider ID %s, err: %v", providerID, err)
	}

	droplet, found := i.resources.DropletByID(id)
	if !found {
		return false, cloudprovider.InstanceNotFound
	}

	return droplet.Status == dropletShutdownStatus, nil
}

// dropletIDFromProviderID returns a droplet's ID from providerID.
//
// The providerID spec should be retrievable from the Kubernetes
// node object. The expected format is: digitalocean://droplet-id
func dropletIDFromProviderID(providerID string) (int, error) {
	if providerID == "" {
		return 0, errors.New("providerID cannot be empty string")
	}

	split := strings.Split(providerID, "/")
	if len(split) != 3 {
		return 0, fmt.Errorf("unexpected providerID format: %s, format should be: digitalocean://12345", providerID)
	}

	// since split[0] is actually "digitalocean:"
	if strings.TrimSuffix(split[0], ":") != providerName {
		return 0, fmt.Errorf("provider name from providerID should be digitalocean: %s", providerID)
	}

	return strconv.Atoi(split[2])
}
