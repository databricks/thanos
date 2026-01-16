// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package receive

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

func TestParseTagFilterValueMap(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		wantCount int
	}{
		{
			name:      "single tag exact match",
			input:     "job:prometheus",
			wantErr:   false,
			wantCount: 1,
		},
		{
			name:      "single tag with wildcard",
			input:     "job:prom*",
			wantErr:   false,
			wantCount: 1,
		},
		{
			name:      "multiple tags AND",
			input:     "job:prometheus namespace:monitoring",
			wantErr:   false,
			wantCount: 2,
		},
		{
			name:      "tag with hash separator",
			input:     "job#prometheus",
			wantErr:   false,
			wantCount: 1,
		},
		{
			name:      "negation tag",
			input:     "!internal:*",
			wantErr:   false,
			wantCount: 1,
		},
		{
			name:    "negation with non-wildcard pattern should error",
			input:   "!internal:value",
			wantErr: true,
		},
		{
			name:    "empty filter",
			input:   "",
			wantErr: false,
		},
		{
			name:    "invalid filter no separator",
			input:   "jobprometheus",
			wantErr: true,
		},
		{
			name:    "invalid filter empty tag name",
			input:   ":prometheus",
			wantErr: true,
		},
		{
			name:    "invalid filter empty pattern",
			input:   "job:",
			wantErr: true,
		},
		{
			name:    "duplicate tag should error",
			input:   "job:prometheus job:alertmanager",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseTagFilterValueMap(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Len(t, result, tt.wantCount)
			}
		})
	}
}

func TestFilterPatterns(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		input   string
		want    bool
	}{
		// Exact match
		{"exact match", "prometheus", "prometheus", true},
		{"exact match fail", "prometheus", "alertmanager", false},
		{"exact match empty", "", "", true},

		// Wildcard patterns
		{"wildcard all", "*", "anything", true},
		{"prefix wildcard", "prom*", "prometheus", true},
		{"prefix wildcard fail", "prom*", "alertmanager", false},
		{"suffix wildcard", "*eus", "prometheus", true},
		{"suffix wildcard fail", "*eus", "alertmanager", false},
		{"contains wildcard", "*met*", "prometheus", true},
		{"contains wildcard fail", "*xyz*", "prometheus", false},
		{"middle wildcard", "prom*eus", "prometheus", true},
		{"middle wildcard fail", "prom*eus", "prometheusX", false},

		// Single char wildcard
		{"single char ?", "pro?", "prod", true},
		{"single char ? fail", "pro?", "production", false},

		// Character set
		{"char set [abc]", "[abc]bc", "abc", true},
		{"char set [abc] fail", "[abc]bc", "dbc", false},

		// Character range
		{"char range [a-z]", "[a-z]bc", "abc", true},
		{"char range [a-z] fail", "[a-z]bc", "1bc", false},

		// Multi-char sequence
		{"multi char {a,b,c}", "{prod,dev,staging}", "prod", true},
		{"multi char {a,b,c} staging", "{prod,dev,staging}", "staging", true},
		{"multi char {a,b,c} fail", "{prod,dev,staging}", "test", false},

		// Negation
		{"negation", "!prometheus", "alertmanager", true},
		{"negation fail", "!prometheus", "prometheus", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := NewFilter([]byte(tt.pattern))
			require.NoError(t, err)
			result := filter.Matches([]byte(tt.input))
			require.Equal(t, tt.want, result, "pattern=%q input=%q", tt.pattern, tt.input)
		})
	}
}

