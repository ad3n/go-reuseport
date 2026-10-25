package reuseport

import (
	"fmt"
	"net"
	"net/netip"
	"reflect"
	"strings"
	"testing"
)

func FuzzLiteralHostValidation(f *testing.F) {
	for _, host := range []string{"", "127.0.0.1", "::", "::1", ":1", "1::", ":::", "::gg", "fe80::1%", "fe80::1%lo0", "::ffff:127.0.0.1", "1:2:3:4:5:6:7:8"} {
		f.Add(host)
	}

	f.Fuzz(func(t *testing.T, host string) {
		want, err := netip.ParseAddr(host)
		parsed, got := parseLiteralHost(host)
		if got != (err == nil) {
			t.Fatalf("parseLiteralHost(%q) = %t; ParseAddr error = %v", host, got, err)
		}

		if err == nil && parsed != want {
			t.Fatalf("parseLiteralHost(%q) = %v; want %v", host, parsed, want)
		}

		if err == nil && want.Is4() {
			got, ok := parseLiteralIPv4(host)
			if !ok || got != want {
				t.Fatalf("IPv4 %q: got %v, want %v", host, got, want)
			}
		}
	})
}

func TestFallbackEdgeCompatibility(t *testing.T) {
	for _, address := range []string{"[:::]:80", "[::gg]:80", "[fe80::1%]:80", "localhost:3com-tsmux"} {
		if address == "localhost:3com-tsmux" {
			if _, err := net.LookupPort("tcp", "3com-tsmux"); err != nil {
				continue
			}
		}

		want, wantErr := standardAddr("tcp", address)
		got, gotErr := ResolveAddr("tcp", address)
		if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(gotErr, wantErr) {
			t.Fatalf("%q: got (%#v, %v), want (%#v, %v)", address, got, gotErr, want, wantErr)
		}

		baseline := testing.AllocsPerRun(20, func() { _, _ = standardAddr("tcp", address) })
		optimized := testing.AllocsPerRun(20, func() { _, _ = ResolveAddr("tcp", address) })
		if optimized > baseline {
			t.Fatalf("%q: %v allocations, standard %v", address, optimized, baseline)
		}
	}
}

func TestIPv4UnspecifiedSharedStorage(t *testing.T) {
	saved := append(net.IP(nil), net.IPv4zero...)
	defer copy(net.IPv4zero, saved)

	for _, network := range []string{"ip4", "tcp4", "udp4"} {
		for _, host := range []string{"::", "::%lo0"} {
			address := host
			if network != "ip4" {
				address = net.JoinHostPort(host, "80")
			}

			got, err := ResolveAddr(network, address)
			if err != nil {
				t.Fatal(err)
			}

			var ip net.IP
			switch a := got.(type) {
			case *net.IPAddr:
				ip = a.IP
			case *net.TCPAddr:
				ip = a.IP
			case *net.UDPAddr:
				ip = a.IP
			}

			if &ip[0] != &net.IPv4zero[0] {
				t.Fatalf("%s %q does not share net.IPv4zero", network, address)
			}

			ip[15] = 7
			want, wantErr := standardAddr(network, address)
			next, nextErr := ResolveAddr(network, address)
			if !reflect.DeepEqual(next, want) || !reflect.DeepEqual(nextErr, wantErr) {
				t.Fatalf("%s %q diverged after mutation: got %v, want %v", network, address, next, want)
			}

			copy(net.IPv4zero, saved)
		}
	}
}

func BenchmarkFallbackEdges(b *testing.B) {
	for _, address := range []string{"[:::]:80", "[::gg]:80", "[fe80::1%]:80", "localhost:3com-tsmux"} {
		for _, standard := range []bool{true, false} {
			b.Run(fmt.Sprintf("%s/standard=%t", address, standard), func(b *testing.B) {
				resolve := ResolveAddr
				if standard {
					resolve = standardAddr
				}

				b.ReportAllocs()
				for b.Loop() {
					_, _ = resolve("tcp", address)
				}
			})
		}
	}
}

func TestLiteralParserIPv6Forms(t *testing.T) {
	groups := []string{"2001", "db8", "1234", "5678", "9abc", "def0", "1234", "5678"}
	hosts := []string{"::ffff:192.0.2.1", "1:2:3:4:5:6:192.0.2.1", "1:2:3:4:5::192.0.2.1", "1:2:3:4:5:6:7:192.0.2.1", "::ffff:192.00.2.1", "2001:db8:12345::", "1:2:3:4:5:6:7:8::", "::1::", ":::", "fe80::1%"}
	for start := range groups {
		for count := 1; count <= len(groups)-start; count++ {
			host := strings.Join(groups[:start], ":") + "::" + strings.Join(groups[start+count:], ":")
			hosts = append(hosts, host, strings.ToUpper(host), host+"%eth0", host+"%zone%with:colons")
		}
	}

	for _, host := range hosts {
		want, err := netip.ParseAddr(host)
		got, ok := parseLiteralHost(host)
		if ok != (err == nil) || ok && got != want {
			t.Fatalf("%q: got (%v, %t), want (%v, %v)", host, got, ok, want, err)
		}
	}
}

func TestLiteralParserZeroAlloc(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1", "2001:db8:1234:5678:9abc:def0:1234:5678", "::ffff:192.0.2.1", "localhost", ":::", "::gg", "fe80::1%"} {
		allocs := testing.AllocsPerRun(100, func() { _, _ = parseLiteralHost(host) })
		if allocs != 0 {
			t.Fatalf("%q parser allocated %v objects", host, allocs)
		}
	}
}
