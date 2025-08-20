// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package store

import (
	"testing"

	"github.com/efficientgo/core/testutil"
	"github.com/prometheus/prometheus/model/labels"
)

func TestMatchesBlockedPattern(t *testing.T) {
	testCases := []struct {
		name        string
		metricName  string
		patterns    []string
		shouldMatch bool
	}{
		{
			name:        "kube metric matches kube pattern",
			metricName:  "kube_node_info",
			patterns:    []string{"kube_*"},
			shouldMatch: true,
		},
		{
			name:        "envoy metric matches envoy pattern",
			metricName:  "envoy_cluster_stats",
			patterns:    []string{"envoy_*"},
			shouldMatch: true,
		},
		{
			name:        "metric matches multiple patterns",
			metricName:  "kube_pod_info",
			patterns:    []string{"kube_*", "envoy_*"},
			shouldMatch: true,
		},
		{
			name:        "metric doesn't match pattern",
			metricName:  "prometheus_metric",
			patterns:    []string{"kube_*", "envoy_*"},
			shouldMatch: false,
		},
		{
			name:        "empty patterns",
			metricName:  "any_metric",
			patterns:    []string{},
			shouldMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := matchesBlockedPattern(tc.metricName, tc.patterns)
			testutil.Equals(t, tc.shouldMatch, result)
		})
	}
}

func TestHasSufficientFilters(t *testing.T) {
	testCases := []struct {
		name       string
		matchers   []*labels.Matcher
		shouldHave bool
	}{
		{
			name: "has label equality matcher",
			matchers: []*labels.Matcher{
				{Name: "__name__", Type: labels.MatchEqual, Value: "kube_node_info"},
				{Name: "instance", Type: labels.MatchEqual, Value: "localhost:9100"},
			},
			shouldHave: true,
		},
		{
			name: "has regex matcher (not sufficient)",
			matchers: []*labels.Matcher{
				{Name: "__name__", Type: labels.MatchEqual, Value: "kube_node_info"},
				{Name: "instance", Type: labels.MatchRegexp, Value: ".*"},
			},
			shouldHave: false,
		},
		{
			name: "only metric name matcher",
			matchers: []*labels.Matcher{
				{Name: "__name__", Type: labels.MatchEqual, Value: "kube_node_info"},
			},
			shouldHave: false,
		},
		{
			name:       "empty matchers",
			matchers:   []*labels.Matcher{},
			shouldHave: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := hasSufficientFilters(tc.matchers)
			testutil.Equals(t, tc.shouldHave, result)
		})
	}
}

func TestProxyStore_ShouldBlockQuery(t *testing.T) {
	testCases := []struct {
		name           string
		patterns       []string
		matchers       []*labels.Matcher
		shouldBlock    bool
		expectedReason string
	}{
		{
			name:     "block kube metric without filters",
			patterns: []string{"kube_*"},
			matchers: []*labels.Matcher{
				{Name: "__name__", Type: labels.MatchEqual, Value: "kube_node_info"},
			},
			shouldBlock:    true,
			expectedReason: "metric 'kube_node_info' matches blocked pattern but query lacks sufficient label filters",
		},
		{
			name:     "allow kube metric with filters",
			patterns: []string{"kube_*"},
			matchers: []*labels.Matcher{
				{Name: "__name__", Type: labels.MatchEqual, Value: "kube_node_info"},
				{Name: "instance", Type: labels.MatchEqual, Value: "localhost:9100"},
			},
			shouldBlock:    false,
			expectedReason: "",
		},
		{
			name:     "allow non-blocked metric",
			patterns: []string{"kube_*"},
			matchers: []*labels.Matcher{
				{Name: "__name__", Type: labels.MatchEqual, Value: "prometheus_metric"},
			},
			shouldBlock:    false,
			expectedReason: "",
		},
		{
			name:     "no patterns configured",
			patterns: []string{},
			matchers: []*labels.Matcher{
				{Name: "__name__", Type: labels.MatchEqual, Value: "kube_node_info"},
			},
			shouldBlock:    false,
			expectedReason: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			proxy := &ProxyStore{
				blockedMetricPatterns: tc.patterns,
			}
			shouldBlock, reason := proxy.shouldBlockQuery(tc.matchers)
			testutil.Equals(t, tc.shouldBlock, shouldBlock)
			testutil.Equals(t, tc.expectedReason, reason)
		})
	}
}
