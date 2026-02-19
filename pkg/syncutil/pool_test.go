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
	p := NewPool(func() *bytes.Buffer { return new(bytes.Buffer) }).
		WithReset(func(b *bytes.Buffer) bool { b.Reset(); return true }).
		Build()

	buf, ret := p.Get()
	buf.WriteString("data")
	ret(buf)

	buf2, ret2 := p.Get()
	defer ret2(buf2)

	// After reset the buffer should be empty (assuming same object was reused,
	// which is not guaranteed by sync.Pool but is the common case in tests).
	if buf == buf2 {
		testutil.Equals(t, 0, buf2.Len())
	}
}

type tracker struct {
	value int
}

func TestPool_DeferPattern(t *testing.T) {
	p := NewPool(func() *tracker { return &tracker{} }).
		WithReset(func(tr *tracker) bool { tr.value = 0; return true }).
		Build()

	func() {
		obj, ret := p.Get()
		defer ret(obj)
		obj.value = 42
		testutil.Equals(t, 42, obj.value)
	}()
}
