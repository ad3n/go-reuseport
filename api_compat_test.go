package reuseport_test

import (
	"net"
	"syscall"
	"time"

	"github.com/ad3n/go-reuseport"
)

// These assignments lock the signatures of the API that existed before the
// module rename. A signature change will fail compilation.
var (
	_ func() bool                                                   = reuseport.Available
	_ func(string, string, syscall.RawConn) error                   = reuseport.Control
	_ func(string, string) (net.Listener, error)                    = reuseport.Listen
	_ func(string, string) (net.PacketConn, error)                  = reuseport.ListenPacket
	_ func(string, string, string) (net.Conn, error)                = reuseport.Dial
	_ func(string, string, string, time.Duration) (net.Conn, error) = reuseport.DialTimeout
	_ func(string, string) (net.Addr, error)                        = reuseport.ResolveAddr
)
