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
	"context"
	"fmt"
	"net/http"

	"github.com/digitalocean/godo"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// LBService represents the v1.service lb object data
type LBService struct {
	APIVersion string
	Kind       string
	Metadata   Metadata
	Spec       map[string]interface{}
	Selector   map[string]interface{}
	Type       string
}

// Metadata represents the metadata field in the lb service object
type Metadata struct {
	Annotations interface{} `json:"annotation data,omitempty"`
	Name        string
	Namespace   string
}

// DOKSLBServiceValidator validates service type LB
type KubernetesLBServiceValidator struct {
	decoder *admission.Decoder
	Log     logr.Logger
	GClient *godo.Client
	Region  string
}

// DOKSLBServiceValidator ...
func (v *KubernetesLBServiceValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	svc := &v1.Service{}

	v.Log.V(6).Info("decoding received request")
	err := v.decoder.Decode(req, svc)
	if err != nil {
		v.Log.Error(err, "failed to decode request")
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to decode request: %v", err))
	}

	v.Log.V(6).Info("checking received request")
	if svc.Spec.Type != v1.ServiceTypeLoadBalancer || svc.DeletionTimestamp != nil {
		return admission.Allowed("ignoring the service which is either not a load balancer or is being deleted")
	}

	// TODO: these forwarding rules are a placeholder. Further development is required to extract the values from the svc object
	forwardingRules := []godo.ForwardingRule{
		{
			EntryProtocol:  "http",
			EntryPort:      80,
			TargetProtocol: "http",
			TargetPort:     80,
			CertificateID:  "",
			TlsPassthrough: false,
		}}

	v.Region, err = dropletRegion(v.GClient.Regions)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to determine region: %v", err))
	}

	lbRequest := v.buildRequest(svc.Name, v.Region, forwardingRules)

	var resp *godo.Response

	// check if old service object exists
	if req.OldObject.Raw != nil {
		// decode raw object
		err = v.decoder.DecodeRaw(req.OldObject, svc)
		if err != nil {
			v.Log.Error(err, "failed to decode existing object")
			return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to decode request: %v", err))
		}

		currentLBID := svc.Annotations[annDOLoadBalancerID]
		// perform update if associated with created lb
		if (len(svc.Status.LoadBalancer.Ingress) > 0 && svc.Status.LoadBalancer.Ingress[0].IP != "") || currentLBID != "" {
			v.Log.Info(fmt.Sprintf("updating lb id: %v", currentLBID))
			resp, err = v.validateUpdate(ctx, currentLBID, lbRequest)
			if err != nil {
				errorCode := getStatusCode(resp)
				if errorCode != http.StatusUnprocessableEntity {
					v.Log.Error(err, "failed to validate lb update, could not get validation response")
					return admission.Errored(int32(errorCode), errors.Wrap(err, "failed to validate lb update, could not get validation response"))
				}
				v.Log.Error(err, "failed to validate lb update, invalid configuration")
				return admission.Denied(fmt.Sprintf("failed to validate lb update: %v", err))
			}
			v.Log.Info("lb update validated")
			return admission.Allowed("valid update request")
		}
	}

	// validate create request otherwise
	v.Log.Info("validating create request")
	resp, err = v.validateCreate(ctx, lbRequest)
	if err != nil {
		errorCode := getStatusCode(resp)
		if errorCode != http.StatusUnprocessableEntity {
			v.Log.Error(err, "failed to validate lb creation, could not get validation response")
			return admission.Errored(int32(errorCode), errors.Wrap(err, "failed to validate lb create, could not get validation response"))
		}
		v.Log.Error(err, "failed to validate lb creation, invalid configuration")
		return admission.Denied(fmt.Sprintf("failed to validate lb creation: %v", err))
	}
	v.Log.Info(fmt.Sprintf("allowing create"))
	return admission.Allowed("valid lb create request")
}

func (v *KubernetesLBServiceValidator) validateCreate(ctx context.Context, lbRequest *godo.LoadBalancerRequest) (*godo.Response, error) {
	_, resp, err := v.GClient.LoadBalancers.Create(ctx, lbRequest)
	return resp, err
}

func (v *KubernetesLBServiceValidator) validateUpdate(ctx context.Context, currentLBID string, lbRequest *godo.LoadBalancerRequest) (*godo.Response, error) {
	_, resp, err := v.GClient.LoadBalancers.Update(ctx, currentLBID, lbRequest)
	return resp, err
}

func (v *KubernetesLBServiceValidator) buildRequest(name string, region string, forwardingRules []godo.ForwardingRule) *godo.LoadBalancerRequest {
	return &godo.LoadBalancerRequest{
		Name:            name,
		Tag:             "",
		Region:          region,
		ForwardingRules: forwardingRules,
		ValidateOnly:    true,
	}
}

func (v *KubernetesLBServiceValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}

func getStatusCode(response *godo.Response) int {
	if response.Response != nil {
		return response.StatusCode
	}

	return http.StatusInternalServerError
}
