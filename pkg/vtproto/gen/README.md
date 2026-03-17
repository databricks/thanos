# protoc-gen-go-grpc-vtpool

A `protoc` plugin that generates pool-aware gRPC server handlers. For messages with vtprotobuf's `mempool` option, the standard `protoc-gen-go-grpc` handlers allocate request objects via `new(Type)` on every call. This plugin generates replacement handlers that acquire objects from the VT pool instead, and patches the `ServiceDesc` at `init()` to swap them in. The original `_grpc.pb.go` is not modified.

## Configuration

Messages opt in via two proto options -- both are required:

```protobuf
import "github.com/planetscale/vtprotobuf/vtproto/ext.proto";
import "vtpool.proto";

message WriteRequest {
  option (vtproto.mempool) = true;                   // enables vtprotobuf pool generation
  option (vtpool.pool_return) = POOL_RETURN_DEFER;   // controls handler return-to-pool behavior
  ...
}
```

Messages without an explicit `pool_return` option are left untouched.

### Pool return modes

| Mode     | Proto Value            | Behavior                                                                                                                            |
| -------- | ---------------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| Defer    | `POOL_RETURN_DEFER`    | `defer in.ReturnToVTPool()` immediately after acquisition. Safe default -- no leaks.                                                |
| Caller   | `POOL_RETURN_CALLER`   | No defer. The RPC handler takes ownership and must return the object to the pool itself. Returns to pool only on decode error.      |
| Refcount | `POOL_RETURN_REFCOUNT` | Reference-counted return. A `sync.WaitGroup` is injected into the context; pool return is deferred until all holders call `Done()`. |

Use **Defer** for most RPCs. Use **Caller** when the handler directly passes ownership and manages the lifecycle itself. Use **Refcount** when the handler fans work out to asynchronous goroutines that outlive the handler call and need the request to remain valid.

### POOL_RETURN_REFCOUNT details

The generated handler for `POOL_RETURN_REFCOUNT`:

1. Acquires the request from the pool.
2. Creates a `sync.WaitGroup`, calls `wg.Add(1)`.
3. Spawns a goroutine that waits on the WG and then returns the object to the pool.
4. Defers `wg.Done()` (the handler's own reference).
5. Injects the WG into the context via `vtproto.WithReturnWG(ctx, wg)`.

The application-level handler extracts the WG with `vtproto.ReturnWGFromContext(ctx)` and passes it through the pipeline. Any goroutine that holds a reference to the request data must call `wg.Add(1)` before the spawning function returns, and `wg.Done()` when it is finished.

**Safety requirements:**

- Every `wg.Add(1)` must have a matching `wg.Done()`. A missing `Done()` leaks the pooled object; a missing `Add()` causes a use-after-return race.
- `wg.Add(1)` must happen synchronously (before the parent function returns), not inside the spawned goroutine. Otherwise the pool-return goroutine may fire before the `Add` executes.
- The WG may be `nil` when the handler is called from a non-gRPC path (e.g. HTTP). Callers must guard against nil before calling `Add`/`Done`.
- The request and its sub-messages (e.g. `TimeSeries`) must not be mutated after the handler returns, since the pool-return goroutine will call `ResetVT()` on them.

## Generated output

For `rpc RemoteWrite(WriteRequest) returns (WriteResponse)` with `POOL_RETURN_DEFER`:

```go
func init() {
    for i, m := range WriteableStore_ServiceDesc.Methods {
        switch m.MethodName {
        case "RemoteWrite":
            WriteableStore_ServiceDesc.Methods[i].Handler = _WriteableStore_RemoteWrite_VTPoolHandler
        }
    }
}

func _WriteableStore_RemoteWrite_VTPoolHandler(srv interface{}, ctx context.Context,
    dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
    in := WriteRequestFromVTPool()
    defer in.ReturnToVTPool()
    if err := dec(in); err != nil { return nil, err }
    if interceptor == nil { return srv.(WriteableStoreServer).RemoteWrite(ctx, in) }
    ...
}
```

## Invocation

The plugin is built from source and wired into `scripts/genproto.sh`, so `make proto` handles everything. To invoke manually:

```bash
protoc \
  --go-grpc-vtpool_out=. \
  --go-grpc-vtpool_opt=paths=source_relative \
  -I="${VTPOOL_PROTO_DIR}" \
  ...
```

`VTPOOL_PROTO_DIR` must point to the directory containing `vtpool.proto` (`pkg/vtproto/gen/`).

## Testing

```bash
go test ./pkg/vtproto/gen/test/
```
