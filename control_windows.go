package reuseport

import (
	"sync"
	"syscall"

	"golang.org/x/sys/windows"
)

type controlState struct {
	err error
	fn  func(uintptr)
}

var controlStatePool = sync.Pool{New: func() any {
	s := new(controlState)
	s.fn = func(fd uintptr) {
		s.err = windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_REUSEADDR, 1)
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
