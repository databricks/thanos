// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package receive

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/efficientgo/core/testutil"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
	"github.com/thanos-io/thanos/pkg/extkingpin"
)

func TestMetricBlocklist_ConfigReload(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	tempFilePath := filepath.Join(tempDir, "blocklist.yaml")

	initialConfig := `rules:
  - name: "block test metric"
    filter: "__name__:test_metric_*"
    drop: true
`
	err := os.WriteFile(tempFilePath, []byte(initialConfig), 0644)
	testutil.Ok(t, err)

	configFile, err := extkingpin.NewStaticPathContent(tempFilePath)
	testutil.Ok(t, err)

	blocklist, err := NewMetricBlocklist(configFile, nil, log.NewLogfmtLogger(os.Stdout), 1*time.Second)
	testutil.Ok(t, err)

	// Verify initial config loaded.
	config := blocklist.Config()
	testutil.Equals(t, 1, len(config.Rules))
	testutil.Equals(t, "block test metric", config.Rules[0].Name)
	testutil.Equals(t, "__name__:test_metric_*", config.Rules[0].Filter)
	testutil.Equals(t, true, config.Rules[0].Drop)

	// Start reloader in background.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = blocklist.StartConfigReloader(ctx)
	testutil.Ok(t, err)

	// Update config file using Rewrite to properly trigger the file watcher.
	updatedConfig := []byte(`rules:
  - name: "block new metric"
    filter: "__name__:new_metric_*"
    drop: true
  - name: "block another metric"
    filter: "__name__:another_metric"
    drop: true
`)
	testutil.Ok(t, configFile.Rewrite(updatedConfig))

	// Wait for reload using Eventually pattern.
	expectedConfig := BlocklistConfig{
		Rules: []BlocklistRule{
			{Name: "block new metric", Filter: "__name__:new_metric_*", Drop: true},
			{Name: "block another metric", Filter: "__name__:another_metric", Drop: true},
		},
	}
	require.Eventually(t, func() bool {
		return reflect.DeepEqual(expectedConfig, blocklist.Config())
	}, 5*time.Second, 100*time.Millisecond)
}

func TestMetricBlocklist_CanReload(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	tempFilePath := filepath.Join(tempDir, "blocklist.yaml")

	err := os.WriteFile(tempFilePath, []byte("rules: []"), 0644)
	testutil.Ok(t, err)

	tests := []struct {
		name       string
		configFile fileContent
		reloadTime time.Duration
		wantReload bool
	}{
		{
			name:       "can reload with file path and reload timer",
			configFile: mustCreateStaticPathContent(t, tempFilePath),
			reloadTime: 1 * time.Second,
			wantReload: true,
		},
		{
			name:       "cannot reload with zero reload timer",
			configFile: mustCreateStaticPathContent(t, tempFilePath),
			reloadTime: 0,
			wantReload: false,
		},
		{
			name:       "cannot reload with nil config file",
			configFile: nil,
			reloadTime: 1 * time.Second,
			wantReload: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blocklist, err := NewMetricBlocklist(tt.configFile, nil, log.NewLogfmtLogger(os.Stdout), tt.reloadTime)
			testutil.Ok(t, err)
			if tt.wantReload {
				testutil.Assert(t, blocklist.CanReload(), "expected CanReload to return true")
			} else {
				testutil.Assert(t, !blocklist.CanReload(), "expected CanReload to return false")
			}
		})
	}
}

func TestMetricBlocklist_OnlyDropRulesLoaded(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	tempFilePath := filepath.Join(tempDir, "blocklist.yaml")

	// Config with both drop and non-drop rules.
	config := `rules:
  - name: "drop rule"
    filter: "__name__:drop_metric"
    drop: true
  - name: "keep rule (should be filtered out)"
    filter: "__name__:keep_metric"
    drop: false
  - name: "another drop rule"
    filter: "__name__:another_drop"
    drop: true
`
	err := os.WriteFile(tempFilePath, []byte(config), 0644)
	testutil.Ok(t, err)

	configFile, err := extkingpin.NewStaticPathContent(tempFilePath)
	testutil.Ok(t, err)

	blocklist, err := NewMetricBlocklist(configFile, nil, log.NewLogfmtLogger(os.Stdout), 0)
	testutil.Ok(t, err)

	// Only drop rules should be loaded.
	loadedConfig := blocklist.Config()
	testutil.Equals(t, 2, len(loadedConfig.Rules))
	testutil.Equals(t, "drop rule", loadedConfig.Rules[0].Name)
	testutil.Equals(t, "another drop rule", loadedConfig.Rules[1].Name)
}

func TestMetricBlocklist_EmptyConfig(t *testing.T) {
	t.Parallel()

	blocklist, err := NewMetricBlocklist(nil, nil, log.NewLogfmtLogger(os.Stdout), 0)
	testutil.Ok(t, err)

	config := blocklist.Config()
	testutil.Equals(t, 0, len(config.Rules))
}

func TestMetricBlocklist_NilBlocklist(t *testing.T) {
	t.Parallel()

	var blocklist *MetricBlocklist

	// Should not panic and return empty config.
	config := blocklist.Config()
	testutil.Equals(t, 0, len(config.Rules))

	// ShouldBlock should return false for nil blocklist.
	blocked, ruleName := blocklist.ShouldBlock(nil)
	testutil.Assert(t, !blocked, "expected ShouldBlock to return false for nil blocklist")
	testutil.Equals(t, "", ruleName)
}

func mustCreateStaticPathContent(t *testing.T, path string) fileContent {
	t.Helper()
	content, err := extkingpin.NewStaticPathContent(path)
	testutil.Ok(t, err)
	return content
}
