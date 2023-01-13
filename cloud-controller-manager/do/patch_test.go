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

package do

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPatchService(t *testing.T) {
	cur := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "service",
			Namespace:   metav1.NamespaceDefault,
			Annotations: map[string]string{},
		},
	}
	mod := cur.DeepCopy()
	mod.Annotations["copy"] = "true"

	cs := fake.NewSimpleClientset()
	if _, err := cs.CoreV1().Services(metav1.NamespaceDefault).Create(context.Background(), cur, metav1.CreateOptions{}); err != nil {
		t.Fatalf("failed to create service: %s", err)
	}

	err := patchService(context.Background(), cs, cur, mod)
	if err != nil {
		t.Fatalf("failed to patch service: %s", err)
	}

	svc, err := cs.CoreV1().Services(metav1.NamespaceDefault).Get(context.Background(), cur.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get service: %s", err)
	}

	want := "true"
	got := svc.Annotations["copy"]
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
