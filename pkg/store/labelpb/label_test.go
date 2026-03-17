// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package labelpb

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/efficientgo/core/testutil"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
	thanostestutil "github.com/thanos-io/thanos/pkg/testutil"
)

func TestExtendLabels(t *testing.T) {
	thanostestutil.ProtoEquals(t,
		FromStrings("a", "1", "replica", "01", "xb", "2"),
		ExtendSortedLabels(
			FromStrings("a", "1", "xb", "2"),
			FromStrings("replica", "01"),
		))

	thanostestutil.ProtoEquals(t,
		FromStrings("replica", "01"),
		ExtendSortedLabels(
			EmptyLabels(),
			FromStrings("replica", "01"),
		))

	thanostestutil.ProtoEquals(t, FromStrings("a", "1", "replica", "01", "xb", "2"),
		ExtendSortedLabels(
			FromStrings("a", "1", "replica", "NOT01", "xb", "2"),
			FromStrings("replica", "01"),
		))

	testInjectExtLabels(testutil.NewTB(t))
}

func TestHashMatchesPrometheus(t *testing.T) {
	cases := [][]string{
		{"__name__", "test"},
		{"a", "1", "b", "2"},
		{"__name__", "http_requests_total", "method", "GET", "status", "200"},
		{}, // empty
	}

	for _, ss := range cases {
		pb := FromStrings(ss...)
		prom := labels.FromStrings(ss...)
		testutil.Equals(t, prom.Hash(), pb.Hash(),
			"hash mismatch for labels %v", ss)
	}

	// Large label set that exceeds the 1024-byte fast path.
	var large []string
	for i := 0; i < 100; i++ {
		large = append(large, fmt.Sprintf("key_%03d", i), strings.Repeat("v", 20))
	}
	pb := FromStrings(large...)
	prom := labels.FromStrings(large...)
	testutil.Equals(t, prom.Hash(), pb.Hash(), "hash mismatch for large label set")
}

func TestRmLabels(t *testing.T) {
	thanostestutil.ProtoEquals(t,
		FromStrings("a", "1", "c", "3"),
		RmLabels(
			FromStrings("a", "1", "b", "2", "c", "3"),
			map[string]struct{}{"b": {}},
		))

	thanostestutil.ProtoEquals(t,
		FromStrings("a", "1", "c", "3"),
		RmLabels(
			FromStrings("a", "1", "b", "2", "c", "3"),
			map[string]struct{}{"b": {}, "d": {}},
		))

	// Removing all labels.
	thanostestutil.ProtoEquals(t,
		Labels(nil),
		RmLabels(
			FromStrings("a", "1"),
			map[string]struct{}{"a": {}},
		))

	// Empty remove set returns a copy.
	orig := FromStrings("a", "1", "b", "2")
	result := RmLabels(orig, nil)
	thanostestutil.ProtoEquals(t, orig, result)
	testutil.Assert(t, &orig[0] != &result[0], "RmLabels should return a copy")

	// Empty input.
	thanostestutil.ProtoEquals(t,
		Labels(nil),
		RmLabels(nil, map[string]struct{}{"a": {}}),
	)
}

func TestRmLabelsInPlace(t *testing.T) {
	// Basic removal.
	lset := FromStrings("a", "1", "b", "2", "c", "3")
	result := RmLabelsInPlace(lset, map[string]struct{}{"b": {}})
	thanostestutil.ProtoEquals(t, FromStrings("a", "1", "c", "3"), result)

	// Verify it reuses the same backing array.
	testutil.Assert(t, &lset[0] == &result[0], "RmLabelsInPlace should reuse the backing array")

	// Remove multiple, including one not present.
	lset = FromStrings("a", "1", "b", "2", "c", "3")
	result = RmLabelsInPlace(lset, map[string]struct{}{"a": {}, "c": {}, "z": {}})
	thanostestutil.ProtoEquals(t, FromStrings("b", "2"), result)

	// Empty remove set returns the same slice unchanged.
	lset = FromStrings("a", "1")
	result = RmLabelsInPlace(lset, nil)
	thanostestutil.ProtoEquals(t, FromStrings("a", "1"), result)
	testutil.Assert(t, &lset[0] == &result[0], "empty remove should return same slice")

	// Remove all.
	lset = FromStrings("a", "1")
	result = RmLabelsInPlace(lset, map[string]struct{}{"a": {}})
	testutil.Equals(t, 0, len(result))

	// Nil input.
	result = RmLabelsInPlace(nil, map[string]struct{}{"a": {}})
	testutil.Equals(t, 0, len(result))
}

