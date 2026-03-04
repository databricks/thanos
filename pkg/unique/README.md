# pkg/unique

String interning backed by [xsync.Map](https://github.com/puzpuzpuz/xsync)
with finalizer-based cleanup, built as a higher-throughput alternative to
Go's `unique.Make[string]`.

## Why not `unique.Make`?

Go's `unique.Make[string]` uses a sharded mutex-based map internally.
Under concurrent workloads typical of the receive path (many goroutines
unmarshalling protobuf labels in parallel), the mutex contention becomes
a bottleneck -- particularly on the write (miss) path.

This package uses xsync.Map (Cache-Line Hash Table) where read operations
are obstruction-free with no writes to shared memory. At steady-state hit
rates the read path is on par with `unique.Make`, while the write path
under contention is significantly faster.

Entries are cleaned up via `runtime.SetFinalizer` when no `Handle`
references remain, following the same pattern as `go4.org/intern`. This
avoids the per-access GC coordination cost of `weak.Pointer.Value()`.

## API

- `Make(s string) Handle` -- intern a string, return a Handle
- `MakeFromBytes(b []byte) Handle` -- intern from a byte slice with
  zero-allocation on cache hit (intended for protobuf unmarshal paths)
- `Handle.Value() string` -- get the interned string

## Benchmarks

Concurrent throughput (32 cores), 100 known keys, batch of 100 lookups
per iteration:

```
BenchmarkIntern_Concurrent/xsync_finalizer/hit99-32     13965890    423.3 ns/op     24 B/op    2 allocs/op
BenchmarkIntern_Concurrent/xsync_finalizer/hit50-32      1724382     3471 ns/op   1369 B/op  109 allocs/op
BenchmarkIntern_Concurrent/stdlib_unique/hit99-32        16620832    541.1 ns/op     65 B/op    3 allocs/op
BenchmarkIntern_Concurrent/stdlib_unique/hit50-32         1066116    5584 ns/op   1617 B/op  119 allocs/op
```

At 99% hit rate (steady state), ~22% faster with fewer allocations.
At 50% hit rate (burst of new series), ~38% faster due to lock-free stores.