func TestTagsFilter(t *testing.T) {
	tests := []struct {
		name   string
		filter string
		labels labels.Labels
		want   bool
	}{
		{
			name:   "single tag match",
			filter: "job:prometheus",
			labels: labels.FromStrings("job", "prometheus", "namespace", "monitoring"),
			want:   true,
		},
		{
			name:   "single tag no match",
			filter: "job:prometheus",
			labels: labels.FromStrings("job", "alertmanager", "namespace", "monitoring"),
			want:   false,
		},
		{
			name:   "multiple tags AND match",
			filter: "job:prometheus namespace:monitoring",
			labels: labels.FromStrings("job", "prometheus", "namespace", "monitoring"),
			want:   true,
		},
		{
			name:   "multiple tags AND partial match fails",
			filter: "job:prometheus namespace:monitoring",
			labels: labels.FromStrings("job", "prometheus", "namespace", "default"),
			want:   false,
		},
		{
			name:   "wildcard prefix match",
			filter: "job:prom*",
			labels: labels.FromStrings("job", "prometheus"),
			want:   true,
		},
		{
			name:   "wildcard suffix match",
			filter: "namespace:*ing",
			labels: labels.FromStrings("namespace", "monitoring"),
			want:   true,
		},
		{
			name:   "wildcard contains match",
			filter: "namespace:*nitor*",
			labels: labels.FromStrings("namespace", "monitoring"),
			want:   true,
		},
		{
			name:   "multi-char sequence match",
			filter: "env:{prod,staging,dev}",
			labels: labels.FromStrings("env", "prod"),
			want:   true,
		},
		{
			name:   "multi-char sequence match staging",
			filter: "env:{prod,staging,dev}",
			labels: labels.FromStrings("env", "staging"),
			want:   true,
		},
		{
			name:   "multi-char sequence no match",
			filter: "env:{prod,staging,dev}",
			labels: labels.FromStrings("env", "test"),
			want:   false,
		},
		{
			name:   "tag not present fails for conjunction",
			filter: "job:prometheus",
			labels: labels.FromStrings("namespace", "monitoring"),
			want:   false,
		},
		{
			name:   "exclude tag match (tag absent)",
			filter: "!internal:*",
			labels: labels.FromStrings("job", "prometheus"),
			want:   true,
		},
		{
			name:   "exclude tag no match (tag present)",
			filter: "!internal:*",
			labels: labels.FromStrings("job", "prometheus", "internal", "true"),
			want:   false,
		},
		{
			name:   "empty labels with filter fails",
			filter: "job:prometheus",
			labels: labels.EmptyLabels(),
			want:   false,
		},
		{
			name:   "empty filter matches all",
			filter: "",
			labels: labels.FromStrings("job", "prometheus"),
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filterValues, err := ParseTagFilterValueMap(tt.filter)
			require.NoError(t, err)

			tagsFilter, err := NewTagsFilter(filterValues, Conjunction, TagsFilterOptions{})
			require.NoError(t, err)

			result := tagsFilter.MatchLabels(tt.labels)
			require.Equal(t, tt.want, result, "filter=%q labels=%v", tt.filter, tt.labels)
		})
	}
}

