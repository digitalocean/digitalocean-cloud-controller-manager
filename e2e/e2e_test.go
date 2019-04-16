// +build integration

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

package e2e

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	schedulerapi "k8s.io/kubernetes/pkg/scheduler/api"
)

const (
	numWantNodes          = 2
	doLabel               = "beta.kubernetes.io/instance-type"
	kopsEnvVarClusterName = "KOPS_CLUSTER_NAME"
	kopsEnvVarStateStore  = "KOPS_STATE_STORE"
)

// TestE2E verifies that the node and service controller work as intended for
// all supported Kubernetes versions; that is, we expect nodes to become ready
// and requests to be routed through a DO-provisioned load balancer.
// The test creates various components and makes sure they get deleted prior to
// (to clean up any previous left-overs) and after testing.
func TestE2E(t *testing.T) {
	var missingEnvs []string
	for _, env := range []string{kopsEnvVarClusterName, kopsEnvVarStateStore} {
		if _, ok := os.LookupEnv(env); !ok {
			missingEnvs = append(missingEnvs, env)
		}
	}
	if len(missingEnvs) > 0 {
		t.Fatalf("missing required environment variable(s): %s", missingEnvs)
	}

	s3Cl, err := createS3Client()
	if err != nil {
		t.Fatalf("failed to create S3 client: %s", err)
	}

	tests := []struct {
		desc    string
		kubeVer string
	}{
		{
			desc:    "latest release",
			kubeVer: "1.12.0",
		},
		{
			desc:    "previous release",
			kubeVer: "1.11.2",
		},
		{
			desc:    "previous previous release",
			kubeVer: "1.10.6",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			l := log.New(os.Stdout, fmt.Sprintf("[%s] ", t.Name()), 0)

			cs, cleanup, err := setupCluster(l, s3Cl, t.Name(), tt.kubeVer)
			defer func() {
				l.Println("Cleaning up cluster")
				if err := cleanup(); err != nil {
					t.Errorf("failed to clean up after test: %s", err)
				}
			}()
			if err != nil {
				t.Fatalf("failed to set up cluster: %s", err)
			}

			// Check that nodes become ready.
			l.Println("Polling for node readiness")
			var (
				gotNodes      []corev1.Node
				numReadyNodes int
			)
			start := time.Now()
			if err := wait.Poll(5*time.Second, 6*time.Minute, func() (bool, error) {
				nl, err := cs.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "kubernetes.io/role=node"})
				if err != nil {
					return false, err
				}

				gotNodes = nl.Items
				numReadyNodes = 0
			Nodes:
				for _, node := range gotNodes {
					// Make sure the "uninitialized" node taint is missing.
					for _, taint := range node.Spec.Taints {
						if taint.Key == schedulerapi.TaintExternalCloudProvider {
							continue Nodes
						}
					}

					// Make sure the node is ready and has a DO-specific label
					// attached.
					for _, cond := range node.Status.Conditions {
						if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
							if _, ok := node.Labels[doLabel]; ok {
								numReadyNodes++
							}
						}
					}
				}

				l.Printf("Found %d/%d ready node(s)", numReadyNodes, numWantNodes)
				return numReadyNodes == numWantNodes, nil
			}); err != nil {
				t.Fatalf("got %d ready node(s), want %d: %s\nnnodes: %v", numReadyNodes, numWantNodes, err, spew.Sdump(gotNodes))
			}
			l.Printf("Took %v for nodes to become ready\n", time.Since(start))

			// Check that load balancer is working.

			// Install example pod to load-balance to.
			appName := "app"
			pod := corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: appName,
					Labels: map[string]string{
						"app": appName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx",
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
								},
							},
						},
					},
				},
			}

			if _, err := cs.CoreV1().Pods(corev1.NamespaceDefault).Create(&pod); err != nil {
				t.Fatalf("failed to create example pod: %s", err)
			}

			// Wait for example pod to become ready.
			l.Println("Polling for pod readiness")
			start = time.Now()
			var appPod *corev1.Pod
			if err := wait.Poll(1*time.Second, 1*time.Minute, func() (bool, error) {
				pod, err := cs.CoreV1().Pods(corev1.NamespaceDefault).Get(appName, metav1.GetOptions{})
				if err != nil {
					if kerrors.IsNotFound(err) {
						return false, nil
					}
					return false, err
				}
				appPod = pod
				for _, cond := range appPod.Status.Conditions {
					if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
						return true, nil
					}
				}
				return false, nil
			}); err != nil {
				t.Fatalf("failed to observe ready example pod %q in time: %s\npod: %v", appName, err, appPod)
			}
			l.Printf("Took %v for pod to become ready\n", time.Since(start))

			// Create service object.
			svcName := "svc"
			svc := corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: svcName,
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app": appName,
					},
					Type: corev1.ServiceTypeLoadBalancer,
					Ports: []corev1.ServicePort{
						{
							Port: 80,
						},
					},
				},
			}

			if _, err := cs.CoreV1().Services(corev1.NamespaceDefault).Create(&svc); err != nil {
				t.Fatalf("failed to create service: %s", err)
			}
			// External LBs don't seem to get deleted when the kops cluster is
			// removed, at least not on DO. Hence, we'll do it explicitly.
			var lbAddr string
			defer func() {
				l.Printf("Deleting service %q\n", svcName)
				if err := cs.CoreV1().Services(corev1.NamespaceDefault).Delete(svcName, &metav1.DeleteOptions{}); err != nil {
					t.Errorf("failed to delete service: %s", err)
				}
				// If this is the last test, CCM might not be able to remove
				// the LB before the cluster gets deleted, leaving the LB
				// dangling. Therefore, we make sure to stick around until it's
				// gone for sure, which we presume is the case if requests
				// cannot be delivered anymore due to a network error.
				cl := &http.Client{
					Timeout: 5 * time.Second,
				}
				u := fmt.Sprintf("http://%s:80", lbAddr)
				var attempts, lastStatusCode int
				if err := wait.Poll(1*time.Second, 3*time.Minute, func() (bool, error) {
					l.Printf("Sending request through winding down LB to %s", u)
					attempts++
					resp, err := cl.Get(u)
					if err == nil {
						lastStatusCode = resp.StatusCode
						resp.Body.Close()
						return false, nil
					}
					return true, nil
				}); err != nil {
					t.Fatalf("continued to deliver requests over LB to example application: %s (last status code: %d / attempts: %d)", err, lastStatusCode, attempts)
				}
				l.Printf("Needed %d attempt(s) to stop delivering sample request\n", attempts)
			}()

			// Wait for service IP address to be assigned.
			l.Println("Polling for service load balancer IP address assignment")
			start = time.Now()
			if err := wait.Poll(5*time.Second, 10*time.Minute, func() (bool, error) {
				svc, err := cs.CoreV1().Services(corev1.NamespaceDefault).Get(svcName, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				if len(svc.Status.LoadBalancer.Ingress) > 0 {
					lbAddr = svc.Status.LoadBalancer.Ingress[0].IP
					return true, nil
				}

				return false, nil
			}); err != nil {
				t.Fatalf("failed to observe load balancer IP address assignment: %s", err)
			}
			l.Printf("Took %v for load balancer to get its IP address assigned\n", time.Since(start))

			// Send request to the pod over the LB.
			cl := &http.Client{
				Timeout: 5 * time.Second,
			}
			u := fmt.Sprintf("http://%s:80", lbAddr)

			var attempts, lastStatusCode int
			if err := wait.Poll(1*time.Second, 3*time.Minute, func() (bool, error) {
				l.Printf("Sending request to %s", u)
				attempts++
				resp, err := cl.Get(u)
				if err != nil {
					return false, nil
				}
				defer resp.Body.Close()

				lastStatusCode = resp.StatusCode
				if resp.StatusCode != http.StatusOK {
					return false, nil
				}

				return true, nil
			}); err != nil {
				t.Fatalf("failed to send request over LB to example application: %s (last status code: %d / attempts: %d)", err, lastStatusCode, attempts)
			}
			l.Printf("Needed %d attempt(s) to successfully deliver sample request\n", attempts)
		})
	}
}
