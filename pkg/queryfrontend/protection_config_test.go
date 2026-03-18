// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

// Tests for protection config loading and file watching:
// LoadRulesFromFile (YAML parsing and rule construction) and WatchConfig (polling reload on file change).
package queryfrontend

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
)

func writeConfigFile(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "protection.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

func TestLoadRulesFromFile_FileNotFound(t *testing.T) {
	_, err := loadRulesFromFile("/nonexistent/path/protection.yaml")
	require.Error(t, err)
}

func TestLoadRulesFromFile_Valid(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigFile(t, dir, `
rules:
  - name: noop-log
    protection: noop
    action: log
    actor: ".*"
    enabled: true
  - name: noop-block
    protection: noop
    action: block
    actor: "^admin$"
    enabled: false
`)

	rules, err := loadRulesFromFile(path)
	require.NoError(t, err)
	require.Len(t, rules, 2)
	require.Equal(t, "noop-log", rules[0].name)
	require.Equal(t, RuleActionLog, rules[0].action)
	require.True(t, rules[0].enabled)
	require.Equal(t, "noop-block", rules[1].name)
	require.Equal(t, RuleActionBlock, rules[1].action)
	require.False(t, rules[1].enabled)
}

func TestLoadRulesFromFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigFile(t, dir, `
rules:
  - name: bad
    protection: noop
    action: log
    actor: ".*"
    args:
      - foo
      - bar
`)

	_, err := loadRulesFromFile(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse protection config")
}

func TestLoadRulesFromFile_UnknownProtection(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigFile(t, dir, `
rules:
  - name: bad
    protection: nonexistent
    action: log
    actor: ".*"
    enabled: true
`)

	_, err := loadRulesFromFile(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "nonexistent")
}

func TestLoadRulesFromFile_UnknownAction(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigFile(t, dir, `
rules:
  - name: bad
    protection: noop
    action: unknown
    actor: ".*"
    enabled: true
`)

	_, err := loadRulesFromFile(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown action")
}

func TestLoadRulesFromFile_EmptyName(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigFile(t, dir, `
rules:
  - protection: noop
    action: log
    actor: ".*"
    enabled: true
`)

	_, err := loadRulesFromFile(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "name is required")
}

func TestLoadRulesFromFile_EmptyActor(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigFile(t, dir, `
rules:
  - name: bad
    protection: noop
    action: log
    enabled: true
`)

	_, err := loadRulesFromFile(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "actor is required")
}

func TestLoadRulesFromFile_InvalidActorRegex(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigFile(t, dir, `
rules:
  - name: bad
    protection: noop
    action: log
    actor: "["
    enabled: true
`)

	_, err := loadRulesFromFile(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "compile actor regex")
}

func TestWatchConfig_InitialLoad(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigFile(t, dir, `
rules:
  - name: initial-rule
    protection: noop
    action: log
    actor: ".*"
    enabled: true
`)

	engine := NewProtectionEngine(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so WatchConfig returns after initial load

	err := WatchConfig(ctx, engine, path, log.NewNopLogger(), time.Hour)
	require.NoError(t, err)

	// Verify the rule was loaded by evaluating a request.
	result, err := engine.Evaluate(context.Background(), thanosQueryReq{actor: "anyone"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "initial-rule", result.RuleName)
}

func TestWatchConfig_InitialLoadFailure(t *testing.T) {
	engine := NewProtectionEngine(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := WatchConfig(ctx, engine, "/nonexistent/path.yaml", log.NewNopLogger(), time.Hour)
	require.Error(t, err)
}

func TestWatchConfig_ReloadsOnFileChange(t *testing.T) {
	dir := t.TempDir()
	path := writeConfigFile(t, dir, `
rules:
  - name: rule-v1
    protection: noop
    action: log
    actor: ".*"
    enabled: true
`)

	engine := NewProtectionEngine(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run WatchConfig with a short interval in the background.
	done := make(chan error, 1)
	go func() {
		done <- WatchConfig(ctx, engine, path, log.NewNopLogger(), 20*time.Millisecond)
	}()

	// Wait for initial load: rule-v1 should be active.
	require.Eventually(t, func() bool {
		result, err := engine.Evaluate(context.Background(), thanosQueryReq{actor: "anyone"})
		return err == nil && result != nil && result.RuleName == "rule-v1"
	}, time.Second, 5*time.Millisecond)

	// Update the file: rename the rule to rule-v2.
	require.NoError(t, os.WriteFile(path, []byte(`
rules:
  - name: rule-v2
    protection: noop
    action: block
    actor: ".*"
    enabled: true
`), 0600))

	// Wait for the reload to pick up the new rule name.
	require.Eventually(t, func() bool {
		result, err := engine.Evaluate(context.Background(), thanosQueryReq{actor: "anyone"})
		return err == nil && result != nil && result.RuleName == "rule-v2"
	}, time.Second, 5*time.Millisecond)

	cancel()
	require.NoError(t, <-done)
}
