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
	"fmt"
	"io"
	"os"

	"github.com/digitalocean/godo"
	"github.com/golang/glog"

	"golang.org/x/oauth2"

	"k8s.io/client-go/informers"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/controller"
)

const (
	doAccessTokenEnv    string = "DO_ACCESS_TOKEN"
	doOverrideAPIURLEnv string = "DO_OVERRIDE_URL"
	doClusterIDEnv      string = "DO_CLUSTER_ID"
	providerName        string = "digitalocean"
)

type tokenSource struct {
	AccessToken string
}

func (t *tokenSource) Token() (*oauth2.Token, error) {
	token := &oauth2.Token{
		AccessToken: t.AccessToken,
	}
	return token, nil
}

type cloud struct {
	clusterID     string
	client        *godo.Client
	instances     cloudprovider.Instances
	zones         cloudprovider.Zones
	loadbalancers cloudprovider.LoadBalancer

	resources *resources
}

func newCloud() (cloudprovider.Interface, error) {
	token := os.Getenv(doAccessTokenEnv)

	opts := []godo.ClientOpt{}

	if overrideURL := os.Getenv(doOverrideAPIURLEnv); overrideURL != "" {
		opts = append(opts, godo.SetBaseURL(overrideURL))
	}

	if token == "" {
		return nil, fmt.Errorf("environment variable %q is required", doAccessTokenEnv)
	}

	tokenSource := &tokenSource{
		AccessToken: token,
	}

	oauthClient := oauth2.NewClient(oauth2.NoContext, tokenSource)
	doClient, err := godo.New(oauthClient, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create godo client: %s", err)
	}

	region, err := dropletRegion()
	if err != nil {
		return nil, fmt.Errorf("failed to get region from droplet metadata: %s", err)
	}

	clusterID := os.Getenv(doClusterIDEnv)
	resources := newResources()

	return &cloud{
		clusterID:     clusterID,
		client:        doClient,
		instances:     newInstances(resources, region),
		zones:         newZones(resources, region),
		loadbalancers: newLoadBalancers(doClient, region, clusterID),

		resources: resources,
	}, nil
}

func init() {
	cloudprovider.RegisterCloudProvider(providerName, func(io.Reader) (cloudprovider.Interface, error) {
		return newCloud()
	})
}

func (c *cloud) Initialize(clientBuilder controller.ControllerClientBuilder) {
	if c.clusterID == "" {
		glog.Info("No cluster ID configured -- skipping resource controller initialization.")
		return
	}

	clientset := clientBuilder.ClientOrDie("do-shared-informers")
	sharedInformer := informers.NewSharedInformerFactory(clientset, 0)

	res := NewResourcesController(c.clusterID, c.resources, sharedInformer.Core().V1().Services(), clientset, c.client)

	sharedInformer.Start(nil)
	sharedInformer.WaitForCacheSync(nil)
	// TODO: pass in stopCh from Initialize once supported upstream
	// see https://github.com/kubernetes/kubernetes/pull/70038 for more details
	go res.Run(nil)
}

func (c *cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return c.loadbalancers, true
}

func (c *cloud) Instances() (cloudprovider.Instances, bool) {
	return c.instances, true
}

func (c *cloud) Zones() (cloudprovider.Zones, bool) {
	return c.zones, true
}

func (c *cloud) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

func (c *cloud) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

func (c *cloud) ProviderName() string {
	return providerName
}

func (c *cloud) ScrubDNS(nameservers, searches []string) (nsOut, srchOut []string) {
	return nil, nil
}

func (c *cloud) HasClusterID() bool {
	return false
}
