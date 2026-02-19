// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package syncutil

import (
	"bytes"
	"testing"

	"github.com/efficientgo/core/testutil"
)

func TestPool_GetReturn(t *testing.T) {
	p := NewPool(func() *bytes.Buffer {
		return new(bytes.Buffer)
	}).Build()

	buf, ret := p.Get()
	buf.WriteString("hello")
	testutil.Equals(t, "hello", buf.String())
	ret(buf)
}

func TestPool_WithReset(t *testing.T) {
	p := NewPool(func() *bytes.Buffer {
		return &bytes.Buffer{}
	}).WithReset(func(b *bytes.Buffer) bool {
		b.Reset()
		b.WriteString("cleared")
		return true
	}).Build()

	buf, ret := p.Get()
	testutil.Equals(t, "", buf.String())
	buf.WriteString("data")
	ret(buf)
	// !!! usually we would never hold onto a reference
	// after returning it to the pool, but this is for testing.
	testutil.Equals(t, "cleared", buf.String())

	buf, ret = p.Get()
	defer ret(buf)
	testutil.Equals(t, "cleared", buf.String())
}

func TestPool_WithByteSlicePointer(t *testing.T) {
	p := NewPool(func() *[]byte {
		b := make([]byte, 0, 10)
		return &b
	}).WithReset(func(b *[]byte) bool {
		*b = (*b)[:0]
		return true
	}).Build()

	buf, ret := p.Get()
	testutil.Equals(t, 10, cap(*buf))
	testutil.Equals(t, 0, len(*buf))
	*buf = append(*buf, make([]byte, 20)...)
	testutil.Equals(t, 24, cap(*buf))
	testutil.Equals(t, 20, len(*buf))
	ret(buf)
	// !!! usually we would never hold onto a reference
	// after returning it to the pool, but this is for testing.
	testutil.Equals(t, 24, cap(*buf))
	testutil.Equals(t, 0, len(*buf))
	buf, ret = p.Get()
	defer ret(buf)
	testutil.Equals(t, 24, cap(*buf))
	testutil.Equals(t, 0, len(*buf))
	testutil.Equals(t, []byte{}, *buf)
}
