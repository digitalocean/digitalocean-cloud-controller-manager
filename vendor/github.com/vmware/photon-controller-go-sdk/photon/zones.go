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

// Contains functionality for zones API.
type ZonesAPI struct {
	client *Client
}

var zoneUrl string = rootUrl + "/zones"

// Creates zone.
func (api *ZonesAPI) Create(zoneSpec *ZoneCreateSpec) (task *Task, err error) {
	body, err := json.Marshal(zoneSpec)
	if err != nil {
		return
	}
	res, err := api.client.restClient.Post(
		api.client.Endpoint+zoneUrl,
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

// Gets zone with the specified ID.
func (api *ZonesAPI) Get(id string) (zone *Zone, err error) {
	res, err := api.client.restClient.Get(api.getEntityUrl(id), api.client.options.TokenOptions)
	if err != nil {
		return
	}
	defer res.Body.Close()
	res, err = getError(res)
	if err != nil {
		return
	}
	zone = &Zone{}
	err = json.NewDecoder(res.Body).Decode(zone)
	return
}

// Returns all zones on an photon instance.
func (api *ZonesAPI) GetAll() (result *Zones, err error) {
	uri := api.client.Endpoint + zoneUrl
	res, err := api.client.restClient.GetList(api.client.Endpoint, uri, api.client.options.TokenOptions)
	if err != nil {
		return
	}

	result = &Zones{}
	err = json.Unmarshal(res, result)
	return
}

// Deletes the zone with specified ID.
func (api *ZonesAPI) Delete(id string) (task *Task, err error) {
	res, err := api.client.restClient.Delete(api.client.Endpoint+zoneUrl+"/"+id, api.client.options.TokenOptions)
	if err != nil {
		return
	}
	defer res.Body.Close()
	task, err = getTask(getError(res))
	return
}

// Gets all tasks with the specified zone ID, using options to filter the results.
// If options is nil, no filtering will occur.
func (api *ZonesAPI) GetTasks(id string, options *TaskGetOptions) (result *TaskList, err error) {
	uri := api.client.Endpoint + zoneUrl + "/" + id + "/tasks"
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

func (api *ZonesAPI) getEntityUrl(id string) (url string) {
	return api.client.Endpoint + zoneUrl + "/" + id
}
