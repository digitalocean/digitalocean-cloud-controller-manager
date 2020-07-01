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
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const scriptDirectory = "scripts/"

var (
	dnsInvalidCharsRE    *regexp.Regexp
	s3NameInvalidCharsRE *regexp.Regexp
)

func init() {
	dnsInvalidCharsRE = regexp.MustCompile("[^a-zA-Z0-9.-]")
	s3NameInvalidCharsRE = regexp.MustCompile("[^a-zA-Z0-9-]")
}

// toDNSName converts the given name into a DNS-conform one, replacing
// prohibited characters by dashes.
// The function does not check for length constraints (neither component-wise
// nor overall).
func toDNSName(name string) string {
	low := strings.ToLower(name)
	return dnsInvalidCharsRE.ReplaceAllString(low, "-")
}

// toS3Name converts the given name into one valid for S3 usage, replacing
// prohibited characters by dashes.
func toS3Name(name string) string {
	low := strings.ToLower(name)
	return s3NameInvalidCharsRE.ReplaceAllString(low, "-")
}

func kubeClient(kubeconfig string) (kubernetes.Interface, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return clientset, nil
}

func runScript(extraEnvs []string, script string, args ...string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %s", err)
	}

	return runCommand(extraEnvs, path.Join(wd, scriptDirectory, script), args...)
}

func runCommand(extraEnvs []string, cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Env = append(c.Env, append(os.Environ(), extraEnvs...)...)
	fmt.Printf("Running command %q with extra envs %s\n", cmd, extraEnvs)
	return c.Run()
}
