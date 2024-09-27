//go:build windows

package scsi

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

type mountManager struct {
	mounter       mounter
	mounts        map[string]*mount // Guest path -> mount struct. Tracks current mounts.
	m             sync.Mutex        // Guards access to the mounts map.
	guestMountFmt string
	mountIndex    atomic.Uint64
}

func newMountManager(mounter mounter, guestMountFmt string) *mountManager {
	return &mountManager{
		mounter:       mounter,
		mounts:        make(map[string]*mount),
		guestMountFmt: guestMountFmt,
	}
}

type mount struct {
	path       string
	controller uint
	lun        uint
	config     *mountConfig
}

type mountConfig struct {
	partition        uint64
	readOnly         bool
	encrypted        bool
	blockDev         bool
	options          []string
	ensureFilesystem bool
	filesystem       string
}

func (mm *mountManager) mount(ctx context.Context, controller, lun uint, path string, c *mountConfig) (_ string, err error) {
	if path == "" {
		path = fmt.Sprintf(mm.guestMountFmt, mm.mountIndex.Add(1))
	}
	mount := &mount{
		path:       path,
		controller: controller,
		lun:        lun,
		config:     c,
	}
	if err := func() error {
		mm.m.Lock()
		defer mm.m.Unlock()

		if _, ok := mm.mounts[path]; ok {
			return fmt.Errorf("another mount is already present at %s", path)
		}

		mm.mounts[path] = mount
		return nil
	}(); err != nil {
		return "", err
	}

	defer func() {
		if err != nil {
			mm.m.Lock()
			delete(mm.mounts, path)
			mm.m.Unlock()
		}
	}()

	if err := mm.mounter.mount(ctx, controller, lun, mount.path, c); err != nil {
		return "", fmt.Errorf("mount scsi controller %d lun %d at %s: %w", controller, lun, mount.path, err)
	}
	return path, nil
}

func (mm *mountManager) unmount(ctx context.Context, path string) error {
	mm.m.Lock()
	defer mm.m.Unlock()

	mount := mm.mounts[path]
	if err := mm.mounter.unmount(ctx, mount.controller, mount.lun, mount.path, mount.config); err != nil {
		return fmt.Errorf("unmount scsi controller %d lun %d at path %s: %w", mount.controller, mount.lun, mount.path, err)
	}

	delete(mm.mounts, path)

	return nil
}
