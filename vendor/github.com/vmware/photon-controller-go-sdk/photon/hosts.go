// Copyright (c) 2016 VMware, Inc. All Rights Reserved.
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

// Contains functionality for hosts API.
type HostsAPI struct {
	client *Client
}

var hostUrl string = rootUrl + "/infrastructure/hosts"

// Sets host's availability zone.
func (api *HostsAPI) SetAvailabilityZone(id string, availabilityZone *HostSetAvailabilityZoneOperation) (task *Task, err error) {
	body, err := json.Marshal(availabilityZone)
	if err != nil {
		return
	}

	res, err := api.client.restClient.Post(
		api.client.Endpoint+hostUrl+"/"+id+"/set_availability_zone",
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

// Gets all tasks with the specified host ID, using options to filter the results.
// If options is nil, no filtering will occur.
func (api *HostsAPI) GetTasks(id string, options *TaskGetOptions) (result *TaskList, err error) {
	uri := api.client.Endpoint + hostUrl + "/" + id + "/tasks"
	if options != nil {
		uri += getQueryString(options)
	}
	res, err := api.client.restClient.GetList(api.client.Endpoint, uri, api.client.options.TokenOptions)
	if err != nil {
		return
	}

	result = &TaskList{}
	err = json.Unmarshal(res, result)
	return
}

// provision the host with the specified id
func (api *HostsAPI) Provision(id string) (task *Task, err error) {
	body := []byte{}
	res, err := api.client.restClient.Post(
		api.client.Endpoint+hostUrl+"/"+id+"/provision",
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
