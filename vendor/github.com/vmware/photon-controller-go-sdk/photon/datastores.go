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
	"encoding/json"
)

// DatastoresAPI is used by the client
type DatastoresAPI struct {
	client *Client
}

var datastoresURL = rootUrl + "/infrastructure/datastores"

// GetAll returns all datastores; requires system administrator privileges
func (api *DatastoresAPI) GetAll() (result *Datastores, err error) {
	res, err := api.client.restClient.Get(api.client.Endpoint+datastoresURL, api.client.options.TokenOptions)
	if err != nil {
		return
	}
	defer res.Body.Close()
	res, err = getError(res)
	if err != nil {
		return
	}
	result = &Datastores{}
	err = json.NewDecoder(res.Body).Decode(result)
	return result, err
}

// Get returns a single datastore with the specified ID; requires system administrator privileges
func (api *DatastoresAPI) Get(id string) (datastore *Datastore, err error) {
	res, err := api.client.restClient.Get(api.client.Endpoint+datastoresURL+"/"+id, api.client.options.TokenOptions)
	if err != nil {
		return
	}
	defer res.Body.Close()
	res, err = getError(res)
	if err != nil {
		return
	}
	var result Datastore
	err = json.NewDecoder(res.Body).Decode(&result)
	return &result, err
}
