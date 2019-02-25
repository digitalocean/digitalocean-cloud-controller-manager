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
	"io/ioutil"
	"net/http"

	"github.com/digitalocean/godo"
)

func newFakeOKResponse() *godo.Response {
	return newFakeResponse(http.StatusOK)
}

func newFakeNotOKResponse() *godo.Response {
	return newFakeResponse(http.StatusInternalServerError)
}

func newFakeResponse(statusCode int) *godo.Response {
	return &godo.Response{
		Response: &http.Response{
			StatusCode: statusCode,
			Body:       ioutil.NopCloser(bytes.NewBufferString("test")),
		},
	}
}
