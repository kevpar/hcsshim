//go:build linux
// +build linux

// TODO: consider moving oci spec specific code from /internal/guest/runtime/hcsv2/spec.go

package spec

import (
	"path/filepath"
	"strings"

	"github.com/Microsoft/hcsshim/internal/guestpath"
	oci "github.com/opencontainers/runtime-spec/specs-go"
)

// networkingMountPaths returns an array of mount paths to enable networking
// inside containers.
func networkingMountPaths() []string {
	return []string{
		"/etc/hostname",
		"/etc/hosts",
		"/etc/resolv.conf",
	}
}

// GenerateWorkloadContainerNetworkMounts generates an array of specs.Mount
// required for container networking. Original spec is left untouched and
// it's the responsibility of a caller to update it.
func GenerateWorkloadContainerNetworkMounts(sandboxNetworkMountsRoot string, spec *oci.Spec) []oci.Mount {
	var nMounts []oci.Mount

	for _, mountPath := range networkingMountPaths() {
		// Don't override if the mount is present in the spec
		if MountPresent(mountPath, spec.Mounts) {
			continue
		}
		options := []string{"bind"}
		if spec.Root != nil && spec.Root.Readonly {
			options = append(options, "ro")
		}
		trimmedMountPath := strings.TrimPrefix(mountPath, "/etc/")
		mt := oci.Mount{
			Destination: mountPath,
			Type:        "bind",
			Source:      filepath.Join(sandboxNetworkMountsRoot, trimmedMountPath),
			Options:     options,
		}
		nMounts = append(nMounts, mt)
	}
	return nMounts
}

// MountPresent checks if mountPath is present in the specMounts array.
func MountPresent(mountPath string, specMounts []oci.Mount) bool {
	for _, m := range specMounts {
		if m.Destination == mountPath {
			return true
		}
	}
	return false
}

func SandboxMountSource(sandboxMountsRoot, path string) string {
	subPath := strings.TrimPrefix(path, guestpath.SandboxMountPrefix)
	return filepath.Join(sandboxMountsRoot, subPath)
}
