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
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"testing"

	"github.com/digitalocean/godo"
)

type fakeClientOpts struct {
	fakeDroplet *fakeDropletService
	fakeLB      *fakeLBService
	fakeCert    *fakeCertService
	fakeDomain  *fakeDomainService
}

func newFakeClient(opts fakeClientOpts) *godo.Client {
	return &godo.Client{
		Certificates:  opts.fakeCert,
		Droplets:      opts.fakeDroplet,
		LoadBalancers: opts.fakeLB,
		Domains:       opts.fakeDomain,
	}
}

func newFakeHTTPResponse(statusCode int) *http.Response {
	return &http.Response{
		Request: &http.Request{
			Method: "FAKE",
			URL:    &url.URL{},
		},
		StatusCode: statusCode,
		Body:       ioutil.NopCloser(bytes.NewBufferString("test")),
	}
}

func newFakeResponse(statusCode int) *godo.Response {
	return &godo.Response{
		Response: newFakeHTTPResponse(statusCode),
	}
}

func newFakeErrorResponse(statusCode int) *godo.ErrorResponse {
	return &godo.ErrorResponse{
		Response: newFakeHTTPResponse(statusCode),
	}
}

func newFakeNotFoundResponse() *godo.Response {
	return newFakeResponse(http.StatusNotFound)
}

func newFakeNotFoundErrorResponse() *godo.ErrorResponse {
	return newFakeErrorResponse(http.StatusNotFound)
}

func newFakeOKResponse() *godo.Response {
	return newFakeResponse(http.StatusOK)
}

func newFakeNotOKResponse() *godo.Response {
	return newFakeResponse(http.StatusInternalServerError)
}

func newFakeBadRequestResponse() *godo.Response {
	return newFakeResponse(http.StatusBadRequest)
}

func newFakeBadRequestErrorResponse() *godo.ErrorResponse {
	return newFakeErrorResponse(http.StatusBadRequest)
}

func linksForPage(page int) *godo.Links {
	switch page {
	case 0, 1:
		// first page mean no prev link
		return &godo.Links{
			Pages: &godo.Pages{
				Next: "https://site?page=2",
				Last: "https://site?page=3",
			},
		}
	case 2:
		return &godo.Links{
			Pages: &godo.Pages{
				Prev: "https://site?page=1",
				Next: "https://site?page=3",
				Last: "https://site?page=3",
			},
		}
	case 3:
		return &godo.Links{
			Pages: &godo.Pages{
				Prev: "https://site?page=2",
			},
		}
	}
	// keep links nil signifying last page
	return nil
}

func TestAllDropletList(t *testing.T) {
	client := newFakeDropletClient(
		&fakeDropletService{
			listFunc: func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
				// Simulate pagination
				droplets := []godo.Droplet{
					{ID: opt.Page},
				}

				resp := &godo.Response{
					Links: linksForPage(opt.Page),
				}

				return droplets, resp, nil
			},
		},
	)

	droplets, err := allDropletList(context.Background(), client)
	if err != nil {
		t.Fatalf("unexpected err: %s", err)
	}

	expectedDroplets := []godo.Droplet{
		{ID: 1}, {ID: 2}, {ID: 3},
	}
	if want, got := expectedDroplets, droplets; !reflect.DeepEqual(want, got) {
		t.Errorf("incorrect droplets\nwant: %#v\n got: %#v", want, got)
	}
}

func TestAllLoadBalancerList(t *testing.T) {
	client := newFakeLBClient(
		&fakeLBService{
			listFn: func(ctx context.Context, opt *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
				// Simulate pagination
				lbs := []godo.LoadBalancer{
					{ID: strconv.Itoa(opt.Page)},
				}

				resp := &godo.Response{
					Links: linksForPage(opt.Page),
				}

				return lbs, resp, nil
			},
		},
	)

	lbs, err := allLoadBalancerList(context.Background(), client)
	if err != nil {
		t.Fatalf("unexpected err: %s", err)
	}

	expectedLBs := []godo.LoadBalancer{
		{ID: "1"}, {ID: "2"}, {ID: "3"},
	}
	if want, got := expectedLBs, lbs; !reflect.DeepEqual(want, got) {
		t.Errorf("incorrect lbs\nwant: %#v\n got: %#v", want, got)
	}
}
