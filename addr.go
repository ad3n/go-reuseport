package reuseport

import (
	"net"
	"net/netip"
	"strings"
)

type (
	literalIPResult struct {
		ip     netip.Addr
		shared bool
	}

	literalSocketResult struct {
		ip   literalIPResult
		port int
	}

	resolvedIPAddr struct {
		addr net.IPAddr
		ip   [16]byte
	}

	resolvedTCPAddr struct {
		addr net.TCPAddr
		ip   [16]byte
	}

	resolvedUDPAddr struct {
		addr net.UDPAddr
		ip   [16]byte
	}
)

func ResolveAddr(network, address string) (net.Addr, error) {
	switch network {
	default:
		return nil, net.UnknownNetworkError(network)
	case "ip", "ip4", "ip6":
		if address == "" {
			return &net.IPAddr{}, nil
		}

		ip, ok, err := literalIP(network, address)
		if err != nil {
			return (*net.IPAddr)(nil), err
		}

		if ok {
			if ip.shared {
				return &net.IPAddr{IP: net.IPv4zero}, nil
			}

			result := &resolvedIPAddr{ip: ip.ip.As16()}
			result.addr = net.IPAddr{IP: net.IP(result.ip[:]), Zone: ip.ip.Zone()}
			return &result.addr, nil
		}

		return net.ResolveIPAddr(network, address)
	case "tcp", "tcp4", "tcp6":
		addr, ok, err := literalAddr(network, address)
		if err != nil {
			return (*net.TCPAddr)(nil), err
		}

		if ok {
			if addr.ip.shared {
				return &net.TCPAddr{IP: net.IPv4zero, Port: addr.port}, nil
			}

			if !addr.ip.ip.IsValid() {
				return &net.TCPAddr{Port: addr.port}, nil
			}

			result := &resolvedTCPAddr{ip: addr.ip.ip.As16()}
			result.addr = net.TCPAddr{IP: net.IP(result.ip[:]), Port: addr.port, Zone: addr.ip.ip.Zone()}
			return &result.addr, nil
		}

		return net.ResolveTCPAddr(network, address)
	case "udp", "udp4", "udp6":
		addr, ok, err := literalAddr(network, address)
		if err != nil {
			return (*net.UDPAddr)(nil), err
		}

		if ok {
			if addr.ip.shared {
				return &net.UDPAddr{IP: net.IPv4zero, Port: addr.port}, nil
			}

			if !addr.ip.ip.IsValid() {
				return &net.UDPAddr{Port: addr.port}, nil
			}

			result := &resolvedUDPAddr{ip: addr.ip.ip.As16()}
			result.addr = net.UDPAddr{IP: net.IP(result.ip[:]), Port: addr.port, Zone: addr.ip.ip.Zone()}
			return &result.addr, nil
		}

		return net.ResolveUDPAddr(network, address)
	case "unix", "unixgram", "unixpacket":
		return net.ResolveUnixAddr(network, address)
	}
}

func literalAddr(network, address string) (literalSocketResult, bool, error) {
	if address == "" {
		return literalSocketResult{}, true, nil
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return literalSocketResult{}, false, err
	}

	portnum, numeric := parseNumericPort(port)
	if numeric && portnum < 0 {
		return literalSocketResult{}, false, &net.AddrError{Err: "invalid port", Addr: port}
	}

	var parsed netip.Addr
	var zone string
	if host != "" && !numeric {
		unzoned, suffix, zoned := strings.Cut(host, "%")
		if zoned && suffix == "" {
			return literalSocketResult{}, false, nil
		}

		var ok bool
		parsed, ok = parseLiteralHost(unzoned)
		if !ok || zoned && !parsed.Is6() {
			return literalSocketResult{}, false, nil
		}

		zone = suffix
	}

	if !numeric {
		portnum, err = net.LookupPort(network, port)
		if err != nil {
			return literalSocketResult{}, false, err
		}
	}

	if host == "" {
		return literalSocketResult{port: portnum}, true, nil
	}

	if !parsed.IsValid() {
		var ok bool
		parsed, ok = parseLiteralHost(host)
		if !ok {
			return literalSocketResult{}, false, nil
		}
	}

	if zone != "" {
		parsed = parsed.WithZone(zone)
	}

	ip, ok, err := selectLiteralIP(network, host, parsed)
	if !ok || err != nil {
		return literalSocketResult{}, false, err
	}

	return literalSocketResult{ip: ip, port: portnum}, true, nil
}

