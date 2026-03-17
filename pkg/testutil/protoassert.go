// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package testutil

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
)

// ProtoEquals fails the test if expected and actual are not equal,
// using protocmp.Transform() to properly compare protobuf messages.
// Unlike reflect.DeepEqual, this ignores internal proto fields
// (state, sizeCache, unknownFields) that can cause spurious failures.
// Nil and empty slices/maps are treated as equivalent.
//
// Equality is checked via a tiered fast path before falling back to
// cmp.Diff (which is only needed to produce a human-readable diff on
// mismatch):
//
//  1. EqualMessageVT — vtprotobuf generated, zero reflection.
//  2. proto.Equal    — standard library optimized reflection.
//  3. Slice iteration — element-wise using (1) then (2).
//  4. cmp.Diff + protocmp.Transform — expensive, only on mismatch.
func ProtoEquals(tb testing.TB, expected, actual any, msgAndArgs ...any) {
	ProtoEqualsWithOptions(tb, expected, actual, nil, msgAndArgs...)
}

// vtEqualable is satisfied by all vtprotobuf-generated messages.
type vtEqualable interface {
	EqualMessageVT(proto.Message) bool
}

// fastProtoEqual returns true if expected and actual are provably equal
// using fast proto-aware checks. Returns false if equality cannot be
// confirmed (either they differ, or the types don't support fast checks).
func fastProtoEqual(expected, actual any) bool {
	// Tier 1: vtprotobuf EqualMessageVT (generated code, no reflection).
	if ev, ok := expected.(vtEqualable); ok {
		if av, ok := actual.(proto.Message); ok {
			return ev.EqualMessageVT(av)
		}
	}

	// Tier 2: proto.Equal (optimized reflection, no diff computation).
	if ep, ok := expected.(proto.Message); ok {
		if ap, ok := actual.(proto.Message); ok {
			return proto.Equal(ep, ap)
		}
	}

	// Tier 3: slice — iterate elements and try tiers 1-2 per element.
	ev := reflect.ValueOf(expected)
	av := reflect.ValueOf(actual)
	if ev.Kind() == reflect.Slice && av.Kind() == reflect.Slice {
		if ev.Len() == 0 && av.Len() == 0 {
			return true
		}
		if ev.Len() != av.Len() {
			return false
		}
		for i := 0; i < ev.Len(); i++ {
			if !fastProtoEqual(ev.Index(i).Interface(), av.Index(i).Interface()) {
				return false
			}
		}
		return true
	}

	return false
}

// ProtoEqualsWithOptions is like ProtoEquals but accepts additional cmp.Options.
// Use this when comparing structs with unexported fields, e.g.:
//
//	thanostestutil.ProtoEqualsWithOptions(t, expected, actual, cmpopts.AllowUnexported(myStruct{}))
func ProtoEqualsWithOptions(tb testing.TB, expected, actual any, opts cmp.Options, msgAndArgs ...any) {
	tb.Helper()

	// When no custom options are provided, use the tiered fast path.
	// Custom options (e.g. AllowUnexported) can change equality semantics,
	// so we skip the fast path in that case.
	if len(opts) == 0 && fastProtoEqual(expected, actual) {
		return
	}

	cmpOpts := append(cmp.Options{protocmp.Transform(), cmpopts.EquateEmpty()}, opts...)
	if diff := cmp.Diff(expected, actual, cmpOpts...); diff != "" {
		_, file, line, _ := runtime.Caller(1)
		if len(msgAndArgs) > 0 {
			tb.Fatalf("\033[31m%s:%d: %s\n\nmismatch (-want +got):\n%s\033[39m", filepath.Base(file), line, msgAndArgs[0], diff)
		}
		tb.Fatalf("\033[31m%s:%d:\n\nmismatch (-want +got):\n%s\033[39m", filepath.Base(file), line, diff)
	}
}
