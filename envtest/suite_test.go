//go:build integration
// +build integration

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

package envtest

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/digitalocean/digitalocean-cloud-controller-manager/cloud-controller-manager/do"
	"github.com/digitalocean/godo"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	//+kubebuilder:scaffold:imports
)

var (
	k8sClient client.Client
	lbStub    *godoLBStub
)

func TestMain(m *testing.M) {
	log := zap.New(zap.WriteTo(os.Stdout), zap.UseDevMode(true))
	crlog.SetLogger(log)
	log = log.WithName("test_main")

	log.Info("configuring test env")
	webhookInstallOptions := &envtest.WebhookInstallOptions{
		ValidatingWebhooks: []*admissionv1.ValidatingWebhookConfiguration{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "validation-webhook.cloud-controller-manager.digitalocean.com",
				},
				Webhooks: []admissionv1.ValidatingWebhook{
					{
						Name:           "validation-webhook.loadbalancer.doks.io",
						FailurePolicy:  ptr.To(admissionv1.Ignore),
						SideEffects:    ptr.To(admissionv1.SideEffectClassNone),
						TimeoutSeconds: ptr.To[int32](10),
						Rules: []admissionv1.RuleWithOperations{
							{
								Operations: []admissionv1.OperationType{admissionv1.Create, admissionv1.Update},
								Rule: admissionv1.Rule{
									APIGroups:   []string{"*"},
									APIVersions: []string{"v1"},
									Resources:   []string{"services"},
									Scope:       ptr.To(admissionv1.AllScopes),
								},
							},
						},
						AdmissionReviewVersions: []string{"v1"},
						ClientConfig: admissionv1.WebhookClientConfig{
							Service: &admissionv1.ServiceReference{
								Path: ptr.To("/lb-service"),
							},
						},
					},
				},
			},
		},
	}
	testEnv := &envtest.Environment{
		WebhookInstallOptions: *webhookInstallOptions,
	}

	log.Info("setting up fake godo client")
	godoClient := godo.NewFromToken("")
	lbStub = &godoLBStub{
		createResponses: make(map[string]*stubLBResponse),
		updateResponses: make(map[string]*stubLBResponse),
	}
	godoClient.LoadBalancers = lbStub

	log.Info("starting admission server")
	cfg, err := testEnv.Start()
	if err := webhookInstallOptions.ModifyWebhookDefinitions(); err != nil {
		log.Error(err, "failed to initialize webhook definition")
		os.Exit(1)
	}

	log.Info("setting up webhook server")
	server := webhook.NewServer(webhook.Options{
		Host:    testEnv.WebhookInstallOptions.LocalServingHost,
		Port:    testEnv.WebhookInstallOptions.LocalServingPort,
		CertDir: testEnv.WebhookInstallOptions.LocalServingCertDir,
	})
	server.Register("/lb-service", &webhook.Admission{Handler: do.NewLBServiceAdmissionHandler(&log, godoClient)})

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		log.Error(err, "failed to initialize k8s client")
		os.Exit(1)
	}

	log.Info("starting webhook server")
	ctx, cancelWebhookServer := context.WithCancel(context.Background())
	var webhookServerWg sync.WaitGroup
	webhookServerWg.Add(1)
	go func() {
		defer webhookServerWg.Done()
		if err := server.Start(ctx); err != nil {
			log.Error(err, "failed to serve webhook server")
			cancelWebhookServer()
		}
	}()

	log.Info("executing tests")
	code := m.Run()

	log.Info("tearing down test env")
	cancelWebhookServer()
	webhookServerWg.Wait()
	if err := testEnv.Stop(); err != nil {
		log.Error(err, "failed shutting down envtest gracefully")
		os.Exit(1)
	}

	os.Exit(code)
}

type stubLBResponse struct {
	lb  *godo.LoadBalancer
	r   *godo.Response
	err error
}

type godoLBStub struct {
	godo.LoadBalancersService
	createResponses map[string]*stubLBResponse
	updateResponses map[string]*stubLBResponse
	mtx             sync.Mutex
}

func (f *godoLBStub) setCreateResp(name string, r *stubLBResponse) {
	f.mtx.Lock()
	defer f.mtx.Unlock()
	f.createResponses[name] = r
}

func (f *godoLBStub) setUpdateResp(name string, r *stubLBResponse) {
	f.mtx.Lock()
	defer f.mtx.Unlock()
	f.updateResponses[name] = r
}

func (f *godoLBStub) Create(ctx context.Context, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
	f.mtx.Lock()
	defer f.mtx.Unlock()
	r, ok := f.createResponses[lbr.Name]
	if !ok {
		return nil, nil, errors.New("expected resp not found")
	}
	return r.lb, r.r, r.err
}

func (f *godoLBStub) Update(ctx context.Context, lbID string, lbr *godo.LoadBalancerRequest) (*godo.LoadBalancer, *godo.Response, error) {
	f.mtx.Lock()
	defer f.mtx.Unlock()
	r, ok := f.updateResponses[lbr.Name]
	if !ok {
		return nil, nil, errors.New("expected resp not found")
	}
	return r.lb, r.r, r.err
}
