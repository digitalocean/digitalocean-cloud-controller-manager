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
	"net/http"
	"os"
	"time"

	"github.com/digitalocean/godo"

	"golang.org/x/oauth2"

	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/client-go/informers"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog"
)

const (
	// ProviderName specifies the name for the DigitalOcean provider
	ProviderName string = "digitalocean"

	// At some point we should revisit how we start up our CCM implementation.
	// Having to look at env vars here instead of in the cmd itself is not ideal.
	// One option is to construct our own command that's specific to us.
	// Alibaba's ccm is an example how this is done.
	// https://github.com/kubernetes/cloud-provider-alibaba-cloud/blob/master/cmd/cloudprovider/app/ccm.go
	doAccessTokenEnv    string = "DO_ACCESS_TOKEN"
	doOverrideAPIURLEnv string = "DO_OVERRIDE_URL"
	doClusterIDEnv      string = "DO_CLUSTER_ID"
	doClusterVPCIDEnv   string = "DO_CLUSTER_VPC_ID"
	debugAddrEnv        string = "DEBUG_ADDR"
	workerFirewallName  string = "WORKER_FIREWALL_NAME"
)

var version string

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
	client        *godo.Client
	instances     cloudprovider.Instances
	zones         cloudprovider.Zones
	loadbalancers cloudprovider.LoadBalancer

	resources *resources

	httpServer *http.Server
}

func newCloud() (cloudprovider.Interface, error) {
	token := os.Getenv(doAccessTokenEnv)

	opts := []godo.ClientOpt{}

	if overrideURL := os.Getenv(doOverrideAPIURLEnv); overrideURL != "" {
		opts = append(opts, godo.SetBaseURL(overrideURL))
	}

	if version == "" {
		version = "dev"
	}
	opts = append(opts, godo.SetUserAgent("digitalocean-cloud-controller-manager/"+version))

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
	clusterVPCID := os.Getenv(doClusterVPCIDEnv)
	firewallName := os.Getenv(workerFirewallName)
	resources := newResources(clusterID, clusterVPCID, firewallName, doClient)

	var httpServer *http.Server
	if debugAddr := os.Getenv(debugAddrEnv); debugAddr != "" {
		debugMux := http.NewServeMux()
		healthz.InstallHandler(debugMux, &godoHealthChecker{client: doClient})
		httpServer = &http.Server{
			Addr:    debugAddr,
			Handler: debugMux,
		}
	}

	return &cloud{
		client:        doClient,
		instances:     newInstances(resources, region),
		zones:         newZones(resources, region),
		loadbalancers: newLoadBalancers(resources, doClient, region),

		resources: resources,

		httpServer: httpServer,
	}, nil
}

func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(io.Reader) (cloudprovider.Interface, error) {
		return newCloud()
	})
}

func (c *cloud) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	clientset := clientBuilder.ClientOrDie("do-shared-informers")
	sharedInformer := informers.NewSharedInformerFactory(clientset, 0)

	res := NewResourcesController(c.resources, sharedInformer.Core().V1().Services(), clientset)

	sharedInformer.Start(nil)
	sharedInformer.WaitForCacheSync(nil)
	firewallService := c.client.Firewalls

	if c.resources.firewallName != "" {
		fw, err := NewFirewallController(c.resources.firewallName, c.resources.kclient, &firewallService, sharedInformer.Core().V1().Services())
		if err != nil {
			klog.Errorf("Failed to initialize firewall controller: %s", err)
		}
		go fw.Run(stop)
	}
	go res.Run(stop)
	go c.serveDebug(stop)
}

func (c *cloud) serveDebug(stop <-chan struct{}) {
	if c.httpServer == nil {
		return
	}

	go func() {
		if err := c.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			klog.Errorf("Debug server failed: %s", err)
		}
	}()

	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.httpServer.Shutdown(ctx); err != nil {
		klog.Errorf("Failed to shut down debug server: %s", err)
	}
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
	return ProviderName
}

func (c *cloud) ScrubDNS(nameservers, searches []string) (nsOut, srchOut []string) {
	return nil, nil
}

func (c *cloud) HasClusterID() bool {
	return false
}
