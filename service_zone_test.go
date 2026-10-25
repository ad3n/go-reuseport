package reuseport

import (
	"fmt"
	"net"
	"reflect"
	"testing"
)

func TestUnknownServiceFreshZoneAllocations(t *testing.T) {
	const service = "codex-no-such-service"
	for _, network := range []string{"tcp", "tcp4", "tcp6", "udp", "udp4", "udp6"} {
		t.Run(network, func(t *testing.T) {
			if _, err := net.LookupPort(network, service); err == nil {
				t.Skip("test service is available on this machine")
			}

			prefix := t.TempDir()
			addresses := make([]string, 21)
			for i := range addresses {
				addresses[i] = net.JoinHostPort(fmt.Sprintf("fe80::1%%%s-%d", prefix, i), service)
			}

			i := 0
			baseline := testing.AllocsPerRun(20, func() { _, _ = standardAddr(network, addresses[i]); i++ })
			i = 0
			optimized := testing.AllocsPerRun(20, func() { _, _ = ResolveAddr(network, addresses[i]); i++ })
			if optimized > baseline {
				t.Fatalf("fresh zones allocated %v objects, standard resolver %v", optimized, baseline)
			}

			for _, address := range addresses {
				want, wantErr := standardAddr(network, address)
				got, gotErr := ResolveAddr(network, address)
				if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(gotErr, wantErr) {
					t.Fatalf("%q: got (%#v,%v), want (%#v,%v)", address, got, gotErr, want, wantErr)
				}
			}
		})
	}
}

func TestServiceZoneCompatibility(t *testing.T) {
	for _, network := range []string{"tcp", "tcp4", "tcp6", "udp", "udp4", "udp6"} {
		for _, host := range []string{"fe80::1%lo0", "fe80::1%a%b:c", "fe80::1%", "::%lo0", "::ffff:127.0.0.1%lo0", "127.0.0.1%lo0", "127.0.0.1%a:b", "::gg%lo0"} {
			for _, service := range []string{"http", "domain", "codex-no-such-service"} {
				address := net.JoinHostPort(host, service)
				want, wantErr := standardAddr(network, address)
				got, gotErr := ResolveAddr(network, address)
				if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(gotErr, wantErr) {
					t.Fatalf("%s %q: got (%#v,%v), want (%#v,%v)", network, address, got, gotErr, want, wantErr)
				}
			}
		}
	}
}

func BenchmarkServiceFreshZone(b *testing.B) {
	for _, standard := range []bool{true, false} {
		b.Run(fmt.Sprintf("standard=%t", standard), func(b *testing.B) {
			resolve := ResolveAddr
			if standard {
				resolve = standardAddr
			}

			prefix := b.TempDir()
			b.ReportAllocs()
			b.ResetTimer()
			for done := 0; done < b.N; {
				b.StopTimer()
				count := min(256, b.N-done)
				addresses := make([]string, count)
				for i := range addresses {
					addresses[i] = net.JoinHostPort(fmt.Sprintf("fe80::1%%%s-%d", prefix, done+i), "codex-no-such-service")
				}
				b.StartTimer()
				for _, address := range addresses {
					_, _ = resolve("tcp", address)
				}
				done += count
			}
		})
	}
}
