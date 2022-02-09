/*
Copyright 2020 DigitalOcean

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
func (i *instances) NodeAddresses(ctx context.Context, nodeName types.NodeName) ([]v1.NodeAddress, error) {
	droplet, err := dropletByName(ctx, i.resources.gclient, nodeName)
	if err != nil {
		return nil, err
	}

	return nodeAddresses(droplet)
}

// NodeAddressesByProviderID returns all the valid addresses of the droplet
// identified by providerID. Only the public/private IPv4 addresses will be
// considered for now.
func (i *instances) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	id, err := dropletIDFromProviderID(providerID)
	if err != nil {
		return nil, err
	}

	droplet, err := dropletByID(ctx, i.resources.gclient, id)
	if err != nil {
		return nil, err
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
func (i *instances) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	droplet, err := dropletByName(ctx, i.resources.gclient, nodeName)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(droplet.ID), nil
}

// InstanceType returns the type of the droplet identified by name.
func (i *instances) InstanceType(ctx context.Context, name types.NodeName) (string, error) {
	droplet, err := dropletByName(ctx, i.resources.gclient, name)
	if err != nil {
		return "", err
	}

	return droplet.SizeSlug, nil
}

// InstanceTypeByProviderID returns the type of the droplet identified by providerID.
func (i *instances) InstanceTypeByProviderID(ctx context.Context, providerID string) (string, error) {
	id, err := dropletIDFromProviderID(providerID)
	if err != nil {
		return "", err
	}

	droplet, err := dropletByID(ctx, i.resources.gclient, id)
	if err != nil {
		return "", err
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
func (i *instances) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	// NOTE: when false is returned with no error, the instance will be
	// immediately deleted by the cloud controller manager.

	id, err := dropletIDFromProviderID(providerID)
	if err != nil {
		return false, err
	}

	_, err = dropletByID(ctx, i.resources.gclient, id)
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

// InstanceShutdownByProviderID returns true if the droplet is turned off
func (i *instances) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	dropletID, err := dropletIDFromProviderID(providerID)
	if err != nil {
		return false, fmt.Errorf("error getting droplet ID from provider ID %q: %s", providerID, err)
	}

	droplet, err := dropletByID(ctx, i.resources.gclient, dropletID)
	if err != nil {
		return false, fmt.Errorf("error getting droplet \"%d\" by ID: %s", dropletID, err)
	}

	return droplet.Status == dropletShutdownStatus, nil
}

// dropletByID returns a *godo.Droplet value for the droplet identified by id.
func dropletByID(ctx context.Context, client *godo.Client, id int) (*godo.Droplet, error) {
	droplet, _, err := client.Droplets.Get(ctx, id)
	return droplet, err
}

// dropletByName returns a *godo.Droplet for the droplet identified by nodeName.
//
// When nodeName identifies more than one droplet, only the first will be
// considered.
func dropletByName(ctx context.Context, client *godo.Client, nodeName types.NodeName) (*godo.Droplet, error) {
	// TODO (andrewsykim): list by tag once a tagging format is determined
	needed := []*godo.Droplet{}
	droplets, err := allDropletList(ctx, client)
	if err != nil {
		return nil, err
	}

	for _, droplet := range droplets {
		droplet := droplet
		if droplet.Name == string(nodeName) {
			needed = append(needed, &droplet)
		}
		addresses, _ := nodeAddresses(&droplet)
		for _, address := range addresses {
			if address.Address == string(nodeName) {
				if !alreadyAdded(needed, droplet) {
					needed = append(needed, &droplet)
				}
			}
		}
	}
	if len(needed) == 0 {
		return nil, cloudprovider.InstanceNotFound
	}
	if len(needed) != 1 {
		return nil, fmt.Errorf("multiple droplets with the same node name `%s` exist: %v", nodeName, dropletIDs(needed))
	}

	return needed[0], nil
}

func dropletIDs(needed []*godo.Droplet) []int {
	ids := []int{}
	for _, droplet := range needed {
		ids = append(ids, droplet.ID)
	}
	return ids
}

func alreadyAdded(droplets []*godo.Droplet, droplet godo.Droplet) bool {
	for _, d := range droplets {
		if d.ID == droplet.ID {
			return true
		}
	}
	return false
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
