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
	"golang.org/x/sync/errgroup"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	metrics_server "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/digitalocean/digitalocean-cloud-controller-manager/cloud-controller-manager/do"
	"github.com/digitalocean/godo"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	doAccessTokenEnv    string = "DO_ACCESS_TOKEN"
	doOverrideAPIURLEnv        = "DO_OVERRIDE_URL"
	doClusterIDEnv             = "DO_CLUSTER_ID"
	doClusterVPCIDEnv          = "DO_CLUSTER_VPC_ID"
	metricsAddr                = "METRICS_ADDR"
)

var loggerVerbosity = flag.Int("v", 0, "logger verbosity")

var webhookAdmissionsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "webhook_admissions_total",
		Help: "The total number of admissions served by a webhook.",
	},
	[]string{"status", "webhook"},
)

func init() {
	flag.Parse()
	log.SetLogger(zap.New(zap.Level(zapcore.Level(-*loggerVerbosity))))
}

func main() {
	ctx := signals.SetupSignalHandler()
	group := errgroup.Group{}
	group.Go(func() error { return startAdmissionServer(ctx) })
	group.Go(func() error { return startMetricServer(ctx) })
	if err := group.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "error serving admission server: %v\n", err)
		os.Exit(1)
	}
}

func startAdmissionServer(ctx context.Context) error {
	ll := ctrl.Log.WithName("digitalocean-cloud-controller-manager-admission-server")

	server := webhook.NewServer(webhook.Options{})

	godoClient, err := getGodoClient()
	if err != nil {
		return fmt.Errorf("failed to get godo client: %s", err)
	}

	lbAdmissionHandler := do.NewLBServiceAdmissionHandler(&ll, godoClient, webhookAdmissionsTotal)
	if err := lbAdmissionHandler.WithRegion(); err != nil {
		return fmt.Errorf("failed to inject region into lb service admission handler: %w", err)
	}

	clusterID := os.Getenv(doClusterIDEnv)
	if clusterID == "" {
		return fmt.Errorf("environment variable %q is required", doClusterIDEnv)
	}
	lbAdmissionHandler.WithClusterID(clusterID)

	vpcID := os.Getenv(doClusterVPCIDEnv)
	if vpcID == "" {
		return fmt.Errorf("environment variable %q is required", doClusterVPCIDEnv)
	}
	lbAdmissionHandler.WithVPCID(vpcID)

	ll.Info("registering admission handlers")
	server.Register("/lb-service", &webhook.Admission{Handler: lbAdmissionHandler})

	return server.Start(ctx)
}

func startMetricServer(ctx context.Context) error {
	addr := os.Getenv(metricsAddr)
	if addr == "" {
		return fmt.Errorf("environment variable %q is required", metricsAddr)
	}

	server, err := metrics_server.NewServer(metrics_server.Options{
		BindAddress: addr,
	}, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to initialize metrics server: %w", err)
	}

	metrics.Registry.MustRegister(webhookAdmissionsTotal)

	return server.Start(ctx)
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
