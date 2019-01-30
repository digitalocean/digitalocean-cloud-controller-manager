// Copyright (c) 2017 VMware, Inc. All Rights Reserved.
//
// This product is licensed to you under the Apache License, Version 2.0 (the "License").
// You may not use this product except in compliance with the License.
//
// This product may include a number of subcomponents with separate copyright notices and
// license terms. Your use of these subcomponents is subject to the terms and conditions
// of the subcomponent's license, as noted in the LICENSE file.

package photon

import (
	"bytes"
	"encoding/json"
)

// Contains functionality for networks API.
type NetworksAPI struct {
	client *Client
}

var networkUrl string = rootUrl + "/networks/"

// Gets a network with the specified ID.
func (api *NetworksAPI) Get(id string) (network *Network, err error) {
	res, err := api.client.restClient.Get(api.client.Endpoint+networkUrl+id, api.client.options.TokenOptions)
	if err != nil {
		return
	}
	defer res.Body.Close()
	res, err = getError(res)
	if err != nil {
		return
	}
	var result Network
	err = json.NewDecoder(res.Body).Decode(&result)
	return &result, err
}

// Updates network's attributes.
func (api *NetworksAPI) UpdateNetwork(id string, networkSpec *NetworkUpdateSpec) (task *Task, err error) {
	body, err := json.Marshal(networkSpec)
	if err != nil {
		return
	}

	res, err := api.client.restClient.Patch(
		api.client.Endpoint+networkUrl+id,
		"application/json",
		bytes.NewReader(body),
		api.client.options.TokenOptions)
	if err != nil {
		return
	}

	defer res.Body.Close()
	task, err = getTask(getError(res))
	return
}

// Deletes a network with specified ID.
func (api *NetworksAPI) Delete(networkID string) (task *Task, err error) {
	res, err := api.client.restClient.Delete(api.client.Endpoint+networkUrl+networkID, api.client.options.TokenOptions)
	if err != nil {
		return
	}

	defer res.Body.Close()
	task, err = getTask(getError(res))
	return
}

// Creates a subnet on the specified network.
func (api *NetworksAPI) CreateSubnet(networkID string, spec *SubnetCreateSpec) (task *Task, err error) {
	body, err := json.Marshal(spec)
	if err != nil {
		return
	}
	res, err := api.client.restClient.Post(
		api.client.Endpoint+networkUrl+networkID+"/subnets",
		"application/json",
		bytes.NewReader(body),
		api.client.options.TokenOptions)
	if err != nil {
		return
	}
	defer res.Body.Close()
	task, err = getTask(getError(res))
	return
}

// Gets subnets for network with the specified ID, using options to filter the results.
// If options is nil, no filtering will occur.
func (api *NetworksAPI) GetSubnets(networkID string, options *SubnetGetOptions) (result *Subnets, err error) {
	uri := api.client.Endpoint + networkUrl + networkID + "/subnets"
	if options != nil {
		uri += getQueryString(options)
	}
	res, err := api.client.restClient.GetList(api.client.Endpoint, uri, api.client.options.TokenOptions)
	if err != nil {
		return
	}

	result = &Subnets{}
	err = json.Unmarshal(res, result)
	return
}
