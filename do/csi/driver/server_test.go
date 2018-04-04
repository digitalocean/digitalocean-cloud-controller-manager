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

	socket := "/tmp/csi.sock"
	endpoint := "unix://" + socket
	if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
		t.Fatalf("failed to remove unix domain socket file %s, error: %s", socket, err)
	}

	// run the driver
	go NewDriver(endpoint).Run()

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
