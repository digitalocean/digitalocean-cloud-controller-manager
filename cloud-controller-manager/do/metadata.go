/*
Copyright 2024 DigitalOcean

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
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
)

const (
	dropletRegionMetadataURL = "http://169.254.169.254/metadata/v1/region"
	resultsPerPage           = 50
)

// dropletRegion returns the region of the currently running program.
func dropletRegion(regionsService godo.RegionsService) (string, error) {
	region := os.Getenv(regionEnv)
	if region == "" {
		return httpGet(dropletRegionMetadataURL)
	}

	validRegion, err := isValidRegion(region, regionsService)
	if err != nil {
		return "", fmt.Errorf("failed to determine if region is valid: %s", err)
	}
	if !validRegion {
		return "", fmt.Errorf("invalid region specified: %s", region)
	}

	klog.Infof("Using region %q from environment variable", region)
	return region, nil
}

// isValidRegion checks whether the given region is a valid DO region
func isValidRegion(region string, regionsService godo.RegionsService) (bool, error) {
	regions, err := listAllRegions(regionsService)
	if err != nil {
		return false, err
	}

	for _, reg := range regions {
		if reg.Slug == region {
			return true, nil
		}
	}

	return false, nil
}

func listAllRegions(regionsService godo.RegionsService) ([]godo.Region, error) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var list []godo.Region

	listOptions := &godo.ListOptions{
		Page:    1,
		PerPage: resultsPerPage,
	}

	for {
		regions, resp, err := regionsService.List(ctx, listOptions)
		if err != nil {
			return nil, fmt.Errorf("regions list request failed: %s", err.Error())
		}

		if resp == nil {
			return nil, errors.New("regions list request returned no response")
		}

		list = append(list, regions...)

		// if we are at the last page, break out the for loop
		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			return nil, fmt.Errorf("failed to get current page number in pagination: %s", err)
		}

		listOptions.Page = page + 1
	}

	return list, nil
}

// httpGet is a convenience function to do an http GET on a provided url
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

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(bodyBytes), nil
}
