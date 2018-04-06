package driver

import (
	"flag"
	"io/ioutil"
	"os"
	"testing"

	"github.com/kubernetes-csi/csi-test/pkg/sanity"
)

var suite = flag.Bool("suite", false, "run the csi test suite")

func TestDriverSuite(t *testing.T) {
	if !*suite {
		t.Skip("skipping test suite. enable by adding the flag '-suite'")
	}

	token := os.Getenv("DIGITALOCEAN_ACCESS_TOKEN")
	if token == "" {
		t.Skip("skipping test suite. DIGITALOCEAN_ACCESS_TOKEN needs to be set")
	}

	socket := "/tmp/csi.sock"
	endpoint := "unix://" + socket
	if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
		t.Fatalf("failed to remove unix domain socket file %s, error: %s", socket, err)
	}

	// run the driver
	driver, err := NewDriver(endpoint, token)
	if err != nil {
		t.Fatal(err)
	}
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
