// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package targetspb

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/thanos-io/thanos/pkg/store/labelpb"
)

func NewTargetsResponse(targets *TargetDiscovery) *TargetsResponse {
	return &TargetsResponse{
		Result: &TargetsResponse_Targets{
			Targets: targets,
		},
	}
}

func NewWarningTargetsResponse(warning error) *TargetsResponse {
	return &TargetsResponse{
		Result: &TargetsResponse_Warning{
			Warning: warning.Error(),
		},
	}
}

func (x *TargetHealth) UnmarshalJSON(entry []byte) error {
	fieldStr, err := strconv.Unquote(string(entry))
	if err != nil {
		return errors.Wrapf(err, "targetHealth: unquote %v", string(entry))
	}

	if fieldStr == "" {
		return errors.New("empty targetHealth")
	}

	state, ok := TargetHealth_value[strings.ToUpper(fieldStr)]
	if !ok {
		return errors.Errorf("unknown targetHealth: %v", string(entry))
	}
	*x = TargetHealth(state)
	return nil
}

func (x *TargetHealth) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(strings.ToLower(x.String()))), nil
}

func (x TargetHealth) Compare(y TargetHealth) int {
	return int(x) - int(y)
}

func (t1 *ActiveTarget) Compare(t2 *ActiveTarget) int {
	if d := strings.Compare(t1.ScrapeUrl, t2.ScrapeUrl); d != 0 {
		return d
	}

	if d := strings.Compare(t1.ScrapePool, t2.ScrapePool); d != 0 {
		return d
	}

	if d := labelpb.Compare(t1.DiscoveredLabels.GetLabels(), t2.DiscoveredLabels.GetLabels()); d != 0 {
		return d
	}

	if d := labelpb.Compare(t1.Labels.GetLabels(), t2.Labels.GetLabels()); d != 0 {
		return d
	}

	return 0
}

func (t1 *ActiveTarget) CompareState(t2 *ActiveTarget) int {
	if d := t1.Health.Compare(t2.Health); d != 0 {
		return d
	}

	s1, s2 := t1.LastScrape.AsTime(), t2.LastScrape.AsTime()
	if s1.Before(s2) {
		return 1
	}
	if s1.After(s2) {
		return -1
	}

	return 0
}

func (t1 *DroppedTarget) Compare(t2 *DroppedTarget) int {
	if d := labelpb.Compare(t1.DiscoveredLabels.GetLabels(), t2.DiscoveredLabels.GetLabels()); d != 0 {
		return d
	}

	return 0
}

// MarshalJSON preserves backward compatibility with the previous gogo-proto
// generated code where LastScrape was time.Time (RFC3339 string).
func (t *ActiveTarget) MarshalJSON() ([]byte, error) {
	type Alias ActiveTarget
	return json.Marshal(&struct {
		*Alias
		LastScrape time.Time `json:"lastScrape"`
	}{
		Alias:      (*Alias)(t),
		LastScrape: t.LastScrape.AsTime(),
	})
}

// UnmarshalJSON preserves backward compatibility with the previous gogo-proto
// generated code where LastScrape was time.Time (RFC3339 string).
func (t *ActiveTarget) UnmarshalJSON(data []byte) error {
	type Alias ActiveTarget
	aux := &struct {
		*Alias
		LastScrape time.Time `json:"lastScrape"`
	}{
		Alias: (*Alias)(t),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if !aux.LastScrape.IsZero() {
		t.LastScrape = timestamppb.New(aux.LastScrape)
	}
	return nil
}

func (t *ActiveTarget) SetLabels(ls labelpb.Labels) {
	if len(ls) == 0 {
		t.Labels = nil
		return
	}
	t.Labels = &labelpb.LabelSet{Labels: ls}
}

func (t *ActiveTarget) SetDiscoveredLabels(ls labelpb.Labels) {
	if len(ls) == 0 {
		t.DiscoveredLabels = nil
		return
	}
	t.DiscoveredLabels = &labelpb.LabelSet{Labels: ls}
}

func (t *DroppedTarget) SetDiscoveredLabels(ls labelpb.Labels) {
	if len(ls) == 0 {
		t.DiscoveredLabels = nil
		return
	}
	t.DiscoveredLabels = &labelpb.LabelSet{Labels: ls}
}
