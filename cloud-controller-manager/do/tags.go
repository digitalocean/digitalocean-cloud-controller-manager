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
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/digitalocean/godo"
	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	v1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	v1lister "k8s.io/client-go/listers/core/v1"
)

const (
	controllerName       = "tagscontroller"
	tagsLabelPrefix      = "digitalocean.com/droplet-tag/"
	controllerSyncPeriod = time.Minute
)

type tagsController struct {
	kclient    kubernetes.Interface
	gclient    *godo.Client
	nodeLister v1lister.NodeLister
}

// NewTagsController returns a new tags controller responsible for applying
// droplet tags to node labels. It will return an error if the kubernetes client
// failed to initialize.
func NewTagsController(n v1informers.NodeInformer,
	k kubernetes.Interface, g *godo.Client) *tagsController {
	return &tagsController{
		kclient:    k,
		gclient:    g,
		nodeLister: n.Lister(),
	}
}

// Run starts the tags controller. Tags controllers runs on a loop rather than watching
// for changes because there is no way to watch for droplet events (yet) and watching
// node events can be expensive as each node update event (mostly node health checks)
// would result in an API call to fetch droplets. Caching droplets in memory
// may be a better solution but this should work for now.
// TODO: pass in stopch once it's supported upstream
func (t *tagsController) Run() error {
	tick := time.Tick(controllerSyncPeriod)
	for {
		select {
		case <-tick:
			err := t.sync()
			if err != nil {
				glog.Errorf("failed to sync droplet tags: %s", err)
			}
		}
	}
}

func (t *tagsController) sync() error {
	droplets, err := allDropletList(context.TODO(), t.gclient)
	if err != nil {
		return fmt.Errorf("failed to list droplets: %s", err)
	}

	// due to a lack of a consistent tagging scheme, we have to
	// assume any droplet can be part of a Kubernetes cluster and filter
	// based on the provider ID set on the node resource
	dropletTagLabelMap := make(map[string][]string)
	for _, droplet := range droplets {
		providerID := "digitalocean://" + strconv.Itoa(droplet.ID)
		dropletTagLabelMap[providerID] = generateTagLabelsFromDroplet(droplet)
	}

	nodes, err := t.nodeLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list nodes from cache: %s", err)
	}

	for _, node := range nodes {
		if node.Spec.ProviderID == "" {
			glog.Warningf("skipping node %s as it is not initailized with a cloud provider", node.Name)
			continue
		}

		actualTagLabels := extractNodeTagLabels(node)
		desiredTagLabels, exists := dropletTagLabelMap[node.Spec.ProviderID]
		if !exists {
			glog.Warningf("droplet for node %s could not be found via cloud API", node.Name)
			continue
		}

		if !shouldUpdate(actualTagLabels, desiredTagLabels) {
			continue
		}

		toAddLabels := labelsToAdd(actualTagLabels, desiredTagLabels)
		toDelLabels := labelsToDel(actualTagLabels, desiredTagLabels)

		// DeepCopy cause cached resources should not be used for updates
		newNode := node.DeepCopy()
		for _, label := range toAddLabels {
			newNode.Labels[label] = ""
		}

		for _, label := range toDelLabels {
			delete(newNode.Labels, label)
		}

		if _, err := t.kclient.CoreV1().Nodes().Update(newNode); err != nil {
			glog.Errorf("failed to update node with droplet tags: %s", err)
		}
	}

	return nil
}

func extractNodeTagLabels(node *corev1.Node) []string {
	var labels []string
	for label := range node.Labels {
		if strings.HasPrefix(label, tagsLabelPrefix) {
			labels = append(labels, label)
		}
	}

	return labels
}

func generateTagLabelsFromDroplet(d godo.Droplet) []string {
	var labels []string
	for _, tag := range d.Tags {
		labels = append(labels, tagsLabelPrefix+tag)
	}

	return labels
}

func shouldUpdate(actualTags, desiredTags []string) bool {
	if len(actualTags) != len(desiredTags) {
		return true
	}

	sort.Strings(actualTags)
	sort.Strings(desiredTags)

	return !reflect.DeepEqual(actualTags, desiredTags)
}

func labelsToAdd(actualTags, desiredTags []string) []string {
	actualTagsMap := make(map[string]struct{})
	desiredTagsMap := make(map[string]struct{})

	for _, tag := range actualTags {
		actualTagsMap[tag] = struct{}{}
	}

	for _, tag := range desiredTags {
		desiredTagsMap[tag] = struct{}{}
	}

	var toAdd []string
	for tag := range desiredTagsMap {
		if _, exists := actualTagsMap[tag]; !exists {
			toAdd = append(toAdd, tag)
		}
	}

	return toAdd
}

func labelsToDel(actualTags, desiredTags []string) []string {
	actualTagsMap := make(map[string]struct{})
	desiredTagsMap := make(map[string]struct{})

	for _, tag := range actualTags {
		actualTagsMap[tag] = struct{}{}
	}

	for _, tag := range desiredTags {
		desiredTagsMap[tag] = struct{}{}
	}

	var toDel []string
	for tag := range actualTagsMap {
		if _, exists := desiredTagsMap[tag]; !exists {
			toDel = append(toDel, tag)
		}
	}

	return toDel
}
