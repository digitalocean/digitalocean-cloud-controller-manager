/*
Copyright 2024 DigitalOcean

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
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// LBServiceAdmissionHandler validates service type LB.
type LBServiceAdmissionHandler struct {
	log        *logr.Logger
	godoClient *godo.Client

	decoder   admission.Decoder
	region    string
	clusterID string
	vpcID     string
}

// NewLBServiceAdmissionHandler returns a configured instance of LBServiceHandler.
func NewLBServiceAdmissionHandler(log *logr.Logger, godoClient *godo.Client) *LBServiceAdmissionHandler {
	return &LBServiceAdmissionHandler{
		log:        log,
		godoClient: godoClient,
		decoder:    admission.NewDecoder(runtime.NewScheme()),
	}
}

// Handle handles admissions requests for load balancer services.
func (h *LBServiceAdmissionHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	resp := h.handle(ctx, req)

	logFields := []any{"object_name", req.Name, "object_namespace", req.Namespace, "object_kind", req.Kind.String()}
	if resp.Allowed {
		h.log.V(2).Info("allowing admission request", logFields...)
	} else {
		h.log.Info("rejecting admission request", append(logFields, "reason", resp.Result.Message)...)
	}

	return resp
}

func (h *LBServiceAdmissionHandler) handle(ctx context.Context, req admission.Request) admission.Response {
	var svc corev1.Service
	err := h.decoder.Decode(req, &svc)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed to decode admission request: %s", err))
	}

	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return admission.Allowed("allowing service that is not a load balancer")
	}

	if svc.DeletionTimestamp != nil {
		return admission.Allowed("allowing service that is being deleted")
	}

	lbID := svc.Annotations[annDOLoadBalancerID]

	lbReq, err := h.buildLoadBalancerRequest(ctx, &svc)
	if err != nil {
		return admission.Denied(fmt.Sprintf("failed to build DO API request: %s", err))
	}

	var resp admission.Response
	switch {
	case req.OldObject.Raw != nil:
		resp = h.validateUpdate(ctx, req, lbID, lbReq)
	default:
		resp = h.validateCreate(ctx, lbReq)
	}

	return resp
}

func (h *LBServiceAdmissionHandler) validateUpdate(ctx context.Context, req admission.Request, lbID string, lbReq *godo.LoadBalancerRequest) admission.Response {
	var oldSvc corev1.Service
	if err := h.decoder.DecodeRaw(req.OldObject, &oldSvc); err != nil {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to decode old object: %s", err))
	}

	// We ignore errors when building the old service's godo request because
	// it is allowed to be wrong. In cases where it errors, it can potentially
	// get fixed after the update.
	oldReq, _ := h.buildLoadBalancerRequest(ctx, &oldSvc)
	if cmp.Equal(oldReq, lbReq) {
		return admission.Allowed("new service has irrelevant changes")
	}

	// We prefer the new LB ID if it is set. If not, we fallback to the old
	// service's LB ID. If the old and new entities don't have an LB id, we
	// fallback to the creation validation.
	reqLbID := lbID
	if reqLbID == "" {
		reqLbID = oldSvc.Annotations[annDOLoadBalancerID]
	}
	if reqLbID == "" {
		return h.validateCreate(ctx, lbReq)
	}

	_, resp, err := h.godoClient.LoadBalancers.Update(ctx, reqLbID, lbReq)
	return h.mapGodoRespToAdmissionResp(resp, err)
}

func (h *LBServiceAdmissionHandler) validateCreate(ctx context.Context, lbReq *godo.LoadBalancerRequest) admission.Response {
	_, resp, err := h.godoClient.LoadBalancers.Create(ctx, lbReq)
	return h.mapGodoRespToAdmissionResp(resp, err)
}

func (h *LBServiceAdmissionHandler) buildLoadBalancerRequest(ctx context.Context, svc *corev1.Service) (*godo.LoadBalancerRequest, error) {
	lbReq, err := buildLoadBalancerRequest(ctx, svc, h.godoClient)
	if err != nil {
		return nil, fmt.Errorf("failed to build base load balancer request: %s", err)
	}
	lbReq.ValidateOnly = true
	lbReq.Region = h.region
	lbReq.VPCUUID = h.vpcID
	if h.clusterID != "" {
		lbReq.Tags = []string{buildK8sTag(h.clusterID)}
	}
	return lbReq, nil
}

// mapGodoRespToAdmissionResp converts godo responses to admission responses. The returned admission response
// has to be permissive enough to allow validation requests when unexpected errors happen. Webhook definitions
// with a failure policy set to `Ignore` will reject `admission.Errored(...)` and `admission.Denied(...)` responses.
func (h *LBServiceAdmissionHandler) mapGodoRespToAdmissionResp(resp *godo.Response, err error) admission.Response {
	switch {
	case err == nil:
		return admission.Allowed("valid load balancer definition")
	case resp == nil:
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed to get a response from DO API: %s", err))
	case resp.StatusCode < 400:
		return admission.Allowed("valid load balancer definition")
	case resp.StatusCode < 500:
		return admission.Denied(fmt.Sprintf("invalid load balancer definition: %s", err))
	default:
		return admission.Allowed(fmt.Sprintf("received unexpected status code (%d) from DO API, allowing to prevent blocking: %s", resp.StatusCode, err))
	}
}

// WithRegion sets the region field of the handler.
func (a *LBServiceAdmissionHandler) WithRegion() error {
	region, err := dropletRegion(a.godoClient.Regions)
	if err != nil {
		return err
	}
	a.region = region
	return nil
}

// WithVPCID sets the vpcID field of the handler.
func (a *LBServiceAdmissionHandler) WithVPCID(vpcID string) {
	a.vpcID = vpcID
}

// WithClusterID sets the clusterID field of the handler.
func (a *LBServiceAdmissionHandler) WithClusterID(clusterID string) {
	a.clusterID = clusterID
}
