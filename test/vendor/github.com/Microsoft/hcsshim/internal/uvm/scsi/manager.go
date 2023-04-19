//go:build windows

package scsi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/Microsoft/hcsshim/internal/protocol/guestresource"
	"github.com/Microsoft/hcsshim/internal/security"
	"github.com/Microsoft/hcsshim/internal/verity"
	"github.com/Microsoft/hcsshim/internal/wclayer"
	"github.com/sirupsen/logrus"
)

var (
	ErrNoAvailableLocation = errors.New("no available location")
	ErrNotInitialized      = errors.New("SCSI manager not initialized")
	ErrAlreadyReleased     = errors.New("mount was already released")
)

type Manager struct {
	attachManager *attachManager
	mountManager  *mountManager
}

type Slot struct {
	Controller uint
	LUN        uint
}

func NewManager(
	attacher Attacher,
	mounter Mounter,
	unplugger Unplugger,
	numControllers int,
	numLUNsPerController int,
	guestMountFmt string,
	reservedSlots []Slot,
) *Manager {
	am := newAttachManager(attacher, unplugger, numControllers, numLUNsPerController, reservedSlots)
	mm := newMountManager(mounter, guestMountFmt)
	return &Manager{am, mm}
}

type MountConfig struct {
	Encrypted bool
	Options   []string
}

type Mount struct {
	mgr         *Manager
	controller  uint
	lun         uint
	guestPath   string
	releaseOnce sync.Once
}

func (m *Mount) Controller() uint {
	return m.controller
}

func (m *Mount) LUN() uint {
	return m.lun
}

func (m *Mount) GuestPath() string {
	return m.guestPath
}

func (m *Mount) Release(ctx context.Context) (err error) {
	err = ErrAlreadyReleased
	m.releaseOnce.Do(func() {
		err = m.mgr.remove(ctx, m.controller, m.lun, m.guestPath)
	})
	return
}

func (m *Manager) AddVirtualDisk(
	ctx context.Context,
	hostPath string,
	readOnly bool,
	vmAccess VMAccessGranter,
	mc *MountConfig,
) (*Mount, error) {
	if m == nil {
		return nil, ErrNotInitialized
	}
	if vmAccess != nil {
		if err := vmAccess(ctx, hostPath); err != nil {
			return nil, err
		}
	}
	var mcInternal *mountConfig
	if mc != nil {
		mcInternal = &mountConfig{
			readOnly:  readOnly,
			encrypted: mc.Encrypted,
			options:   mc.Options,
			verity:    readVerityInfo(ctx, hostPath),
		}
	}
	return m.add(ctx,
		&attachConfig{
			path:     hostPath,
			readOnly: readOnly,
			typ:      "VirtualDisk",
		},
		mcInternal)
}

func (m *Manager) AddPhysicalDisk(
	ctx context.Context,
	hostPath string,
	readOnly bool,
	vmAccess VMAccessGranter,
	mc *MountConfig,
) (*Mount, error) {
	if m == nil {
		return nil, ErrNotInitialized
	}
	if vmAccess != nil {
		if err := vmAccess(ctx, hostPath); err != nil {
			return nil, err
		}
	}
	var mcInternal *mountConfig
	if mc != nil {
		mcInternal = &mountConfig{
			readOnly:  readOnly,
			encrypted: mc.Encrypted,
			options:   mc.Options,
			verity:    readVerityInfo(ctx, hostPath),
		}
	}
	return m.add(ctx,
		&attachConfig{
			path:     hostPath,
			readOnly: readOnly,
			typ:      "PassThru",
		},
		mcInternal)
}

func (m *Manager) AddExtensibleVirtualDisk(
	ctx context.Context,
	hostPath string,
	readOnly bool,
	mc *MountConfig,
) (*Mount, error) {
	if m == nil {
		return nil, ErrNotInitialized
	}
	evdType, mountPath, err := parseExtensibleVirtualDiskPath(hostPath)
	if err != nil {
		return nil, err
	}
	var mcInternal *mountConfig
	if mc != nil {
		mcInternal = &mountConfig{
			readOnly:  readOnly,
			encrypted: mc.Encrypted,
			options:   mc.Options,
		}
	}
	return m.add(ctx,
		&attachConfig{
			path:     mountPath,
			readOnly: readOnly,
			typ:      "ExtensibleVirtualDisk",
			evdType:  evdType,
		},
		mcInternal)
}

func (m *Manager) add(ctx context.Context, attachConfig *attachConfig, mountConfig *mountConfig) (_ *Mount, err error) {
	controller, lun, err := m.attachManager.attach(ctx, attachConfig)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			m.attachManager.detach(ctx, controller, lun)
		}
	}()

	var guestPath string
	if mountConfig != nil {
		guestPath, err = m.mountManager.mount(ctx, controller, lun, mountConfig)
		if err != nil {
			return nil, err
		}
	}

	return &Mount{mgr: m, controller: controller, lun: lun, guestPath: guestPath}, nil
}

func (m *Manager) remove(ctx context.Context, controller, lun uint, guestPath string) error {
	if guestPath != "" {
		removed, err := m.mountManager.unmount(ctx, guestPath)
		if err != nil {
			return err
		}

		if !removed {
			return nil
		}
	}

	if _, err := m.attachManager.detach(ctx, controller, lun); err != nil {
		return err
	}

	return nil
}

func readVerityInfo(ctx context.Context, path string) *guestresource.DeviceVerityInfo {
	if v, iErr := verity.ReadVeritySuperBlock(ctx, path); iErr != nil {
		log.G(ctx).WithError(iErr).WithField("hostPath", path).Debug("unable to read dm-verity information from VHD")
	} else {
		if v != nil {
			log.G(ctx).WithFields(logrus.Fields{
				"hostPath":   path,
				"rootDigest": v.RootDigest,
			}).Debug("adding SCSI with dm-verity")
		}
		return v
	}
	return nil
}

type VMAccessGranter func(ctx context.Context, path string) error

// IndividualVMAccess gives the specified VM access to the path.
func IndividualVMAccess(vmID string) VMAccessGranter {
	return func(ctx context.Context, path string) error {
		return wclayer.GrantVmAccess(ctx, vmID, path)
	}
}

// GroupVMAccess gives the VM group SID access to the path.
func GroupVMAccess() VMAccessGranter {
	return func(ctx context.Context, path string) error {
		log.G(ctx).WithField("path", path).Debug("granting vm group access")
		return security.GrantVmGroupAccess(path)
	}
}

// parseExtensibleVirtualDiskPath parses the evd path provided in the config.
// extensible virtual disk path has format "evd://<evdType>/<evd-mount-path>"
// this function parses that and returns the `evdType` and `evd-mount-path`.
func parseExtensibleVirtualDiskPath(hostPath string) (evdType, mountPath string, err error) {
	trimmedPath := strings.TrimPrefix(hostPath, "evd://")
	separatorIndex := strings.Index(trimmedPath, "/")
	if separatorIndex <= 0 {
		return "", "", fmt.Errorf("invalid extensible vhd path: %s", hostPath)
	}
	return trimmedPath[:separatorIndex], trimmedPath[separatorIndex+1:], nil
}
