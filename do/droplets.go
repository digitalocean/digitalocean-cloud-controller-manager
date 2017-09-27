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
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/cloudprovider"

	"github.com/digitalocean/godo"
	"github.com/digitalocean/godo/context"
)

// instances Implements cloudprovider.Instances
type instances struct {
	client *godo.Client
	region string
}

func newInstances(client *godo.Client, region string) cloudprovider.Instances {
	return &instances{client, region}
}

// NodeAddresses returns all the valid addresses of the specified node
// For DO, this is the public/private ipv4 addresses only for now
func (i *instances) NodeAddresses(nodeName types.NodeName) ([]v1.NodeAddress, error) {
	droplet, err := i.dropletByName(context.TODO(), nodeName)
	if err != nil {
		return nil, err
	}

	return nodeAddresses(droplet)
}

// NodeAddressesByProviderID returns all the valid addresses of the specified
// node by providerId. For DO this is the public/private ipv4 addresses for now.
func (i *instances) NodeAddressesByProviderID(providerId string) ([]v1.NodeAddress, error) {
	// we can technically get all the required data from metadata service
	droplet, err := i.dropletById(context.TODO(), providerId)
	if err != nil {
		return nil, err
	}

	return nodeAddresses(droplet)
}

// nodeAddresses extracts droplet data into []v1.NodeAddress
func nodeAddresses(droplet *godo.Droplet) ([]v1.NodeAddress, error) {
	var addresses []v1.NodeAddress
	addresses = append(addresses, v1.NodeAddress{Type: v1.NodeHostName, Address: droplet.Name})

	privateIP, err := droplet.PrivateIPv4()
	if err != nil || privateIP == "" {
		return nil, fmt.Errorf("could not get private ip: %v", err)
	}
	addresses = append(addresses, v1.NodeAddress{Type: v1.NodeInternalIP, Address: privateIP})

	publicIP, err := droplet.PublicIPv4()
	if err != nil || publicIP == "" {
		return nil, fmt.Errorf("could not get public ip: %v", err)
	}
	addresses = append(addresses, v1.NodeAddress{Type: v1.NodeExternalIP, Address: publicIP})

	return addresses, nil
}

// ExternalID returns the cloud provider ID of the node with the specified NodeName.
// Note that if the instance does not exist or is no longer running, we must return ("", cloudprovider.InstanceNotFound)
func (i *instances) ExternalID(nodeName types.NodeName) (string, error) {
	return i.InstanceID(nodeName)
}

// InstanceID returns the cloud provider ID of the node with the specified NodeName.
func (i *instances) InstanceID(nodeName types.NodeName) (string, error) {
	droplet, err := i.dropletByName(context.TODO(), nodeName)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(droplet.ID), nil
}

// InstanceType returns the type of the specified instance.
// Droplet types are defined by amount of memory available
func (i *instances) InstanceType(name types.NodeName) (string, error) {
	droplet, err := i.dropletByName(context.TODO(), name)
	if err != nil {
		return "", err
	}

	return droplet.SizeSlug, nil
}

// InstanceTypeByProviderID returns the type of the specified instance.
func (i *instances) InstanceTypeByProviderID(providerId string) (string, error) {
	droplet, err := i.dropletById(context.TODO(), providerId)
	if err != nil {
		return "", err
	}

	return droplet.SizeSlug, err
}

// AddSSHKeyToAllInstances adds an SSH public key as a legal identity for all instances
// expected format for the key is standard ssh-keygen format: <protocol> <blob>
func (i *instances) AddSSHKeyToAllInstances(user string, keyData []byte) error {
	return errors.New("not implemented yet")
}

// CurrentNodeName returns the name of the node we are currently running on
// On most clouds (e.g. GCE) this is the hostname, so we provide the hostname
func (i *instances) CurrentNodeName(hostname string) (types.NodeName, error) {
	return types.NodeName(hostname), nil
}

// dropletById returns the godo Droplet type corresponding to the provided id
func (i *instances) dropletById(ctx context.Context, id string) (*godo.Droplet, error) {
	intId, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("error converting droplet id to string: %v", err)
	}

	droplet, resp, err := i.client.Droplets.Get(ctx, intId)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DO API returned non-200 status code: %d", resp.StatusCode)
	}

	return droplet, nil
}

// dropletByName returns the godo Droplet type corresponding to the node name
// since we can only get droplets by id, we do a list of all droplets and return
// the first one that matches the provided name
func (i *instances) dropletByName(ctx context.Context, nodeName types.NodeName) (*godo.Droplet, error) {
	// TODO (andrewsykim): list by tag once a tagging format is determined
	droplets, err := allDropletList(ctx, i.client)
	if err != nil {
		return nil, err
	}

	for _, droplet := range droplets {
		if droplet.Name == string(nodeName) {
			return &droplet, nil
		}
	}

	return nil, cloudprovider.InstanceNotFound
}
