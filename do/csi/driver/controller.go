package driver

import (
	"context"
	"errors"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
)

// CreateVolume creates a new volume
func (d *Driver) CreateVolume(context.Context, *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	return nil, errors.New("not implemented")
}

// DeleteVolume deletes the given volume. This doesn't mean it needs to be
// purged away. It needs to be deprovisioned, which means it should not be
// accesible by anymore.
func (d *Driver) DeleteVolume(context.Context, *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	return nil, errors.New("not implemented")
}

// ControllerPublishVolume ...
func (d *Driver) ControllerPublishVolume(context.Context, *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	return nil, errors.New("not implemented")
}

// ControllerUnpublishVolume ...
func (d *Driver) ControllerUnpublishVolume(context.Context, *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	return nil, errors.New("not implemented")
}

// ValidateVolumeCapabilities ...
func (d *Driver) ValidateVolumeCapabilities(context.Context, *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	return nil, errors.New("not implemented")
}

// ListVolumes ...
func (d *Driver) ListVolumes(context.Context, *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	return nil, errors.New("not implemented")
}

// GetCapacity ...
func (d *Driver) GetCapacity(context.Context, *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, errors.New("not implemented")
}

// ControllerGetCapabilities ...
func (d *Driver) ControllerGetCapabilities(context.Context, *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	return nil, errors.New("not implemented")
}
