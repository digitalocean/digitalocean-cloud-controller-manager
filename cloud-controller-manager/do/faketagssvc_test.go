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

	"github.com/digitalocean/godo"
)

var _ godo.TagsService = new(fakeTagsService)

type fakeTagsService struct {
	*fakeService
	tags        map[string]bool
	tagRequests []*godo.TagResourcesRequest
}

func newFakeTagsService(tags ...string) *fakeTagsService {
	return newFakeTagsServiceWithFailure(-1, nil, tags...)
}

func newFakeTagsServiceWithFailure(failOnReq int, failErr error, tags ...string) *fakeTagsService {
	t := map[string]bool{}
	for _, tag := range tags {
		t[tag] = true
	}

	return &fakeTagsService{
		fakeService: newFakeService(failOnReq, failErr),
		tags:        t,
	}
}

func (f *fakeTagsService) List(ctx context.Context, opt *godo.ListOptions) ([]godo.Tag, *godo.Response, error) {
	panic("not implemented")
}

func (f *fakeTagsService) Get(ctx context.Context, name string) (*godo.Tag, *godo.Response, error) {
	panic("not implemented")
}

func (f *fakeTagsService) Create(ctx context.Context, createRequest *godo.TagCreateRequest) (*godo.Tag, *godo.Response, error) {
	if f.shouldFail() {
		return nil, nil, f.failError
	}

	name := createRequest.Name
	if name == "" {
		return nil, newFakeResponse(http.StatusBadRequest), errors.New("missing name in request")
	}

	f.tags[name] = true

	return &godo.Tag{Name: name}, newFakeOKResponse(), nil
}

func (f *fakeTagsService) Delete(ctx context.Context, name string) (*godo.Response, error) {
	panic("not implemented")
}

func (f *fakeTagsService) TagResources(ctx context.Context, name string, tagRequest *godo.TagResourcesRequest) (*godo.Response, error) {
	if f.shouldFail() {
		return nil, f.failError
	}

	if name == "" {
		return newFakeResponse(http.StatusBadRequest), errors.New("missing name")
	}

	if len(tagRequest.Resources) == 0 {
		return newFakeResponse(http.StatusBadRequest), errors.New("missing resources in request")
	}

	if !f.tags[name] {
		return newFakeResponse(http.StatusNotFound), fmt.Errorf("tag %q does not exist", name)
	}

	f.tagRequests = append(f.tagRequests, tagRequest)

	return newFakeOKResponse(), nil
}

func (f *fakeTagsService) UntagResources(ctx context.Context, name string, untagRequest *godo.UntagResourcesRequest) (*godo.Response, error) {
	panic("not implemented")
}
