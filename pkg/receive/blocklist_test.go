// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package receive

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/efficientgo/core/testutil"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
	"github.com/thanos-io/thanos/pkg/extkingpin"
	"github.com/thanos-io/thanos/pkg/store/labelpb"
	"github.com/thanos-io/thanos/pkg/store/storepb/prompb"
	"go.uber.org/atomic"
)

func TestParseBlocklistConfig(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		input   string
		wantErr bool
		nRules  int
	}{
		{
			name:    "empty config",
			input:   "[]",
			wantErr: false,
			nRules:  0,
		},
		{
			name:    "single rule",
			input:   `["__name__:kube_pod_status_phase job:dblet-ksm"]`,
			wantErr: false,
			nRules:  1,
		},
		{
			name: "multiple rules",
			input: `
- "__name__:logger_messageCount_histogram_bucket kubernetes_namespace:dblet*"
- "__name__:kube_pod_status_phase job:dblet-ksm phase:!{Running,Succeeded}"
`,
			wantErr: false,
			nRules:  2,
		},
		{
			name:    "empty strings skipped",
			input:   `["__name__:foo", ""]`,
			wantErr: false,
			nRules:  1,
		},
		{
			name:    "invalid filter pattern",
			input:   `["invalid_no_colon"]`,
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rules, err := parseBlocklistConfig([]byte(tc.input))
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, rules, tc.nRules)
		})
	}
}

