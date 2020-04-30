package vm

import (
	"context"
	"net"

	"github.com/Microsoft/go-winio/pkg/guid"
)

type UVMSource interface {
	NewLinuxUVM(id string, owner string) (UVM, error)
}

type UVM interface {
	ID() string
	State() State
	Create(ctx context.Context) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Wait() error
}

type State uint8

const (
	StatePreCreated State = iota
	StateCreated
	StateRunning
	StateTerminated
)

type MemoryControl interface {
	SetMemoryLimit(ctx context.Context, memoryMB uint64) error
}

type ProcessorControl interface {
	SetProcessorCount(ctx context.Context, count uint64) error
}

type SCSI interface {
	AddSCSIController(ctx context.Context, id uint32) error
	AddSCSIDisk(ctx context.Context, controller uint32, lun uint32, path string, typ SCSIDiskType, readOnly bool) error
}

type SCSIDiskType uint8

const (
	SCSIDiskTypeVirtualDisk SCSIDiskType = iota
	SCSIDiskTypePassThrough
)

type VPMem interface {
	AddVPMemController(ctx context.Context, maximumDevices uint32, maximumSizeBytes uint64) error
	AddVPMemDevice(ctx context.Context, id uint32, path string, readOnly bool, imageFormat VPMemImageFormat) error
}

type VPMemImageFormat uint8

const (
	VPMemImageFormatVHD1 VPMemImageFormat = iota
	VPMemImageFormatVHDX
)

type LinuxBootConfig interface {
	SetLinuxUEFIBoot(ctx context.Context, dir string, kernel string, cmd string) error
	SetLinuxKernelDirectBoot(ctx context.Context, kernel string, initRD string, cmd string) error
}

type HVSocketListen interface {
	HVSocketListen(ctx context.Context, serviceID guid.GUID) (net.Listener, error)
}
