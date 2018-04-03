package driver

import (
	"context"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
)

// GetPluginInfo returns metadata of the plugin
func (d *Driver) GetPluginInfo(context.Context, *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	panic("not implemented")
}

// GetPluginCapabilities returns available capabilities of the plugin
func (d *Driver) GetPluginCapabilities(context.Context, *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	panic("not implemented")
}

// Probe returns the health and readiness of the plugin
func (d *Driver) Probe(context.Context, *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	panic("not implemented")
}
