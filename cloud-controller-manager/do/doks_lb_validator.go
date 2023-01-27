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
	"net/http"

	"github.com/go-logr/logr"
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
	v.Log.V(10).Info("decoding received request")
	err := v.decoder.Decode(req, svc)
	if err != nil {
		v.Log.Error(err, "bad request")
		return admission.Errored(http.StatusBadRequest, err)
	}

	v.Log.V(10).Info("checking received request")
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return admission.Allowed("")
	}

	//TODO add actual validation logic. See CON-7851 for more context.

	return admission.Denied("Rule denied by DOKS validating webhook.")
}

func (v *DOKSLBServiceValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
