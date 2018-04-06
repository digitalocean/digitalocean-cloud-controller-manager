package driver

import (
	"context"
	"errors"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/digitalocean/godo"
)

const (
	_  = iota
	KB = 1 << (10 * iota)
	MB
	GB
	TB
)

const (
	defaultVolumeSizeInGB = 16 * GB

	defaultMinVolumeSizeInGB = 1 * GB
	defaultMaxVolumeSizeInGB = 16 * TB
)

// CreateVolume creates a new volume
func (d *Driver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	volumeName := req.Name

	// get volume first
	//  3. if created, do nothing
	//  2. if not created, create it
	volumes, _, err := d.doClient.Storage.ListVolumes(ctx, &godo.ListVolumeParams{
		Region: d.region,
		Name:   volumeName,
	})
	// TODO(arslan): return if error is *not* type "not found"
	if err == nil {
		// TODO(arslan): what happens if there are multiple volumes with the
		// same name, possible? Check it out
		vol := volumes[0]

		return &csi.CreateVolumeResponse{
			Volume: &csi.Volume{
				Id:            vol.ID,
				CapacityBytes: vol.SizeGigaBytes * GB,
			},
		}, nil
	}

	size, err := extractStorage(req.CapacityRange)
	if err != nil {
		return nil, err
	}

	volumeReq := &godo.VolumeCreateRequest{
		Region:        d.nodeId,
		Name:          volumeName,
		Description:   "Created by DigitalOcean CSI driver",
		SizeGigaBytes: size,
	}

	vol, _, err := d.doClient.Storage.CreateVolume(ctx, volumeReq)
	if err != nil {
		return nil, err
	}

	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			Id:            vol.ID,
			CapacityBytes: size * GB,
		},
	}, nil
}

// DeleteVolume deletes the given volume. The function is idempotent.
func (d *Driver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	_, err := d.doClient.Storage.DeleteVolume(ctx, req.VolumeId)
	if err != nil {
		return nil, err
	}

	return &csi.DeleteVolumeResponse{}, nil
}

// ControllerPublishVolume attaches the given volume to the node
// TODO(arslan): check volume capabilities as well
func (d *Driver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	dropletID, err := strconv.Atoi(req.NodeId)
	if err != nil {
		return nil, fmt.Errorf("malformed nodeId %q detected: %s", req.NodeId, err)
	}

	// TODO(arslan): wait volume to attach
	_, resp, err := d.doClient.StorageActions.Attach(ctx, req.VolumeId, dropletID)
	if err != nil {
		// don't do anything if attached
		if resp.StatusCode == http.StatusUnprocessableEntity || strings.Contains(err.Error(), "This volume is already attached") {
			return &csi.ControllerPublishVolumeResponse{}, nil
		}

		return nil, err
	}

	return &csi.ControllerPublishVolumeResponse{}, nil
}

// ControllerUnpublishVolume deattaches the given volume from the node
// TODO(arslan): check volume capabilities as well
func (d *Driver) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	dropletID, err := strconv.Atoi(req.NodeId)
	if err != nil {
		return nil, fmt.Errorf("malformed nodeId %q detected: %s", req.NodeId, err)
	}

	// TODO(arslan): wait volume to deattach
	_, resp, err := d.doClient.StorageActions.DetachByDropletID(ctx, req.NodeId, dropletID)
	if err != nil {
		if resp.StatusCode == http.StatusUnprocessableEntity || strings.Contains(err.Error(), "Attachment not found") {
			return &csi.ControllerUnpublishVolumeResponse{}, nil
		}
		return nil, err
	}
	return &csi.ControllerUnpublishVolumeResponse{}, nil
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

// ControllerGetCapabilities returns the capabilities of the controller service.
func (d *Driver) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	newCap := func(cap csi.ControllerServiceCapability_RPC_Type) *csi.ControllerServiceCapability {
		return &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		}
	}

	// TODO(arslan): checkout if the capabilities are worth supporting
	var caps []*csi.ControllerServiceCapability
	for _, cap := range []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
		csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
		csi.ControllerServiceCapability_RPC_GET_CAPACITY,
	} {
		caps = append(caps, newCap(cap))
	}

	return &csi.ControllerGetCapabilitiesResponse{
		Capabilities: caps,
	}, nil
}

// extractStorage extracts the storage size in GB from the given capacity
// range. If the capacity range is not satisfied it returns the default volume
// size.
func extractStorage(capRange *csi.CapacityRange) (int64, error) {
	if capRange == nil {
		return defaultVolumeSizeInGB, nil
	}

	minSize := (capRange.RequiredBytes) / GB
	maxSize := (capRange.LimitBytes) / GB

	if minSize == maxSize {
		return minSize, nil
	}

	return 0, errors.New("requiredBytes and LimitBytes are not the same")
}
