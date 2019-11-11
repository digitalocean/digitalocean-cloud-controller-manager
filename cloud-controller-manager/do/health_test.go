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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/digitalocean/godo"
)

func TestGodoHealthChecker_Name(t *testing.T) {
	c := godoHealthChecker{}
	if want, got := "godo", c.Name(); want != got {
		t.Errorf("incorrect name\nwant: %#v \n got: %#v", want, got)
	}
}

func TestGodoHealthCheker_Check(t *testing.T) {
	c := godoHealthChecker{}

	t.Run("healthy godo", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"account":null}`))
		}))
		defer ts.Close()

		var err error
		c.client, err = godo.New(http.DefaultClient, godo.SetBaseURL(ts.URL))
		if err != nil {
			t.Fatal(err)
		}

		req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
		if err != nil {
			t.Fatal(err)
		}

		err = c.Check(req)
		if err != nil {
			t.Errorf("expected no error: %s", err)
		}
	})

	t.Run("unhealthy godo", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer ts.Close()

		var err error
		c.client, err = godo.New(http.DefaultClient, godo.SetBaseURL(ts.URL))
		if err != nil {
			t.Fatal(err)
		}

		req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
		if err != nil {
			t.Fatal(err)
		}

		err = c.Check(req)
		if err == nil {
			t.Error("expected error but got none")
		}
	})
}
