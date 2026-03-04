// Package vtproto registers the vtprotobuf gRPC codec so that all gRPC
// marshal/unmarshal operations use the generated MarshalVT/UnmarshalVT
// methods when available, falling back to standard proto for messages
// that don't implement them.
//
// Import this package with a blank identifier to activate the codec:
//
//	import _ "github.com/thanos-io/thanos/pkg/vtproto"
package vtproto

import (
	"fmt"

	"google.golang.org/grpc/encoding"
	_ "google.golang.org/grpc/encoding/proto"
	"google.golang.org/grpc/mem"
)

const codecName = "proto"

type vtMarshal interface {
	SizeVT() int
	MarshalToSizedBufferVT([]byte) (int, error)
}

type vtUnmarshal interface {
	UnmarshalVT([]byte) error
}

// codecV2 implements encoding.CodecV2 using VT marshal/unmarshal with
// pooled buffers from gRPC's mem.DefaultBufferPool. This avoids the
// per-call allocations of the V1 codec bridge which calls MarshalVT()
// (allocates a new []byte every time) and Materialize() (copies
// incoming data into a fresh contiguous slice).
type codecV2 struct{}

func (codecV2) Marshal(v any) (mem.BufferSlice, error) {
	vt, ok := v.(vtMarshal)
	if !ok {
		return nil, fmt.Errorf("vtproto codec: failed to marshal, message is %T (missing MarshalToSizedBufferVT)", v)
	}

	size := vt.SizeVT()
	if mem.IsBelowBufferPoolingThreshold(size) {
		buf := make([]byte, size)
		n, err := vt.MarshalToSizedBufferVT(buf)
		if err != nil {
			return nil, err
		}
		return mem.BufferSlice{mem.SliceBuffer(buf[:n])}, nil
	}

	pool := mem.DefaultBufferPool()
	buf := pool.Get(size)
	n, err := vt.MarshalToSizedBufferVT((*buf)[:size])
	if err != nil {
		pool.Put(buf)
		return nil, err
	}
	*buf = (*buf)[:n]
	return mem.BufferSlice{mem.NewBuffer(buf, pool)}, nil
}

func (codecV2) Unmarshal(data mem.BufferSlice, v any) error {
	vt, ok := v.(vtUnmarshal)
	if !ok {
		return fmt.Errorf("vtproto codec: failed to unmarshal, message is %T (missing UnmarshalVT)", v)
	}

	// MaterializeToBuffer avoids a copy when data is already a single
	// contiguous buffer (common case for unary RPCs). When it does need
	// to merge chunks, it pulls from gRPC's buffer pool.
	buf := data.MaterializeToBuffer(mem.DefaultBufferPool())
	defer buf.Free()
	return vt.UnmarshalVT(buf.ReadOnlyData())
}

func (codecV2) Name() string {
	return codecName
}

func init() {
	// RegisterCodecV2 overwrites any prior V1 registration with the same
	// name ("proto"). gRPC's getCodec checks V1 first, but since
	// RegisterCodecV2 stores into the same map, the old V1 entry is
	// replaced and GetCodec returns nil, causing the fallthrough to
	// GetCodecV2 which finds our V2 codec.
	encoding.RegisterCodecV2(&codecV2{})
}
