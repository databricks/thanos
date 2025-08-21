// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package store

import (
	"testing"

	"github.com/prometheus/prometheus/model/labels"
)

func TestMatchesBlockedPattern(t *testing.T) {
	tests := []struct {
		name           string
		patterns       []string
		metricName     string
		expectedResult bool
	}{
		{
			name:           "no patterns",
			patterns:       []string{},
			metricName:     "some_metric",
			expectedResult: false,
		},
		{
			name:           "pattern matches",
			patterns:       []string{"high_cardinality"},
			metricName:     "high_cardinality_metric",
			expectedResult: true,
		},
		{
			name:           "pattern does not match",
			patterns:       []string{"high_cardinality"},
			metricName:     "low_cardinality_metric",
			expectedResult: false,
		},
		{
			name:           "multiple patterns, one matches",
			patterns:       []string{"high_cardinality", "another_pattern"},
			metricName:     "high_cardinality_metric",
			expectedResult: true,
		},
		{
			name:           "empty pattern ignored",
			patterns:       []string{"", "high_cardinality"},
			metricName:     "high_cardinality_metric",
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ProxyStore{
				blockedMetricPatterns: tt.patterns,
			}
			result := s.matchesBlockedPattern(tt.metricName)
			if result != tt.expectedResult {
				t.Errorf("matchesBlockedPattern() = %v, want %v", result, tt.expectedResult)
			}
		})
	}
}

func TestHasSufficientFilters(t *testing.T) {
	tests := []struct {
		name           string
		matchers       []*labels.Matcher
		expectedResult bool
	}{
		{
			name:           "no matchers",
			matchers:       []*labels.Matcher{},
			expectedResult: false,
		},
		{
			name: "only __name__ matcher",
			matchers: []*labels.Matcher{
				labels.MustNewMatcher(labels.MatchEqual, "__name__", "some_metric"),
			},
			expectedResult: false,
		},
		{
			name: "has exact label matcher",
			matchers: []*labels.Matcher{
				labels.MustNewMatcher(labels.MatchEqual, "__name__", "some_metric"),
				labels.MustNewMatcher(labels.MatchEqual, "job", "my_job"),
			},
			expectedResult: true,
		},
		{
			name: "only regex matcher",
			matchers: []*labels.Matcher{
				labels.MustNewMatcher(labels.MatchEqual, "__name__", "some_metric"),
				labels.MustNewMatcher(labels.MatchRegexp, "job", ".*"),
			},
			expectedResult: false,
		},
		{
			name: "mix of exact and regex matchers",
			matchers: []*labels.Matcher{
				labels.MustNewMatcher(labels.MatchEqual, "__name__", "some_metric"),
				labels.MustNewMatcher(labels.MatchEqual, "job", "my_job"),
				labels.MustNewMatcher(labels.MatchRegexp, "instance", ".*"),
			},
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ProxyStore{}
			result := s.hasSufficientFilters(tt.matchers)
			if result != tt.expectedResult {
				t.Errorf("hasSufficientFilters() = %v, want %v", result, tt.expectedResult)
			}
		})
	}
}

