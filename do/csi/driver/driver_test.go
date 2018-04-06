package driver

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/digitalocean/godo"
	"github.com/kubernetes-csi/csi-test/pkg/sanity"
)

var suite = flag.Bool("suite", false, "run the csi test suite")

func TestDriverSuite(t *testing.T) {
	if !*suite {
		t.Skip("skipping test suite. enable by adding the flag '-suite'")
	}

	socket := "/tmp/csi.sock"
	endpoint := "unix://" + socket
	if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
		t.Fatalf("failed to remove unix domain socket file %s, error: %s", socket, err)
	}

	// fake DO Server, not working yet ...
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var resp = struct {
			Volume *godo.Volume
			Links  *godo.Links
		}{
			Volume: &godo.Volume{
				Region: &godo.Region{
					Slug: "tor1",
				},
				ID:            "123456",
				Name:          "my-app",
				Description:   "Created by VolumeController",
				SizeGigaBytes: 15,
			},
		}
		_ = json.NewEncoder(w).Encode(&resp)
	}))
	defer ts.Close()

	doClient := godo.NewClient(nil)
	url, _ := url.Parse(ts.URL)
	doClient.BaseURL = url

	driver := &Driver{
		endpoint: endpoint,
		nodeId:   "987654",
		region:   "nyc3",
		doClient: doClient,
	}
	defer driver.Stop()

	go driver.Run()

	mntDir, err := ioutil.TempDir("", "mnt")
	if err != nil {
		t.Fatal(err)
	}

	cfg := &sanity.Config{
		TargetPath: mntDir,
		Address:    endpoint,
	}

	sanity.Test(t, cfg)
}
