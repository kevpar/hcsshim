package guestpath

const (
	// WCOWRootPrefixInUVM is the path inside UVM where WCOW container's root
	// file system will be mounted
	WCOWRootPrefixInUVM = `C:\c`
	// SandboxMountPrefix is mount prefix used in container spec to mark a
	// sandbox-mount
	SandboxMountPrefix = "sandbox://"
	// HugePagesMountPrefix is mount prefix used in container spec to mark a
	// huge-pages mount
	HugePagesMountPrefix = "hugepages://"
	// BlockDevMountPrefix is mount prefix used in container spec to mark a
	// block-device mount.
	BlockDevMountPrefix = "blockdev://"
	// PipePrefix is the mount prefix used in container spec to mark a named pipe
	PipePrefix = `\\.\pipe`
	// LCOWGlobalDriverPrefixFmt is the path format in the LCOW UVM where drivers
	// are mounted as read/write
	LCOWGlobalDriverPrefixFmt = "/run/drivers/%s"
	// WCOWGlobalMountPrefixFmt is the path prefix format in the WCOW UVM where
	// mounts are added
	WCOWGlobalMountPrefixFmt = "C:\\mounts\\m%d"
)
