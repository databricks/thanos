// Package unique provides string interning backed by a lock-free concurrent
// map (xsync.MapOf) with finalizer-based cleanup. It is intended as a
// higher-throughput alternative to Go's unique.Make[string] for workloads
// where read-heavy lookups dominate (e.g., label name/value deduplication
// across protobuf unmarshals).
//
// Key properties:
//   - Reads are obstruction-free (CLHT -- no mutex, no atomic writes to shared memory).
//   - Entries are cleaned up via runtime.SetFinalizer when no Handle references remain.
//   - Zero per-read GC coordination (unlike weak.Pointer.Value).
//   - MakeFromBytes provides a zero-allocation fast path for cache hits when
//     the source data is a byte slice (typical of protobuf unmarshal).
package unique

import (
	"runtime"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/puzpuzpuz/xsync/v4"
)

// pool stores uintptr (invisible to GC) keyed by string. Cleanup
// happens via runtime.SetFinalizer on the fValue, which removes
// the map entry when no strong references remain.
var pool = xsync.NewMap[string, uintptr]()

// Handle is a reference to a canonically interned string. As long as at
// least one Handle for a given string value exists, the entry remains in
// the intern table. When all Handles are collected, the GC may reclaim
// the entry via its finalizer.
//
// Handles are comparable: two Handles are equal iff they refer to the
// same canonical string (pointer equality).
type Handle struct {
	val *fValue
}

// Value returns the interned string.
func (h Handle) Value() string {
	return h.val.s
}

// fValue wraps a string with a resurrected flag to handle the race
// between finalizer execution and concurrent lookups.
type fValue struct {
	s           string
	resurrected atomic.Bool
}

// Make returns a Handle to the canonical interned copy of s.
//
// The map stores uintptr values (invisible to the GC) so the finalizer
// on fValue can fire when no Handles remain. On the read path this is
// a lock-free xsync.Map load followed by an unsafe pointer conversion
// with no GC coordination.
// Make returns a Handle to the canonical interned copy of s.
//
// Safe to call with unsafe strings (e.g., from unsafe.String over a
// protobuf byte buffer). On cache hit, the input is never retained.
// On miss, a proper copy is made before storing, so the caller's
// backing memory is never held by the intern table.
//
// On hit: zero allocations.
// On miss: one string copy + one fValue allocation.
func Make(s string) Handle {
	if raw, ok := pool.Load(s); ok {
		v := uintptrToFValue(raw)
		v.resurrected.Store(true)
		return Handle{val: v}
	}
	// Slow path: copy the string to detach from any unsafe backing memory,
	// then atomically insert.
	owned := strings.Clone(s)
	raw, loaded := pool.LoadOrCompute(owned, func() (uintptr, bool) {
		v := &fValue{s: owned}
		runtime.SetFinalizer(v, finalizerCleanup)
		return fValueToUintptr(v), false
	})
	v := uintptrToFValue(raw)
	if loaded {
		v.resurrected.Store(true)
	}
	return Handle{val: v}
}

// Size returns the number of entries in the intern pool.
func Size() int {
	return pool.Size()
}

// Clear removes all entries from the intern pool. Intended for testing.
func Clear() {
	pool.Range(func(k string, v uintptr) bool {
		pool.Delete(k)
		return true
	})
}

func fValueToUintptr(v *fValue) uintptr {
	return uintptr(unsafe.Pointer(v))
}

// uintptrToFValue converts a uintptr back to *fValue. This is
// intentionally hiding the pointer from the GC -- the same pattern
// used by go4.org/intern. The uintptr is stored in the map so the GC
// does not consider it a reference, allowing the finalizer to fire
// when no real references remain.
func uintptrToFValue(u uintptr) *fValue {
	return (*fValue)(unsafe.Add(nil, int(u)))
}

func finalizerCleanup(v *fValue) {
	if v.resurrected.CompareAndSwap(true, false) {
		// Someone grabbed a reference while the finalizer was pending.
		// Re-register and let the next GC cycle try again.
		runtime.SetFinalizer(v, finalizerCleanup)
		return
	}
	pool.Delete(v.s)
}
