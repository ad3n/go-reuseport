package reuseport

import (
	"net"
	"net/netip"
	"strconv"
)

func ResolveAddr(network, address string) (net.Addr, error) {
	switch network {
	default:
		return nil, net.UnknownNetworkError(network)
	case "ip", "ip4", "ip6":
		return net.ResolveIPAddr(network, address)
	case "tcp", "tcp4", "tcp6":
		if addr, ok := literalAddr(network, address); ok {
			ip := addr.Addr().As16()
			return &net.TCPAddr{IP: net.IP(ip[:]), Port: int(addr.Port()), Zone: addr.Addr().Zone()}, nil
		}

		return net.ResolveTCPAddr(network, address)
	case "udp", "udp4", "udp6":
		if addr, ok := literalAddr(network, address); ok {
			ip := addr.Addr().As16()
			return &net.UDPAddr{IP: net.IP(ip[:]), Port: int(addr.Port()), Zone: addr.Addr().Zone()}, nil
		}

		return net.ResolveUDPAddr(network, address)
	case "unix", "unixgram", "unixpacket":
		return net.ResolveUnixAddr(network, address)
	}
}

func literalAddr(network, address string) (netip.AddrPort, bool) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || host == "" || port == "" {
		return netip.AddrPort{}, false
	}

	portnum, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return netip.AddrPort{}, false
	}

	ip, err := netip.ParseAddr(host)
	if err != nil || ip.IsUnspecified() {
		return netip.AddrPort{}, false
	}

	if network[len(network)-1] == '4' && !ip.Is4In6() && !ip.Is4() {
		return netip.AddrPort{}, false
	}

	if network[len(network)-1] == '6' && (ip.Is4In6() || ip.Is4()) {
		return netip.AddrPort{}, false
	}

	return netip.AddrPortFrom(ip, uint16(portnum)), true
}
