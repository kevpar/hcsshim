package uvm

import (
	"net"
	"syscall"

	"github.com/sirupsen/logrus"
)

const _ERROR_CONNECTION_ABORTED syscall.Errno = 1236

func forwardGcsLogs(l net.Listener) {
	c, err := l.Accept()
	l.Close()
	if err != nil {
		logrus.Error("accepting log socket: ", err)
		return
	}
	defer c.Close()
	// io.Copy(os.Stdout, c)
}

// Start synchronously starts the utility VM.
func (uvm *UtilityVM) Start() error {
	if uvm.gcslog != nil {
		go forwardGcsLogs(uvm.gcslog)
		uvm.gcslog = nil
	}
	return uvm.hcsSystem.Start()
}
