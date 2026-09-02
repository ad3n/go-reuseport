//go:build !plan9 && !windows && !wasm && !freebsd

package reuseport

import (
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

type controlState struct {
	err error
	fn  func(uintptr)
}

var controlStatePool = sync.Pool{New: func() any {
	s := new(controlState)
	s.fn = func(fd uintptr) {
		s.err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
		if s.err == nil {
			s.err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
		}
	}
	return s
}}

func Control(network, address string, c syscall.RawConn) error {
	s := controlStatePool.Get().(*controlState)
	controlErr := c.Control(s.fn)
	if controlErr != nil {
		s.err = controlErr
	}
	err := s.err
	s.err = nil
	controlStatePool.Put(s)
	return err
}