func TestShouldBlockQuery(t *testing.T) {
	tests := []struct {
		name                  string
		patterns              []string
		matchers              []*labels.Matcher
		expectedResult        bool
		expectedMetricName    string
		expectedMatchedPattern string
	}{
		{
			name:                  "no blocked patterns",
			patterns:              []string{},
			matchers: []*labels.Matcher{
				labels.MustNewMatcher(labels.MatchEqual, "__name__", "high_cardinality_metric"),
			},
			expectedResult:        false,
			expectedMetricName:    "",
			expectedMatchedPattern: "",
		},
		{
			name:     "no metric name matcher",
			patterns: []string{"high_cardinality"},
			matchers: []*labels.Matcher{
				labels.MustNewMatcher(labels.MatchEqual, "job", "my_job"),
			},
			expectedResult:        false,
			expectedMetricName:    "",
			expectedMatchedPattern: "",
		},
		{
			name:     "metric does not match blocked patterns",
			patterns: []string{"high_cardinality"},
			matchers: []*labels.Matcher{
				labels.MustNewMatcher(labels.MatchEqual, "__name__", "low_cardinality_metric"),
				labels.MustNewMatcher(labels.MatchEqual, "job", "my_job"),
			},
			expectedResult:        false,
			expectedMetricName:    "",
			expectedMatchedPattern: "",
		},
		{
			name:     "metric matches pattern but has sufficient filters",
			patterns: []string{"high_cardinality"},
			matchers: []*labels.Matcher{
				labels.MustNewMatcher(labels.MatchEqual, "__name__", "high_cardinality_metric"),
				labels.MustNewMatcher(labels.MatchEqual, "job", "my_job"),
			},
			expectedResult:        false,
			expectedMetricName:    "high_cardinality_metric",
			expectedMatchedPattern: "high_cardinality",
		},
		{
			name:     "metric matches pattern and lacks sufficient filters",
			patterns: []string{"high_cardinality"},
			matchers: []*labels.Matcher{
				labels.MustNewMatcher(labels.MatchEqual, "__name__", "high_cardinality_metric"),
			},
			expectedResult:        true,
			expectedMetricName:    "high_cardinality_metric",
			expectedMatchedPattern: "high_cardinality",
		},
		{
			name:     "metric matches pattern but only has regex filters",
			patterns: []string{"high_cardinality"},
			matchers: []*labels.Matcher{
				labels.MustNewMatcher(labels.MatchEqual, "__name__", "high_cardinality_metric"),
				labels.MustNewMatcher(labels.MatchRegexp, "job", ".*"),
			},
			expectedResult:        true,
			expectedMetricName:    "high_cardinality_metric",
			expectedMatchedPattern: "high_cardinality",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ProxyStore{
				blockedMetricPatterns: tt.patterns,
			}
			shouldBlock, metricName, matchedPattern := s.shouldBlockQuery(tt.matchers)
			if shouldBlock != tt.expectedResult {
				t.Errorf("shouldBlockQuery() shouldBlock = %v, want %v", shouldBlock, tt.expectedResult)
			}
			if metricName != tt.expectedMetricName {
				t.Errorf("shouldBlockQuery() metricName = %v, want %v", metricName, tt.expectedMetricName)
			}
			if matchedPattern != tt.expectedMatchedPattern {
				t.Errorf("shouldBlockQuery() matchedPattern = %v, want %v", matchedPattern, tt.expectedMatchedPattern)
			}
		})
	}
}

func TestCountExactFilters(t *testing.T) {
	tests := []struct {
		name           string
		matchers       []*labels.Matcher
		expectedCount  int
	}{
		{
			name:          "no matchers",
			matchers:      []*labels.Matcher{},
			expectedCount: 0,
		},
		{
			name: "only __name__ matcher",
			matchers: []*labels.Matcher{
				labels.MustNewMatcher(labels.MatchEqual, "__name__", "some_metric"),
			},
			expectedCount: 0,
		},
		{
			name: "one exact label matcher",
			matchers: []*labels.Matcher{
				labels.MustNewMatcher(labels.MatchEqual, "__name__", "some_metric"),
				labels.MustNewMatcher(labels.MatchEqual, "job", "my_job"),
			},
			expectedCount: 1,
		},
		{
			name: "regex matcher not counted",
			matchers: []*labels.Matcher{
				labels.MustNewMatcher(labels.MatchEqual, "__name__", "some_metric"),
				labels.MustNewMatcher(labels.MatchRegexp, "job", ".*"),
			},
			expectedCount: 0,
		},
		{
			name: "multiple exact matchers",
			matchers: []*labels.Matcher{
				labels.MustNewMatcher(labels.MatchEqual, "__name__", "some_metric"),
				labels.MustNewMatcher(labels.MatchEqual, "job", "my_job"),
				labels.MustNewMatcher(labels.MatchEqual, "instance", "localhost"),
				labels.MustNewMatcher(labels.MatchRegexp, "handler", ".*"),
			},
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ProxyStore{}
			count := s.countExactFilters(tt.matchers)
			if count != tt.expectedCount {
				t.Errorf("countExactFilters() = %v, want %v", count, tt.expectedCount)
			}
		})
	}
}
