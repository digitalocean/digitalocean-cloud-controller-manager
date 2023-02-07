package main

import (
	"fmt"
	"github.com/digitalocean/digitalocean-cloud-controller-manager/cloud-controller-manager/do"
	"k8s.io/apimachinery/pkg/runtime"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	scheme = runtime.NewScheme()
)


func main () {
	if err := startWebhookServer(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start webhook server: %v\n", err)
		os.Exit(1)
	}
}

func startWebhookServer() error {
	// default server running at port 9443, looking for tls.crt, tls.key in /tmp/k8s-webhook-server/serving-certs
	ll := ctrl.Log.WithName("dokswebhooks")
	ll.Info("getting config")
	config := ctrl.GetConfigOrDie()
	server := webhook.Server{}
	c, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		ll.Error(err, "failed to construct cr client")
		return err
	}

	server.Register("/validate-doks-lb-service", &webhook.Admission{Handler: &do.DOKSLBServiceValidator{Client: c, Log: ll}})
	if err := server.StartStandalone(ctrl.SetupSignalHandler(), scheme); err != nil {
		return err
	}

	return nil
}