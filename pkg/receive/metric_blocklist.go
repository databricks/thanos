// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package receive

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/thanos-io/thanos/pkg/extkingpin"
	"go.uber.org/atomic"
	"gopkg.in/yaml.v2"
)

// BlocklistRule represents a single metric blocking rule.
// This matches the m3 coordinator mapping rule format.
type BlocklistRule struct {
	// Name is a human-readable identifier for the rule.
	Name string `yaml:"name"`
	// Filter is the filter expression in m3 coordinator format.
	// Format: "__name__:metric_pattern label1:value1 label2:value2"
	// Should support glob patterns with '*' for prefix/suffix matching.
	Filter string `yaml:"filter"`
	// Drop indicates whether matching metrics should be dropped.
	// Only rules with drop=true are considered blocking rules.
	Drop bool `yaml:"drop"`
}

// BlocklistConfig is a collection of blocklist rules.
type BlocklistConfig struct {
	Rules []BlocklistRule `yaml:"rules"`
}

// MetricBlocklist is responsible for managing metric blocklist configuration
// and evaluating whether metrics should be blocked from ingestion.
// The configuration supports hot-reloading via atomic pointer swap.
type MetricBlocklist struct {
	configPathOrContent       fileContent
	config                    *atomic.Pointer[BlocklistConfig]
	logger                    log.Logger
	configReloadCounter       prometheus.Counter
	configReloadFailedCounter prometheus.Counter
	configReloadTimer         time.Duration

	// Metrics for blocked writes.
	blockedSeriesTotal *prometheus.CounterVec
}

// MetricBlocklistMetrics holds Prometheus metrics for the blocklist.
type MetricBlocklistMetrics struct {
	BlockedSeriesTotal *prometheus.CounterVec
}

// NewMetricBlocklistMetrics creates metrics for the metric blocklist.
func NewMetricBlocklistMetrics(reg prometheus.Registerer) *MetricBlocklistMetrics {
	return &MetricBlocklistMetrics{
		BlockedSeriesTotal: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "thanos",
				Subsystem: "receive",
				Name:      "blocked_series_total",
				Help:      "Total number of series blocked by blocklist rules.",
			},
			[]string{"rule_name", "tenant"},
		),
	}
}

// NewMetricBlocklist creates a new MetricBlocklist and loads the configuration.
func NewMetricBlocklist(
	configFile fileContent,
	reg prometheus.Registerer,
	logger log.Logger,
	configReloadTimer time.Duration,
) (*MetricBlocklist, error) {
	var config atomic.Pointer[BlocklistConfig]
	config.Store(&BlocklistConfig{})

	blocklist := &MetricBlocklist{
		configPathOrContent: configFile,
		config:              &config,
		logger:              logger,
		configReloadTimer:   configReloadTimer,
	}

	if reg != nil {
		blocklist.configReloadCounter = promauto.With(reg).NewCounter(
			prometheus.CounterOpts{
				Namespace: "thanos",
				Subsystem: "receive",
				Name:      "blocklist_config_reload_total",
				Help:      "Total number of blocklist configuration reloads.",
			},
		)
		blocklist.configReloadFailedCounter = promauto.With(reg).NewCounter(
			prometheus.CounterOpts{
				Namespace: "thanos",
				Subsystem: "receive",
				Name:      "blocklist_config_reload_err_total",
				Help:      "Total number of failed blocklist configuration reloads.",
			},
		)
		blocklist.blockedSeriesTotal = promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "thanos",
				Subsystem: "receive",
				Name:      "blocked_series_total",
				Help:      "Total number of series blocked by blocklist rules.",
			},
			[]string{"rule_name", "tenant"},
		)
	}

	if configFile == nil {
		return blocklist, nil
	}

	if err := blocklist.loadConfig(); err != nil {
		return nil, errors.Wrap(err, "load blocklist config")
	}

	return blocklist, nil
}

// Config returns the current blocklist configuration.
// This is concurrent safe.
func (b *MetricBlocklist) Config() BlocklistConfig {
	if b == nil {
		return BlocklistConfig{}
	}
	return *b.config.Load()
}

