// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package prompb

import (
	gogoproto "github.com/gogo/protobuf/proto"
	"github.com/prometheus/prometheus/model/histogram"
)

func init() {
	gogoproto.RegisterType((*ChunkedReadResponse)(nil), "prometheus_copy.ChunkedReadResponse")
}

// Marshal/Unmarshal bridge methods allow gogo protobuf's proto.Unmarshal
// (used by Prometheus's ChunkedReader.NextProto) to decode protoc-gen-go
// messages without falling back to gogo's reflection-based codec.
func (m *ChunkedReadResponse) Marshal() ([]byte, error) { return m.MarshalVT() }
func (m *ChunkedReadResponse) Unmarshal(b []byte) error { return m.UnmarshalVT(b) }

func (h *Histogram) IsFloatHistogram() bool {
	_, ok := h.GetCount().(*Histogram_CountFloat)
	return ok
}

func FromProtoHistogram(h *Histogram) *histogram.FloatHistogram {
	if h.IsFloatHistogram() {
		return FloatHistogramProtoToFloatHistogram(h)
	} else {
		return HistogramProtoToFloatHistogram(h)
	}
}
