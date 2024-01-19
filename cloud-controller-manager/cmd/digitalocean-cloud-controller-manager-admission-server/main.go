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
	"context"
	"flag"
	"fmt"
	"os"

	"go.uber.org/zap/zapcore"
	"golang.org/x/oauth2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/digitalocean/digitalocean-cloud-controller-manager/cloud-controller-manager/do"
	"github.com/digitalocean/godo"
)

const (
	doAccessTokenEnv    string = "DO_ACCESS_TOKEN"
	doOverrideAPIURLEnv        = "DO_OVERRIDE_URL"
	doClusterIDEnv             = "DO_CLUSTER_ID"
	doClusterVPCIDEnv          = "DO_CLUSTER_VPC_ID"
)

var loggerVerbosity = flag.Int("v", 0, "logger verbosity")

func init() {
	flag.Parse()
	log.SetLogger(zap.New(zap.Level(zapcore.Level(-*loggerVerbosity))))
}

func main() {
	if err := startAdmissionServer(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start admission server: %s\n", err)
		os.Exit(1)
	}
}

func startAdmissionServer() error {
	ll := ctrl.Log.WithName("digitalocean-cloud-controller-manager-admission-server")

	server := webhook.NewServer(webhook.Options{})

	godoClient, err := getGodoClient()
	if err != nil {
		return fmt.Errorf("failed to get godo client: %s", err)
	}

	lbAdmissionHandler := do.NewLBServiceAdmissionHandler(&ll, godoClient)
	if err := lbAdmissionHandler.WithRegion(); err != nil {
		return fmt.Errorf("failed to inject region into lb service admission handler: %w", err)
	}

	// Cluster ID is optional.
	clusterID := os.Getenv(doClusterIDEnv)
	lbAdmissionHandler.WithClusterID(clusterID)

	// VPC ID is optional.
	vpcID := os.Getenv(doClusterVPCIDEnv)
	lbAdmissionHandler.WithVPCID(vpcID)

	ll.Info("registering admission handlers")
	server.Register("/lb-service", &webhook.Admission{Handler: lbAdmissionHandler})

	if err = server.Start(signals.SetupSignalHandler()); err != nil {
		return fmt.Errorf("failed to serve webhook server: %s", err)
	}
	ll.Info("Webhooks server started")

	return nil
}

func getGodoClient() (*godo.Client, error) {
	token := os.Getenv(doAccessTokenEnv)
	if token == "" {
		return nil, fmt.Errorf("environment variable %q is required", doAccessTokenEnv)
	}

	opts := []godo.ClientOpt{}

	if overrideURL := os.Getenv(doOverrideAPIURLEnv); overrideURL != "" {
		opts = append(opts, godo.SetBaseURL(overrideURL))
	}

	tokenSource := &tokenSource{AccessToken: token}
	oauthClient := oauth2.NewClient(context.Background(), tokenSource)
	client, err := godo.New(oauthClient, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize client: %s", err)
	}

	return client, nil
}

type tokenSource struct {
	AccessToken string
}

func (t *tokenSource) Token() (*oauth2.Token, error) {
	token := &oauth2.Token{
		AccessToken: t.AccessToken,
	}
	return token, nil
}
