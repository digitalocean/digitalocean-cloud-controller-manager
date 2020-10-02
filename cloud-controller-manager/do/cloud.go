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

package do

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/digitalocean/godo"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"golang.org/x/oauth2"

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
	doAccessTokenEnv            string = "DO_ACCESS_TOKEN"
	doOverrideAPIURLEnv         string = "DO_OVERRIDE_URL"
	doClusterIDEnv              string = "DO_CLUSTER_ID"
	doClusterVPCIDEnv           string = "DO_CLUSTER_VPC_ID"
	debugAddrEnv                string = "DEBUG_ADDR"
	metricsAddrEnv              string = "METRICS_ADDR"
	publicAccessFirewallNameEnv string = "PUBLIC_ACCESS_FIREWALL_NAME"
	publicAccessFirewallTagsEnv string = "PUBLIC_ACCESS_FIREWALL_TAGS"
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
	metrics       metrics

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
	firewallName := os.Getenv(publicAccessFirewallNameEnv)
	firewallTags := os.Getenv(publicAccessFirewallTagsEnv)
	if firewallName == "" && firewallTags != "" {
		return nil, fmt.Errorf("environment variable %q is required when managing firewalls", publicAccessFirewallNameEnv)
	}
	if firewallName != "" && firewallTags == "" {
		return nil, fmt.Errorf("environment variable %q is required when managing firewalls", publicAccessFirewallTagsEnv)
	}
	tags := strings.Split(firewallTags, ",")
	resources := newResources(clusterID, clusterVPCID, publicAccessFirewall{firewallName, tags}, doClient)

	var httpServer *http.Server
	if debugAddr := os.Getenv(debugAddrEnv); debugAddr != "" {
		debugMux := http.NewServeMux()
		debugMux.Handle("/healthz", &godoHealthChecker{client: doClient})
		httpServer = &http.Server{
			Addr:    debugAddr,
			Handler: debugMux,
		}
	}

	var addr string
	if metricsAddr := os.Getenv(metricsAddrEnv); metricsAddr != "" {
		addrHost, addrPort, err := net.SplitHostPort(metricsAddr)
		if err != nil {
			return nil, fmt.Errorf("environment variable %q requires host and port in the format <host>:<port> when exposing metrics: %v", metricsAddr, err)
		}
		addr = fmt.Sprintf("%s:%s", addrHost, addrPort)
	}

	return &cloud{
		client:        doClient,
		instances:     newInstances(resources, region),
		zones:         newZones(resources, region),
		loadbalancers: newLoadBalancers(resources, region),
		metrics:       newMetrics(addr),
		resources:     resources,

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

	go res.Run(stop)
	go c.serveDebug(stop)
	go c.serveMetrics()

	if c.resources.firewall.name == "" {
		klog.Info("Nothing to manage since firewall name was not provided")
		return
	}
	klog.Infof("Managing the firewall using provided firewall worker name: %s", c.resources.firewall.name)
	fm := firewallManager{
		client: c.client,
		fwCache: firewallCache{
			mu: new(sync.RWMutex),
		},
		workerFirewallName: c.resources.firewall.name,
		workerFirewallTags: c.resources.firewall.tags,
		metrics:            c.metrics,
	}
	ctx := context.Background()
	fc := NewFirewallController(ctx, c.resources.kclient, c.client, sharedInformer.Core().V1().Services(), fm, fm.workerFirewallTags, fm.workerFirewallName, c.metrics)
	go fc.runWorker()
	go fc.Run(ctx, stop, firewallReconcileFrequency)
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

func (c *cloud) serveMetrics() {
	http.Handle("/metrics", promhttp.Handler())

	// register metrics
	prometheus.MustRegister(apiRequestDuration)
	prometheus.MustRegister(runLoopDuration)
	prometheus.MustRegister(reconcileDuration)

	if err := http.ListenAndServe(c.metrics.host, nil); err != http.ErrServerClosed {
		klog.Warningf("Metrics server has not been configured: %s", err)
	}
}

func (c *cloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return c.loadbalancers, true
}

func (c *cloud) Instances() (cloudprovider.Instances, bool) {
	return c.instances, true
}

func (c *cloud) InstancesV2() (cloudprovider.InstancesV2, bool) {
	// TODO: Implement the InstancesV2 interface. Our API should be sufficient
	// to fetch all the necessary implementation, but it's not required at the
	// moment.
	return nil, false
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