// setConfig sets the blocklist config atomically.
func (b *MetricBlocklist) setConfig(config BlocklistConfig) {
	b.config.Store(&config)
}

// loadConfig loads the blocklist configuration from the configured source.
func (b *MetricBlocklist) loadConfig() error {
	content, err := b.configPathOrContent.Content()
	if err != nil {
		// If file does not exist, set an empty config.
		if errors.Is(err, os.ErrNotExist) {
			level.Debug(b.logger).Log("msg", "blocklist config file does not exist")
			b.setConfig(BlocklistConfig{})
			return nil
		}
		return errors.Wrap(err, "getting content of blocklist config")
	}

	var config BlocklistConfig
	if err := yaml.UnmarshalStrict(content, &config); err != nil {
		return errors.Wrap(err, "parsing blocklist config")
	}

	// Filter to only include drop rules.
	dropRules := make([]BlocklistRule, 0, len(config.Rules))
	for _, rule := range config.Rules {
		if rule.Drop {
			dropRules = append(dropRules, rule)
		}
	}
	config.Rules = dropRules

	level.Info(b.logger).Log("msg", "loaded blocklist config", "num_rules", len(config.Rules))
	b.setConfig(config)
	return nil
}

// StartConfigReloader starts the automatic configuration reloader.
func (b *MetricBlocklist) StartConfigReloader(ctx context.Context) error {
	if !b.CanReload() {
		return nil
	}

	return extkingpin.PathContentReloader(ctx, b.configPathOrContent, b.logger, func() {
		level.Info(b.logger).Log("msg", "reloading blocklist config")

		if err := b.loadConfig(); err != nil {
			if b.configReloadFailedCounter != nil {
				b.configReloadFailedCounter.Inc()
			}
			errMsg := fmt.Sprintf("error reloading blocklist config from %s", b.configPathOrContent.Path())
			level.Error(b.logger).Log("msg", errMsg, "err", err)
			return
		}

		if b.configReloadCounter != nil {
			b.configReloadCounter.Inc()
		}
	}, b.configReloadTimer)
}

// CanReload returns whether the blocklist can be hot-reloaded.
func (b *MetricBlocklist) CanReload() bool {
	if b.configReloadTimer == 0 {
		return false
	}
	if b.configPathOrContent == nil {
		return false
	}
	if b.configPathOrContent.Path() == "" {
		return false
	}
	return true
}

// ShouldBlock evaluates whether a time series should be blocked based on the blocklist rules.
// Returns (shouldBlock, matchedRuleName).
// NOTE: The actual filter matching logic is implemented in a separate PR.
// This method will be updated to use the filter matching implementation.
func (b *MetricBlocklist) ShouldBlock(lset labels.Labels) (bool, string) {
	if b == nil {
		return false, ""
	}

	config := b.Config()
	if len(config.Rules) == 0 {
		return false, ""
	}

	for _, rule := range config.Rules {
		if matchesFilter(lset, rule.Filter) {
			return true, rule.Name
		}
	}

	return false, ""
}

// RecordBlocked records a blocked series metric.
func (b *MetricBlocklist) RecordBlocked(ruleName, tenant string) {
	if b == nil || b.blockedSeriesTotal == nil {
		return
	}
	b.blockedSeriesTotal.WithLabelValues(ruleName, tenant).Inc()
}

// matchesFilter checks if a label set matches the given filter expression.
// NOTE: This is a placeholder. The actual implementation is in a separate PR.
// Filter format: "__name__:metric_pattern label1:value1 label2:*".
func matchesFilter(lset labels.Labels, filter string) bool {
	// TODO: Implement filter matching logic in Yuchen's PR.
	// The filter matching will parse the filter string and check if the label set matches.
	// It should support:
	// - Exact matches: "label:value"
	// - Glob patterns: "label:prefix*", "label:*suffix", "label:*"
	// - Multiple label conditions (AND logic)
	return false
}