func TestBlocklistFilter_FilterTimeSeries(t *testing.T) {
	t.Parallel()

	mkTS := func(lbls ...string) prompb.TimeSeries {
		zlabels := make([]labelpb.ZLabel, 0, len(lbls)/2)
		for i := 0; i < len(lbls); i += 2 {
			zlabels = append(zlabels, labelpb.ZLabel{Name: lbls[i], Value: lbls[i+1]})
		}
		return prompb.TimeSeries{
			Labels:  zlabels,
			Samples: []prompb.Sample{{Timestamp: 1, Value: 1}},
		}
	}

	for _, tc := range []struct {
		name            string
		config          string
		input           []prompb.TimeSeries
		expectedRemain  int
		expectedDropped int
	}{
		{
			name:   "no rules drops nothing",
			config: "[]",
			input: []prompb.TimeSeries{
				mkTS("__name__", "foo", "job", "bar"),
			},
			expectedRemain:  1,
			expectedDropped: 0,
		},
		{
			name:   "exact match drops series",
			config: `["__name__:kube_pod_status_phase job:dblet-ksm"]`,
			input: []prompb.TimeSeries{
				mkTS("__name__", "kube_pod_status_phase", "job", "dblet-ksm"),
				mkTS("__name__", "kube_pod_status_phase", "job", "other"),
				mkTS("__name__", "up", "job", "dblet-ksm"),
			},
			expectedRemain:  2,
			expectedDropped: 1,
		},
		{
			name:   "wildcard match",
			config: `["__name__:logger_messageCount_histogram_bucket kubernetes_namespace:dblet*"]`,
			input: []prompb.TimeSeries{
				mkTS("__name__", "logger_messageCount_histogram_bucket", "kubernetes_namespace", "dblet-foo"),
				mkTS("__name__", "logger_messageCount_histogram_bucket", "kubernetes_namespace", "dblet-bar"),
				mkTS("__name__", "logger_messageCount_histogram_bucket", "kubernetes_namespace", "other-ns"),
				mkTS("__name__", "other_metric", "kubernetes_namespace", "dblet-foo"),
			},
			expectedRemain:  2,
			expectedDropped: 2,
		},
		{
			name: "negation pattern — drop series NOT matching phase Running or Succeeded",
			config: `
- "__name__:kube_pod_status_phase job:dblet-ksm phase:!{Running,Succeeded}"
`,
			input: []prompb.TimeSeries{
				mkTS("__name__", "kube_pod_status_phase", "job", "dblet-ksm", "phase", "Pending"),
				mkTS("__name__", "kube_pod_status_phase", "job", "dblet-ksm", "phase", "Running"),
				mkTS("__name__", "kube_pod_status_phase", "job", "dblet-ksm", "phase", "Succeeded"),
				mkTS("__name__", "kube_pod_status_phase", "job", "dblet-ksm", "phase", "Failed"),
			},
			expectedRemain:  2,
			expectedDropped: 2,
		},
		{
			name: "multi-range alternation pattern",
			config: `
- "__name__:envoy_cluster_upstream_cx_connect_ms_bucket kubernetes_namespace:dp-ingress envoy_cluster_name:{SERVERLESS_PLATFORM-,driver-pool-}*"
`,
			input: []prompb.TimeSeries{
				mkTS("__name__", "envoy_cluster_upstream_cx_connect_ms_bucket", "kubernetes_namespace", "dp-ingress", "envoy_cluster_name", "SERVERLESS_PLATFORM-foo"),
				mkTS("__name__", "envoy_cluster_upstream_cx_connect_ms_bucket", "kubernetes_namespace", "dp-ingress", "envoy_cluster_name", "driver-pool-bar"),
				mkTS("__name__", "envoy_cluster_upstream_cx_connect_ms_bucket", "kubernetes_namespace", "dp-ingress", "envoy_cluster_name", "other-cluster"),
				mkTS("__name__", "envoy_cluster_upstream_cx_connect_ms_bucket", "kubernetes_namespace", "other-ns", "envoy_cluster_name", "SERVERLESS_PLATFORM-foo"),
			},
			expectedRemain:  2,
			expectedDropped: 2,
		},
		{
			name: "multiple rules — any matching rule drops",
			config: `
- "__name__:metric_a"
- "__name__:metric_b"
`,
			input: []prompb.TimeSeries{
				mkTS("__name__", "metric_a"),
				mkTS("__name__", "metric_b"),
				mkTS("__name__", "metric_c"),
			},
			expectedRemain:  1,
			expectedDropped: 2,
		},
		{
			name:            "empty write request",
			config:          `["__name__:foo"]`,
			input:           []prompb.TimeSeries{},
			expectedRemain:  0,
			expectedDropped: 0,
		},
		{
			name:   "all series blocked",
			config: `["__name__:*"]`,
			input: []prompb.TimeSeries{
				mkTS("__name__", "foo"),
				mkTS("__name__", "bar"),
			},
			expectedRemain:  0,
			expectedDropped: 2,
		},
		{
			name:   "no series blocked",
			config: `["__name__:nonexistent_metric"]`,
			input: []prompb.TimeSeries{
				mkTS("__name__", "foo"),
				mkTS("__name__", "bar"),
			},
			expectedRemain:  2,
			expectedDropped: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rules, err := parseBlocklistConfig([]byte(tc.config))
			require.NoError(t, err)

			bf := &BlocklistFilter{
				logger: log.NewNopLogger(),
			}
			compiled := blocklistRules(rules)
			var ptr atomic.Pointer[blocklistRules]
			ptr.Store(&compiled)
			bf.rules = &ptr

			wreq := &prompb.WriteRequest{
				Timeseries: make([]prompb.TimeSeries, len(tc.input)),
			}
			copy(wreq.Timeseries, tc.input)

			dropped := bf.FilterTimeSeries(wreq)
			require.Equal(t, tc.expectedDropped, dropped, "unexpected number of dropped series")
			require.Len(t, wreq.Timeseries, tc.expectedRemain, "unexpected number of remaining series")
		})
	}
}

func TestBlocklistFilter_NilFilter(t *testing.T) {
	t.Parallel()

	var bf *BlocklistFilter
	wreq := &prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels:  []labelpb.ZLabel{{Name: "__name__", Value: "test"}},
				Samples: []prompb.Sample{{Timestamp: 1, Value: 1}},
			},
		},
	}
	dropped := bf.FilterTimeSeries(wreq)
	require.Equal(t, 0, dropped)
	require.Len(t, wreq.Timeseries, 1)
}

