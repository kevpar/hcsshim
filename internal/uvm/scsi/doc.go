// Package scsi handles SCSI device attachment and mounting for VMs.
// The primary entrypoint to the package is [Manager].
//
// The backend implementation of working with disks for a given VM is
// provided by the interfaecs [Attacher], [Mounter], and [Unplugger].
package scsi
