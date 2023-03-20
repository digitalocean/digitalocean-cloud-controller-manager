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

func main() {
	if err := startWebhookServer(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start webhook server: %v\n", err)
		os.Exit(1)
	}
}

func startWebhookServer() error {
	// default server running at port 9443, looking for tls.crt, tls.key in /tmp/k8s-webhook-server/serving-certs
	var logOpts []zap.Opts
	ll := zap.New(logOpts...).WithName("webhook-validation-server")
	ctrlruntimelog.SetLogger(ll)

	ll.Info("getting config")

	server := webhook.Server{}

	server.Register("/validate-doks-lb-service", &webhook.Admission{Handler: &do.DOKSLBServiceValidator{Log: ll}})
	ll.Info("Registering Webhook server handlers")
	if err := server.StartStandalone(ctrl.SetupSignalHandler(), scheme); err != nil {
		return err
	}
	ll.Info("Webhooks server started")

	return nil
}
