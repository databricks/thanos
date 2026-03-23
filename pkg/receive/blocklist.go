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
	"github.com/thanos-io/thanos/pkg/extkingpin"
	"github.com/thanos-io/thanos/pkg/store/labelpb"
	"github.com/thanos-io/thanos/pkg/store/storepb/prompb"
	"go.uber.org/atomic"
	"gopkg.in/yaml.v2"
)

// blocklistRules is a slice of compiled TagsFilter rules used by BlocklistFilter.
type blocklistRules []TagsFilter

// BlocklistFilter filters time series against a set of M3-style filter rules.
// Any time series matching any rule is dropped. The filter supports hot-reload
// of its configuration via an atomic pointer swap, following the same pattern
// as Relabeller.
type BlocklistFilter struct {
	configPathOrContent       fileContent
	rules                     *atomic.Pointer[blocklistRules]
	logger                    log.Logger
	configReloadTimer         time.Duration
	configReloadCounter       prometheus.Counter
	configReloadFailedCounter prometheus.Counter

	droppedSeriesTotal prometheus.Counter
	activeRulesGauge   prometheus.Gauge
}

// NewBlocklistFilter creates a new BlocklistFilter and loads the initial configuration.
// If configFile is nil, the filter is a no-op (drops nothing).
func NewBlocklistFilter(configFile fileContent, reg prometheus.Registerer, logger log.Logger, configReloadTimer time.Duration) (*BlocklistFilter, error) {
	var rules atomic.Pointer[blocklistRules]
	empty := blocklistRules{}
	rules.Store(&empty)

	bf := &BlocklistFilter{
		configPathOrContent: configFile,
		rules:               &rules,
		logger:              logger,
		configReloadTimer:   configReloadTimer,
	}

	if reg != nil {
		bf.configReloadCounter = promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Namespace: "thanos",
			Subsystem: "receive",
			Name:      "blocklist_config_reload_total",
			Help:      "Total number of blocklist configuration reloads.",
		})
		bf.configReloadFailedCounter = promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Namespace: "thanos",
			Subsystem: "receive",
			Name:      "blocklist_config_reload_err_total",
			Help:      "Total number of failed blocklist configuration reloads.",
		})
		bf.droppedSeriesTotal = promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Namespace: "thanos",
			Subsystem: "receive",
			Name:      "blocklist_dropped_series_total",
			Help:      "Total number of time series dropped by the blocklist filter.",
		})
		bf.activeRulesGauge = promauto.With(reg).NewGauge(prometheus.GaugeOpts{
			Namespace: "thanos",
			Subsystem: "receive",
			Name:      "blocklist_active_rules",
			Help:      "Number of active blocklist filter rules.",
		})
	}

	if configFile == nil {
		return bf, nil
	}

	if err := bf.loadConfig(); err != nil {
		return nil, errors.Wrap(err, "load blocklist config")
	}

	return bf, nil
}

// loadConfig reads and parses the blocklist config from the configured source.
func (bf *BlocklistFilter) loadConfig() error {
	contentYaml, err := bf.configPathOrContent.Content()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			level.Debug(bf.logger).Log("msg", "blocklist config file does not exist")
			empty := blocklistRules{}
			bf.rules.Store(&empty)
			bf.setActiveRulesGauge(0)
			return nil
		}
		return errors.Wrap(err, "getting content of blocklist config")
	}

	rules, err := parseBlocklistConfig(contentYaml)
	if err != nil {
		return errors.Wrap(err, "parsing blocklist config")
	}

	bf.rules.Store(&rules)
	bf.setActiveRulesGauge(len(rules))
	level.Info(bf.logger).Log("msg", "blocklist config loaded", "rules", len(rules))
	return nil
}

// parseBlocklistConfig parses a YAML list of M3 filter strings and compiles them.
func parseBlocklistConfig(data []byte) (blocklistRules, error) {
	var filterStrings []string
	if err := yaml.Unmarshal(data, &filterStrings); err != nil {
		return nil, fmt.Errorf("unmarshaling blocklist config: %w", err)
	}

	rules := make(blocklistRules, 0, len(filterStrings))
	for i, fs := range filterStrings {
		if fs == "" {
			continue
		}
		filterValues, err := ParseTagFilterValueMap(fs)
		if err != nil {
			return nil, fmt.Errorf("rule %d: parsing filter %q: %w", i, fs, err)
		}
		filter, err := NewTagsFilter(filterValues, Conjunction, TagsFilterOptions{})
		if err != nil {
			return nil, fmt.Errorf("rule %d: creating filter %q: %w", i, fs, err)
		}
		rules = append(rules, filter)
	}
	return rules, nil
}

// FilterTimeSeries removes any time series from the write request that match
// any blocklist rule. It returns the number of series dropped.
func (bf *BlocklistFilter) FilterTimeSeries(wreq *prompb.WriteRequest) int {
	if bf == nil {
		return 0
	}

	rules := *bf.rules.Load()
	if len(rules) == 0 {
		return 0
	}

	dropped := 0
	for i, ts := range wreq.Timeseries {
		lbls := labelpb.ZLabelsToPromLabels(ts.Labels)
		blocked := false
		for _, rule := range rules {
			if rule.MatchLabels(lbls) {
				blocked = true
				break
			}
		}
		if blocked {
			dropped++
		} else if dropped > 0 {
			wreq.Timeseries[i-dropped] = ts
		}
	}

	if dropped > 0 {
		wreq.Timeseries = wreq.Timeseries[:len(wreq.Timeseries)-dropped]
		if bf.droppedSeriesTotal != nil {
			bf.droppedSeriesTotal.Add(float64(dropped))
		}
	}
	return dropped
}

// StartConfigReloader starts the automatic configuration reloader based off of
// the file indicated by the configured path/content source.
func (bf *BlocklistFilter) StartConfigReloader(ctx context.Context) error {
	if !bf.CanReload() {
		return nil
	}

	return extkingpin.PathContentReloader(ctx, bf.configPathOrContent, bf.logger, func() {
		level.Info(bf.logger).Log("msg", "reloading blocklist config")

		if err := bf.loadConfig(); err != nil {
			if bf.configReloadFailedCounter != nil {
				bf.configReloadFailedCounter.Inc()
			}
			errMsg := fmt.Sprintf("error reloading blocklist config from %s", bf.configPathOrContent.Path())
			level.Error(bf.logger).Log("msg", errMsg, "err", err)
		}
		if bf.configReloadCounter != nil {
			bf.configReloadCounter.Inc()
		}
	}, bf.configReloadTimer)
}

// CanReload returns true if the filter is configured with a file path and a reload timer.
func (bf *BlocklistFilter) CanReload() bool {
	if bf.configReloadTimer == 0 {
		return false
	}
	if bf.configPathOrContent == nil {
		return false
	}
	if bf.configPathOrContent.Path() == "" {
		return false
	}
	return true
}

func (bf *BlocklistFilter) setActiveRulesGauge(n int) {
	if bf.activeRulesGauge != nil {
		bf.activeRulesGauge.Set(float64(n))
	}
}
