// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package dedup

import (
	"math"

	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
)

// quorumSeries is a storage.Series that implements quorum algorithm.
// when replicas has conflict values at the same timestamp, the value in majority replica will be selected.
type quorumSeries struct {
	lset     labels.Labels
	replicas []storage.Series

	disablePenalty bool
	isCounter      bool
}

func NewQuorumSeries(lset labels.Labels, replicas []storage.Series, f string) storage.Series {
	return &quorumSeries{
		lset:           lset,
		replicas:       replicas,
		disablePenalty: true, // Default to no penalty for receiver-only setups
		isCounter:      isCounter(f),
	}
}

// NewQuorumSeriesWithPenalty creates a quorum series with configurable penalty behavior
func NewQuorumSeriesWithPenalty(lset labels.Labels, replicas []storage.Series, f string, disablePenalty bool) storage.Series {
	return &quorumSeries{
		lset:           lset,
		replicas:       replicas,
		disablePenalty: disablePenalty,
		isCounter:      isCounter(f),
	}
}

func (m *quorumSeries) Labels() labels.Labels {
	return m.lset
}

func (m *quorumSeries) Iterator(_ chunkenc.Iterator) chunkenc.Iterator {
	iters := make([]adjustableSeriesIterator, 0, len(m.replicas))
	oks := make([]bool, 0, len(m.replicas))
	for _, r := range m.replicas {
		var it adjustableSeriesIterator
		if m.isCounter {
			it = &counterErrAdjustSeriesIterator{Iterator: r.Iterator(nil)}
		} else {
			it = &noopAdjustableSeriesIterator{Iterator: r.Iterator(nil)}
		}
		ok := it.Next() != chunkenc.ValNone // iterate to the first value.
		iters = append(iters, it)
		oks = append(oks, ok)
	}
	return &quorumSeriesIterator{
		iters:          iters,
		oks:            oks,
		lastT:          math.MinInt64,
		lastIter:       nil, // behavior is undefined if At() is called before Next(), here we panic if it happens.
		disablePenalty: m.disablePenalty,
	}
}

type quorumValuePicker struct {
	currentValue float64
	cnt          int
}

func NewQuorumValuePicker(v float64) *quorumValuePicker {
	return &quorumValuePicker{
		currentValue: v,
		cnt:          1,
	}
}

// Return true if this is the new majority value.
func (q *quorumValuePicker) addValue(v float64) bool {
	if q.currentValue == v {
		q.cnt++
	} else {
		q.cnt--
		if q.cnt == 0 {
			q.currentValue = v
			q.cnt = 1
			return true
		}
	}
	return false
}

type quorumSeriesIterator struct {
	iters []adjustableSeriesIterator
	oks   []bool

	lastT    int64
	lastV    float64
	lastIter adjustableSeriesIterator

	disablePenalty bool
}

func (m *quorumSeriesIterator) Next() chunkenc.ValueType {
	if m.disablePenalty {
		return m.nextWithoutPenalty()
	}
	return m.nextWithPenalty()
}

func (m *quorumSeriesIterator) nextWithPenalty() chunkenc.ValueType {
	// Original penalty-based algorithm for backward compatibility
	minT := int64(math.MaxInt64)
	var lastIter adjustableSeriesIterator
	quorumValue := NewQuorumValuePicker(0.0)
	for i, it := range m.iters {
		if !m.oks[i] {
			continue
		}
		// apply penalty to avoid selecting samples too close
		m.oks[i] = it.Seek(m.lastT+initialPenalty) != chunkenc.ValNone
		// The it.Seek() call above should guarantee that it.AtT() > m.lastT.
		if m.oks[i] {
			// adjust the current value for counter functions to avoid unexpected resets
			it.adjustAtValue(m.lastV)
			t, v := it.At()
			if t < minT {
				minT = t
				lastIter = it
				quorumValue = NewQuorumValuePicker(v)
			} else if t == minT {
				if quorumValue.addValue(v) {
					lastIter = it
				}
			}
		}
	}
	m.lastIter = lastIter
	if m.lastIter == nil {
		return chunkenc.ValNone
	}
	m.lastV = quorumValue.currentValue
	m.lastT = minT
	return chunkenc.ValFloat
}

func (m *quorumSeriesIterator) nextWithoutPenalty() chunkenc.ValueType {
	// Find minimum timestamp across all active iterators without applying penalties
	minT := int64(math.MaxInt64)
	var lastIter adjustableSeriesIterator
	quorumValue := NewQuorumValuePicker(0.0)

	for i, it := range m.iters {
		if !m.oks[i] {
			continue
		}
		t, v := it.At()
		if t <= m.lastT {
			// Move to next value if current is not newer
			m.oks[i] = it.Next() != chunkenc.ValNone
			if m.oks[i] {
				it.adjustAtValue(m.lastV)
				t, v = it.At()
			} else {
				continue
			}
		}
		if t < minT {
			minT = t
			lastIter = it
			quorumValue = NewQuorumValuePicker(v)
		} else if t == minT {
			if quorumValue.addValue(v) {
				lastIter = it
			}
		}
	}

	m.lastIter = lastIter
	if m.lastIter == nil {
		return chunkenc.ValNone
	}
	m.lastV = quorumValue.currentValue
	m.lastT = minT
	return chunkenc.ValFloat
}

func (m *quorumSeriesIterator) Seek(t int64) chunkenc.ValueType {
	// Don't use underlying Seek, but iterate over next to not miss gaps.
	for m.lastT < t && m.Next() != chunkenc.ValNone {
	}
	// Don't call m.Next() again!
	if m.lastIter == nil {
		return chunkenc.ValNone
	}
	return chunkenc.ValFloat
}

func (m *quorumSeriesIterator) At() (t int64, v float64) {
	return m.lastT, m.lastV
}

func (m *quorumSeriesIterator) AtHistogram(h *histogram.Histogram) (int64, *histogram.Histogram) {
	return m.lastIter.AtHistogram(h)
}

func (m *quorumSeriesIterator) AtFloatHistogram(fh *histogram.FloatHistogram) (int64, *histogram.FloatHistogram) {
	return m.lastIter.AtFloatHistogram(fh)
}

func (m *quorumSeriesIterator) AtT() int64 {
	return m.lastT
}

// Err All At() funcs should panic if called after Next() or Seek() return ValNone.
// Only Err() should return nil even after Next() or Seek() return ValNone.
func (m *quorumSeriesIterator) Err() error {
	if m.lastIter == nil {
		return nil
	}
	return m.lastIter.Err()
}
