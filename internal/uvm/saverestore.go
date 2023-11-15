package uvm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Microsoft/hcsshim/hcn"
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
}

func (uvm *UtilityVM) StartSave(ctx context.Context, path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
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

func RestoreUVM(ctx context.Context, path string, netNS string, scratchPath string, id string) (*UtilityVM, error) {
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
	}
	config, err := statepkg.Read[hcsschema.ComputeSystem](filepath.Join(path, "config.json"))
	if err != nil {
		return nil, err
	}

	// NICs
	hcnNamespace, err := hcn.GetNamespaceByID(netNS)
	if err != nil {
		return nil, err
	}
	uvm.namespaces = map[string]*namespaceInfo{
		hcnNamespace.Id: {make(map[string]*nicInfo)},
	}
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
	e := endpoints[0]
	for k := range config.VirtualMachine.Devices.NetworkAdapters {
		config.VirtualMachine.Devices.NetworkAdapters[k] = hcsschema.NetworkAdapter{
			EndpointId: e.Id,
			MacAddress: e.MacAddress,
		}
		uvm.namespaces[hcnNamespace.Id].nics[k] = &nicInfo{
			ID:       k,
			Endpoint: e,
		}
	}

	// Scratch disk
	type disk struct {
		controller string
		lun        string
	}
	var writableDisks []disk
	for i := range config.VirtualMachine.Devices.Scsi {
		for j := range config.VirtualMachine.Devices.Scsi[i].Attachments {
			if !config.VirtualMachine.Devices.Scsi[i].Attachments[j].ReadOnly {
				writableDisks = append(writableDisks, disk{i, j})
			}
		}
	}
	if len(writableDisks) != 1 {
		return nil, fmt.Errorf("expected 1 writable disk but got %d", len(writableDisks))
	}
	d := writableDisks[0]
	scratch := config.VirtualMachine.Devices.Scsi[d.controller].Attachments[d.lun]
	scratch.Path = scratchPath
	config.VirtualMachine.Devices.Scsi[d.controller].Attachments[d.lun] = scratch
	if err := wclayer.GrantVmAccess(ctx, fmt.Sprintf("%s@vm", id), scratchPath); err != nil {
		return nil, err
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

	// uvm.outputListener, err = uvm.listenVsock(linuxLogVsockPort)
	// if err != nil {
	// 	return nil, err
	// }
	uvm.gcListener, err = uvm.listenVsock(gcs.LinuxGcsVsockPort)
	if err != nil {
		return nil, err
	}

	if err := uvm.Start(ctx); err != nil {
		return nil, err
	}

	uvm.SCSIRestorer, err = scsi.RestoreManager(ctx, filepath.Join(path, "scsi"))
	if err != nil {
		return nil, err
	}

	return uvm, nil
}
