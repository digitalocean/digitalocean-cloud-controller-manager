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

	"github.com/digitalocean/godo"
)

type stubLBService struct {
	getFn                   func(context.Context, string) (*godo.LoadBalancer, *godo.Response, error)
	listFn                  func(context.Context, *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error)
	createFn                func(context.Context, *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error)
	updateFn                func(ctx context.Context, lbID string, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error)
	deleteFn                func(ctx context.Context, lbID string) (*godo.Response, error)
	addDropletsFn           func(ctx context.Context, lbID string, dropletIDs ...int) (*godo.Response, error)
	removeDropletsFn        func(ctx context.Context, lbID string, dropletIDs ...int) (*godo.Response, error)
	addForwardingRulesFn    func(ctx context.Context, lbID string, rules ...godo.ForwardingRule) (*godo.Response, error)
	removeForwardingRulesFn func(ctx context.Context, lbID string, rules ...godo.ForwardingRule) (*godo.Response, error)
}

func (f *stubLBService) Get(ctx context.Context, lbID string) (*godo.LoadBalancer, *godo.Response, error) {
	return f.getFn(ctx, lbID)
}

func (f *stubLBService) List(ctx context.Context, listOpts *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
	return f.listFn(ctx, listOpts)
}

func (f *stubLBService) Create(ctx context.Context, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
	return f.createFn(ctx, lbr)
}

func (f *stubLBService) Update(ctx context.Context, lbID string, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
	return f.updateFn(ctx, lbID, lbr)
}

func (f *stubLBService) Delete(ctx context.Context, lbID string) (*godo.Response, error) {
	return f.deleteFn(ctx, lbID)
}

func (f *stubLBService) AddDroplets(ctx context.Context, lbID string, dropletIDs ...int) (*godo.Response, error) {
	return f.addDropletsFn(ctx, lbID, dropletIDs...)
}

func (f *stubLBService) RemoveDroplets(ctx context.Context, lbID string, dropletIDs ...int) (*godo.Response, error) {
	return f.removeDropletsFn(ctx, lbID, dropletIDs...)
}
func (f *stubLBService) AddForwardingRules(ctx context.Context, lbID string, rules ...godo.ForwardingRule) (*godo.Response, error) {
	return f.addForwardingRulesFn(ctx, lbID, rules...)
}

func (f *stubLBService) RemoveForwardingRules(ctx context.Context, lbID string, rules ...godo.ForwardingRule) (*godo.Response, error) {
	return f.removeForwardingRulesFn(ctx, lbID, rules...)
}