func TestTenantAttributor(t *testing.T) {
	// Create a temporary config file
	configContent := `
- filter: "job:prometheus"
  tenant: "prometheus-tenant"
- filter: "namespace:prod*"
  tenant: "production"
- filter: "job:alertmanager namespace:monitoring"
  tenant: "alerting-tenant"
- filter: "env:{staging,dev}"
  tenant: "non-prod"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tenant-rules.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	logger := log.NewNopLogger()
	reg := prometheus.NewRegistry()

	// Test without verify mode
	ta, err := NewTenantAttributor(configPath, "default-tenant", false, reg, logger)
	require.NoError(t, err)
	require.NotNil(t, ta)
	require.Len(t, ta.rules, 4)
	require.False(t, ta.IsVerifyMode())

	tests := []struct {
		name       string
		labels     labels.Labels
		wantTenant string
	}{
		{
			name:       "first rule match - prometheus",
			labels:     labels.FromStrings("job", "prometheus"),
			wantTenant: "prometheus-tenant",
		},
		{
			name:       "second rule match - production namespace",
			labels:     labels.FromStrings("namespace", "production"),
			wantTenant: "production",
		},
		{
			name:       "second rule match - prod prefix",
			labels:     labels.FromStrings("namespace", "prod-us-east"),
			wantTenant: "production",
		},
		{
			name:       "third rule match - alertmanager in monitoring",
			labels:     labels.FromStrings("job", "alertmanager", "namespace", "monitoring"),
			wantTenant: "alerting-tenant",
		},
		{
			name:       "fourth rule match - staging env",
			labels:     labels.FromStrings("env", "staging"),
			wantTenant: "non-prod",
		},
		{
			name:       "fourth rule match - dev env",
			labels:     labels.FromStrings("env", "dev"),
			wantTenant: "non-prod",
		},
		{
			name:       "no rule match - default tenant",
			labels:     labels.FromStrings("job", "unknown"),
			wantTenant: "default-tenant",
		},
		{
			name:       "empty labels - default tenant",
			labels:     labels.EmptyLabels(),
			wantTenant: "default-tenant",
		},
		{
			name:       "first match wins - prometheus also has prod namespace",
			labels:     labels.FromStrings("job", "prometheus", "namespace", "production"),
			wantTenant: "prometheus-tenant", // First rule wins
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tenant := ta.GetTenantFromLabels(tt.labels)
			require.Equal(t, tt.wantTenant, tenant)
		})
	}
}

func TestTenantAttributorVerifyMode(t *testing.T) {
	configContent := `
- filter: "job:prometheus"
  tenant: "prometheus-tenant"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tenant-rules.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	logger := log.NewNopLogger()
	reg := prometheus.NewRegistry()

	ta, err := NewTenantAttributor(configPath, "default-tenant", true, reg, logger)
	require.NoError(t, err)
	require.True(t, ta.IsVerifyMode())
	require.NotNil(t, ta.attributionMatches)
	require.NotNil(t, ta.attributionMismatches)

	// Test recording verification
	lbls := labels.FromStrings("job", "prometheus")
	attributedTenant := ta.GetTenantFromLabels(lbls)
	require.Equal(t, "prometheus-tenant", attributedTenant)

	// Record match
	ta.RecordVerification(attributedTenant, "prometheus-tenant")

	// Record mismatch
	ta.RecordVerification(attributedTenant, "other-tenant")

	// Verify metrics
	matchCount, err := getCounterValue(ta.attributionMatches)
	require.NoError(t, err)
	require.Equal(t, float64(1), matchCount)

	mismatchCount, err := getCounterValue(ta.attributionMismatches)
	require.NoError(t, err)
	require.Equal(t, float64(1), mismatchCount)
}

func TestTenantAttributorConfigErrors(t *testing.T) {
	logger := log.NewNopLogger()
	reg := prometheus.NewRegistry()
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		config      string
		wantErrMsg  string
		emptyConfig bool
	}{
		{
			name:       "empty config path",
			config:     "",
			wantErrMsg: "tenant rules config path is required",
		},
		{
			name:       "missing filter",
			config:     "- tenant: my-tenant\n",
			wantErrMsg: "filter is required",
		},
		{
			name:       "missing tenant",
			config:     "- filter: \"job:prometheus\"\n",
			wantErrMsg: "tenant is required",
		},
		{
			name:       "invalid filter syntax",
			config:     "- filter: \"invalid\"\n  tenant: my-tenant\n",
			wantErrMsg: "parsing filter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := ""
			if tt.config != "" {
				configPath = filepath.Join(tmpDir, tt.name+".yaml")
				err := os.WriteFile(configPath, []byte(tt.config), 0644)
				require.NoError(t, err)
			}

			_, err := NewTenantAttributor(configPath, "default", false, reg, logger)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErrMsg)
		})
	}
}

func TestTenantAttributorFileNotFound(t *testing.T) {
	logger := log.NewNopLogger()
	reg := prometheus.NewRegistry()

	_, err := NewTenantAttributor("/nonexistent/path/config.yaml", "default", false, reg, logger)
	require.Error(t, err)
	require.Contains(t, err.Error(), "reading tenant rules config")
}

// Helper function to get counter value.
func getCounterValue(c prometheus.Counter) (float64, error) {
	var m dto.Metric
	if err := c.Write(&m); err != nil {
		return 0, err
	}
	return m.GetCounter().GetValue(), nil
}
