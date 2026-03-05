# pkg/unique

String interning backed by [xsync.Map](https://github.com/puzpuzpuz/xsync)
built as a higher-throughput alternative to Go's `unique.Make[string]`.

## Why not `unique.Make`?

Go's `unique.Make[string]` uses a sharded mutex-based map internally.
Under concurrent workloads typical of the receive path (many goroutines
unmarshalling protobuf labels in parallel), the mutex contention becomes
a bottleneck -- particularly on the write (miss) path.

This package uses xsync.Map, a [Cache-Line Hash Table](https://github.com/LPD-EPFL/CLHT)
where read operations are obstruction-free with no writes to shared memory.
This is the same concurrent map design used by
[puzpuzpuz/xsync](https://github.com/puzpuzpuz/xsync). Cleanup of unused
entries follows the `uintptr` + `runtime.SetFinalizer` pattern from
[go4.org/intern](https://pkg.go.dev/go4.org/intern), where pointers are
hidden from the GC so that finalizers fire when no live `Handle` references
remain.

## Build variants

There are two implementations, selected at compile time via build tag:

### Default (finalizer-based cleanup)

Entries are automatically reclaimed when no `Handle` references remain.
This is the safe default for general use.

### `fast_intern_nogc` (no cleanup)

Build with `-tags fast_intern_nogc` to use a simpler implementation where
interned strings are never freed. The `Handle` type is a plain string with
no pointer indirection, making the hot-path lookup cheaper.

This variant will leak memory for strings that are interned and then never
seen again. In the context of a metric system where the same labels and
values are reused constantly, the interned set reaches a bounded steady
state and this is acceptable. Do not use this variant if your interned
key space is unbounded.

**How bad is the leak in practice?** Interning happens during protobuf
unmarshal of metric `Label` name/value pairs -- things like `__name__`,
`job`, `instance`, `pod`. The highest-churn values are instance and pod
identifiers (~50 bytes each). Trace IDs, request IDs, etc. are not a
concern -- they would only appear if someone explicitly added them as
metric labels, which is a cardinality anti-pattern that would break TSDB
long before the intern pool matters.

Assuming a large system with 10,000 instances turning over daily, each
producing ~5 unique churning label values at ~66 bytes per interned entry
(50 bytes content + 16 bytes Go string header):

```
Daily leak:    10,000 × 5 × 66 bytes  ≈   3 MB/day
Weekly leak:                           ≈  23 MB
Monthly leak:                          ≈ 100 MB
```

For a receiver process typically using 4-16 GB, this is 1-2% of working
memory per month -- and redeployed well before it accumulates.

```
go build -tags fast_intern_nogc ./...
go test  -tags fast_intern_nogc ./pkg/unique/
```

## API

- `Make(s string) Handle` -- intern a string, return a Handle
- `Handle.Value() string` -- get the interned string

## Benchmarks

Concurrent throughput (32 cores), 10,000 known keys (unsafe.String-backed,
simulating protobuf unmarshal), batch of 10,000 lookups per iteration:

### Default (finalizer) vs stdlib

```
BenchmarkIntern_Concurrent/xsync_finalizer/hit99.99-32     172182     27430 ns/op       39 B/op      2 allocs/op
BenchmarkIntern_Concurrent/xsync_finalizer/hit99-32         87534     50415 ns/op     2704 B/op    225 allocs/op
BenchmarkIntern_Concurrent/xsync_finalizer/hit50-32          5779    575061 ns/op   150252 B/op  11740 allocs/op
BenchmarkIntern_Concurrent/stdlib_unique/hit99.99-32         90100     41133 ns/op       22 B/op      2 allocs/op
BenchmarkIntern_Concurrent/stdlib_unique/hit99-32            50424     79038 ns/op     2758 B/op    232 allocs/op
BenchmarkIntern_Concurrent/stdlib_unique/hit50-32             1602   1918517 ns/op   155432 B/op  12202 allocs/op
```

### `fast_intern_nogc` vs stdlib

```
BenchmarkIntern_Concurrent/xsync_nogc/hit99.99-32          235810     16473 ns/op       19 B/op      2 allocs/op
BenchmarkIntern_Concurrent/xsync_nogc/hit99-32             188784     23952 ns/op     2610 B/op    206 allocs/op
BenchmarkIntern_Concurrent/xsync_nogc/hit50-32              22046    154803 ns/op   142594 B/op  10906 allocs/op
BenchmarkIntern_Concurrent/stdlib_unique/hit99.99-32         90100     41133 ns/op       22 B/op      2 allocs/op
BenchmarkIntern_Concurrent/stdlib_unique/hit99-32            50424     79038 ns/op     2758 B/op    232 allocs/op
BenchmarkIntern_Concurrent/stdlib_unique/hit50-32             1602   1918517 ns/op   155432 B/op  12202 allocs/op
```
