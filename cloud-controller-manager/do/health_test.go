/*
Copyright 2023 DigitalOcean

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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/digitalocean/godo"
)

func TestGodoHealthChecker(t *testing.T) {
	tests := []struct {
		name        string
		stubHandler func(http.ResponseWriter, *http.Request)
		wantCode    int
		wantBody    string
	}{
		{
			name: "healthy",
			stubHandler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"account": null}`))
			},
			wantCode: http.StatusOK,
		},
		{
			name: "not healthy",
			stubHandler: func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, `{"message": "not you"}`, http.StatusUnauthorized)
			},
			wantCode: http.StatusInternalServerError,
			wantBody: "not you",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(test.stubHandler))
			defer ts.Close()

			c := godoHealthChecker{}
			var err error
			c.client, err = godo.New(http.DefaultClient, godo.SetBaseURL(ts.URL))
			if err != nil {
				t.Fatal(err)
			}

			req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
			if err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			c.ServeHTTP(w, req)

			resp := w.Result()
			if gotCode := resp.StatusCode; gotCode != test.wantCode {
				t.Errorf("got code %d, want %d", gotCode, test.wantCode)
			}

			if test.wantBody != "" {
				gotBodyRaw, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("failed to read response body: %s", err)
				}
				gotBody := string(gotBodyRaw)
				if !strings.Contains(gotBody, test.wantBody) {
					t.Errorf("body %q does not contain %q", gotBody, test.wantBody)
				}
			}
		})
	}
}
