// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package hintspb

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/types/known/durationpb"

	thanostestutil "github.com/thanos-io/thanos/pkg/testutil"
)

func TestQueryStatsMerge(t *testing.T) {
	// Use reflection for int64 fields so newly added counters are
	// automatically covered without updating this test.
	setExportedInt64Fields := func(v reflect.Value, val int64) {
		for i := 0; i < v.NumField(); i++ {
			f := v.Field(i)
			if !f.CanSet() || f.Kind() != reflect.Int64 {
				continue
			}
			f.SetInt(val)
		}
	}

	s := &QueryStats{}
	setExportedInt64Fields(reflect.Indirect(reflect.ValueOf(s)), 1)
	s.GetAllDuration = &durationpb.Duration{Seconds: 1, Nanos: 1}
	s.MergeDuration = &durationpb.Duration{Seconds: 1, Nanos: 1}

	o := &QueryStats{}
	setExportedInt64Fields(reflect.Indirect(reflect.ValueOf(o)), 100)
	o.GetAllDuration = &durationpb.Duration{Seconds: 100, Nanos: 100}
	o.MergeDuration = &durationpb.Duration{Seconds: 100, Nanos: 100}

	s.Merge(o)

	e := &QueryStats{}
	setExportedInt64Fields(reflect.Indirect(reflect.ValueOf(e)), 101)
	e.GetAllDuration = &durationpb.Duration{Seconds: 101, Nanos: 101}
	e.MergeDuration = &durationpb.Duration{Seconds: 101, Nanos: 101}

	thanostestutil.ProtoEquals(t, e, s)
}
