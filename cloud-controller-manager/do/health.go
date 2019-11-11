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
	"fmt"
	"net/http"
	"time"

	"github.com/digitalocean/godo"
)

var godoHealthTimeout = 15 * time.Second

type godoHealthChecker struct {
	client *godo.Client
}

func (c *godoHealthChecker) Name() string {
	return "godo"
}

func (c *godoHealthChecker) Check(r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), godoHealthTimeout)
	defer cancel()
	_, _, err := c.client.Account.Get(ctx)
	if err != nil {
		return fmt.Errorf("checking godo health: %w", err)
	}
	return nil
}
