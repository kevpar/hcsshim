package uvm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Microsoft/hcsshim/internal/cow"
	"github.com/Microsoft/hcsshim/internal/gcs"
	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/hcs/schema1"
	hcsschema "github.com/Microsoft/hcsshim/internal/hcs/schema2"
	"github.com/Microsoft/hcsshim/internal/protocol/guestrequest"
	statepkg "github.com/Microsoft/hcsshim/internal/state"
	"github.com/Microsoft/hcsshim/internal/uvm/scsi"
	"github.com/Microsoft/hcsshim/internal/wclayer"
	"github.com/sirupsen/logrus"
)

func ensure_VirtualMachine(config *hcsschema.ComputeSystem) {
	if config.VirtualMachine == nil {
		config.VirtualMachine = &hcsschema.VirtualMachine{}
	}
}

func ensure_VirtualMachine_Devices(config *hcsschema.ComputeSystem) {
	ensure_VirtualMachine(config)
	if config.VirtualMachine.Devices == nil {
		config.VirtualMachine.Devices = &hcsschema.Devices{}
	}
}

func ensure_VirtualMachine_Devices_NetworkAdapters(config *hcsschema.ComputeSystem) {
	ensure_VirtualMachine_Devices(config)
	if config.VirtualMachine.Devices.NetworkAdapters == nil {
		config.VirtualMachine.Devices.NetworkAdapters = map[string]hcsschema.NetworkAdapter{}
	}
}

func ensure_VirtualMachine_Devices_Scsi(config *hcsschema.ComputeSystem) {
	ensure_VirtualMachine_Devices(config)
	if config.VirtualMachine.Devices.Scsi == nil {
		config.VirtualMachine.Devices.Scsi = map[string]hcsschema.Scsi{}
	}
}

func ensure_VirtualMachine_Devices_Scsi_K(config *hcsschema.ComputeSystem, k1 string) {
	ensure_VirtualMachine_Devices_Scsi(config)
	if _, ok := config.VirtualMachine.Devices.Scsi[k1]; !ok {
		config.VirtualMachine.Devices.Scsi[k1] = hcsschema.Scsi{Attachments: map[string]hcsschema.Attachment{}}
	}
}

func updateConfig(config *hcsschema.ComputeSystem, changeAny any) error {
	change, ok := changeAny.(*hcsschema.ModifySettingRequest)
	if !ok {
		return fmt.Errorf("bad type for change request: %T", changeAny)
	}
	switch change.RequestType {
	case guestrequest.RequestTypeAdd:
		var matched bool
		for _, s := range []struct {
			regex string
			f     func(*hcsschema.ComputeSystem, []string, any) error
		}{
			{
				regex: `^VirtualMachine/Devices/NetworkAdapters/([^/]+)$`,
				f: func(config *hcsschema.ComputeSystem, m []string, settings any) error {
					ensure_VirtualMachine_Devices_NetworkAdapters(config)
					_, ok := config.VirtualMachine.Devices.NetworkAdapters[m[1]]
					if ok {
						return fmt.Errorf("collision")
					}
					v, ok := settings.(hcsschema.NetworkAdapter)
					if !ok {
						return fmt.Errorf("bad type: %T", settings)
					}
					config.VirtualMachine.Devices.NetworkAdapters[m[1]] = v
					return nil
				},
			},
			{
				regex: `^VirtualMachine/Devices/Scsi/([^/]+)/Attachments/([^/]+)$`,
				f: func(config *hcsschema.ComputeSystem, m []string, settings any) error {
					ensure_VirtualMachine_Devices_Scsi_K(config, m[1])
					_, ok := config.VirtualMachine.Devices.Scsi[m[1]].Attachments[m[2]]
					if ok {
						return fmt.Errorf("collision")
					}
					v, ok := settings.(hcsschema.Attachment)
					if !ok {
						return fmt.Errorf("bad type: %T", settings)
					}
					config.VirtualMachine.Devices.Scsi[m[1]].Attachments[m[2]] = v
					return nil
				},
			},
		} {
			if m := regexp.MustCompile(s.regex).FindStringSubmatch(change.ResourcePath); m != nil {
				matched = true
				if err := s.f(config, m, change.Settings); err != nil {
					return fmt.Errorf("update error for %s: %w", change.ResourcePath, err)
				}
				break
			}
		}
		if !matched {
			return fmt.Errorf("unrecognized update path: %s with payload type %T", change.ResourcePath, change.Settings)
		}
		j, err := json.Marshal(config)
		if err != nil {
			return err
		}
		j2, err := json.Marshal(change)
		if err != nil {
			return err
		}
		logrus.WithFields(logrus.Fields{
			"config": string(j),
			"change": string(j2),
		}).Info("UPDATED CONFIG")
	case guestrequest.RequestTypeRemove:
		if strings.HasPrefix(change.ResourcePath, "VirtualMachine/Devices/NetworkAdapters/") {
			return nil
		}
		return fmt.Errorf("unrecognized update path: %s with payload type %T", change.ResourcePath, change.Settings)
	default:
		return fmt.Errorf("unrecognized request type: %s", change.RequestType)
	}
	return nil
}

type uvmState struct {
	ProcessorCount          int32
	PhysicallyBacked        bool
	DevicesPhysicallyBacked bool
	Protocol                uint32
	GuestCaps               schema1.GuestDefinedCapabilities
	ContainerCounter        uint64
	SCSIControllerCount     uint32
	ReservedSCSISlots       []scsi.Slot
	MountCounter            uint64
	FirstPort               uint32
}

