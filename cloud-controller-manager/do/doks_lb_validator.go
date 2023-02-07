package do

import (
	"context"
	"github.com/go-logr/logr"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// DOKSLBServiceValidator validates service type LB
type DOKSLBServiceValidator struct {
	Client  client.Client
	decoder *admission.Decoder
	Log     logr.Logger
}

// DOKSLBServiceValidator ...
func (v *DOKSLBServiceValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	svc := &corev1.Service{}
	v.Log.Info("decoding received request")
	err := v.decoder.Decode(req, svc)
	if err != nil {
		v.Log.Info("bad request")
		return admission.Errored(http.StatusBadRequest, err)
	}

	v.Log.Info("checking received request")
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return admission.Allowed("")
	}

	return admission.Denied("Rule denied by DOKS validating webhook.")
}

func (v *DOKSLBServiceValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}