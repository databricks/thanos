package vtproto

import (
	"context"
	"sync"
)

type returnWGKey struct{}

// WithReturnWG returns a child context carrying the given WaitGroup.
// Used by the generated POOL_RETURN_REFCOUNT gRPC handler to let the
// application-level handler participate in reference counting.
func WithReturnWG(ctx context.Context, wg *sync.WaitGroup) context.Context {
	return context.WithValue(ctx, returnWGKey{}, wg)
}

// ReturnWGFromContext extracts the pool-return WaitGroup injected by the
// generated POOL_RETURN_REFCOUNT handler, or nil if none is present.
func ReturnWGFromContext(ctx context.Context) *sync.WaitGroup {
	wg, _ := ctx.Value(returnWGKey{}).(*sync.WaitGroup)
	return wg
}
