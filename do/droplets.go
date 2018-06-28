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
	"net/http"
	"strconv"
	"strings"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/cloudprovider"

	"github.com/digitalocean/godo"
)

type instances struct {
	client *godo.Client
	region string
}

func newInstances(client *godo.Client, region string) cloudprovider.Instances {
	return &instances{client, region}
}

// NodeAddresses returns all the valid addresses of the droplet identified by
// nodeName. Only the public/private IPv4 addresses are considered for now.
//
// When nodeName identifies more than one droplet, only the first will be
// considered.
func (i *instances) NodeAddresses(ctx context.Context, nodeName types.NodeName) ([]v1.NodeAddress, error) {
	droplet, err := dropletByName(ctx, i.client, nodeName)
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

	droplet, err := dropletByID(ctx, i.client, id)
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
	droplet, err := dropletByName(ctx, i.client, nodeName)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(droplet.ID), nil
}

// InstanceType returns the type of the droplet identified by name.
func (i *instances) InstanceType(ctx context.Context, name types.NodeName) (string, error) {
	droplet, err := dropletByName(ctx, i.client, name)
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

	droplet, err := dropletByID(ctx, i.client, id)
	if err != nil {
		return "", err
	}

	return droplet.SizeSlug, err
}

// AddSSHKeyToAllInstances is not implemented; it always returns an error.
func (i *instances) AddSSHKeyToAllInstances(ctx context.Context, user string, keyData []byte) error {
	return errors.New("not implemented")
}

// CurrentNodeName returns hostname as a NodeName value.
func (i *instances) CurrentNodeName(ctx context.Context, hostname string) (types.NodeName, error) {
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

	_, err = dropletByID(ctx, i.client, id)
	if err == nil {
		return true, nil
	}

	godoErr, ok := err.(*godo.ErrorResponse)
	if !ok {
		return false, fmt.Errorf("unexpected error type from godo: %T, msg: %v", err, err)
	}

	if godoErr.Response.StatusCode != http.StatusNotFound {
		return false, fmt.Errorf("error checking if instance exists: %v", err)
	}

	return false, nil
}

// InstanceShutdownByProviderID returns true if the droplet is turned off
func (i *instances) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	return false, errors.New("not implemented yet")
}

// dropletByID returns a *godo.Droplet value for the droplet identified by id.
func dropletByID(ctx context.Context, client *godo.Client, id string) (*godo.Droplet, error) {
	intID, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("error converting droplet id to string: %v", err)
	}

	droplet, _, err := client.Droplets.Get(ctx, intID)
	if err != nil {
		return nil, err
	}

	return droplet, nil
}

// dropletByName returns a *godo.Droplet for the droplet identified by nodeName.
//
// When nodeName identifies more than one droplet, only the first will be
// considered.
func dropletByName(ctx context.Context, client *godo.Client, nodeName types.NodeName) (*godo.Droplet, error) {
	// TODO (andrewsykim): list by tag once a tagging format is determined
	droplets, err := allDropletList(ctx, client)
	if err != nil {
		return nil, err
	}

	for _, droplet := range droplets {
		if droplet.Name == string(nodeName) {
			return &droplet, nil
		}
		addresses, _ := nodeAddresses(&droplet)
		for _, address := range addresses {
			if address.Address == string(nodeName) {
				return &droplet, nil
			}
		}
	}

	return nil, cloudprovider.InstanceNotFound
}

// dropletIDFromProviderID returns a droplet's ID from providerID.
//
// The providerID spec should be retrievable from the Kubernetes
// node object. The expected format is: digitalocean://droplet-id
func dropletIDFromProviderID(providerID string) (string, error) {
	if providerID == "" {
		return "", errors.New("providerID cannot be empty string")
	}

	split := strings.Split(providerID, "/")
	if len(split) != 3 {
		return "", fmt.Errorf("unexpected providerID format: %s, format should be: digitalocean://12345", providerID)
	}

	// since split[0] is actually "digitalocean:"
	if strings.TrimSuffix(split[0], ":") != providerName {
		return "", fmt.Errorf("provider name from providerID should be digitalocean: %s", providerID)
	}

	return split[2], nil
}
