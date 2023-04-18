package scsi

import (
	"context"
	"fmt"

	"github.com/Microsoft/hcsshim/internal/gcs"
	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/hcs/resourcepaths"
	hcsschema "github.com/Microsoft/hcsshim/internal/hcs/schema2"
	"github.com/Microsoft/hcsshim/internal/protocol/guestrequest"
	"github.com/Microsoft/hcsshim/internal/protocol/guestresource"
)

// The concrete types here (not the Attacher/Mounter/Unplugger interfaces) would be a good option
// to move out to another package eventually. There is no real reason for them to live in
// the scsi package, and it could cause cyclical dependencies in the future.

type Attacher interface {
	attach(ctx context.Context, controller, lun uint, config *attachConfig) error
	detach(ctx context.Context, controller, lun uint) error
}

type Mounter interface {
	mount(ctx context.Context, controller, lun uint, path string, config *mountConfig) error
	unmount(ctx context.Context, controller, lun uint, path string, config *mountConfig) error
}

type Unplugger interface {
	unplug(ctx context.Context, controller, lun uint) error
}

var _ Attacher = &hcsAttacher{}

type hcsAttacher struct {
	system *hcs.System
}

func NewHCSAttacher(system *hcs.System) Attacher {
	return &hcsAttacher{system}
}

func (ha *hcsAttacher) attach(ctx context.Context, controller, lun uint, config *attachConfig) error {
	req := &hcsschema.ModifySettingRequest{
		RequestType: guestrequest.RequestTypeAdd,
		Settings: hcsschema.Attachment{
			Path:                      config.path,
			Type_:                     config.typ,
			ReadOnly:                  config.readOnly,
			ExtensibleVirtualDiskType: config.evdType,
		},
		ResourcePath: fmt.Sprintf(resourcepaths.SCSIResourceFormat, guestrequest.ScsiControllerGuids[controller], lun),
	}
	return ha.system.Modify(ctx, req)
}

func (ha *hcsAttacher) detach(ctx context.Context, controller, lun uint) error {
	req := &hcsschema.ModifySettingRequest{
		RequestType:  guestrequest.RequestTypeRemove,
		ResourcePath: fmt.Sprintf(resourcepaths.SCSIResourceFormat, guestrequest.ScsiControllerGuids[controller], lun),
	}
	return ha.system.Modify(ctx, req)
}

var _ Mounter = &bridgeMounter{}

type bridgeMounter struct {
	gc     *gcs.GuestConnection
	osType string
}

func NewBridgeMounter(gc *gcs.GuestConnection, osType string) Mounter {
	return &bridgeMounter{gc, osType}
}

func (bm *bridgeMounter) mount(ctx context.Context, controller, lun uint, path string, config *mountConfig) error {
	req := guestrequest.ModificationRequest{
		ResourceType: guestresource.ResourceTypeMappedVirtualDisk,
		RequestType:  guestrequest.RequestTypeAdd,
	}
	switch bm.osType {
	case "windows":
		req.Settings = guestresource.WCOWMappedVirtualDisk{
			ContainerPath: path,
			Lun:           int32(lun),
		}
	case "linux":
		req.Settings = guestresource.LCOWMappedVirtualDisk{
			MountPath:  path,
			Controller: uint8(controller),
			Lun:        uint8(lun),
			Partition:  uint32(config.partition),
			ReadOnly:   config.readOnly,
			Encrypted:  config.encrypted,
			Options:    config.options,
			VerityInfo: config.verity,
		}
	default:
		return fmt.Errorf("unsupported os type: %s", bm.osType)
	}
	return bm.gc.Modify(ctx, req)
}

func (bm *bridgeMounter) unmount(ctx context.Context, controller, lun uint, path string, config *mountConfig) error {
	var req guestrequest.ModificationRequest
	switch bm.osType {
	case "windows":
		req = guestrequest.ModificationRequest{
			ResourceType: guestresource.ResourceTypeMappedVirtualDisk,
			RequestType:  guestrequest.RequestTypeRemove,
			Settings: guestresource.WCOWMappedVirtualDisk{
				ContainerPath: path,
				Lun:           int32(lun),
			},
		}
	case "linux":
		req = guestrequest.ModificationRequest{
			ResourceType: guestresource.ResourceTypeMappedVirtualDisk,
			RequestType:  guestrequest.RequestTypeRemove,
			Settings: guestresource.LCOWMappedVirtualDisk{
				MountPath:  path,
				Lun:        uint8(lun),
				Controller: uint8(controller),
				VerityInfo: config.verity,
			},
		}
	default:
		return fmt.Errorf("unsupported os type: %s", bm.osType)
	}
	return bm.gc.Modify(ctx, req)
}

var _ Unplugger = &bridgeUnplugger{}

type bridgeUnplugger struct {
	gc     *gcs.GuestConnection
	osType string
}

func NewBridgeUnplugger(gc *gcs.GuestConnection, osType string) Unplugger {
	return &bridgeUnplugger{gc, osType}
}

func (bu *bridgeUnplugger) unplug(ctx context.Context, controller, lun uint) error {
	var req guestrequest.ModificationRequest
	switch bu.osType {
	case "windows":
		// Windows doesn't support an unplug operation, so treat as no-op.
	case "linux":
		req = guestrequest.ModificationRequest{
			ResourceType: guestresource.ResourceTypeSCSIDevice,
			RequestType:  guestrequest.RequestTypeRemove,
			Settings: guestresource.SCSIDevice{
				Controller: uint8(controller),
				Lun:        uint8(lun),
			},
		}
	default:
		return fmt.Errorf("unsupported os type: %s", bu.osType)
	}
	return bu.gc.Modify(ctx, req)
}