func TestBlocklistFilter_LoadConfig(t *testing.T) {
	t.Parallel()

	configYAML := `
- "__name__:metric_to_block"
- "__name__:another_metric job:some_job"
`
	dir := t.TempDir()
	configPath := filepath.Join(dir, "blocklist.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(configYAML), 0644))

	fc, err := extkingpin.NewStaticPathContent(configPath)
	require.NoError(t, err)

	bf, err := NewBlocklistFilter(fc, nil, log.NewNopLogger(), 0)
	require.NoError(t, err)

	rules := *bf.rules.Load()
	require.Len(t, rules, 2)
}

func TestBlocklistFilter_NilConfig(t *testing.T) {
	t.Parallel()

	bf, err := NewBlocklistFilter(nil, nil, log.NewNopLogger(), 0)
	require.NoError(t, err)
	rules := *bf.rules.Load()
	require.Len(t, rules, 0)
}

func TestBlocklistFilter_HotReload(t *testing.T) {
	t.Parallel()

	initialConfig := `
- "__name__:metric_a"
`
	updatedConfig := `
- "__name__:metric_a"
- "__name__:metric_b"
`
	dir := t.TempDir()
	configPath := filepath.Join(dir, "blocklist.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(initialConfig), 0644))

	fc, err := extkingpin.NewStaticPathContent(configPath)
	require.NoError(t, err)

	bf, err := NewBlocklistFilter(fc, nil, log.NewNopLogger(), 1*time.Second)
	require.NoError(t, err)

	// Verify initial config
	rules := *bf.rules.Load()
	require.Len(t, rules, 1)

	// Start the reloader
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	testutil.Ok(t, bf.StartConfigReloader(ctx))

	// Update config via Rewrite (which triggers the reloader)
	testutil.Ok(t, fc.Rewrite([]byte(updatedConfig)))

	// Wait for reload to take effect
	require.Eventually(t, func() bool {
		rules := *bf.rules.Load()
		return len(rules) == 2
	}, 5*time.Second, 100*time.Millisecond)
}

func TestBlocklistFilter_HotReload_InvalidConfig(t *testing.T) {
	t.Parallel()

	initialConfig := `
- "__name__:metric_a"
`
	invalidConfig := `not a valid yaml list`

	dir := t.TempDir()
	configPath := filepath.Join(dir, "blocklist.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(initialConfig), 0644))

	fc, err := extkingpin.NewStaticPathContent(configPath)
	require.NoError(t, err)

	bf, err := NewBlocklistFilter(fc, nil, log.NewNopLogger(), 1*time.Second)
	require.NoError(t, err)

	// Verify initial config
	rules := *bf.rules.Load()
	require.Len(t, rules, 1)

	// Start the reloader
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	testutil.Ok(t, bf.StartConfigReloader(ctx))

	// Write invalid config — should not affect the loaded rules
	testutil.Ok(t, fc.Rewrite([]byte(invalidConfig)))
	require.Never(t, func() bool {
		rules := *bf.rules.Load()
		return len(rules) != 1
	}, 3*time.Second, 100*time.Millisecond)
}

func TestBlocklistFilter_CanReload(t *testing.T) {
	t.Parallel()

	// nil config
	bf, err := NewBlocklistFilter(nil, nil, log.NewNopLogger(), 1*time.Second)
	require.NoError(t, err)
	require.False(t, bf.CanReload())

	// zero timer
	dir := t.TempDir()
	configPath := filepath.Join(dir, "blocklist.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("[]"), 0644))

	fc, err := extkingpin.NewStaticPathContent(configPath)
	require.NoError(t, err)

	bf, err = NewBlocklistFilter(fc, nil, log.NewNopLogger(), 0)
	require.NoError(t, err)
	require.False(t, bf.CanReload())

	// valid path + timer
	bf, err = NewBlocklistFilter(fc, nil, log.NewNopLogger(), 1*time.Second)
	require.NoError(t, err)
	require.True(t, bf.CanReload())
}
