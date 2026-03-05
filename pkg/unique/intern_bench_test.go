package unique

import (
	"fmt"
	"strconv"
	"testing"
	stdunique "unique"
	"unsafe"
)

// Strategies under test.
type strategy struct {
	name   string
	intern func(string) string
	setup  func()
}

func strategies() []strategy {
	return []strategy{
		{
			name: "xsync_finalizer",
			intern: func(s string) string {
				return Make(s).Value()
			},
			setup: Clear,
		},
		{
			name: "stdlib_unique",
			intern: func(s string) string {
				return stdunique.Make(s).Value()
			},
			setup: func() {},
		},
	}
}

// generateKeys builds n distinct byte-buffer-backed strings via unsafe.String,
// simulating the protobuf unmarshal code path where strings reference a
// reusable decode buffer.
func generateKeys(n int) []string {
	keys := make([]string, n)
	for i := range keys {
		buf := fmt.Appendf(nil, "label_name_%06d", i)
		keys[i] = unsafe.String(unsafe.SliceData(buf), len(buf))
	}
	return keys
}

// BenchmarkIntern_Serial measures single-goroutine throughput at different
// hit rates. Each b.N iteration performs a batch of 10,000 intern calls
// matching the target ratio.
func BenchmarkIntern_Serial(b *testing.B) {
	keys := generateKeys(10000)

	type hitRate struct {
		name        string
		missPerTenK int // misses per 10,000 iterations
	}
	rates := []hitRate{
		{"hit99.99", 1},
		{"hit99", 100},
		{"hit50", 5000},
	}

	for _, s := range strategies() {
		b.Run(s.name, func(b *testing.B) {
			for _, rate := range rates {
				b.Run(rate.name, func(b *testing.B) {
					s.setup()
					for _, k := range keys {
						s.intern(k)
					}
					b.ResetTimer()
					b.ReportAllocs()
					j := 0
					for i := 0; i < b.N; i++ {
						for k := range 10000 {
							if k < rate.missPerTenK {
								s.intern("miss/" + strconv.Itoa(j))
								j++
							} else {
								s.intern(keys[k])
							}
						}
					}
				})
			}
		})
	}
}

// BenchmarkIntern_Concurrent measures parallel throughput under contention
// at different hit rates. Each pb.Next() iteration performs a batch of 10,000
// intern calls matching the target ratio.
func BenchmarkIntern_Concurrent(b *testing.B) {
	keys := generateKeys(10000)

	type hitRate struct {
		name        string
		missPerTenK int
	}
	rates := []hitRate{
		{"hit99.99", 1},
		{"hit99", 100},
		{"hit50", 5000},
	}

	for _, s := range strategies() {
		b.Run(s.name, func(b *testing.B) {
			for _, rate := range rates {
				b.Run(rate.name, func(b *testing.B) {
					s.setup()
					for _, k := range keys {
						s.intern(k)
					}
					b.ResetTimer()
					b.ReportAllocs()
					b.RunParallel(func(pb *testing.PB) {
						j := 0
						for pb.Next() {
							for i := range 10000 {
								if i < rate.missPerTenK {
									s.intern("miss/" + strconv.Itoa(j))
									j++
								} else {
									s.intern(keys[i])
								}
							}
						}
					})
				})
			}
		})
	}
}
