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
	"io"
	"io/ioutil"
)

// Contains functionality for projects API.
type ProjectsAPI struct {
	client *Client
}

// Options for GetDisks API.
type DiskGetOptions struct {
	Name string `urlParam:"name"`
}

// Options for GetDisks API.
type VmGetOptions struct {
	Name string `urlParam:"name"`
}

// Options for GetRouters API.
type RouterGetOptions struct {
	Name string `urlParam:"name"`
}

// Options for GetNetworks API.
type NetworkGetOptions struct {
	Name string `urlParam:"name"`
}

var projectUrl string = rootUrl + "/projects/"

// Deletes the project with specified ID. Any VMs, disks, etc., owned by the project must be deleted first.
func (api *ProjectsAPI) Delete(projectID string) (task *Task, err error) {
	res, err := api.client.restClient.Delete(api.client.Endpoint+projectUrl+projectID, api.client.options.TokenOptions)
	if err != nil {
		return
	}
	defer res.Body.Close()
	task, err = getTask(getError(res))
	return
}

// Creates a disk on the specified project.
func (api *ProjectsAPI) CreateDisk(projectID string, spec *DiskCreateSpec) (task *Task, err error) {
	body, err := json.Marshal(spec)
	if err != nil {
		return
	}
	res, err := api.client.restClient.Post(
		api.client.Endpoint+projectUrl+projectID+"/disks",
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

// Gets disks for project with the specified ID, using options to filter the results.
// If options is nil, no filtering will occur.
func (api *ProjectsAPI) GetDisks(projectID string, options *DiskGetOptions) (result *DiskList, err error) {
	uri := api.client.Endpoint + projectUrl + projectID + "/disks"
	if options != nil {
		uri += getQueryString(options)
	}
	res, err := api.client.restClient.GetList(api.client.Endpoint, uri, api.client.options.TokenOptions)
	if err != nil {
		return
	}

	result = &DiskList{}
	err = json.Unmarshal(res, result)
	return
}

// Creates a VM on the specified project.
func (api *ProjectsAPI) CreateVM(projectID string, spec *VmCreateSpec) (task *Task, err error) {
	body, err := json.Marshal(spec)
	if err != nil {
		return
	}
	res, err := api.client.restClient.Post(
		api.client.Endpoint+projectUrl+projectID+"/vms",
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

// Gets all tasks with the specified project ID, using options to filter the results.
// If options is nil, no filtering will occur.
func (api *ProjectsAPI) GetTasks(id string, options *TaskGetOptions) (result *TaskList, err error) {
	uri := api.client.Endpoint + projectUrl + id + "/tasks"
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

// Gets vms for project with the specified ID, using options to filter the results.
// If options is nil, no filtering will occur.
func (api *ProjectsAPI) GetVMs(projectID string, options *VmGetOptions) (result *VMs, err error) {
	uri := api.client.Endpoint + projectUrl + projectID + "/vms"
	if options != nil {
		uri += getQueryString(options)
	}
	res, err := api.client.restClient.GetList(api.client.Endpoint, uri, api.client.options.TokenOptions)
	if err != nil {
		return
	}

	result = &VMs{}
	err = json.Unmarshal(res, result)
	return
}

// Creates a service on the specified project.
func (api *ProjectsAPI) CreateService(projectID string, spec *ServiceCreateSpec) (task *Task, err error) {
	body, err := json.Marshal(spec)
	if err != nil {
		return
	}
	res, err := api.client.restClient.Post(
		api.client.Endpoint+projectUrl+projectID+"/services",
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

// Creates an image on the specified project.
func (api *ProjectsAPI) CreateImage(projectID string, reader io.ReadSeeker, name string, options *ImageCreateOptions) (task *Task, err error) {
	params := imageCreateOptionsToMap(options)
	res, err := api.client.restClient.MultipartUpload(api.client.Endpoint+projectUrl+projectID+"/images", reader, name, params, api.client.options.TokenOptions)
	if err != nil {
		return
	}
	defer res.Body.Close()
	result, err := getTask(getError(res))
	return result, err
}

// Gets services for project with the specified ID
func (api *ProjectsAPI) GetServices(projectID string) (result *Services, err error) {
	uri := api.client.Endpoint + projectUrl + projectID + "/services"
	res, err := api.client.restClient.GetList(api.client.Endpoint, uri, api.client.options.TokenOptions)
	if err != nil {
		return
	}

	result = &Services{}
	err = json.Unmarshal(res, result)
	return
}

// Gets the project with a specified ID.
func (api *ProjectsAPI) Get(id string) (project *ProjectCompact, err error) {
	res, err := api.client.restClient.Get(api.getEntityUrl(id), api.client.options.TokenOptions)
	if err != nil {
		return
	}
	defer res.Body.Close()
	res, err = getError(res)
	if err != nil {
		return
	}
	project = &ProjectCompact{}
	err = json.NewDecoder(res.Body).Decode(project)
	return
}

// Set security groups for this project, overwriting any existing ones.
func (api *ProjectsAPI) SetSecurityGroups(projectID string, securityGroups *SecurityGroupsSpec) (*Task, error) {
	return setSecurityGroups(api.client, api.getEntityUrl(projectID), securityGroups)
}

func (api *ProjectsAPI) getEntityUrl(id string) string {
	return api.client.Endpoint + projectUrl + id
}

// Creates a router on the specified project.
func (api *ProjectsAPI) CreateRouter(projectID string, spec *RouterCreateSpec) (task *Task, err error) {
	body, err := json.Marshal(spec)
	if err != nil {
		return
	}
	res, err := api.client.restClient.Post(
		api.client.Endpoint+projectUrl+projectID+"/routers",
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

// Gets routers for project with the specified ID, using options to filter the results.
// If options is nil, no filtering will occur.
func (api *ProjectsAPI) GetRouters(projectID string, options *RouterGetOptions) (result *Routers, err error) {
	uri := api.client.Endpoint + projectUrl + projectID + "/routers"
	if options != nil {
		uri += getQueryString(options)
	}
	res, err := api.client.restClient.GetList(api.client.Endpoint, uri, api.client.options.TokenOptions)
	if err != nil {
		return
	}

	result = &Routers{}
	err = json.Unmarshal(res, result)
	return
}

// Creates a network on the specified project.
func (api *ProjectsAPI) CreateNetwork(projectID string, spec *NetworkCreateSpec) (task *Task, err error) {
	body, err := json.Marshal(spec)
	if err != nil {
		return
	}
	res, err := api.client.restClient.Post(
		api.client.Endpoint+projectUrl+projectID+"/networks",
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

// Gets networks for project with the specified ID, using options to filter the results.
// If options is nil, no filtering will occur.
func (api *ProjectsAPI) GetNetworks(projectID string, options *NetworkGetOptions) (result *Networks, err error) {
	uri := api.client.Endpoint + projectUrl + projectID + "/networks"
	if options != nil {
		uri += getQueryString(options)
	}
	res, err := api.client.restClient.GetList(api.client.Endpoint, uri, api.client.options.TokenOptions)
	if err != nil {
		return
	}

	result = &Networks{}
	err = json.Unmarshal(res, result)
	return
}

// Get quota for project with the specified ID.
func (api *ProjectsAPI) GetQuota(projectId string) (quota *Quota, err error) {
	uri := api.client.Endpoint + projectUrl + projectId + "/quota"
	res, err := api.client.restClient.Get(uri, api.client.options.TokenOptions)

	if err != nil {
		return
	}

	defer res.Body.Close()
	res, err = getError(res)
	if err != nil {
		return
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}

	quota = &Quota{}
	err = json.Unmarshal(body, quota)
	return
}

// Set (replace) the whole project quota with the quota line items specified in quota spec.
func (api *ProjectsAPI) SetQuota(projectId string, spec *QuotaSpec) (task *Task, err error) {
	task, err = api.modifyQuota("PUT", projectId, spec)
	return
}

// Update portion of the project quota with the quota line items specified in quota spec.
func (api *ProjectsAPI) UpdateQuota(projectId string, spec *QuotaSpec) (task *Task, err error) {
	task, err = api.modifyQuota("PATCH", projectId, spec)
	return
}

// Exclude project quota line items from the specific quota spec.
func (api *ProjectsAPI) ExcludeQuota(projectId string, spec *QuotaSpec) (task *Task, err error) {
	task, err = api.modifyQuota("DELETE", projectId, spec)
	return
}

// A private common function for modifying quota for the specified project with the quota line items specified
// in quota spec.
func (api *ProjectsAPI) modifyQuota(method string, projectId string, spec *QuotaSpec) (task *Task, err error) {
	body, err := json.Marshal(spec)
	if err != nil {
		return
	}
	res, err := api.client.restClient.SendRequestCommon(
		method,
		api.client.Endpoint+projectUrl+projectId+"/quota",
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

// Gets IAM Policy of a project.
func (api *ProjectsAPI) GetIam(projectId string) (policy []*RoleBinding, err error) {
	res, err := api.client.restClient.Get(
		api.client.Endpoint+projectUrl+projectId+"/iam",
		api.client.options.TokenOptions)
	if err != nil {
		return
	}
	defer res.Body.Close()
	res, err = getError(res)
	if err != nil {
		return
	}
	err = json.NewDecoder(res.Body).Decode(&policy)
	return policy, err
}

// Sets IAM Policy on a project.
func (api *ProjectsAPI) SetIam(projectId string, policy []*RoleBinding) (task *Task, err error) {
	body, err := json.Marshal(policy)
	if err != nil {
		return
	}
	res, err := api.client.restClient.Post(
		api.client.Endpoint+projectUrl+projectId+"/iam",
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

// Modifies IAM Policy on a project.
func (api *ProjectsAPI) ModifyIam(projectId string, policyDelta []*RoleBindingDelta) (task *Task, err error) {
	body, err := json.Marshal(policyDelta)
	if err != nil {
		return
	}
	res, err := api.client.restClient.Patch(
		api.client.Endpoint+projectUrl+projectId+"/iam",
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
