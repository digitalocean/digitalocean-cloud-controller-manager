/*
Copyright 2017 DigitalOcean

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
	"flag"
	"fmt"
	"os"

	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/cmd/cloud-controller-manager/app"
	_ "k8s.io/kubernetes/pkg/client/metrics/prometheus" // for client metric registration
	_ "k8s.io/kubernetes/pkg/version/prometheus"        // for version metric registration

	_ "github.com/digitalocean/digitalocean-cloud-controller-manager/cloud-controller-manager/do"
)

func init() {
	healthz.DefaultHealthz()
}

func main() {
	// TODO(timoreimann): Remove bogus GCE parameter when
	// https://github.com/kubernetes/kubernetes/issues/76205 gets shipped in
	// Kubernetes 1.15.
	flag.CommandLine.String("cloud-provider-gce-lb-src-cidrs", "", "NOT USED (workaround for https://github.com/kubernetes/kubernetes/issues/76205)")

	command := app.NewCloudControllerManagerCommand()

	// (The following comment is copied from upstream:)
	// TODO: once we switch everything over to Cobra commands, we can go back to calling
	// utilflag.InitFlags() (by removing its pflag.Parse() call). For now, we have to set the
	// normalize func and add the go flag set by hand.
	// utilflag.InitFlags()
	logs.InitLogs()
	defer logs.FlushLogs()

	if err := command.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
