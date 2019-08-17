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
	"testing"
	"time"

	"github.com/digitalocean/godo"
	"github.com/pborman/uuid"
)

const lbIngressIP = "10.0.0.1"

type fakeLoadBalancerService struct {
	*fakeService
	lbs                     []godo.LoadBalancer
	wantGets, gotGets       int
	wantLists, gotLists     int
	wantCreates, gotCreates int
	wantUpdates, gotUpdates int
	createdActiveOn         int
}

func newFakeLoadBalancerService(lbs ...godo.LoadBalancer) *fakeLoadBalancerService {
	return newFakeLoadBalancerServiceWithFailure(-1, nil, lbs...)
}

func newFakeLoadBalancerServiceWithFailure(failOnReq int, failErr error, lbs ...godo.LoadBalancer) *fakeLoadBalancerService {
	for i, lb := range lbs {
		if lb.Status == "" {
			lbs[i].Status = lbStatusNew
		}
	}
	return &fakeLoadBalancerService{
		fakeService: newFakeService(failOnReq, failErr),
		// TODO: consider auto-/force-filling LBs for params like IP and state.
		lbs:             lbs,
		wantGets:        -1,
		wantLists:       -1,
		wantCreates:     -1,
		wantUpdates:     -1,
		createdActiveOn: 2,
	}
}

func (f *fakeLoadBalancerService) expectGets(i int) *fakeLoadBalancerService {
	f.wantGets = i
	return f
}

func (f *fakeLoadBalancerService) expectLists(i int) *fakeLoadBalancerService {
	f.wantLists = i
	return f
}

func (f *fakeLoadBalancerService) expectCreates(i int) *fakeLoadBalancerService {
	f.wantCreates = i
	return f
}

func (f *fakeLoadBalancerService) expectUpdates(i int) *fakeLoadBalancerService {
	f.wantUpdates = i
	return f
}

func (f *fakeLoadBalancerService) setCreatedActiveOn(i int) *fakeLoadBalancerService {
	f.createdActiveOn = i
	return f
}

func (f *fakeLoadBalancerService) assertCounts(t *testing.T) {
	if f.wantGets >= 0 && f.gotGets != f.wantGets {
		t.Errorf("got %d get(s), want %d", f.gotGets, f.wantGets)
	}
	if f.wantLists >= 0 && f.gotLists != f.wantLists {
		t.Errorf("got %d lists(s), want %d", f.gotLists, f.wantLists)
	}
	if f.wantCreates >= 0 && f.gotCreates != f.wantCreates {
		t.Errorf("got %d create(s), want %d", f.gotCreates, f.wantCreates)
	}
	if f.wantUpdates >= 0 && f.gotUpdates != f.wantUpdates {
		t.Errorf("got %d update(s), want %d", f.gotUpdates, f.wantUpdates)
	}
}

func (f *fakeLoadBalancerService) deepCopyP(lb godo.LoadBalancer) *godo.LoadBalancer {
	copy := f.deepCopy(lb)
	return &copy
}

func (f *fakeLoadBalancerService) deepCopy(lb godo.LoadBalancer) godo.LoadBalancer {
	if f.createdActiveOn >= 0 && f.gotGets+f.gotLists+f.gotUpdates >= f.createdActiveOn {
		lb.Status = lbStatusActive
		lb.IP = lbIngressIP
	}
	return mustCopy(lb).(godo.LoadBalancer)
}

func (f *fakeLoadBalancerService) Get(_ context.Context, lbID string) (*godo.LoadBalancer, *godo.Response, error) {
	f.gotGets++
	if f.shouldFail() {
		return nil, newFakeNotOKResponse(), f.failError
	}

	for _, lb := range f.lbs {
		if lb.ID == lbID {
			return f.deepCopyP(lb), newFakeOKResponse(), nil
		}
	}

	return nil, newFakeResponse(http.StatusNotFound), fmt.Errorf("load-balancer %q not found", lbID)
}

func (f *fakeLoadBalancerService) List(_ context.Context, listOpts *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
	f.gotLists++
	if f.shouldFail() {
		return nil, newFakeNotOKResponse(), f.failError
	}

	lbs := make([]godo.LoadBalancer, 0, len(f.lbs))
	for _, lb := range f.lbs {
		lbs = append(lbs, f.deepCopy(lb))
	}

	return lbs, newFakeOKResponse(), nil
}

func (f *fakeLoadBalancerService) Create(ctx context.Context, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
	f.gotCreates++
	if f.shouldFail() {
		return nil, newFakeNotOKResponse(), f.failError
	}

	lb := &godo.LoadBalancer{
		ID:     uuid.New(),
		Status: lbStatusNew,
	}

	setLBFromReq(lb, lbr)
	f.lbs = append(f.lbs, *lb)
	return f.deepCopyP(*lb), newFakeResponse(http.StatusAccepted), nil
}

func (f *fakeLoadBalancerService) Update(_ context.Context, lbID string, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
	f.gotUpdates++
	if f.shouldFail() {
		return nil, newFakeNotOKResponse(), f.failError
	}

	for _, lb := range f.lbs {
		if lb.ID == lbID {
			setLBFromReq(&lb, lbr)
			return f.deepCopyP(lb), newFakeResponse(http.StatusOK), nil
		}
	}

	return nil, newFakeResponse(http.StatusNotFound), fmt.Errorf("load-balancer %q not found", lbID)
}

func (f *fakeLoadBalancerService) Delete(_ context.Context, lbID string) (*godo.Response, error) {
	if f.shouldFail() {
		return newFakeNotOKResponse(), f.failError
	}

	for i, lb := range f.lbs {
		if lb.ID == lbID {
			f.lbs = append(f.lbs[:i], f.lbs[i:]...)
			return newFakeResponse(http.StatusNoContent), nil
		}
	}

	return newFakeResponse(http.StatusNotFound), fmt.Errorf("load-balancer %q not found", lbID)
}

func (f *fakeLoadBalancerService) AddDroplets(ctx context.Context, lbID string, dropletIDs ...int) (*godo.Response, error) {
	panic("not implemented")
}

func (f *fakeLoadBalancerService) RemoveDroplets(ctx context.Context, lbID string, dropletIDs ...int) (*godo.Response, error) {
	panic("not implemented")
}

func (f *fakeLoadBalancerService) AddForwardingRules(ctx context.Context, lbID string, rules ...godo.ForwardingRule) (*godo.Response, error) {
	panic("not implemented")
}

func (f *fakeLoadBalancerService) RemoveForwardingRules(ctx context.Context, lbID string, rules ...godo.ForwardingRule) (*godo.Response, error) {
	panic("not implemented")
}

func setLBFromReq(lb *godo.LoadBalancer, lbr *godo.LoadBalancerRequest) {
	lb.Name = lbr.Name
	lb.Algorithm = lbr.Algorithm
	lb.Created = time.Now().Format(time.RFC3339)
	lb.ForwardingRules = mustCopy(lbr.ForwardingRules).([]godo.ForwardingRule)
	lb.HealthCheck = mustCopy(lbr.HealthCheck).(*godo.HealthCheck)
	lb.StickySessions = mustCopy(lbr.StickySessions).(*godo.StickySessions)
	lb.DropletIDs = lbr.DropletIDs[:]
	lb.Tag = lbr.Tag
	lb.Tags = lbr.Tags[:]
	lb.RedirectHttpToHttps = lbr.RedirectHttpToHttps
	lb.EnableProxyProtocol = lbr.EnableProxyProtocol
	lb.VPCUUID = lbr.VPCUUID
}
