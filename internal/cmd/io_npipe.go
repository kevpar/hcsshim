package cmd

import (
	"context"
	"io"
	"net"
	"sync"

	"github.com/Microsoft/go-winio"
	"github.com/Microsoft/hcsshim/internal/log"
	"github.com/sirupsen/logrus"
)

// type redirWriter struct {
// 	m      *sync.Mutex
// 	cond   *sync.Cond
// 	w      io.WriteCloser
// 	closed bool
// }

// func newRedirWriter(w io.WriteCloser) *redirWriter {
// 	m := &sync.Mutex{}
// 	cond := sync.NewCond(m)
// 	return &redirWriter{w: w, m: m, cond: cond}
// }

// func (rw *redirWriter) Write(p []byte) (int, error) {
// 	rw.m.Lock()
// 	defer rw.m.Unlock()
// 	for rw.w == nil && !rw.closed {
// 		rw.cond.Wait()
// 	}
// 	if rw.closed {
// 		return 0, io.ErrClosedPipe
// 	}
// 	return rw.w.Write(p)
// }

// func (rw *redirWriter) Replace(w io.WriteCloser) io.WriteCloser {
// 	rw.m.Lock()
// 	defer rw.m.Unlock()
// 	prev := rw.w
// 	rw.w = w
// 	if rw.w != nil {
// 		rw.cond.Signal()
// 	}
// 	return prev
// }

// func (rw *redirWriter) Close() error {
// 	rw.m.Lock()
// 	defer rw.m.Unlock()
// 	if rw.closed {
// 		return io.ErrClosedPipe
// 	}
// 	rw.closed = true
// 	err := rw.w.Close()
// 	rw.w = nil
// 	rw.cond.Signal()
// 	return err
// }

// type reconnPipeReader struct {
// 	l     net.Listener
// 	r     io.ReadCloser
// 	nextR io.ReadCloser
// 	log   *logrus.Entry
// 	m     *sync.Mutex
// 	cond  *sync.Cond
// }

// func (rp *reconnPipeReader) Read(p []byte) (int, error) {
// 	if rp.r == nil {
// 		for rp.nextR == nil {
// 			rp.cond.Wait()
// 		}
// 		rp.r = rp.nextR
// 		rp.nextR = nil
// 		rp.m.Unlock()
// 	}
// 	n, err := rp.r.Read(p)
// 	if err == io.EOF {
// 		if err := rp.r.Close(); err != nil {
// 			rp.log.WithError(err).Error("rpReader: failed to close existing connection")
// 		}
// 		rp.r = nil
// 		return n, nil
// 	}
// 	return n, err
// }

// func (rp *reconnPipeReader) Close() (err error) {
// 	rp.log.Info("rp Close called")
// 	if err := rp.l.Close(); err != nil {
// 		return err
// 	}
// 	if err := rp.r.Close(); err != nil {
// 		return err
// 	}
// 	return nil
// }

// func (rp *reconnPipeReader) writeLoop() {
// 	for {
// 		c, err := rp.l.Accept()
// 		if err == winio.ErrPipeListenerClosed {
// 			rp.log.Info("rp closed")
// 			return
// 		} else if err != nil {
// 			rp.log.WithField("err", err).Error("failed to accept new rp conn")
// 			time.Sleep(100 * time.Millisecond)
// 			continue
// 		}
// 		rp.log.Info("rp reconnected")
// 		rp.m.Lock()
// 		if rp.nextR != nil {
// 			if err := rp.nextR.Close(); err != nil {
// 				rp.log.WithError(err).Error("rpReader: failed to close next connection")
// 			}
// 		}
// 		rp.nextR = c
// 		rp.cond.Signal()
// 		rp.m.Unlock()
// 		break
// 	}
// }

// func newReconnPipeReader(ctx context.Context, path string, id string) (io.ReadCloser, error) {
// 	log := logrus.WithField("id", id)
// 	c, err := winio.DialPipeContext(ctx, path)
// 	if err != nil {
// 		return nil, err
// 	}
// 	l, err := winio.ListenPipe(path, &winio.PipeConfig{AllowOthers: true, MessageMode: true})
// 	if err != nil {
// 		return nil, err
// 	}
// 	p := &reconnPipeReader{l: l, r: nil, log: log}
// 	go p.writeLoop(c)
// 	return p, nil
// }

// type reconnPipeWriter struct {
// 	l   net.Listener
// 	w   *redirWriter
// 	log *logrus.Entry
// }

// func (rp *reconnPipeWriter) Write(p []byte) (int, error) {
// 	var total int
// 	for total < len(p) {
// 		n, err := rp.w.Write(p[total:])
// 		if n > 0 {
// 			total += n
// 		}
// 		if err != nil {
// 			rp.log.WithError(err).Warn("rp write error")
// 			// some error happened such as
// 			// the other end was closed unexpectedly. we lost whatever was currently in the pipe buffer and skip the rest of this write.
// 			// just pretend we wrote everything without issue.
// 			// if we return the actual number we've written here, we risk aborting an io.Copy to this writer (ErrShortRead).
// 			// number of bytes written to a named pipe in an error condition is not reliable.
// 			prevW := rp.w.Replace(nil)
// 			if prevW != nil {
// 				prevW.Close()
// 			}
// 			// keep going with this write?
// 		}
// 	}
// 	return total, nil
// }

// func (rp *reconnPipeWriter) Close() (err error) {
// 	rp.log.Info("rp Close called")
// 	if err := rp.l.Close(); err != nil {
// 		return err
// 	}
// 	if err := rp.w.Close(); err != nil {
// 		return err
// 	}
// 	return nil
// }

