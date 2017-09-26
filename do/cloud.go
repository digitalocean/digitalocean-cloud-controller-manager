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
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/digitalocean/godo"

	"golang.org/x/oauth2"

	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/controller"
)

const DOAccessTokenEnv string = "DO_ACCESS_TOKEN"
const providerName string = "digitalocean"

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
}

func newCloud(config io.Reader) (cloudprovider.Interface, error) {
	token := os.Getenv(DOAccessTokenEnv)

	if token == "" {
		return nil, fmt.Errorf("environment variable %q is required", DOAccessTokenEnv)
	}

	tokenSource := &tokenSource{
		AccessToken: token,
	}

	oauthClient := oauth2.NewClient(oauth2.NoContext, tokenSource)
	doClient := godo.NewClient(oauthClient)

	region, err := dropletRegion()
	if err != nil {
		return nil, errors.New("faild to get region from droplet metadata")
	}

	return &cloud{
		client:        doClient,
		instances:     newInstances(doClient, region),
		zones:         newZones(region),
		loadbalancers: newLoadbalancers(doClient, region),
	}, nil
}

func init() {
	cloudprovider.RegisterCloudProvider(providerName, func(config io.Reader) (cloudprovider.Interface, error) {
		return newCloud(config)
	})
}

func (c *cloud) Initialize(clientBuilder controller.ControllerClientBuilder) {
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
