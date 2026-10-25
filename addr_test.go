package reuseport

import (
	"fmt"
	"net"
	"reflect"
	"testing"
)

func standardAddr(network, address string) (net.Addr, error) {
	switch network {
	case "ip", "ip4", "ip6":
		return net.ResolveIPAddr(network, address)
	case "tcp", "tcp4", "tcp6":
		return net.ResolveTCPAddr(network, address)
	default:
		return net.ResolveUDPAddr(network, address)
	}
}

func TestResolveAddrCompatibility(t *testing.T) {
	for _, network := range []string{"tcp", "tcp4", "tcp6", "udp", "udp4", "udp6"} {
		for _, address := range []string{
			"", ":", "[]:", "[]:80", ":0", ":65535", ":65536", ":-1", ":+80", ":http", "127.0.0.1:80", "127.0.0.1:00080", "127.0.0.1:+80",
			"127.0.0.1:", "127.0.0.1:65536", "127.0.0.1:-1", "127.0.0.1:http", "127.0.0.1:00000000000000000000000000000000080",
			"127.0.0.1:9999999999999999999999999",
			"[127.0.0.1]:80", "[::1]:80", "[::ffff:127.0.0.1]:80",
			"[fe80::1%lo0]:80", "[fe80::1%a[b]:80", "[fe80::1%a]b]:80",
			"[::ffff:127.0.0.1%lo0]:80", "[::]:80", "0.0.0.0:80",
			"[::ffff:0.0.0.0]:80", "[::%lo0]:80", "[0:0:0:0:0:0:0:0%lo0]:80", "localhost:80", "localhost:http", "[::1]", "::1:80",
		} {
			t.Run(network+"/"+address, func(t *testing.T) {
				want, wantErr := standardAddr(network, address)
				got, gotErr := ResolveAddr(network, address)
				if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(gotErr, wantErr) {
					t.Fatalf("got (%#v, %v), want (%#v, %v)", got, gotErr, want, wantErr)
				}
			})
		}
	}
}

func TestResolveAddrIndependentResults(t *testing.T) {
	first, _ := ResolveAddr("tcp", "127.0.0.1:80")
	second, _ := ResolveAddr("tcp", "127.0.0.1:80")
	first.(*net.TCPAddr).IP[15] = 2
	if second.String() != "127.0.0.1:80" {
		t.Fatal("resolved addresses share mutable IP storage")
	}
}

func BenchmarkResolveAddr(b *testing.B) {
	for _, address := range []string{"127.0.0.1:80", "[::1]:80", "[2001:db8:1234:5678:9abc:def0:1234:5678]:80", ":0", "", "127.0.0.1:"} {
		for _, standard := range []bool{true, false} {
			b.Run(fmt.Sprintf("%s/standard=%t", address, standard), func(b *testing.B) {
				resolve := ResolveAddr
				if standard {
					resolve = standardAddr
				}

				b.ReportAllocs()
				for b.Loop() {
					addr, err := resolve("tcp", address)
					if err != nil || addr == nil {
						b.Fatal(addr, err)
					}
				}
			})
		}
	}
}

func FuzzResolveAddrLiteralCompatibility(f *testing.F) {
	for _, address := range []string{"", ":", ":80", "[]:80", "127.0.0.1:", "127.0.0.1:80", "[::1]:80", "[fe80::1%lo0]:80", "[::ffff:127.0.0.1]:80"} {
		f.Add(address)
	}

	f.Fuzz(func(t *testing.T, address string) {
		for _, network := range []string{"tcp", "tcp4", "tcp6", "udp", "udp4", "udp6"} {
			if _, ok, err := literalAddr(network, address); !ok && err == nil {
				continue
			}

			want, wantErr := standardAddr(network, address)
			got, gotErr := ResolveAddr(network, address)
			if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(gotErr, wantErr) {
				t.Fatalf("%s %q: got (%#v, %v), want (%#v, %v)", network, address, got, gotErr, want, wantErr)
			}
		}
	})
}

