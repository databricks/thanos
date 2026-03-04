package unique

import (
	"fmt"
	"strconv"
	"testing"
	stdunique "unique"
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

// generateKeys builds n distinct strings that look like label name/value pairs.
func generateKeys(n int) []string {
	keys := make([]string, n)
	for i := range keys {
		keys[i] = fmt.Sprintf("label_name_%06d", i)
	}
	return keys
}

// BenchmarkIntern_Serial measures single-goroutine throughput at different
// hit rates. Each b.N iteration performs a batch of 100 intern calls
// matching the target ratio.
func BenchmarkIntern_Serial(b *testing.B) {
	keys := generateKeys(100)

	type hitRate struct {
		name    string
		missPct int
	}
	rates := []hitRate{
		{"hit99", 1},
		{"hit50", 50},
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
						for k := range 100 {
							if k < rate.missPct {
								s.intern("miss/" + strconv.Itoa(j))
								j++
							} else {
								s.intern(keys[k%len(keys)])
							}
						}
					}
				})
			}
		})
	}
}

// BenchmarkIntern_Concurrent measures parallel throughput under contention
// at different hit rates. Each pb.Next() iteration performs a batch of 100
// intern calls matching the target ratio.
//
// Results (32 cores):
//
//	xsync_finalizer/hit99-32     13965890    423.3 ns/op     24 B/op    2 allocs/op
//	xsync_finalizer/hit50-32      1724382     3471 ns/op   1369 B/op  109 allocs/op
//	stdlib_unique/hit99-32        16620832    541.1 ns/op     65 B/op    3 allocs/op
//	stdlib_unique/hit50-32         1066116     5584 ns/op   1617 B/op  119 allocs/op
func BenchmarkIntern_Concurrent(b *testing.B) {
	keys := generateKeys(100)

	type hitRate struct {
		name    string
		missPct int
	}
	rates := []hitRate{
		{"hit99", 1},
		{"hit50", 50},
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
							for i := range 100 {
								if i < rate.missPct {
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
