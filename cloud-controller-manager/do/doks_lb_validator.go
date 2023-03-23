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
	"github.com/digitalocean/godo"
	"github.com/go-logr/logr"
	"golang.org/x/oauth2"
	v1 "k8s.io/api/core/v1"
	"net/http"
	"os"
	"strconv"

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
type DOKSLBServiceValidator struct {
	decoder *admission.Decoder
	Log     logr.Logger
	gClient *godo.Client
}

func initDOClient() (doClient *godo.Client, err error) {
	token := os.Getenv(doAccessTokenEnv)

	opts := []godo.ClientOpt{}

	opts = append(opts, godo.SetUserAgent("digitalocean-webhook-server/"+version))

	if token == "" {
		return nil, fmt.Errorf("environment variable %q is required", doAccessTokenEnv)
	}

	tokenSource := &tokenSource{
		AccessToken: token,
	}

	oauthClient := oauth2.NewClient(oauth2.NoContext, tokenSource)
	doClient, err = godo.New(oauthClient, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create godo client: %s", err)
	}

	return doClient, nil
}

// DOKSLBServiceValidator ...
func (v *DOKSLBServiceValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	svc := &v1.Service{}
	v.Log.V(6).Info("decoding received request")
	err := v.decoder.Decode(req, svc)
	if err != nil {
		v.Log.Error(err, "bad request")
		return admission.Errored(http.StatusBadRequest, err)
	}

	v.Log.V(6).Info("checking received request")
	if svc.Spec.Type != v1.ServiceTypeLoadBalancer {
		return admission.Allowed("")
	}

	// check if request is for LB deletion
	if svc.Spec.Type == v1.ServiceTypeLoadBalancer && svc.DeletionTimestamp != nil {
		return admission.Allowed("the loadbalancer is deleting")
	}

	// initialize DO Client if not already initialized
	if v.gClient == nil {
		doClient, err := initDOClient()
		if err != nil {
			v.Log.Error(err, "failed to initialize DO client")
			return admission.Errored(http.StatusConflict, err)
		}
		v.gClient = doClient
	}

	clusterID := os.Getenv(doClusterIDEnv)
	if clusterID == "" {
		fmt.Println("missing cluster id")
	}
	region := "nyc1"
	if region == "" {
		fmt.Println("missing region id")
	}

	nodePools, _, err := v.gClient.Kubernetes.ListNodePools(ctx, clusterID, &godo.ListOptions{})
	if err != nil {
		v.Log.Error(err, "no nodes found")
		return admission.Errored(http.StatusBadRequest, err)
	}

	var dropletIDs []int
	for _, pool := range nodePools {
		for _, node := range pool.Nodes {
			dropletID, err := strconv.Atoi(node.DropletID)
			if err != nil {
				v.Log.Error(err, "failed to retrieve droplet ids")
				return admission.Errored(http.StatusConflict, err)
			}
			dropletIDs = append(dropletIDs, dropletID)
		}
	}

	forwardingRules := []godo.ForwardingRule{
		{
			EntryProtocol:  "http",
			EntryPort:      80,
			TargetProtocol: "http",
			TargetPort:     80,
			CertificateID:  "",
			TlsPassthrough: false,
		}}

	lbRequest, err := v.buildRequest(svc.Name, region, dropletIDs, forwardingRules)
	if err != nil {
		v.Log.Error(err, "failed to build lb request")
		//fmt.Printf("failed to build lb request", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	err = v.decoder.DecodeRaw(req.OldObject, svc)
	if err != nil {
		err = v.validateCreate(ctx, lbRequest, v.gClient)
		if err != nil {
			v.Log.Error(err, "failed to validate lb creation")
			return admission.Denied("failed to validate load balancer request")
		}
		return admission.Allowed("valid lb create request")
	}

	err = v.validateUpdate(ctx, svc, lbRequest, v.gClient)
	if err != nil {
		fmt.Printf("failed to update load balancer")
		v.Log.Error(err, "failed to update load balancer")
		return admission.Errored(http.StatusBadRequest, err)
	}
	return admission.Allowed("validating if request type is UPDATE")
}

func (v *DOKSLBServiceValidator) validateCreate(ctx context.Context, lbRequest *godo.LoadBalancerRequest, doClient *godo.Client) error {
	_, _, err := doClient.LoadBalancers.Create(ctx, lbRequest)
	if err != nil {
		v.Log.Error(err, "failed to create load balancer")
		return err
	}

	return nil
}

func (v *DOKSLBServiceValidator) validateUpdate(ctx context.Context, svc *v1.Service, lbRequest *godo.LoadBalancerRequest, doClient *godo.Client) error {
	currentLBID := svc.Annotations["kubernetes.digitalocean.com/load-balancer-id"]
	_, _, err := doClient.LoadBalancers.Update(ctx, currentLBID, lbRequest)
	if err != nil {
		fmt.Printf("THIS IS AN ERROR: %v", err)
		return err
	}
	return nil
}

func (v *DOKSLBServiceValidator) buildRequest(name string, region string, dropletIDs []int, forwardingRules []godo.ForwardingRule) (*godo.LoadBalancerRequest, error) {
	return &godo.LoadBalancerRequest{
		Name:            name,
		DropletIDs:      dropletIDs,
		Region:          region,
		ForwardingRules: forwardingRules,
		ValidateOnly:    true,
	}, nil
}

func (v *DOKSLBServiceValidator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
