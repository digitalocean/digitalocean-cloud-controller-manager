package do

import (
	"io/ioutil"
	"net/http"
)

const dropletIDMetadataURL = "http://169.254.169.254/metadata/v1/id"
const dropletRegionMetadataURL = "http://169.254.169.254/metadata/v1/region"

// dropletID returns the currently running droplet id
// using the metadata service available on all running droplets
func dropletID() (string, error) {
	return httpGet(dropletIDMetadataURL)
}

// dropletRegion returns the currently the region of the currently
// running program
func dropletRegion() (string, error) {
	return httpGet(dropletRegionMetadataURL)
}

// httpGet is a convienance function to do an http GET on a provided url
// and return the string version of the response body.
// In this package it is used for retrieving droplet metadata
//     e.g. http://169.254.169.254/metadata/v1/id"
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
