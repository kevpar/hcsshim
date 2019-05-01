package cimfs

import (
	"errors"
	"unsafe"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/go-winio/pkg/guid"
	"golang.org/x/sys/windows"
)

type imageHandle uintptr
type streamHandle uintptr

type g = guid.GUID

type eaInternal struct {
	Name       string
	NameLength uint32

	Flags uint8

	Buffer     unsafe.Pointer
	BufferSize uint32
}

type FileInfo struct {
	Size int64

	CreationTime   windows.Filetime
	LastWriteTime  windows.Filetime
	ChangeTime     windows.Filetime
	LastAccessTime windows.Filetime

	Attributes uint32

	SecurityDescriptor []byte
	ReparseData        []byte
	EAs                []winio.ExtendedAttribute
}

type fileInfoInternal struct {
	Size     uint32
	FileSize int64

	CreationTime   windows.Filetime
	LastWriteTime  windows.Filetime
	ChangeTime     windows.Filetime
	LastAccessTime windows.Filetime

	Attributes uint32

	SecurityDescriptorBuffer unsafe.Pointer
	SecurityDescriptorSize   uint32

	ReparseDataBuffer unsafe.Pointer
	ReparseDataSize   uint32

	EAs     unsafe.Pointer
	EACount uint32
}

//go:generate go run ../../mksyscall_windows.go -output zsyscall_windows.go syscall.go

// CimAddStream
// CimAddLink
// CimRemoveFile

//sys cimMountImage(cimPath string, volumeID *g) (hr error) = cimfs.CimMountImage
//sys cimUnmountImage(volumeID *g) (hr error) = cimfs.CimUnmountImage

//sys cimInitializeImage(cimPath string, flags uint32, cimFSHandle *imageHandle) (hr error) = cimfs.CimInitializeImage
//sys cimFinalizeImage(cimFSHandle imageHandle, cimPath string) (hr error) = cimfs.CimFinalizeImage

//sys cimAddFile(cimFSHandle imageHandle, path string, file *fileInfoInternal, flags uint32, cimStreamHandle *streamHandle) (hr error) = cimfs.CimAddFile
//sys cimFinalizeStream(cimStreamHandle streamHandle) (hr error) = cimfs.CimFinalizeStream
//sys cimWriteStream(cimStreamHandle streamHandle, buffer uintptr, bufferSize uint64) (hr error) = cimfs.CimWriteStream
//sys cimRemoveFile(cimFSHandle imageHandle, path string) (hr error) = cimfs.CimRemoveFile

type Image struct {
	handle       imageHandle
	activeStream streamHandle
}

func Open(path string) (*Image, error) {
	var handle imageHandle
	if err := cimInitializeImage(path, 0, &handle); err != nil {
		return nil, err
	}

	return &Image{handle: handle}, nil
}

func (cim *Image) AddFile(path string, info *FileInfo) error {
	infoInternal := &fileInfoInternal{
		FileSize:       info.Size,
		CreationTime:   info.CreationTime,
		LastWriteTime:  info.LastWriteTime,
		ChangeTime:     info.ChangeTime,
		LastAccessTime: info.LastAccessTime,
		Attributes:     info.Attributes,
	}

	if len(info.SecurityDescriptor) > 0 {
		infoInternal.SecurityDescriptorBuffer = unsafe.Pointer(&info.SecurityDescriptor[0])
		infoInternal.SecurityDescriptorSize = uint32(len(info.SecurityDescriptor))
	}

	if len(info.ReparseData) > 0 {
		infoInternal.ReparseDataBuffer = unsafe.Pointer(&info.ReparseData[0])
		infoInternal.ReparseDataSize = uint32(len(info.ReparseData))
	}

	easInternal := []eaInternal{}
	for _, ea := range info.EAs {
		eaInternal := eaInternal{
			Name:       ea.Name,
			NameLength: uint32(len(ea.Name)),
			Flags:      ea.Flags,
		}

		if len(ea.Value) > 0 {
			eaInternal.Buffer = unsafe.Pointer(&ea.Value[0])
			eaInternal.BufferSize = uint32(len(ea.Value))
		}

		easInternal = append(easInternal, eaInternal)
	}

	return cimAddFile(cim.handle, path, infoInternal, 0, &cim.activeStream)
}

func (cim *Image) Write(p []byte) (int, error) {
	if cim.activeStream == 0 {
		return 0, errors.New("No active stream")
	}

	// TODO: pass p directly to gen'd syscall
	err := cimWriteStream(cim.activeStream, uintptr(unsafe.Pointer(&p[0])), uint64(len(p)))
	if err != nil {
		return 0, err
	}

	return len(p), nil
}

func (cim *Image) CloseStream() error {
	return cimFinalizeStream(cim.activeStream)
}

func (cim *Image) Close(path string) error {
	return cimFinalizeImage(cim.handle, path)
}

func (cim *Image) RemoveFile(path string) error {
	return cimRemoveFile(cim.handle, path)
}

func MountImage(path string) (*guid.GUID, error) {
	g, err := guid.NewV4()
	if err != nil {
		return nil, err
	}
	return g, cimMountImage(path, g)
}

func UnmountImage(g *guid.GUID) error {
	return cimUnmountImage(g)
}
