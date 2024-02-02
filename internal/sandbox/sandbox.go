package sandbox

import (
	"context"

	"github.com/Microsoft/hcsshim/internal/uvm"
)

type Sandbox struct {
	uvm *uvm.UtilityVM
}

func NewSandboxWithUVM(uvm *uvm.UtilityVM) *Sandbox {
	return &Sandbox{
		uvm: uvm,
	}
}

func (s *Sandbox) AddContainerMounts(mounts any) error {
	return nil
}

func (s *Sandbox) ConfigureNetworking(ctx context.Context, nsid string) error {
	if err := s.uvm.ConfigureNetworking(ctx, nsid); err == uvm.ErrNoNetworkSetup {
		if err := s.uvm.CreateAndAssignNetworkSetup(ctx, "", ""); err != nil {
			return err
		}
		if err := s.uvm.ConfigureNetworking(ctx, nsid); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}
