package reuseport

import (
	"fmt"
	"net"
	"reflect"
	"strconv"
	"testing"
)

func TestSignedPortFreshZoneAllocations(t *testing.T) {
	for _, network := range []string{"tcp", "tcp4", "tcp6", "udp", "udp4", "udp6"} {
		for _, port := range []string{"-1", "+65536", "-65536", "-999999999999999999999x", "+999999999999999999999x"} {
			t.Run(network+"/"+port, func(t *testing.T) {
				prefix := t.TempDir()
				addresses := make([]string, 101)
				for i := range addresses {
					addresses[i] = net.JoinHostPort(fmt.Sprintf("fe80::1%%%s-%d", prefix, i), port)
				}

				i := 0
				allocs := testing.AllocsPerRun(100, func() {
					_, _ = ResolveAddr(network, addresses[i])
					i++
				})
				if allocs != 1 {
					t.Fatalf("fresh-zone invalid port allocated %v objects; want 1", allocs)
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
}

func TestSignedPortCompatibility(t *testing.T) {
	for _, network := range []string{"tcp", "tcp4", "tcp6", "udp", "udp4", "udp6"} {
		for _, host := range []string{"", "127.0.0.1", "fe80::1%lo0", ":::", "localhost"} {
			for _, port := range []string{"+", "-", "+0", "-0", "+80", "+65535", "-01", "+65536", "+99999999999999999999x", "-99999999999999999999x", "+1073741824x", "-1073741824x", "+00000000000080", "+8589934592", "+85899345920x", "-85899345920x", "+4294967296", "+42949672960x"} {
				address := net.JoinHostPort(host, port)
				want, wantErr := standardAddr(network, address)
				got, gotErr := ResolveAddr(network, address)
				if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(gotErr, wantErr) {
					t.Fatalf("%s %q: got (%#v,%v), want (%#v,%v)", network, address, got, gotErr, want, wantErr)
				}
			}
		}
	}
}

func BenchmarkSignedPort(b *testing.B) {
	for _, address := range []string{"[fe80::1%warm-zone]:-1", "[2001:db8:1234:5678:9abc:def0:1234:5678]:-1", "[fe80::1%warm-zone]:+65536"} {
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

func FuzzNumericPortClassification(f *testing.F) {
	for _, port := range []string{"", "+", "-", "-1", "+65536", "3com-tsmux", "+1073741824x", "+999999999999999999999x", "+85899345920x", "4294967296", "65535"} {
		f.Add(port)
	}

	f.Fuzz(func(t *testing.T, port string) {
		wantPort, err := net.LookupPort("numeric-classifier-test", port)
		want := true
		if e, ok := err.(*net.AddrError); ok && e.Err == "unknown network" {
			want = false
		}

		gotPort, got := parseNumericPort(port)
		if got != want {
			t.Fatalf("%q: numeric=%t, standard classification=%t (error %v)", port, got, want, err)
		}

		if got && (err == nil && gotPort != wantPort || err != nil && gotPort >= 0) {
			t.Fatalf("%q: parsed port=%d, standard=(%d, %v)", port, gotPort, wantPort, err)
		}
	})
}

func TestNumericPortRangeCompatibility(t *testing.T) {
	for value := 0; value <= 65536; value++ {
		digits := strconv.Itoa(value)
		for _, prefix := range []string{"", "+", "-"} {
			port := prefix + digits
			want, err := net.LookupPort("tcp", port)
			got, numeric := parseNumericPort(port)
			if !numeric || err == nil && got != want || err != nil && got >= 0 {
				t.Fatalf("%q: got (%d,%t), standard=(%d,%v)", port, got, numeric, want, err)
			}
		}
	}
}
