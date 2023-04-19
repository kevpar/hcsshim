package scsi

import (
	"context"
	"fmt"
	"sync"

	"github.com/Microsoft/hcsshim/internal/protocol/guestresource"
)

type mountManager struct {
	m        sync.Mutex
	mounter  Mounter
	mounts   []*mount
	mountFmt string
	mountNum uint
}

func newMountManager(mounter Mounter, mountFmt string) *mountManager {
	return &mountManager{
		mounter:  mounter,
		mountFmt: mountFmt,
	}
}

type mount struct {
	path       string
	index      int
	controller uint
	lun        uint
	config     *mountConfig
	waitErr    error
	waitCh     chan struct{}
	refCount   uint
}

type mountConfig struct {
	readOnly  bool
	encrypted bool
	verity    *guestresource.DeviceVerityInfo
	options   []string
}

func mountConfigEquals(a, b *mountConfig) bool {
	return a.readOnly == b.readOnly &&
		a.encrypted == b.encrypted &&
		*a.verity == *b.verity &&
		strSliceEquals(a.options, b.options)
}

func strSliceEquals(a, b []string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (mm *mountManager) mount(ctx context.Context, controller, lun uint, c *mountConfig) (_ string, err error) {
	mount, existed := mm.trackMount(controller, lun, c)
	if existed {
		<-mount.waitCh
		if mount.waitErr != nil {
			return "", mount.waitErr
		}
		return mount.path, nil
	}

	defer func() {
		if err != nil {
			mm.untrackMount(mount, true)
		}

		mount.waitErr = err
		close(mount.waitCh)
	}()

	if err := mm.mounter.mount(ctx, controller, lun, mount.path, c); err != nil {
		return "", fmt.Errorf("mount scsi controller %d lun %d at %s: %w", controller, lun, mount.path, err)
	}
	return mount.path, nil
}

func (mm *mountManager) unmount(ctx context.Context, path string) (bool, error) {
	mm.m.Lock()
	defer mm.m.Unlock()

	var mount *mount
	for _, mount = range mm.mounts {
		if mount.path == path {
			break
		}
	}

	mount.refCount--
	if mount.refCount > 0 {
		return false, nil
	}

	if err := mm.mounter.unmount(ctx, mount.controller, mount.lun, mount.path, mount.config); err != nil {
		return false, fmt.Errorf("unmount scsi controller %d lun %d at path %s: %w", mount.controller, mount.lun, mount.path, err)
	}
	mm.untrackMount(mount, false)

	return true, nil
}

func (mm *mountManager) trackMount(controller, lun uint, c *mountConfig) (*mount, bool) {
	mm.m.Lock()
	defer mm.m.Unlock()

	var freeIndex int = -1
	for i, mount := range mm.mounts {
		if mount == nil && freeIndex == -1 {
			freeIndex = i
		}
		if controller == mount.controller &&
			lun == mount.lun &&
			mountConfigEquals(c, mount.config) {

			mount.refCount++
			return mount, true
		}
	}

	// New mount.
	mount := &mount{
		path:       mm.mountPath(),
		controller: controller,
		lun:        lun,
		config:     c,
		refCount:   1,
		waitCh:     make(chan struct{}),
	}
	if freeIndex == -1 {
		mount.index = len(mm.mounts)
		mm.mounts = append(mm.mounts, mount)
	} else {
		mount.index = freeIndex
		mm.mounts[freeIndex] = mount
	}
	return mount, false
}

func (mm *mountManager) untrackMount(mount *mount, takeLock bool) {
	if takeLock {
		mm.m.Lock()
		defer mm.m.Unlock()
	}

	mm.mounts[mount.index] = nil
}

func (mm *mountManager) mountPath() string {
	mountNum := mm.mountNum
	mm.mountNum++
	return fmt.Sprintf(mm.mountFmt, mountNum)
}
