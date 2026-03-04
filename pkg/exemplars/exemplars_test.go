// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package exemplars

import (
	"testing"

	"github.com/thanos-io/thanos/pkg/exemplars/exemplarspb"
	"github.com/thanos-io/thanos/pkg/store/labelpb"
	thanostestutil "github.com/thanos-io/thanos/pkg/testutil"
	"github.com/thanos-io/thanos/pkg/testutil/custom"
)

func TestMain(m *testing.M) {
	custom.TolerantVerifyLeakMain(m)
}

func TestDedupExemplarsResponse(t *testing.T) {
	for _, tc := range []struct {
		name            string
		exemplars, want []*exemplarspb.ExemplarData
		replicaLabels   []string
	}{
		{
			name:      "nil slice",
			exemplars: nil,
			want:      nil,
		},
		{
			name:      "empty exemplars data slice",
			exemplars: []*exemplarspb.ExemplarData{},
			want:      []*exemplarspb.ExemplarData{},
		},
		{
			name: "empty exemplars data",
			exemplars: []*exemplarspb.ExemplarData{
				{
					SeriesLabels: labelpb.LabelSetFromStrings(
						"__name__", "test_exemplar_metric_total",
						"instance", "localhost:8090",
						"job", "prometheus",
						"service", "bar",
					),
				},
			},
			want: []*exemplarspb.ExemplarData{},
		},
		{
			name:          "multiple series",
			replicaLabels: []string{"replica"},
			exemplars: []*exemplarspb.ExemplarData{
				{
					SeriesLabels: labelpb.LabelSetFromStrings(
						"__name__", "test_exemplar_metric_total",
						"instance", "localhost:8090",
						"job", "prometheus",
						"service", "bar",
						"replica", "0",
					),
					Exemplars: []*exemplarspb.Exemplar{
						{
							Labels: labelpb.LabelSetFromStrings("traceID", "EpTxMJ40fUus7aGY"),
							Value:  19,
							Ts:     1600096955479,
						},
						{
							Labels: labelpb.LabelSetFromStrings("traceID", "EpTxMJ40fUus7aGY"),
							Value:  19,
							Ts:     1600096955479,
						},
					},
				},
				{
					SeriesLabels: labelpb.LabelSetFromStrings(
						"__name__", "test_exemplar_metric_total",
						"instance", "localhost:8090",
						"job", "prometheus",
						"service", "bar",
						"replica", "1",
					),
					Exemplars: []*exemplarspb.Exemplar{
						{
							Labels: labelpb.LabelSetFromStrings("traceID", "EpTxMJ40fUus7aGY"),
							Value:  19,
							Ts:     1600096955479,
						},
					},
				},
			},
			want: []*exemplarspb.ExemplarData{
				{
					SeriesLabels: labelpb.LabelSetFromStrings(
						"__name__", "test_exemplar_metric_total",
						"instance", "localhost:8090",
						"job", "prometheus",
						"service", "bar",
					),
					Exemplars: []*exemplarspb.Exemplar{
						{
							Labels: labelpb.LabelSetFromStrings("traceID", "EpTxMJ40fUus7aGY"),
							Value:  19,
							Ts:     1600096955479,
						},
					},
				},
			},
		},
		{
			name:          "multiple series with multiple exemplars data",
			replicaLabels: []string{"replica"},
			exemplars: []*exemplarspb.ExemplarData{
				{
					SeriesLabels: labelpb.LabelSetFromStrings(
						"__name__", "test_exemplar_metric_total",
						"instance", "localhost:8090",
						"job", "prometheus",
						"service", "bar",
						"replica", "0",
					),
					Exemplars: []*exemplarspb.Exemplar{
						{
							Labels: labelpb.LabelSetFromStrings("traceID", "EpTxMJ40fUus7aGY"),
							Value:  19,
							Ts:     1600096955479,
						},
						{
							Labels: labelpb.LabelSetFromStrings("traceID", "foo"),
							Value:  19,
							Ts:     1600096955470,
						},
					},
				},
				{
					SeriesLabels: labelpb.LabelSetFromStrings(
						"__name__", "test_exemplar_metric_total",
						"instance", "localhost:8090",
						"job", "prometheus",
						"service", "bar",
						"replica", "1",
					),
					Exemplars: []*exemplarspb.Exemplar{
						{
							Labels: labelpb.LabelSetFromStrings("traceID", "bar"),
							Value:  19,
							Ts:     1600096955579,
						},
						{
							Labels: labelpb.LabelSetFromStrings("traceID", "EpTxMJ40fUus7aGY"),
							Value:  19,
							Ts:     1600096955479,
						},
						// Same ts but different labels, cannot dedup.
						{
							Labels: labelpb.LabelSetFromStrings("traceID", "test"),
							Value:  19,
							Ts:     1600096955479,
						},
					},
				},
			},
			want: []*exemplarspb.ExemplarData{
				{
					SeriesLabels: labelpb.LabelSetFromStrings(
						"__name__", "test_exemplar_metric_total",
						"instance", "localhost:8090",
						"job", "prometheus",
						"service", "bar",
					),
					Exemplars: []*exemplarspb.Exemplar{
						{
							Labels: labelpb.LabelSetFromStrings("traceID", "foo"),
							Value:  19,
							Ts:     1600096955470,
						},
						{
							Labels: labelpb.LabelSetFromStrings("traceID", "EpTxMJ40fUus7aGY"),
							Value:  19,
							Ts:     1600096955479,
						},
						{
							Labels: labelpb.LabelSetFromStrings("traceID", "test"),
							Value:  19,
							Ts:     1600096955479,
						},
						{
							Labels: labelpb.LabelSetFromStrings("traceID", "bar"),
							Value:  19,
							Ts:     1600096955579,
						},
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			replicaLabels := make(map[string]struct{})
			for _, lbl := range tc.replicaLabels {
				replicaLabels[lbl] = struct{}{}
			}
			thanostestutil.ProtoEquals(t, tc.want, dedupExemplarsResponse(tc.exemplars, replicaLabels))
		})
	}
}
