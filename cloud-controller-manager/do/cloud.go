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
	"io"
	"os"
	"sync"
	"time"

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

	cacheDuration = 1 * time.Minute
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

	cloudResources := newCachedAPI(cacheDuration, doClient)

	return &cloud{
		clusterID:     clusterID,
		client:        doClient,
		instances:     newInstances(cloudResources, region),
		zones:         newZones(cloudResources, region),
		loadbalancers: newLoadBalancers(doClient, region, clusterID),
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

	res := NewResourcesController(buildK8sTag(c.clusterID), sharedInformer.Core().V1().Services(), clientset, c.client)

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

type cloudResources interface {
	DropletByID(ctx context.Context, id int) (*godo.Droplet, bool, error)
	DropletByName(ctx context.Context, name string) (*godo.Droplet, bool, error)

	Reload(ctx context.Context) error
}

type memResources struct {
	dropletIDMap   map[int]*godo.Droplet
	dropletNameMap map[string]*godo.Droplet
	mutex          sync.RWMutex
}

func (c *memResources) Reload(ctx context.Context) error {
	// Noop
	return nil
}

func (c *memResources) DropletByID(ctx context.Context, id int) (droplet *godo.Droplet, found bool, err error) {
	err = c.Reload(ctx)
	if err != nil {
		return nil, false, err
	}

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	droplet, found = c.dropletIDMap[id]
	return droplet, found, nil
}

func (c *memResources) DropletByName(ctx context.Context, name string) (droplet *godo.Droplet, found bool, err error) {
	err = c.Reload(ctx)
	if err != nil {
		return nil, false, err
	}

	c.mutex.RLock()
	defer c.mutex.RUnlock()

	droplet, found = c.dropletNameMap[name]
	return droplet, found, nil
}

type cachedAPI struct {
	*memResources

	client *godo.Client

	ttl        time.Duration
	expiration time.Time

	now func() time.Time
}

func newCachedAPI(ttl time.Duration, client *godo.Client) *cachedAPI {
	return &cachedAPI{
		memResources: &memResources{},

		client: client,
		ttl:    ttl,
		now:    time.Now,
	}
}

func (c *cachedAPI) Reload(ctx context.Context) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.now().Before(c.expiration) {
		return nil
	}

	newDropletIDMap := make(map[int]*godo.Droplet)
	newDropletNameMap := make(map[string]*godo.Droplet)

	droplets, err := allDropletList(ctx, c.client)
	if err != nil {
		return err
	}

	for _, droplet := range droplets {
		droplet := droplet
		newDropletIDMap[droplet.ID] = &droplet
		newDropletNameMap[droplet.Name] = &droplet
	}

	c.dropletIDMap = newDropletIDMap
	c.dropletNameMap = newDropletNameMap
	c.expiration = c.now().Add(c.ttl)
	return nil
}
