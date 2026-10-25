package reuseport

import (
	"fmt"
	"net"
	"reflect"
	"testing"
)

func standardAddr(network, address string) (net.Addr, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
		return net.ResolveTCPAddr(network, address)
	default:
		return net.ResolveUDPAddr(network, address)
	}
}

func TestResolveAddrCompatibility(t *testing.T) {
	for _, network := range []string{"tcp", "tcp4", "tcp6", "udp", "udp4", "udp6"} {
		for _, address := range []string{
			"", ":0", "127.0.0.1:80", "127.0.0.1:00080", "127.0.0.1:+80",
			"127.0.0.1:", "127.0.0.1:65536", "127.0.0.1:-1", "127.0.0.1:http",
			"[127.0.0.1]:80", "[::1]:80", "[::ffff:127.0.0.1]:80",
			"[fe80::1%lo0]:80", "[fe80::1%a[b]:80", "[fe80::1%a]b]:80",
			"[::ffff:127.0.0.1%lo0]:80", "[::]:80", "0.0.0.0:80",
			"[::ffff:0.0.0.0]:80", "localhost:80", "[::1]", "::1:80",
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
	for _, address := range []string{"127.0.0.1:80", "[::1]:80", ":0"} {
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
