// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package store

import (
	"github.com/thanos-io/thanos/pkg/store/storepb"
)

// batchableServer wraps a storepb.Store_SeriesServer and batches series responses
// to reduce gRPC overhead. When the batch size is reached, it sends all buffered
// series as a single SeriesBatch response.
type batchableServer struct {
	storepb.Store_SeriesServer
	batchSize int
	series    []storepb.Series
}

func newBatchableServer(upstream storepb.Store_SeriesServer, batchSize int) storepb.Store_SeriesServer {
	if batchSize <= 1 {
		return &passthroughServer{Store_SeriesServer: upstream}
	}
	return &batchableServer{
		Store_SeriesServer: upstream,
		batchSize:          batchSize,
		series:             make([]storepb.Series, 0, batchSize),
	}
}

// Send buffers series responses and sends them as batches when the buffer is full.
// Non-series responses (warnings, hints) trigger an immediate flush before being sent.
func (b *batchableServer) Send(response *storepb.SeriesResponse) error {
	series := response.GetSeries()
	if series == nil {
		// Non-series response (warning/hints): flush batch first, then send
		if err := b.Flush(); err != nil {
			return err
		}
		return b.Store_SeriesServer.Send(response)
	}

	b.series = append(b.series, *series)
	if len(b.series) >= b.batchSize {
		return b.Flush()
	}
	return nil
}

// Flush sends any buffered series as a batch response.
// Implements the flushableServer interface.
func (b *batchableServer) Flush() error {
	if len(b.series) == 0 {
		return nil
	}
	if err := b.Store_SeriesServer.Send(storepb.NewBatchResponse(b.series)); err != nil {
		return err
	}
	b.series = b.series[:0]
	return nil
}
