package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Microsoft/hcsshim/internal/save"
)

func (s *service) startSave(ctx context.Context, path string) ([]*save.SaveResource, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, err
	}
	v := s.taskOrPod.Load()
	if v == nil {
		return nil, fmt.Errorf("invalid state: no pod")
	}
	p, ok := v.(shimPod)
	if !ok {
		return nil, fmt.Errorf("only works with pod, not standalone task")
	}
	resources, err := p.StartSave(ctx, path)
	if err != nil {
		return nil, err
	}
	return resources, nil
}

func (s *service) completeSave(ctx context.Context, path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}
	v := s.taskOrPod.Load()
	if v == nil {
		return fmt.Errorf("invalid state: no pod")
	}
	p, ok := v.(shimPod)
	if !ok {
		return fmt.Errorf("only works with pod, not standalone task")
	}
	if err := p.CompleteSave(ctx, path); err != nil {
		return err
	}
	return nil
}

func (s *service) restore(ctx context.Context, path string) error {
	// p, err := restorePod(ctx, filepath.Join(path, "pod"))
	// if err != nil {
	// 	return err
	// }
	// s.standbyPod = p
	return nil
}
