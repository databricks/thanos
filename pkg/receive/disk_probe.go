// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package receive

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// DiskProbe performs synthetic write+fsync operations on the TSDB data directory
// to measure actual disk I/O latency. It exposes the results as Prometheus metrics.
//
// The probe performs the following operations:
//   - Writes a fixed payload to a file in the data directory
//   - Calls fsync to flush data to disk (bypassing OS page cache)
//   - Measures and records the duration
//   - Tracks "stuck" writes that have not yet completed
type DiskProbe struct {
	logger   log.Logger
	dir      string
	interval time.Duration
	payload  []byte

	duration  prometheus.Histogram
	stuckDur  prometheus.Gauge
	failTotal prometheus.Counter

	// mu protects writeStart for stuck-write detection.
	mu         sync.Mutex
	writeStart time.Time
	writing    bool
}

// DiskProbeOptions configures the disk probe.
type DiskProbeOptions struct {
	// Dir is the directory to probe (should be on the same filesystem as the TSDB data).
	Dir string
	// Interval between probe writes. Default: 5s.
	Interval time.Duration
	// WriteSize is the number of bytes written per probe. Default: 1024.
	WriteSize int
}

func (o *DiskProbeOptions) defaults() {
	if o.Interval == 0 {
		o.Interval = 5 * time.Second
	}
	if o.WriteSize == 0 {
		o.WriteSize = 1024
	}
}

// NewDiskProbe creates a new DiskProbe.
func NewDiskProbe(logger log.Logger, opts DiskProbeOptions, reg prometheus.Registerer) *DiskProbe {
	opts.defaults()

	payload := make([]byte, opts.WriteSize)

	return &DiskProbe{
		logger:   logger,
		dir:      opts.Dir,
		interval: opts.Interval,
		payload:  payload,
		duration: promauto.With(reg).NewHistogram(prometheus.HistogramOpts{
			Namespace: "thanos",
			Subsystem: "receive",
			Name:      "disk_probe_duration_seconds",
			Help:      "Duration of synthetic disk write+fsync probe operations on the TSDB data directory.",
			Buckets:   []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		}),
		stuckDur: promauto.With(reg).NewGauge(prometheus.GaugeOpts{
			Namespace: "thanos",
			Subsystem: "receive",
			Name:      "disk_probe_stuck_duration_seconds",
			Help:      "Duration in seconds that the current disk probe write has been in-flight. 0 if no write is stuck.",
		}),
		failTotal: promauto.With(reg).NewCounter(prometheus.CounterOpts{
			Namespace: "thanos",
			Subsystem: "receive",
			Name:      "disk_probe_failures_total",
			Help:      "Total number of disk probe failures (open, write, or fsync errors).",
		}),
	}
}

// Run starts the disk probe loop. It blocks until stop is closed.
// It runs two loops:
//   - The probe loop, which performs a write+fsync at the configured interval.
//   - The stuck-write monitor, which updates the stuck duration gauge every second.
func (p *DiskProbe) Run(stop <-chan struct{}) {
	probePath := filepath.Join(p.dir, ".thanos_disk_probe")

	// Clean up probe file on exit.
	defer os.Remove(probePath)

	// Start the stuck-write monitor in a separate goroutine.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		p.monitorStuckWrites(stop)
	}()

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			wg.Wait()
			return
		case <-ticker.C:
			p.probe(probePath)
		}
	}
}

// probe performs a single write+fsync and records the duration.
func (p *DiskProbe) probe(path string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		level.Warn(p.logger).Log("msg", "disk probe open failed", "err", err)
		p.failTotal.Inc()
		return
	}

	p.mu.Lock()
	p.writeStart = time.Now()
	p.writing = true
	p.mu.Unlock()

	start := time.Now()
	_, writeErr := f.Write(p.payload)
	if writeErr != nil {
		level.Warn(p.logger).Log("msg", "disk probe write failed", "err", writeErr)
		p.failTotal.Inc()
		f.Close()
		p.markDone()
		return
	}

	syncErr := f.Sync()
	duration := time.Since(start)

	f.Close()
	p.markDone()

	if syncErr != nil {
		level.Warn(p.logger).Log("msg", "disk probe fsync failed", "err", syncErr)
		p.failTotal.Inc()
		return
	}

	p.duration.Observe(duration.Seconds())

	if duration > 1*time.Second {
		level.Warn(p.logger).Log("msg", "slow disk probe detected", "duration", duration)
	}
}

func (p *DiskProbe) markDone() {
	p.mu.Lock()
	p.writing = false
	p.writeStart = time.Time{}
	p.mu.Unlock()
}

// monitorStuckWrites updates the stuck duration gauge every second.
// If a probe write is in-flight, it reports how long it's been stuck.
func (p *DiskProbe) monitorStuckWrites(stop <-chan struct{}) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			p.mu.Lock()
			if p.writing {
				stuckDuration := time.Since(p.writeStart)
				p.stuckDur.Set(stuckDuration.Seconds())
				if stuckDuration > 5*time.Second {
					level.Warn(p.logger).Log("msg", "disk probe write appears stuck", "stuck_duration", stuckDuration)
				}
			} else {
				p.stuckDur.Set(0)
			}
			p.mu.Unlock()
		}
	}
}
