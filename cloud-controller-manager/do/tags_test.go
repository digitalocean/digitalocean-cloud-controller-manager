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

package do

import (
	"context"
	"reflect"
	"testing"

	"github.com/digitalocean/godo"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_sync(t *testing.T) {
	testcases := []struct {
		name          string
		droplets      []godo.Droplet
		existingNodes []*corev1.Node
		updatedNodes  []*corev1.Node
		err           error
	}{
		{
			name: "nodes need droplet tag labels",
			droplets: []godo.Droplet{
				{
					ID:   11111,
					Tags: []string{"foo", "bar"},
				},
				{
					ID:   22222,
					Tags: []string{"foo", "bar"},
				},
			},
			existingNodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "node11111",
						Labels: map[string]string{},
					},
					Spec: corev1.NodeSpec{
						ProviderID: "digitalocean://11111",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "node22222",
						Labels: map[string]string{},
					},
					Spec: corev1.NodeSpec{
						ProviderID: "digitalocean://22222",
					},
				},
			},
			updatedNodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node11111",
						Labels: map[string]string{
							"digitalocean.com/droplet-tag/foo": "",
							"digitalocean.com/droplet-tag/bar": "",
						},
					},
					Spec: corev1.NodeSpec{
						ProviderID: "digitalocean://11111",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node22222",
						Labels: map[string]string{
							"digitalocean.com/droplet-tag/foo": "",
							"digitalocean.com/droplet-tag/bar": "",
						},
					},
					Spec: corev1.NodeSpec{
						ProviderID: "digitalocean://22222",
					},
				},
			},
			err: nil,
		},
		{
			name: "nodes need droplet tag labels, but some tags already exist",
			droplets: []godo.Droplet{
				{
					ID:   11111,
					Tags: []string{"foo", "bar", "foobar"},
				},
				{
					ID:   22222,
					Tags: []string{"foo", "bar"},
				},
			},
			existingNodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node11111",
						Labels: map[string]string{
							"digitalocean.com/droplet-tag/foo": "",
							"digitalocean.com/droplet-tag/bar": "",
						},
					},
					Spec: corev1.NodeSpec{
						ProviderID: "digitalocean://11111",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node22222",
						Labels: map[string]string{
							"digitalocean.com/droplet-tag/foo": "",
						},
					},
					Spec: corev1.NodeSpec{
						ProviderID: "digitalocean://22222",
					},
				},
			},
			updatedNodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node11111",
						Labels: map[string]string{
							"digitalocean.com/droplet-tag/foo":    "",
							"digitalocean.com/droplet-tag/bar":    "",
							"digitalocean.com/droplet-tag/foobar": "",
						},
					},
					Spec: corev1.NodeSpec{
						ProviderID: "digitalocean://11111",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node22222",
						Labels: map[string]string{
							"digitalocean.com/droplet-tag/foo": "",
							"digitalocean.com/droplet-tag/bar": "",
						},
					},
					Spec: corev1.NodeSpec{
						ProviderID: "digitalocean://22222",
					},
				},
			},
			err: nil,
		},
		{
			name: "tag was removed from droplet",
			droplets: []godo.Droplet{
				{
					ID:   11111,
					Tags: []string{"foo"},
				},
			},
			existingNodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node11111",
						Labels: map[string]string{
							"digitalocean.com/droplet-tag/foo": "",
							"digitalocean.com/droplet-tag/bar": "",
						},
					},
					Spec: corev1.NodeSpec{
						ProviderID: "digitalocean://11111",
					},
				},
			},
			updatedNodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node11111",
						Labels: map[string]string{
							"digitalocean.com/droplet-tag/foo": "",
						},
					},
					Spec: corev1.NodeSpec{
						ProviderID: "digitalocean://11111",
					},
				},
			},
			err: nil,
		},
		{
			name: "ensure any other labels on the node are not deleted",
			droplets: []godo.Droplet{
				{
					ID:   11111,
					Tags: []string{"foo", "bar", "dog"},
				},
				{
					ID:   22222,
					Tags: []string{"foo"},
				},
			},
			existingNodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node11111",
						Labels: map[string]string{
							"digitalocean.com/droplet-tag/foo": "",
							"digitalocean.com/droplet-tag/bar": "",
							"kubernetes.io/hostname":           "node11111",
							"beta.kubernetes.io/os":            "dogOS",
						},
					},
					Spec: corev1.NodeSpec{
						ProviderID: "digitalocean://11111",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node22222",
						Labels: map[string]string{
							"digitalocean.com/droplet-tag/foo": "",
							"digitalocean.com/droplet-tag/bar": "",
							"kubernetes.io/hostname":           "node22222",
							"beta.kubernetes.io/os":            "catOS",
						},
					},
					Spec: corev1.NodeSpec{
						ProviderID: "digitalocean://22222",
					},
				},
			},
			updatedNodes: []*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node11111",
						Labels: map[string]string{
							"digitalocean.com/droplet-tag/foo": "",
							"digitalocean.com/droplet-tag/bar": "",
							"digitalocean.com/droplet-tag/dog": "",
							"kubernetes.io/hostname":           "node11111",
							"beta.kubernetes.io/os":            "dogOS",
						},
					},
					Spec: corev1.NodeSpec{
						ProviderID: "digitalocean://11111",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "node22222",
						Labels: map[string]string{
							"digitalocean.com/droplet-tag/foo": "",
							"kubernetes.io/hostname":           "node22222",
							"beta.kubernetes.io/os":            "catOS",
						},
					},
					Spec: corev1.NodeSpec{
						ProviderID: "digitalocean://22222",
					},
				},
			},
			err: nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			fakeDropletService := &fakeDropletService{
				listFunc: func(ctx context.Context, opt *godo.ListOptions) ([]godo.Droplet, *godo.Response, error) {
					droplets := tc.droplets
					resp := newFakeOKResponse()
					return droplets, resp, nil
				},
			}

			gclient := godo.NewClient(nil)
			gclient.Droplets = fakeDropletService
			kclient := fake.NewSimpleClientset()

			for _, node := range tc.existingNodes {
				_, err := kclient.CoreV1().Nodes().Create(node)
				if err != nil {
					t.Fatalf("failed to create fake node resource %s", err)
				}
			}

			sharedInformer := informers.NewSharedInformerFactory(kclient, 0)
			tags := NewTagsController(sharedInformer.Core().V1().Nodes(), kclient, gclient)

			sharedInformer.Start(nil)
			sharedInformer.WaitForCacheSync(nil)

			err := tags.sync()
			if !reflect.DeepEqual(err, tc.err) {
				t.Logf("actual err: %s", err)
				t.Logf("expected err: %s", tc.err)
				t.Error("unexpected error")
			}

			for _, expectedNode := range tc.updatedNodes {
				actualNode, err := kclient.CoreV1().Nodes().Get(expectedNode.Name, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("failed to get updated node: %s", err)
				}

				if !reflect.DeepEqual(actualNode, expectedNode) {
					t.Logf("actual node: %v", actualNode)
					t.Logf("expected node: %v", expectedNode)
					t.Errorf("unexpected node")
				}
			}

		})
	}
}