func (uvm *UtilityVM) StartSave(ctx context.Context, path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}
	for ns := range uvm.namespaces {
		if err := uvm.RemoveNetNS(ctx, ns); err != nil {
			return fmt.Errorf("remove netns %s: %w", ns, err)
		}
	}
	if err := uvm.hcsSystem.Pause(ctx); err != nil {
		return err
	}
	if err := statepkg.Write(filepath.Join(path, "config.json"), uvm.config); err != nil {
		return err
	}
	state := uvmState{
		ProcessorCount:          uvm.processorCount,
		PhysicallyBacked:        uvm.physicallyBacked,
		DevicesPhysicallyBacked: uvm.devicesPhysicallyBacked,
		Protocol:                uvm.protocol,
		GuestCaps:               uvm.guestCaps,
		ContainerCounter:        uvm.containerCounter,
		SCSIControllerCount:     uvm.scsiControllerCount,
		ReservedSCSISlots:       uvm.reservedSCSISlots,
		MountCounter:            uvm.mountCounter,
		FirstPort:               uvm.gc.NextPort(),
	}
	if err := statepkg.Write(filepath.Join(path, "state.json"), &state); err != nil {
		return err
	}
	if err := uvm.SCSIManager.Save(ctx, filepath.Join(path, "scsi")); err != nil {
		return err
	}
	if err := uvm.hcsSystem.Save(ctx, &hcsschema.SaveOptions{SaveStateFilePath: filepath.Join(path, "vm.state")}); err != nil {
		return err
	}

	return nil
}

func (uvm *UtilityVM) CompleteSave(ctx context.Context, path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}
	if err := uvm.hcsSystem.Resume(ctx); err != nil {
		return err
	}
	return nil
}

type SCSIDisk struct {
	LUN        string
	Controller string
	Path       string
}

type Resource struct {
	SCSIDisk *SCSIDisk
}

type RestoreSCSI struct {
	Origin OriginSCSIDisk
	Path   string
}

type Edit struct {
	// what goes here?
	// do we use RestoreSCSI?
	// who maps from container (rootfs) -> vm (scsi disk) domain?
	// how does this work with Amit's refactoring?
	Controller string
	LUN        string
	Path       string
}

func RestoreUVM(ctx context.Context, path string, netNS string, id string, edits []*Edit) (*UtilityVM, error) {
	state, err := statepkg.Read[uvmState](filepath.Join(path, "state.json"))
	if err != nil {
		return nil, err
	}
	uvm := &UtilityVM{
		id:                      fmt.Sprintf("%s@vm", id),
		owner:                   "shim-reborn",
		operatingSystem:         "linux",
		scsiControllerCount:     state.SCSIControllerCount,
		physicallyBacked:        state.PhysicallyBacked,
		devicesPhysicallyBacked: state.DevicesPhysicallyBacked,
		reservedSCSISlots:       state.ReservedSCSISlots,
		processorCount:          state.ProcessorCount,
		// protocol:                state.Protocol,
		// guestCaps:               state.GuestCaps,
		containerCounter:     state.ContainerCounter,
		mountCounter:         state.MountCounter,
		exitCh:               make(chan struct{}),
		outputProcessingDone: make(chan struct{}),
		outputHandler:        parseLogrus(&Options{ID: fmt.Sprintf("%s@vm", id)}),
		firstPort:            state.FirstPort,
	}
	config, err := statepkg.Read[hcsschema.ComputeSystem](filepath.Join(path, "config.json"))
	if err != nil {
		return nil, err
	}

	// NICs
	endpoints, err := GetNamespaceEndpoints(ctx, netNS)
	if err != nil {
		return nil, err
	}
	if len(endpoints) != len(config.VirtualMachine.Devices.NetworkAdapters) {
		return nil, fmt.Errorf("expected %d NICs but got %d", len(config.VirtualMachine.Devices.NetworkAdapters), len(endpoints))
	}
	if len(endpoints) != 1 {
		return nil, fmt.Errorf("can only support one endpoint right now")
	}
	// e := endpoints[0]
	config.VirtualMachine.Devices.NetworkAdapters = nil

	for _, e := range edits {
		att := config.VirtualMachine.Devices.Scsi[e.Controller].Attachments[e.LUN]
		att.Path = e.Path
		config.VirtualMachine.Devices.Scsi[e.Controller].Attachments[e.LUN] = att
		if err := wclayer.GrantVmAccess(ctx, fmt.Sprintf("%s@vm", id), e.Path); err != nil {
			return nil, err
		}
	}

	config.VirtualMachine.RestoreState = &hcsschema.RestoreState{SaveStateFilePath: filepath.Join(path, "vm.state")}

	system, err := hcs.CreateComputeSystem(ctx, uvm.id, config)
	if err != nil {
		return nil, err
	}
	properties, err := system.Properties(ctx)
	if err != nil {
		return nil, err
	}
	uvm.runtimeID = properties.RuntimeID
	uvm.hcsSystem = system
	uvm.config = config

	uvm.SCSIRestorer, err = scsi.RestoreManager(ctx, filepath.Join(path, "scsi"))
	if err != nil {
		return nil, err
	}

	uvm.outputListener, err = uvm.listenVsock(linuxLogVsockPort)
	if err != nil {
		return nil, err
	}
	uvm.gcListener, err = uvm.listenVsock(gcs.LinuxGcsVsockPort)
	if err != nil {
		return nil, err
	}

	if err := uvm.Start(ctx); err != nil {
		return nil, err
	}

	if err := uvm.SetupNetworkNamespace(ctx, netNS); err != nil {
		return nil, fmt.Errorf("add netns %s as %s: %w", netNS, GuestNamespaceID, err)
	}

	return uvm, nil
}

func (uvm *UtilityVM) RestoreProcess(ctx context.Context, path string) (cow.Process, error) {
	return nil, fmt.Errorf("not implemented")
}
