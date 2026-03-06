//go:build fast_intern_nogc

// Package unique provides high-throughput string interning backed by
// a lock-free concurrent map (xsync.Map) without finalizer-based
// cleanup. Interned strings live for the lifetime of the process.
// This is appropriate for workloads where the interned set is bounded
// (e.g., metric label names/values in a metric system where the same
// labels and values are reused constantly).
//
// Key properties:
//   - Reads are obstruction-free (xsync CLHT).
//   - Handle is a plain string -- no pointer indirection, no fValue wrapper.
//   - No finalizers, no resurrection flags, no GC coordination.
package unique

import (
	"strings"

	"github.com/puzpuzpuz/xsync/v3"
)

// pool is the shared intern table. Entries are never removed.
var pool = xsync.NewMapOf[string, string]()

// Handle is a reference to a canonically interned string.
// It wraps the canonical string directly -- no pointer chase.
type Handle struct {
	s string
}

// Value returns the interned string.
func (h Handle) Value() string {
	return h.s
}

// Make returns a Handle to the canonical interned copy of s.
//
// Safe to call with unsafe strings (e.g., from unsafe.String over a
// protobuf byte buffer). On cache hit, the input is never retained.
// On miss, a proper copy is made before storing.
//
// On hit: zero allocations.
// On miss: one string copy.
func Make(s string) Handle {
	if v, ok := pool.Load(s); ok {
		return Handle{s: v}
	}

	owned := strings.Clone(s)
	v, _ := pool.LoadOrCompute(owned, func() string {
		return owned
	})
	return Handle{s: v}
}

// Size returns the number of entries in the intern pool.
func Size() int {
	return pool.Size()
}

// Clear removes all entries from the intern pool. Intended for testing.
func Clear() {
	pool.Range(func(k string, v string) bool {
		pool.Delete(k)
		return true
	})
}
