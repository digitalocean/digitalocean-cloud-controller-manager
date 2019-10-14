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
	"github.com/pborman/uuid"
)

const lbIngressIP = "10.0.0.1"

type fakeLoadBalancerService struct {
	lbs     []godo.LoadBalancer
	actions []fakeAction
}

func newFakeLoadBalancerService(lbs ...godo.LoadBalancer) *fakeLoadBalancerService {
	for i, lb := range lbs {
		if lb.Status == "" {
			lbs[i].Status = lbStatusActive
		}
		if lbs[i].Status == lbStatusActive {
			lbs[i].IP = lbIngressIP
		}
	}

	return &fakeLoadBalancerService{
		lbs: lbs,
	}
}

func (f *fakeLoadBalancerService) appendAction(action fakeAction) *fakeLoadBalancerService {
	f.actions = append(f.actions, action)
	return f
}

func (f *fakeLoadBalancerService) processActions(methodKind methodKind) (handled bool, lbResp interface{}, godResp *godo.Response, err error) {
	for _, action := range f.actions {
		handled, res, err := action.react(methodKind)
		if !handled {
			continue
		}

		godoResp := newFakeOKResponse()
		if err != nil {
			godoResp = newFakeNotOKResponse()
		}
		return true, res, godoResp, err
	}

	return false, nil, nil, nil
}

func (f *fakeLoadBalancerService) deepCopyP(lb godo.LoadBalancer) *godo.LoadBalancer {
	copy := f.deepCopy(lb)
	return &copy
}

func (f *fakeLoadBalancerService) deepCopy(lb godo.LoadBalancer) godo.LoadBalancer {
	return mustCopy(lb).(godo.LoadBalancer)
}

func (f *fakeLoadBalancerService) Get(_ context.Context, lbID string) (*godo.LoadBalancer, *godo.Response, error) {
	handled, res, godoResp, err := f.processActions(methodKindGet)
	if handled {
		var lbResp *godo.LoadBalancer
		if res != nil {
			lbResp = res.(*godo.LoadBalancer)
		}
		return lbResp, godoResp, err
	}

	for _, lb := range f.lbs {
		if lb.ID == lbID {
			return f.deepCopyP(lb), newFakeOKResponse(), nil
		}
	}

	return nil, newFakeResponse(http.StatusNotFound), fmt.Errorf("load-balancer %q not found", lbID)
}

func (f *fakeLoadBalancerService) List(_ context.Context, listOpts *godo.ListOptions) ([]godo.LoadBalancer, *godo.Response, error) {
	handled, res, godoResp, err := f.processActions(methodKindList)
	if handled {
		var lbResp []godo.LoadBalancer
		if res != nil {
			lbResp = res.([]godo.LoadBalancer)
		}
		return lbResp, godoResp, err
	}

	lbs := make([]godo.LoadBalancer, 0, len(f.lbs))
	for _, lb := range f.lbs {
		lbs = append(lbs, f.deepCopy(lb))
	}

	return lbs, newFakeOKResponse(), nil
}

func (f *fakeLoadBalancerService) Create(ctx context.Context, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
	handled, res, godoResp, err := f.processActions(methodKindCreate)
	if handled {
		var lbResp *godo.LoadBalancer
		if res != nil {
			lbResp = res.(*godo.LoadBalancer)
		}
		return lbResp, godoResp, err
	}

	lb := &godo.LoadBalancer{
		ID: uuid.New(),
		// Create LB that's active right from the start. Although not realistic,
		// this is the use case we test predominantly.
		Status: lbStatusActive,
		IP:     lbIngressIP,
	}

	setLBFromReq(lb, lbr)
	f.lbs = append(f.lbs, *lb)
	return f.deepCopyP(*lb), newFakeResponse(http.StatusAccepted), nil
}

func (f *fakeLoadBalancerService) Update(_ context.Context, lbID string, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
	handled, res, godoResp, err := f.processActions(methodKindUpdate)
	if handled {
		var lbResp *godo.LoadBalancer
		if res != nil {
			lbResp = res.(*godo.LoadBalancer)
		}
		return lbResp, godoResp, err
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
	handled, _, godoResp, err := f.processActions(methodKindDelete)
	if handled {
		return godoResp, err
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
