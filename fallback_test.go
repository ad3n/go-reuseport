package reuseport

import (
	"fmt"
	"net"
	"reflect"
	"testing"
)

func BenchmarkResolveFallback(b *testing.B) {
	for _, tc := range []struct{ network, address string }{
		{"tcp", "localhost:80"}, {"tcp", "localhost:http"}, {"ip", "localhost"}, {"tcp", "127.0.0.1:http"},
		{"tcp", "missing-port"}, {"tcp", "[::1]"}, {"tcp", "[::]:80"},
		{"tcp4", "[::1]:80"}, {"tcp6", "127.0.0.1:80"}, {"tcp4", "[::%lo0]:80"}, {"tcp", "127.0.0.1:65536"}, {"tcp", "0.0.0.0:80"},
	} {
		for _, standard := range []bool{true, false} {
			b.Run(fmt.Sprintf("%s/%s/standard=%t", tc.network, tc.address, standard), func(b *testing.B) {
				resolve := ResolveAddr
				if standard {
					resolve = standardAddr
				}
				b.ReportAllocs()
				for b.Loop() {
					_, _ = resolve(tc.network, tc.address)
				}
			})
		}
	}
}

func TestResolveFallbackAllocationParity(t *testing.T) {
	for _, address := range []string{"missing-port", "[::1]", "[fe80::1%a[b]:80", "127.0.0.1:65536", "[::]:80", "[::1]:80", "0.0.0.0:80"} {
		for _, network := range []string{"tcp", "tcp4", "tcp6", "udp", "udp4", "udp6"} {
			want := testing.AllocsPerRun(100, func() { _, _ = standardAddr(network, address) })
			got := testing.AllocsPerRun(100, func() { _, _ = ResolveAddr(network, address) })
			if got > want {
				t.Fatalf("%s %q allocated %v objects; standard resolver allocated %v", network, address, got, want)
			}
		}
	}
}

func FuzzResolveMalformedAddressCompatibility(f *testing.F) {
	for _, address := range []string{"missing-port", "[::1]", "[fe80::1%a[b]:80"} {
		f.Add(address)
	}

	f.Fuzz(func(t *testing.T, address string) {
		if _, _, err := net.SplitHostPort(address); err == nil || address == "" {
			return
		}

		for _, network := range []string{"tcp", "tcp4", "tcp6", "udp", "udp4", "udp6"} {
			want, wantErr := standardAddr(network, address)
			got, gotErr := ResolveAddr(network, address)
			if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(gotErr, wantErr) {
				t.Fatalf("%s %q: got (%#v, %v), want (%#v, %v)", network, address, got, gotErr, want, wantErr)
			}
		}
	})
}

func TestResolvePortCompatibility(t *testing.T) {
	for _, network := range []string{"tcp", "tcp4", "tcp6", "udp", "udp4", "udp6"} {
		for _, host := range []string{"", "127.0.0.1", "::1", "::", "::%lo0"} {
			for _, port := range []string{"", "+", "-", "-0", "+0", "+80", "-80", "65535", "65536", "65536x", "10737418240x", "00000000000000000000000000000000000080", "999999999999999999999999999999999999999999"} {
				address := net.JoinHostPort(host, port)
				want, wantErr := standardAddr(network, address)
				got, gotErr := ResolveAddr(network, address)
				if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(gotErr, wantErr) {
					t.Fatalf("%s %q: got (%#v, %v), want (%#v, %v)", network, address, got, gotErr, want, wantErr)
				}
			}
		}
	}
}