func BenchmarkExtendLabels(b *testing.B) {
	testInjectExtLabels(testutil.NewTB(b))
}

var x Labels

func testInjectExtLabels(tb testutil.TB) {
	in := FromStrings(
		"__name__", "subscription_labels",
		"_id", "0dfsdfsdsfdsffd1e96-4432-9abe-e33436ea969a",
		"account", "1afsdfsddsfsdfsdfsdfsdfs",
		"ebs_account", "1asdasdad45",
		"email_domain", "asdasddgfkw.example.com",
		"endpoint", "metrics",
		"external_organization", "dfsdfsdf",
		"instance", "10.128.4.231:8080",
		"job", "sdd-acct-mngr-metrics",
		"managed", "false",
		"namespace", "production",
		"organization", "dasdadasdasasdasaaFGDSG",
		"pod", "sdd-acct-mngr-6669c947c8-xjx7f",
		"prometheus", "telemeter-production/telemeter",
		"prometheus_replica", "prometheus-telemeter-1",
		"risk", "5",
		"service", "sdd-acct-mngr-metrics",
		"support", "Self-Support",
	)
	extLset := FromStrings(
		"replica", "1",
		"support", "Host-Support",
		"tenant", "2342",
	)
	tb.ResetTimer()
	for i := 0; i < tb.N(); i++ {
		x = ExtendSortedLabels(in, extLset)

		if !tb.IsBenchmark() {
			thanostestutil.ProtoEquals(tb, FromStrings(
				"__name__", "subscription_labels",
				"_id", "0dfsdfsdsfdsffd1e96-4432-9abe-e33436ea969a",
				"account", "1afsdfsddsfsdfsdfsdfsdfs",
				"ebs_account", "1asdasdad45",
				"email_domain", "asdasddgfkw.example.com",
				"endpoint", "metrics",
				"external_organization", "dfsdfsdf",
				"instance", "10.128.4.231:8080",
				"job", "sdd-acct-mngr-metrics",
				"managed", "false",
				"namespace", "production",
				"organization", "dasdadasdasasdasaaFGDSG",
				"pod", "sdd-acct-mngr-6669c947c8-xjx7f",
				"prometheus", "telemeter-production/telemeter",
				"prometheus_replica", "prometheus-telemeter-1",
				"replica", "1",
				"risk", "5",
				"service", "sdd-acct-mngr-metrics",
				"support", "Host-Support",
				"tenant", "2342",
			), x)
		}
	}
	fmt.Fprint(io.Discard, x)
}

func TestHashWithoutLabels(t *testing.T) {
	tests := []struct {
		name  string
		ls    Labels
		names []string
	}{
		{
			name:  "empty",
			ls:    Labels{},
			names: nil,
		},
		{
			name:  "no exclusions",
			ls:    FromStrings("a", "1", "b", "2"),
			names: nil,
		},
		{
			name:  "exclude one",
			ls:    FromStrings("a", "1", "b", "2", "c", "3"),
			names: []string{"b"},
		},
		{
			name:  "exclude metric name",
			ls:    FromStrings("__name__", "metric", "a", "1"),
			names: nil,
		},
		{
			name:  "exclude all",
			ls:    FromStrings("__name__", "metric", "a", "1"),
			names: []string{"a"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			promLbls := ToPromLabels(tt.ls)
			wantHash, _ := promLbls.HashWithoutLabels(nil, tt.names...)
			gotHash, _ := tt.ls.HashWithoutLabels(nil, tt.names...)
			require.Equal(t, wantHash, gotHash, "hash mismatch for %v excluding %v", tt.ls, tt.names)
		})
	}
}

func TestHashDeterministic(t *testing.T) {
	cases := [][]string{
		{"__name__", "test"},
		{"a", "1", "b", "2"},
		{"__name__", "http_requests_total", "method", "GET", "status", "200"},
		{}, // empty
	}

	for _, ss := range cases {
		pb := FromStrings(ss...)
		// Same input must always produce the same hash.
		testutil.Equals(t, pb.Hash(), pb.Hash(),
			"hash must be deterministic for labels %v", ss)
	}

	// Large label set that exceeds the 1024-byte fast path.
	var large []string
	for i := 0; i < 100; i++ {
		large = append(large, fmt.Sprintf("key_%03d", i), strings.Repeat("v", 20))
	}
	pb := FromStrings(large...)
	testutil.Equals(t, pb.Hash(), pb.Hash(), "hash must be deterministic for large label set")
}
