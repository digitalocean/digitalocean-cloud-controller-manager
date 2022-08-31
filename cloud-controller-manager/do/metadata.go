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
	"io/ioutil"
	"net/http"
	"os"

	"github.com/digitalocean/godo"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
)

const dropletRegionMetadataURL = "http://169.254.169.254/metadata/v1/region"

// dropletRegion returns the region of the currently running program.
func dropletRegion() (string, error) {
	// REGION can be specified for local testing.
	region := os.Getenv("REGION")
	if region == "" {
		return httpGet(dropletRegionMetadataURL)
	}

	if !validRegion(region) {
		return "", errors.New("invalid region specified")
	}

	klog.Warningf("Using region %q from environment variable", region)
	return region, nil
}

// checks whether the given region is a valid DO region
func validRegion(region string) bool {
	regionsService := godo.RegionsServiceOp{}
	regions, _, err := regionsService.List(context.Background(), &godo.ListOptions{})
	if err != nil {
		klog.Errorf("Failed to get a list of regions; %v", err)
	}

	for _, reg := range regions {
		if reg.Name == region {
			return true
		}
	}
	return false
}

// httpGet is a convienance function to do an http GET on a provided url
// and return the string version of the response body.
// In this package it is used for retrieving droplet metadata
//
//	e.g. http://169.254.169.254/metadata/v1/id"
func httpGet(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("droplet metadata returned non-200 status code: %d", resp.StatusCode)
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(bodyBytes), nil
}