// func (rp *reconnPipeWriter) readLoop(c net.Conn) {
// 	for {
// 		n, err := c.Read(nil)
// 		rp.log.WithFields(logrus.Fields{
// 			"n":   n,
// 			"err": err,
// 		}).Info("rp read completed")
// 		// If we get here either there was a CloseWrite, or the pipe closed altogether.
// 		if err == io.EOF {
// 			prevW := rp.w.Replace(nil)
// 			if prevW != nil {
// 				prevW.Close()
// 			}
// 			for {
// 				c, err = rp.l.Accept()
// 				if err == winio.ErrPipeListenerClosed {
// 					rp.log.Info("rp closed")
// 					return
// 				} else if err != nil {
// 					rp.log.WithField("err", err).Error("failed to accept new rp conn")
// 					time.Sleep(500 * time.Millisecond)
// 					continue
// 				}
// 				rp.log.Info("rp reconnected")
// 				// Redirect our writer to the new connection. This will unblock any pending write operation.
// 				rp.w.Replace(c)
// 				break
// 			}
// 		}
// 	}
// }

// func newReconnPipeWriter(ctx context.Context, path string, id string) (io.WriteCloser, error) {
// 	log := logrus.WithField("id", id)
// 	c, err := winio.DialPipeContext(ctx, path)
// 	if err != nil {
// 		return nil, err
// 	}
// 	l, err := winio.ListenPipe(path, &winio.PipeConfig{AllowOthers: true, MessageMode: true})
// 	if err != nil {
// 		return nil, err
// 	}
// 	p := &reconnPipeWriter{l: l, w: newRedirWriter(c), log: log}
// 	go p.readLoop(c)
// 	return p, nil
// }

type reconnectPipe struct {
	c      net.Conn
	path   string
	ctx    context.Context
	cancel func()
}

func DialReconnectPipe(ctx context.Context, path string) (io.ReadWriteCloser, error) {
	c, err := winio.DialPipeContext(ctx, path)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &reconnectPipe{
		c:      c,
		path:   path,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

func (rc *reconnectPipe) Read(b []byte) (int, error) {
	return 0, nil
}

func (rc *reconnectPipe) Write(b []byte) (int, error) {
	return 0, nil
}

func (rc *reconnectPipe) Close() error {
	return nil
}

// NewNpipeIO creates connected upstream io. It is the callers responsibility to
// validate that `if terminal == true`, `stderr == ""`.
func NewNpipeIO(ctx context.Context, stdin, stdout, stderr string, terminal bool) (_ UpstreamIO, err error) {
	log.G(ctx).WithFields(logrus.Fields{
		"stdin":    stdin,
		"stdout":   stdout,
		"stderr":   stderr,
		"terminal": terminal}).Debug("NewNpipeIO")

	nio := &npipeio{
		stdin:    stdin,
		stdout:   stdout,
		stderr:   stderr,
		terminal: terminal,
	}
	defer func() {
		if err != nil {
			nio.Close(ctx)
		}
	}()
	if stdin != "" {
		c, err := DialReconnectPipe(ctx, stdin)
		if err != nil {
			return nil, err
		}
		nio.sin = c
	}
	if stdout != "" {
		c, err := DialReconnectPipe(ctx, stdout)
		if err != nil {
			return nil, err
		}
		nio.sout = c
	}
	if stderr != "" {
		c, err := DialReconnectPipe(ctx, stderr)
		if err != nil {
			return nil, err
		}
		nio.serr = c
	}
	return nio, nil
}

var _ = (UpstreamIO)(&npipeio{})

type npipeio struct {
	// stdin, stdout, stderr are the original paths used to open the connections.
	//
	// They MUST be treated as readonly in the lifetime of the pipe io.
	stdin, stdout, stderr string
	// terminal is the original setting passed in on open.
	//
	// This MUST be treated as readonly in the lifetime of the pipe io.
	terminal bool

	// sin is the upstream `stdin` connection.
	//
	// `sin` MUST be treated as readonly in the lifetime of the pipe io after
	// the return from `NewNpipeIO`.
	sin       io.ReadCloser
	sinCloser sync.Once

	// sout and serr are the upstream `stdout` and `stderr` connections.
	//
	// `sout` and `serr` MUST be treated as readonly in the lifetime of the pipe
	// io after the return from `NewNpipeIO`.
	sout, serr   io.WriteCloser
	outErrCloser sync.Once
}

func (nio *npipeio) Close(ctx context.Context) {
	nio.sinCloser.Do(func() {
		if nio.sin != nil {
			log.G(ctx).Debug("npipeio::sinCloser")
			nio.sin.Close()
		}
	})
	nio.outErrCloser.Do(func() {
		if nio.sout != nil {
			log.G(ctx).Debug("npipeio::outErrCloser - stdout")
			nio.sout.Close()
		}
		if nio.serr != nil {
			log.G(ctx).Debug("npipeio::outErrCloser - stderr")
			nio.serr.Close()
		}
	})
}

func (nio *npipeio) CloseStdin(ctx context.Context) {
	nio.sinCloser.Do(func() {
		if nio.sin != nil {
			log.G(ctx).Debug("npipeio::sinCloser")
			nio.sin.Close()
		}
	})
}

func (nio *npipeio) Stdin() io.Reader {
	return nio.sin
}

func (nio *npipeio) StdinPath() string {
	return nio.stdin
}

func (nio *npipeio) Stdout() io.Writer {
	return nio.sout
}

func (nio *npipeio) StdoutPath() string {
	return nio.stdout
}

func (nio *npipeio) Stderr() io.Writer {
	return nio.serr
}

func (nio *npipeio) StderrPath() string {
	return nio.stderr
}

func (nio *npipeio) Terminal() bool {
	return nio.terminal
}
