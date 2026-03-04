// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package testutil

import (
	"testing"

	"github.com/thanos-io/thanos/pkg/store/labelpb"
)

func TestProtoEquals_SingleMessage(t *testing.T) {
	a := &labelpb.Label{Name: "foo", Value: "bar"}
	b := &labelpb.Label{Name: "foo", Value: "bar"}
	ProtoEquals(t, a, b)
}

func TestProtoEquals_IgnoresInternalFields(t *testing.T) {
	a := &labelpb.Label{Name: "foo", Value: "bar"}
	// Force internal proto state to diverge by serialising and deserialising.
	data, err := a.MarshalVT()
	if err != nil {
		t.Fatal(err)
	}
	b := &labelpb.Label{}
	if err := b.UnmarshalVT(data); err != nil {
		t.Fatal(err)
	}
	ProtoEquals(t, a, b)
}

func TestProtoEquals_SliceOfPointers(t *testing.T) {
	a := []*labelpb.Label{
		{Name: "a", Value: "1"},
		{Name: "b", Value: "2"},
	}
	b := []*labelpb.Label{
		{Name: "a", Value: "1"},
		{Name: "b", Value: "2"},
	}
	ProtoEquals(t, a, b)
}

func TestProtoEquals_NestedProtoInSlice(t *testing.T) {
	a := []*labelpb.LabelSet{
		{Labels: labelpb.FromStrings("a", "1", "b", "2")},
		{Labels: labelpb.FromStrings("c", "3")},
	}
	b := []*labelpb.LabelSet{
		{Labels: labelpb.FromStrings("a", "1", "b", "2")},
		{Labels: labelpb.FromStrings("c", "3")},
	}
	ProtoEquals(t, a, b)
}

func TestProtoEquals_SliceOrderMatters(t *testing.T) {
	a := []*labelpb.Label{
		{Name: "a", Value: "1"},
		{Name: "b", Value: "2"},
	}
	reversed := []*labelpb.Label{
		{Name: "b", Value: "2"},
		{Name: "a", Value: "1"},
	}

	mock := &mockTB{}
	ProtoEquals(mock, a, reversed)
	if !mock.failed {
		t.Fatal("expected ProtoEquals to fail for differently-ordered slices")
	}
}

func TestProtoEquals_DifferentValues(t *testing.T) {
	a := &labelpb.Label{Name: "foo", Value: "bar"}
	b := &labelpb.Label{Name: "foo", Value: "baz"}

	mock := &mockTB{}
	ProtoEquals(mock, a, b)
	if !mock.failed {
		t.Fatal("expected ProtoEquals to fail for different values")
	}
}

func TestProtoEquals_DifferentLengthSlices(t *testing.T) {
	a := []*labelpb.Label{
		{Name: "a", Value: "1"},
		{Name: "b", Value: "2"},
	}
	b := []*labelpb.Label{
		{Name: "a", Value: "1"},
	}

	mock := &mockTB{}
	ProtoEquals(mock, a, b)
	if !mock.failed {
		t.Fatal("expected ProtoEquals to fail for different-length slices")
	}
}

func TestProtoEquals_NilVsEmptySlice(t *testing.T) {
	var a []*labelpb.Label
	b := []*labelpb.Label{}
	ProtoEquals(t, a, b)
}

func TestProtoEquals_NilVsEmptyMap(t *testing.T) {
	var a map[string]*labelpb.Label
	b := map[string]*labelpb.Label{}
	ProtoEquals(t, a, b)
}

func TestProtoEquals_MsgAndArgs(t *testing.T) {
	a := &labelpb.Label{Name: "x", Value: "1"}
	b := &labelpb.Label{Name: "x", Value: "2"}

	mock := &mockTB{}
	ProtoEquals(mock, a, b, "custom message")
	if !mock.failed {
		t.Fatal("expected failure")
	}
	if mock.msg == "" {
		t.Fatal("expected non-empty failure message")
	}
}

// mockTB captures Fatalf calls without killing the test process.
type mockTB struct {
	testing.TB
	failed bool
	msg    string
}

func (m *mockTB) Helper() {}

func (m *mockTB) Fatalf(format string, args ...interface{}) {
	m.failed = true
	m.msg = format
}