func TestResolveIPAddrCompatibility(t *testing.T) {
	for _, network := range []string{"ip", "ip4", "ip6"} {
		for _, address := range []string{"", "127.0.0.1", "::1", "::ffff:127.0.0.1", "fe80::1%lo0", "::", "0.0.0.0", "::ffff:0.0.0.0", "::%lo0", "0:0:0:0:0:0:0:0%lo0", "localhost"} {
			t.Run(network+"/"+address, func(t *testing.T) {
				want, wantErr := standardAddr(network, address)
				got, gotErr := ResolveAddr(network, address)
				if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(gotErr, wantErr) {
					t.Fatalf("got (%#v, %v), want (%#v, %v)", got, gotErr, want, wantErr)
				}
			})
		}
	}
}

func TestResolveIPAndWildcardIndependentResults(t *testing.T) {
	for _, network := range []string{"ip", "tcp", "udp"} {
		first, _ := ResolveAddr(network, "")
		second, _ := ResolveAddr(network, "")
		switch addr := first.(type) {
		case *net.IPAddr:
			addr.Zone = "changed"
		case *net.TCPAddr:
			addr.Port = 1234
		case *net.UDPAddr:
			addr.Port = 1234
		}

		want, _ := standardAddr(network, "")
		if !reflect.DeepEqual(second, want) {
			t.Fatalf("%s wildcard results share mutable storage", network)
		}
	}

	first, _ := ResolveAddr("ip", "127.0.0.1")
	second, _ := ResolveAddr("ip", "127.0.0.1")
	first.(*net.IPAddr).IP[15] = 2
	if second.String() != "127.0.0.1" {
		t.Fatal("IP results share mutable storage")
	}
}

func BenchmarkResolveIPAddr(b *testing.B) {
	for _, address := range []string{"127.0.0.1", "::1", ""} {
		for _, standard := range []bool{true, false} {
			b.Run(fmt.Sprintf("%s/standard=%t", address, standard), func(b *testing.B) {
				resolve := ResolveAddr
				if standard {
					resolve = standardAddr
				}

				b.ReportAllocs()
				for b.Loop() {
					addr, err := resolve("ip", address)
					if err != nil || addr == nil {
						b.Fatal(addr, err)
					}
				}
			})
		}
	}
}

func FuzzResolveIPAddrCompatibility(f *testing.F) {
	for _, address := range []string{"", "127.0.0.1", "::1", "fe80::1%lo0", "::ffff:127.0.0.1"} {
		f.Add(address)
	}

	f.Fuzz(func(t *testing.T, address string) {
		for _, network := range []string{"ip", "ip4", "ip6"} {
			if _, ok, err := literalIP(network, address); !ok && err == nil && address != "" {
				continue
			}

			want, wantErr := standardAddr(network, address)
			got, gotErr := ResolveAddr(network, address)
			if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(gotErr, wantErr) {
				t.Fatalf("%s %q: got (%#v, %v), want (%#v, %v)", network, address, got, gotErr, want, wantErr)
			}
		}
	})
}

func TestResolveAddrAllocationBudget(t *testing.T) {
	for _, network := range []string{"ip", "tcp", "udp"} {
		addresses := []string{"", "127.0.0.1", "::1"}
		if network != "ip" {
			addresses = []string{"", ":0", "127.0.0.1:80", "[::1]:80", "127.0.0.1:"}
		}

		for _, address := range addresses {
			t.Run(network+"/"+address, func(t *testing.T) {
				allocs := testing.AllocsPerRun(100, func() {
					addr, err := ResolveAddr(network, address)
					if err != nil || addr == nil {
						t.Fatal(addr, err)
					}
				})
				if allocs > 1 {
					t.Fatalf("got %v allocations; want at most 1", allocs)
				}
			})
		}
	}
}
