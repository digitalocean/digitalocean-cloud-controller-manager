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

package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/digitalocean/godo"
	"golang.org/x/oauth2"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlruntimelog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/digitalocean/digitalocean-cloud-controller-manager/cloud-controller-manager/do"
)

var (
	scheme = runtime.NewScheme()
)

const (
	doAccessTokenEnv     string = "DO_LB_WEBHOOK_ACCESS_TOKEN"
	doAPIRateLimitQPSEnv string = "DO_LB_WEBHOOK_API_RATE_LIMIT_QPS"
)

var version string

var logOpts []zap.Opts

type tokenSource struct {
	AccessToken string
}

func (t *tokenSource) Token() (*oauth2.Token, error) {
	token := &oauth2.Token{
		AccessToken: t.AccessToken,
	}
	return token, nil
}

func initDOClient() (doClient *godo.Client, err error) {

	ll := zap.New(logOpts...).WithName("digitalocean-lb-validation-webhook-validation-server")

	token := os.Getenv(doAccessTokenEnv)

	if token == "" {
		return nil, fmt.Errorf("environment variable %q is required", doAccessTokenEnv)
	}

	tokenSource := &tokenSource{
		AccessToken: token,
	}

	var opts []godo.ClientOpt

	opts = append(opts, godo.SetUserAgent("digitalocean-lb-validation-webhook-server/"+version))

	if qpsRaw := os.Getenv(doAPIRateLimitQPSEnv); qpsRaw != "" {
		qps, err := strconv.ParseFloat(qpsRaw, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse value from environment variable %s: %s", doAPIRateLimitQPSEnv, err)
		}
		ll.Info("Setting DO API rate limit to %.2f QPS", qps)
		opts = append(opts, godo.SetStaticRateLimit(qps))
	}

	oauthClient := oauth2.NewClient(oauth2.NoContext, tokenSource)
	doClient, err = godo.New(oauthClient, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create godo client: %s", err)
	}

	return doClient, nil
}

func main() {
	if err := startWebhookServer(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start webhook server: %v\n", err)
		os.Exit(1)
	}
}

func startWebhookServer() error {

	ll := zap.New(logOpts...).WithName("validation-webhook-server")
	ctrlruntimelog.SetLogger(ll)
	ll.Info("THIS  VALIDATION WEBHOOK SERVER IS STILL IN DEVELOPMENT AND IS PURELY EXPERIMENTAL")

	server := webhook.Server{}

	DOClient, err := initDOClient()
	if err != nil {
		return fmt.Errorf("failed to initialize DO client: %s", err)
	}

	server.Register("/validate-doks-lb-service", &webhook.Admission{Handler: &do.KubernetesLBServiceValidator{
		Log:     ll,
		GClient: DOClient,
	}})

	ll.Info("Registering loadbalancer validation webhook server handlers")
	if err := server.StartStandalone(ctrl.SetupSignalHandler(), scheme); err != nil {
		ll.Error(err, "failed to start loadbalancer validation webhook server")
		os.Exit(1)
	}
	ll.Info("loadbalancer validation webhook server started")

	return nil
}
