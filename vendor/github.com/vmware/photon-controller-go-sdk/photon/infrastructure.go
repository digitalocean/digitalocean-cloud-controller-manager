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

// Contains functionality for Infrastructure API.
type InfraAPI struct {
	client *Client
}

var infraUrl string = rootUrl + "/infrastructure"

// Synchronizes hosts configurations
func (api *InfraAPI) SyncHostsConfig() (task *Task, err error) {
	res, err := api.client.restClient.Post(
		api.client.Endpoint+infraUrl+"/sync-hosts-config",
		"application/json",
		bytes.NewReader([]byte("")),
		api.client.options.TokenOptions)
	if err != nil {
		return
	}
	defer res.Body.Close()

	task, err = getTask(getError(res))
	return
}

// Set image datastores.
func (api *InfraAPI) SetImageDatastores(imageDatastores *ImageDatastores) (task *Task, err error) {
	body, err := json.Marshal(imageDatastores)
	if err != nil {
		return
	}

	res, err := api.client.restClient.Post(
		api.client.Endpoint+infraUrl+"/image-datastores",
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
