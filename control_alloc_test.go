//go:build !plan9 && !windows && !wasm && !freebsd

package reuseport

import "testing"

type benchmarkRawConn struct{}

func (benchmarkRawConn) Control(f func(uintptr)) error {
	f(^uintptr(0)) // Invalid descriptor: exercise callback state without a socket.
	return nil
}

func (benchmarkRawConn) Read(func(uintptr) bool) error  { return nil }
func (benchmarkRawConn) Write(func(uintptr) bool) error { return nil }

func BenchmarkControl(b *testing.B) {
	raw := benchmarkRawConn{}
	_ = Control("tcp", "", raw) // Warm the pool before measuring.
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = Control("tcp", "", raw)
	}
}

func TestControlSteadyStateZeroAlloc(t *testing.T) {
	raw := benchmarkRawConn{}
	_ = Control("tcp", "", raw)
	allocs := testing.AllocsPerRun(100, func() {
		_ = Control("tcp", "", raw)
	})
	if allocs != 0 {
		t.Fatalf("Control allocated %v objects per call; want zero", allocs)
	}
}