func parseNumericPort(port string) (int, bool) {
	start := 0
	negative := false
	if len(port) > 0 && (port[0] == '+' || port[0] == '-') {
		negative = port[0] == '-'
		start = 1
	}

	var value uint32
	for i := start; i < len(port); i++ {
		if port[i] < '0' || port[i] > '9' {
			return 0, false
		}

		if value >= 1<<30 {
			return -1, true
		}

		value *= 10
		next := value + uint32(port[i]-'0')
		if next < value {
			return -1, true
		}

		value = next
	}

	if value > 65535 || negative && value != 0 {
		return -1, true
	}

	return int(value), true
}

func parseLiteralIPv4(host string) (netip.Addr, bool) {
	var bytes [4]byte
	pos := 0
	for part := range bytes {
		start := pos
		n := 0
		for pos < len(host) && host[pos] != '.' {
			c := host[pos]
			if c < '0' || c > '9' || pos-start == 3 || pos > start && host[start] == '0' {
				return netip.Addr{}, false
			}

			n = n*10 + int(c-'0')
			pos++
		}

		if pos == start || n > 255 {
			return netip.Addr{}, false
		}

		bytes[part] = byte(n)
		if part == 3 {
			if pos != len(host) {
				return netip.Addr{}, false
			}

			return netip.AddrFrom4(bytes), true
		}

		if pos == len(host) {
			return netip.Addr{}, false
		}

		pos++
	}

	return netip.Addr{}, false
}

func parseLiteralHost(host string) (netip.Addr, bool) {
	if !strings.Contains(host, ":") {
		return parseLiteralIPv4(host)
	}

	host, zone, zoned := strings.Cut(host, "%")
	if zoned && zone == "" {
		return netip.Addr{}, false
	}

	var bytes [16]byte
	pos, used, compressed := 0, 0, -1
	if strings.HasPrefix(host, "::") {
		pos, compressed = 2, 0
	}

	for pos < len(host) {
		start := pos
		value := uint16(0)
		for pos < len(host) && host[pos] != ':' && host[pos] != '.' {
			c := host[pos]
			switch {
			case c >= '0' && c <= '9':
				value = value<<4 | uint16(c-'0')
			case c|0x20 >= 'a' && c|0x20 <= 'f':
				value = value<<4 | uint16((c|0x20)-'a'+10)
			default:
				return netip.Addr{}, false
			}

			pos++
			if pos-start > 4 {
				return netip.Addr{}, false
			}
		}

		if pos < len(host) && host[pos] == '.' {
			if used > 12 {
				return netip.Addr{}, false
			}

			ip, ok := parseLiteralIPv4(host[start:])
			if !ok {
				return netip.Addr{}, false
			}

			v4 := ip.As4()
			copy(bytes[used:], v4[:])
			used += 4
			pos = len(host)
			break
		}

		if pos == start || used > 14 {
			return netip.Addr{}, false
		}

		bytes[used], bytes[used+1] = byte(value>>8), byte(value)
		used += 2
		if pos == len(host) {
			break
		}

		pos++
		if pos == len(host) {
			return netip.Addr{}, false
		}

		if host[pos] == ':' {
			if compressed >= 0 {
				return netip.Addr{}, false
			}

			compressed = used
			pos++
		}
	}

	if compressed >= 0 {
		if used == 16 {
			return netip.Addr{}, false
		}

		gap := 16 - used
		copy(bytes[compressed+gap:], bytes[compressed:used])
		clear(bytes[compressed : compressed+gap])
		return netip.AddrFrom16(bytes).WithZone(zone), true
	}

	if used != 16 {
		return netip.Addr{}, false
	}

	return netip.AddrFrom16(bytes).WithZone(zone), true
}

func literalIP(network, host string) (literalIPResult, bool, error) {
	ip, ok := parseLiteralHost(host)
	if !ok {
		return literalIPResult{}, false, nil
	}

	return selectLiteralIP(network, host, ip)
}

func selectLiteralIP(network, host string, ip netip.Addr) (literalIPResult, bool, error) {
	if network[len(network)-1] == '4' && !ip.Is4In6() && !ip.Is4() {
		bytes := ip.As16()
		if net.IP(bytes[:]).Equal(net.IPv6unspecified) && net.IPv4zero.To4() != nil {
			return literalIPResult{shared: true}, true, nil
		}

		return literalIPResult{}, false, &net.AddrError{Err: "no suitable address found", Addr: host}
	}

	if network[len(network)-1] == '6' && (ip.Is4In6() || ip.Is4()) {
		return literalIPResult{}, false, &net.AddrError{Err: "no suitable address found", Addr: host}
	}

	return literalIPResult{ip: ip}, true, nil
}
