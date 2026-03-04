// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package exemplarspb

import (
	"encoding/json"
	"math/big"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/labels"

	"github.com/thanos-io/thanos/pkg/store/labelpb"
)

// ExemplarStore wraps the ExemplarsClient and contains the info of external labels.
type ExemplarStore struct {
	ExemplarsClient
	LabelSets []labelpb.Labels
}

// UnmarshalJSON implements json.Unmarshaler.
func (m *Exemplar) UnmarshalJSON(b []byte) error {
	v := struct {
		Labels    *labelpb.LabelSet
		TimeStamp model.Time
		Value     model.SampleValue
	}{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}

	m.Labels = v.Labels
	m.Ts = int64(v.TimeStamp)
	m.Value = float64(v.Value)

	return nil
}

// MarshalJSON implements json.Marshaler.
func (m *Exemplar) MarshalJSON() ([]byte, error) {
	v := struct {
		Labels    labels.Labels     `json:"labels"`
		TimeStamp model.Time        `json:"timestamp"`
		Value     model.SampleValue `json:"value"`
	}{
		Labels:    m.GetLabels().PromLabels(),
		TimeStamp: model.Time(m.Ts),
		Value:     model.SampleValue(m.Value),
	}
	return json.Marshal(v)
}

func NewExemplarsResponse(e *ExemplarData) *ExemplarsResponse {
	return &ExemplarsResponse{
		Result: &ExemplarsResponse_Data{
			Data: e,
		},
	}
}

func NewWarningExemplarsResponse(warning error) *ExemplarsResponse {
	return &ExemplarsResponse{
		Result: &ExemplarsResponse_Warning{
			Warning: warning.Error(),
		},
	}
}

// Compare only compares the series labels of two exemplar data.
func (s1 *ExemplarData) Compare(s2 *ExemplarData) int {
	return labelpb.Compare(s1.SeriesLabels.GetLabels(), s2.SeriesLabels.GetLabels())
}

// Compare is used for sorting and comparing exemplars. Start from timestamp, then labels, finally values.
func (e1 *Exemplar) Compare(e2 *Exemplar) int {
	if e1.Ts < e2.Ts {
		return -1
	}
	if e1.Ts > e2.Ts {
		return 1
	}

	if d := labelpb.Compare(e1.Labels.GetLabels(), e2.Labels.GetLabels()); d != 0 {
		return d
	}
	return big.NewFloat(e1.Value).Cmp(big.NewFloat(e2.Value))
}

func ExemplarsFromPromExemplars(exemplars []exemplar.Exemplar) []*Exemplar {
	ex := make([]*Exemplar, 0, len(exemplars))
	for _, e := range exemplars {
		r := &Exemplar{
			Value: e.Value,
			Ts:    e.Ts,
		}
		if len(e.Labels) > 0 {
			r.Labels = &labelpb.LabelSet{Labels: labelpb.FromPromLabels(e.Labels)}
		}
		ex = append(ex, r)
	}
	return ex
}
