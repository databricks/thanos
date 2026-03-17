// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package queryfrontend

import (
	gogoproto "github.com/gogo/protobuf/proto"

	"github.com/thanos-io/thanos/internal/cortex/querier/queryrange"
)

// Register response types with the gogo proto registry so that the cortex
// cache layer (which uses gogo's types.MarshalAny / types.EmptyAny) can
// round-trip these messages through protobuf Any.
func init() {
	gogoproto.RegisterType((*ThanosLabelsResponse)(nil), "queryfrontend.ThanosLabelsResponse")
	gogoproto.RegisterType((*ThanosSeriesResponse)(nil), "queryfrontend.ThanosSeriesResponse")
}

// Marshal/Unmarshal bridge methods allow gogo protobuf's types.MarshalAny /
// types.UnmarshalAny (used by the cortex cache layer) to round-trip these
// protoc-gen-go messages without falling back to gogo's reflection-based
// codec, which is incompatible with the protoc-gen-go struct layout.

func (m *ThanosLabelsResponse) Marshal() ([]byte, error) { return m.MarshalVT() }
func (m *ThanosLabelsResponse) Unmarshal(b []byte) error { return m.UnmarshalVT(b) }
func (m *ThanosSeriesResponse) Marshal() ([]byte, error) { return m.MarshalVT() }
func (m *ThanosSeriesResponse) Unmarshal(b []byte) error { return m.UnmarshalVT(b) }
func (m *ResponseHeader) Marshal() ([]byte, error)       { return m.MarshalVT() }
func (m *ResponseHeader) Unmarshal(b []byte) error       { return m.UnmarshalVT(b) }

// ThanosResponseExtractor helps to extract specific info from Query Response.
type ThanosResponseExtractor struct{}

// Extract extracts response for specific a range from a response.
// This interface is not used for labels and series responses.
func (ThanosResponseExtractor) Extract(_, _ int64, resp queryrange.Response) queryrange.Response {
	return resp
}

// ResponseWithoutHeaders returns the response without HTTP headers.
func (ThanosResponseExtractor) ResponseWithoutHeaders(resp queryrange.Response) queryrange.Response {
	switch tr := resp.(type) {
	case *ThanosLabelsResponse:
		return &ThanosLabelsResponse{Status: queryrange.StatusSuccess, Data: tr.Data}
	case *ThanosSeriesResponse:
		return &ThanosSeriesResponse{Status: queryrange.StatusSuccess, Data: tr.Data}
	}
	return resp
}

func (ThanosResponseExtractor) ResponseWithoutStats(resp queryrange.Response) queryrange.Response {
	switch tr := resp.(type) {
	case *ThanosLabelsResponse:
		return &ThanosLabelsResponse{Status: queryrange.StatusSuccess, Data: tr.Data}
	case *ThanosSeriesResponse:
		return &ThanosSeriesResponse{Status: queryrange.StatusSuccess, Data: tr.Data}
	}
	return resp
}

func headersToQueryRangeHeaders(headers []*ResponseHeader) []*queryrange.PrometheusResponseHeader {
	result := make([]*queryrange.PrometheusResponseHeader, len(headers))
	for i, h := range headers {
		result[i] = &queryrange.PrometheusResponseHeader{
			Name:   h.Name,
			Values: h.Values,
		}
	}
	return result
}

func (m *ThanosLabelsResponse) PrometheusHeaders() []*queryrange.PrometheusResponseHeader {
	return headersToQueryRangeHeaders(m.Headers)
}

func (m *ThanosSeriesResponse) PrometheusHeaders() []*queryrange.PrometheusResponseHeader {
	return headersToQueryRangeHeaders(m.Headers)
}

// GetStats returns response stats. Unimplemented for ThanosLabelsResponse.
func (m *ThanosLabelsResponse) GetStats() *queryrange.PrometheusResponseStats {
	return nil
}

// GetStats returns response stats. Unimplemented for ThanosSeriesResponse.
func (m *ThanosSeriesResponse) GetStats() *queryrange.PrometheusResponseStats {
	return nil
}
