package main

import (
	"flag"
	"log"

	"github.com/digitalocean/digitalocean-cloud-controller-manager/do/csi/driver"
)

func main() {
	var (
		endpoint = flag.String("endpoint", "unix:///var/lib/kubelet/plugins/com.digitalocean.csi.dobs/csi.sock", "CSI endpoint")
	)

	flag.Parse()

	drv := driver.NewDriver(*endpoint)
	if err := drv.Run(); err != nil {
		log.Fatalln(err)
	}
}
