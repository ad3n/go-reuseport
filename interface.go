// Package reuseport provides Listen and Dial functions that set socket
// options in order to be able to reuse ports. You should only use this
// package if you know what SO_REUSEADDR and SO_REUSEPORT are.
//
// For example:
//
//	// listen on the same port. oh yeah.
//	l1, _ := reuseport.Listen("tcp", "127.0.0.1:1234")
//	l2, _ := reuseport.Listen("tcp", "127.0.0.1:1234")
//
//	// dial from the same port. oh yeah.
//	l1, _ := reuseport.Listen("tcp", "127.0.0.1:1234")
//	l2, _ := reuseport.Listen("tcp", "127.0.0.1:1235")
//	c, _ := reuseport.Dial("tcp", "127.0.0.1:1234", "127.0.0.1:1235")
//
// Note: cant dial self because tcp/ip stacks use 4-tuples to identify connections,
// and doing so would clash.
package reuseport

import (
	"context"
	"fmt"
	"net"
	"time"
)

// Available returns whether or not SO_REUSEPORT or equivalent behaviour is
// available in the OS.
func Available() bool {
	return true
}

var listenConfig = net.ListenConfig{
	Control: Control,
}

// Listen listens at the given network and address. see net.Listen
// Returns a net.Listener created from a file discriptor for a socket
// with SO_REUSEPORT and SO_REUSEADDR option set.
func Listen(network, address string) (net.Listener, error) {
	return listenConfig.Listen(context.Background(), network, address)
}

// ListenPacket listens at the given network and address. see net.ListenPacket
// Returns a net.Listener created from a file discriptor for a socket
// with SO_REUSEPORT and SO_REUSEADDR option set.
func ListenPacket(network, address string) (net.PacketConn, error) {
	return listenConfig.ListenPacket(context.Background(), network, address)
}

// Dial dials the given network and address. see net.Dial
// Returns a net.Conn created from a file descriptor for a socket
// with SO_REUSEPORT and SO_REUSEADDR option set.
func Dial(network, laddr, raddr string) (net.Conn, error) {
	return DialTimeout(network, laddr, raddr, time.Duration(0))
}

// Dial dials the given network and address, with the given timeout. see
// net.DialTimeout Returns a net.Conn created from a file descriptor for
// a socket with SO_REUSEPORT and SO_REUSEADDR option set.
func DialTimeout(network, laddr, raddr string, timeout time.Duration) (net.Conn, error) {
	d, err := NewDialer(network, laddr, timeout)
	if err != nil {
		return nil, err
	}
	return d.Dial(network, raddr)
}

// Dialer is a reusable dialer configured for port reuse. Unlike Dial and
// DialTimeout, it resolves the local address only once. Reusing a Dialer for
// multiple connections reduces allocations and DNS resolver work.
//
// A Dialer is safe for concurrent use.
type Dialer struct {
	dialer net.Dialer
}

// NewDialer returns a reusable Dialer bound to laddr.
func NewDialer(network, laddr string, timeout time.Duration) (Dialer, error) {
	nla, err := ResolveAddr(network, laddr)
	if err != nil {
		return Dialer{}, fmt.Errorf("failed to resolve local addr: %w", err)
	}
	return Dialer{dialer: net.Dialer{
		Control:   Control,
		LocalAddr: nla,
		Timeout:   timeout,
	}}, nil
}

// Dial connects to address using the dialer's pre-resolved local address.
func (d *Dialer) Dial(network, address string) (net.Conn, error) {
	return d.dialer.Dial(network, address)
}

// DialContext connects to address using the provided context and the dialer's
// pre-resolved local address.
func (d *Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d.dialer.DialContext(ctx, network, address)
}
