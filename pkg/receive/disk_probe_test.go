// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package receive

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/efficientgo/core/testutil"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	promtest "github.com/prometheus/client_golang/prometheus/testutil"
)

func TestDiskProbe_SingleProbe(t *testing.T) {
	dir := t.TempDir()
	reg := prometheus.NewRegistry()

	probe := NewDiskProbe(log.NewNopLogger(), DiskProbeOptions{
		Dir:       dir,
		Interval:  1 * time.Second,
		WriteSize: 1024,
	}, reg)

	probePath := filepath.Join(dir, ".thanos_disk_probe")
	probe.probe(probePath)

	// Verify no failures occurred.
	testutil.Equals(t, float64(0), promtest.ToFloat64(probe.failTotal))
}

func TestDiskProbe_FailureOnBadDir(t *testing.T) {
	reg := prometheus.NewRegistry()

	probe := NewDiskProbe(log.NewNopLogger(), DiskProbeOptions{
		Dir:       "/nonexistent/path/that/should/not/exist",
		Interval:  1 * time.Second,
		WriteSize: 1024,
	}, reg)

	probePath := filepath.Join("/nonexistent/path/that/should/not/exist", ".thanos_disk_probe")
	probe.probe(probePath)

	// Should have recorded a failure.
	testutil.Equals(t, float64(1), promtest.ToFloat64(probe.failTotal))
}

func TestDiskProbe_RunAndStop(t *testing.T) {
	dir := t.TempDir()
	reg := prometheus.NewRegistry()

	probe := NewDiskProbe(log.NewNopLogger(), DiskProbeOptions{
		Dir:      dir,
		Interval: 100 * time.Millisecond,
	}, reg)

	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		probe.Run(stop)
		close(done)
	}()

	// Let it run a few probes.
	time.Sleep(350 * time.Millisecond)

	// Stop the probe.
	close(stop)

	// Wait for Run to return.
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after stop was closed")
	}

	// Verify no failures.
	failCount := promtest.ToFloat64(probe.failTotal)
	testutil.Equals(t, float64(0), failCount)

	// Probe file should be cleaned up.
	probePath := filepath.Join(dir, ".thanos_disk_probe")
	_, err := os.Stat(probePath)
	testutil.Assert(t, os.IsNotExist(err), "probe file should be removed on shutdown")
}

func TestDiskProbe_StuckWriteDetection(t *testing.T) {
	dir := t.TempDir()
	reg := prometheus.NewRegistry()

	probe := NewDiskProbe(log.NewNopLogger(), DiskProbeOptions{
		Dir:      dir,
		Interval: 1 * time.Hour, // Long interval so the probe loop doesn't interfere.
	}, reg)

	// Simulate a stuck write by manually setting the write state.
	probe.mu.Lock()
	probe.writing = true
	probe.writeStart = time.Now().Add(-3 * time.Second) // Pretend write started 3s ago.
	probe.mu.Unlock()

	// Start and quickly check the stuck monitor.
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		probe.monitorStuckWrites(stop)
		close(done)
	}()

	// Wait for the monitor to tick.
	time.Sleep(1500 * time.Millisecond)

	stuckDur := promtest.ToFloat64(probe.stuckDur)
	// Should report approximately 4.5s (3s initial + ~1.5s from sleep).
	testutil.Assert(t, stuckDur > 3.0, "stuck duration should be > 3s, got %f", stuckDur)

	// Clear the write state.
	probe.markDone()

	// Wait for monitor to update.
	time.Sleep(1500 * time.Millisecond)

	stuckDur = promtest.ToFloat64(probe.stuckDur)
	testutil.Equals(t, float64(0), stuckDur)

	close(stop)
	<-done
}

func TestDiskProbe_DefaultOptions(t *testing.T) {
	opts := DiskProbeOptions{Dir: "/tmp/test"}
	opts.defaults()

	testutil.Equals(t, 5*time.Second, opts.Interval)
	testutil.Equals(t, 1024, opts.WriteSize)
}

func TestDiskProbe_CustomWriteSize(t *testing.T) {
	dir := t.TempDir()
	reg := prometheus.NewRegistry()

	writeSize := 4096
	probe := NewDiskProbe(log.NewNopLogger(), DiskProbeOptions{
		Dir:       dir,
		Interval:  1 * time.Second,
		WriteSize: writeSize,
	}, reg)

	testutil.Equals(t, writeSize, len(probe.payload))

	// Run a probe and verify the file was written with the correct size.
	probePath := filepath.Join(dir, ".thanos_disk_probe")
	probe.probe(probePath)

	info, err := os.Stat(probePath)
	testutil.Ok(t, err)
	testutil.Equals(t, int64(writeSize), info.Size())

	// Clean up.
	os.Remove(probePath)
}
