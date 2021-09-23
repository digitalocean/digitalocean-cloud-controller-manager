//go:build integration
// +build integration

/*
Copyright 2020 DigitalOcean

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

package e2e

import (
	"fmt"
	"log"
	"os"
	"path"
	"strconv"

	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
)

func setupCluster(l *log.Logger, s3Cl *s3Client, name, kubeVer string) (cs kubernetes.Interface, cleanup func() error, err error) {
	// Prepare a compound cleanup function that invokes all cleanup functions
	// built along the cluster creation. The caller is expected to defer-call
	// it.
	var cleanupFuncs []func() error
	cleanup = func() error {
		var errs []error
		// Iterate backwards to maintain LIFO cleanup order.
		for i := len(cleanupFuncs) - 1; i >= 0; i-- {
			err := cleanupFuncs[i]()
			errs = append(errs, err)
		}
		return kerrors.NewAggregate(errs)
	}

	dnsName := toDNSName(name)

	wd, err := os.Getwd()
	if err != nil {
		return nil, cleanup, fmt.Errorf("failed to get working directory: %s", err)
	}
	kubeConfFile := path.Join(wd, "kubeconfig-e2e."+dnsName)

	// Delete old kubeconfig
	if err := os.Remove(kubeConfFile); err != nil && !os.IsNotExist(err) {
		return nil, cleanup, fmt.Errorf("failed to delete kubeconfig %q: %s", kubeConfFile, err)
	}

	// Create space.
	storeName := toS3Name(fmt.Sprintf("%s-%s", os.Getenv(kopsEnvVarClusterName), dnsName))
	if err := s3Cl.deleteSpace(storeName); err != nil {
		return nil, cleanup, fmt.Errorf("failed to delete space %q (pre-test): %s", storeName, err)
	}
	if err := s3Cl.ensureSpace(storeName); err != nil {
		return nil, cleanup, fmt.Errorf("failed to ensure space %q: %s", storeName, err)
	}
	cleanupFuncs = append(cleanupFuncs, func() error {
		l.Printf("Deleting space %q\n", storeName)
		if err := s3Cl.deleteSpace(storeName); err != nil {
			return fmt.Errorf("failed to delete space %q (post-test): %s", storeName, err)
		}
		return nil
	})

	// Create cluster.
	extraEnvs := []string{
		fmt.Sprintf("%s=do://%s", kopsEnvVarStateStore, storeName),
		"KUBECONFIG=" + kubeConfFile,
	}
	if err := runScript(extraEnvs, "destroy_cluster.sh"); err != nil {
		return nil, cleanup, fmt.Errorf("failed to destroy cluster (pre-test): %s", err)
	}
	if err := runScript(extraEnvs, "setup_cluster.sh", kubeVer, strconv.Itoa(numWantNodes)); err != nil {
		return nil, cleanup, fmt.Errorf("failed to set up cluster: %s", err)
	}
	cleanupFuncs = append(cleanupFuncs, func() error {
		l.Println("Destroying cluster")
		if err := runScript(extraEnvs, "destroy_cluster.sh"); err != nil {
			return fmt.Errorf("failed to destroy cluster (post-test): %s", err)
		}
		return nil
	})

	cs, err = kubeClient(kubeConfFile)
	if err != nil {
		return nil, cleanup, fmt.Errorf("failed to create Kubernetes client: %s", err)
	}

	return cs, cleanup, nil
}
