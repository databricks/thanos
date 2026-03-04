// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package rulespb

import (
	"encoding/json"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/thanos-io/thanos/pkg/store/labelpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	RuleRecordingType = "recording"
	RuleAlertingType  = "alerting"
)

func NewRuleGroupRulesResponse(rg *RuleGroup) *RulesResponse {
	return &RulesResponse{
		Result: &RulesResponse_Group{
			Group: rg,
		},
	}
}

func NewWarningRulesResponse(warning error) *RulesResponse {
	return &RulesResponse{
		Result: &RulesResponse_Warning{
			Warning: warning.Error(),
		},
	}
}

func NewRecordingRule(r *RecordingRule) *Rule {
	return &Rule{
		Result: &Rule_Recording{Recording: r},
	}
}

// Compare compares equal recording rules r1 and r2 and returns:
//
//	< 0 if r1 < r2  if rule r1 is lexically before rule r2
//	  0 if r1 == r2
//	> 0 if r1 > r2  if rule r1 is lexically after rule r2
//
// More formally, the ordering is determined in the following order:
//
// 1. recording rule last evaluation (earlier evaluation comes first)
//
// Note: This method assumes r1 and r2 are logically equal as per Rule#Compare.
func (r1 *RecordingRule) Compare(r2 *RecordingRule) int {
	t1, t2 := r1.LastEvaluation.AsTime(), r2.LastEvaluation.AsTime()
	if t1.Before(t2) {
		return 1
	}
	if t1.After(t2) {
		return -1
	}

	return 0
}

func NewAlertingRule(a *Alert) *Rule {
	return &Rule{
		Result: &Rule_Alert{Alert: a},
	}
}

func (r *Rule) GetLabels() labelpb.Labels {
	switch v := r.Result.(type) {
	case *Rule_Recording:
		return v.Recording.GetLabels().GetLabels()
	case *Rule_Alert:
		return v.Alert.GetLabels().GetLabels()
	default:
		return nil
	}
}

func (r *Rule) SetLabels(ls labelpb.Labels) {
	var result *labelpb.LabelSet
	if len(ls) > 0 {
		result = &labelpb.LabelSet{Labels: ls}
	}

	switch v := r.Result.(type) {
	case *Rule_Recording:
		v.Recording.Labels = result
	case *Rule_Alert:
		v.Alert.Labels = result
	}
}

func (r *Rule) GetName() string {
	switch v := r.Result.(type) {
	case *Rule_Recording:
		return v.Recording.Name
	case *Rule_Alert:
		return v.Alert.Name
	default:
		return ""
	}
}

func (r *Rule) GetQuery() string {
	switch v := r.Result.(type) {
	case *Rule_Recording:
		return v.Recording.Query
	case *Rule_Alert:
		return v.Alert.Query
	default:
		return ""
	}
}

func (r *Rule) GetLastEvaluation() time.Time {
	switch v := r.Result.(type) {
	case *Rule_Recording:
		return v.Recording.GetLastEvaluation().AsTime()
	case *Rule_Alert:
		return v.Alert.GetLastEvaluation().AsTime()
	default:
		return time.Time{}
	}
}

// Compare compares recording and alerting rules r1 and r2 and returns:
//
//	< 0 if r1 < r2  if rule r1 is not equal and lexically before rule r2
//	  0 if r1 == r2 if rule r1 is logically equal to r2 (r1 and r2 are the "same" rules)
//	> 0 if r1 > r2  if rule r1 is not equal and lexically after rule r2
//
// More formally, ordering and equality is determined in the following order:
//
// 1. rule type (alerting rules come before recording rules)
// 2. rule name
// 3. rule labels
// 4. rule query
// 5. for alerting rules: duration
//
// Note: this can still leave ordering undetermined for equal rules (x == y).
// For determining ordering of equal rules, use Alert#Compare or RecordingRule#Compare.
func (r1 *Rule) Compare(r2 *Rule) int {
	_, r1IsAlert := r1.Result.(*Rule_Alert)
	_, r2IsAlert := r2.Result.(*Rule_Alert)
	if r1IsAlert != r2IsAlert {
		if r1IsAlert {
			return -1
		}
		return 1
	}

	if d := strings.Compare(r1.GetName(), r2.GetName()); d != 0 {
		return d
	}

	if d := labelpb.Compare(r1.GetLabels(), r2.GetLabels()); d != 0 {
		return d
	}

	if d := strings.Compare(r1.GetQuery(), r2.GetQuery()); d != 0 {
		return d
	}

	if r1IsAlert { // we already asserted that the rules are the same type
		if d := big.NewFloat(r1.GetAlert().DurationSeconds).Cmp(big.NewFloat(r2.GetAlert().DurationSeconds)); d != 0 {
			return d
		}
	}

	return 0
}

func (r *RuleGroups) MarshalJSON() ([]byte, error) {
	if r.Groups == nil {
		// Ensure that empty slices are marshaled as '[]' and not 'null'.
		return []byte(`{"groups":[]}`), nil
	}
	type plain RuleGroups
	return json.Marshal((*plain)(r))
}

// Compare compares rule group x and y and returns:
//
//	< 0 if x < y   if rule group r1 is not equal and lexically before rule group r2
//	  0 if x == y  if rule group r1 is logically equal to r2 (r1 and r2 are the "same" rule groups)
//	> 0 if x > y   if rule group r1 is not equal and lexically after rule group r2
func (r1 *RuleGroup) Compare(r2 *RuleGroup) int {
	return strings.Compare(r1.Key(), r2.Key())
}

// Key returns the group key similar resembling Prometheus logic.
// See https://github.com/prometheus/prometheus/blob/869f1bc587e667b79721852d5badd9f70a39fc3f/rules/manager.go#L1062-L1065
func (r *RuleGroup) Key() string {
	if r == nil {
		return ""
	}

	return r.File + ";" + r.Name
}

func (m *Rule) UnmarshalJSON(entry []byte) error {
	decider := struct {
		Type string `json:"type"`
	}{}
	if err := json.Unmarshal(entry, &decider); err != nil {
		return errors.Wrapf(err, "rule: type field unmarshal: %v", string(entry))
	}

	switch strings.ToLower(decider.Type) {
	case "recording":
		r := &RecordingRule{}
		if err := json.Unmarshal(entry, r); err != nil {
			return errors.Wrapf(err, "rule: recording rule unmarshal: %v", string(entry))
		}

		m.Result = &Rule_Recording{Recording: r}
	case "alerting":
		r := &Alert{}
		if err := json.Unmarshal(entry, r); err != nil {
			return errors.Wrapf(err, "rule: alerting rule unmarshal: %v", string(entry))
		}

		m.Result = &Rule_Alert{Alert: r}
	case "":
		return errors.Errorf("rule: no type field provided: %v", string(entry))
	default:
		return errors.Errorf("rule: unknown type field provided %s; %v", decider.Type, string(entry))
	}
	return nil
}

// MarshalJSON is required to maintain backward compatibility with the previous
// gogo-proto generated code. Gogo generated timestamp fields as time.Time,
// which marshals to RFC 3339 strings like "2026-02-26T12:00:00Z". The new
// proto-generated *timestamppb.Timestamp marshals as {"seconds":N, "nanos":N}.
// This method shadows timestamp fields with time.Time to preserve the expected
// JSON format. It also uses type aliases to avoid recursion with the custom
// MarshalJSON methods on RecordingRule and Alert.
func (m *Rule) MarshalJSON() ([]byte, error) {
	if r := m.GetRecording(); r != nil {
		// Ensure nil pointer fields serialize as {} instead of null for JSON backward compatibility.
		if r.Labels == nil {
			r.Labels = &labelpb.LabelSet{}
		}
		type RecAlias RecordingRule
		return json.Marshal(struct {
			*RecAlias
			LastEvaluation time.Time `json:"lastEvaluation"`
			Type           string    `json:"type"`
		}{
			RecAlias:       (*RecAlias)(r),
			LastEvaluation: timestampToTime(r.LastEvaluation),
			Type:           RuleRecordingType,
		})
	}
	a := m.GetAlert()
	if a.Alerts == nil {
		a.Alerts = make([]*AlertInstance, 0)
	}
	// Ensure nil pointer fields serialize as {} instead of null for JSON backward compatibility.
	if a.Labels == nil {
		a.Labels = &labelpb.LabelSet{}
	}
	if a.Annotations == nil {
		a.Annotations = &labelpb.LabelSet{}
	}
	type AlertAlias Alert
	return json.Marshal(struct {
		*AlertAlias
		LastEvaluation time.Time `json:"lastEvaluation"`
		Type           string    `json:"type"`
	}{
		AlertAlias:     (*AlertAlias)(a),
		LastEvaluation: timestampToTime(a.LastEvaluation),
		Type:           RuleAlertingType,
	})
}

// MarshalJSON is required to maintain backward compatibility with the previous
// gogo-proto generated code. Gogo generated timestamp fields as time.Time,
// which marshals to RFC 3339 strings like "2026-02-26T12:00:00Z". The new
// proto-generated *timestamppb.Timestamp marshals as {"seconds":N, "nanos":N}.
// This method shadows timestamp fields with time.Time to preserve the expected
// JSON format.
func (r *RuleGroup) MarshalJSON() ([]byte, error) {
	if r.Rules == nil {
		r.Rules = make([]*Rule, 0)
	}
	type Alias RuleGroup
	return json.Marshal(&struct {
		*Alias
		LastEvaluation time.Time `json:"lastEvaluation"`
	}{
		Alias:          (*Alias)(r),
		LastEvaluation: timestampToTime(r.LastEvaluation),
	})
}

// UnmarshalJSON is required to maintain backward compatibility with the previous
// gogo-proto generated code. Gogo generated timestamp fields as time.Time,
// which unmarshals from RFC 3339 strings like "2026-02-26T12:00:00Z". The new
// proto-generated *timestamppb.Timestamp expects {"seconds":N, "nanos":N}.
// This method shadows timestamp fields with time.Time to accept the expected
// JSON format.
func (r *RuleGroup) UnmarshalJSON(data []byte) error {
	type Alias RuleGroup
	aux := &struct {
		*Alias
		LastEvaluation time.Time `json:"lastEvaluation"`
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	r.LastEvaluation = timeToTimestamp(aux.LastEvaluation)
	return nil
}

func timeToTimestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func timestampToTime(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}

// MarshalJSON is required to maintain backward compatibility with the previous
// gogo-proto generated code. Gogo generated timestamp fields as time.Time,
// which marshals to RFC 3339 strings like "2026-02-26T12:00:00Z". The new
// proto-generated *timestamppb.Timestamp marshals as {"seconds":N, "nanos":N}.
// This method shadows timestamp fields with time.Time to preserve the expected
// JSON format.
func (r *RecordingRule) MarshalJSON() ([]byte, error) {
	// Ensure nil pointer fields serialize as {} instead of null for JSON backward compatibility.
	if r.Labels == nil {
		r.Labels = &labelpb.LabelSet{}
	}
	type Alias RecordingRule
	return json.Marshal(&struct {
		*Alias
		LastEvaluation time.Time `json:"lastEvaluation"`
	}{
		Alias:          (*Alias)(r),
		LastEvaluation: timestampToTime(r.LastEvaluation),
	})
}

// UnmarshalJSON is required to maintain backward compatibility with the previous
// gogo-proto generated code. Gogo generated timestamp fields as time.Time,
// which unmarshals from RFC 3339 strings like "2026-02-26T12:00:00Z". The new
// proto-generated *timestamppb.Timestamp expects {"seconds":N, "nanos":N}.
// This method shadows timestamp fields with time.Time to accept the expected
// JSON format.
func (r *RecordingRule) UnmarshalJSON(data []byte) error {
	type Alias RecordingRule
	aux := &struct {
		*Alias
		LastEvaluation time.Time `json:"lastEvaluation"`
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	r.LastEvaluation = timeToTimestamp(aux.LastEvaluation)
	return nil
}

// MarshalJSON is required to maintain backward compatibility with the previous
// gogo-proto generated code. Gogo generated timestamp fields as time.Time,
// which marshals to RFC 3339 strings like "2026-02-26T12:00:00Z". The new
// proto-generated *timestamppb.Timestamp marshals as {"seconds":N, "nanos":N}.
// This method shadows timestamp fields with time.Time to preserve the expected
// JSON format.
func (a *Alert) MarshalJSON() ([]byte, error) {
	if a.Alerts == nil {
		a.Alerts = make([]*AlertInstance, 0)
	}
	// Ensure nil pointer fields serialize as {} instead of null for JSON backward compatibility.
	if a.Labels == nil {
		a.Labels = &labelpb.LabelSet{}
	}
	if a.Annotations == nil {
		a.Annotations = &labelpb.LabelSet{}
	}
	type Alias Alert
	return json.Marshal(&struct {
		*Alias
		LastEvaluation time.Time `json:"lastEvaluation"`
	}{
		Alias:          (*Alias)(a),
		LastEvaluation: timestampToTime(a.LastEvaluation),
	})
}

// UnmarshalJSON is required to maintain backward compatibility with the previous
// gogo-proto generated code. Gogo generated timestamp fields as time.Time,
// which unmarshals from RFC 3339 strings like "2026-02-26T12:00:00Z". The new
// proto-generated *timestamppb.Timestamp expects {"seconds":N, "nanos":N}.
// This method shadows timestamp fields with time.Time to accept the expected
// JSON format.
func (a *Alert) UnmarshalJSON(data []byte) error {
	type Alias Alert
	aux := &struct {
		*Alias
		LastEvaluation time.Time `json:"lastEvaluation"`
	}{
		Alias: (*Alias)(a),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	a.LastEvaluation = timeToTimestamp(aux.LastEvaluation)
	return nil
}

// MarshalJSON is required to maintain backward compatibility with the previous
// gogo-proto generated code. Gogo generated timestamp fields as time.Time,
// which marshals to RFC 3339 strings like "2026-02-26T12:00:00Z". The new
// proto-generated *timestamppb.Timestamp marshals as {"seconds":N, "nanos":N}.
// This method shadows timestamp fields with time.Time to preserve the expected
// JSON format.
func (ai *AlertInstance) MarshalJSON() ([]byte, error) {
	// Ensure nil pointer fields serialize as {} instead of null for JSON backward compatibility.
	if ai.Labels == nil {
		ai.Labels = &labelpb.LabelSet{}
	}
	if ai.Annotations == nil {
		ai.Annotations = &labelpb.LabelSet{}
	}
	type Alias AlertInstance
	return json.Marshal(&struct {
		*Alias
		ActiveAt *time.Time `json:"activeAt,omitempty"`
	}{
		Alias: (*Alias)(ai),
		ActiveAt: func() *time.Time {
			if ai.ActiveAt == nil {
				return nil
			}
			t := ai.ActiveAt.AsTime()
			return &t
		}(),
	})
}

// UnmarshalJSON is required to maintain backward compatibility with the previous
// gogo-proto generated code. Gogo generated timestamp fields as time.Time,
// which unmarshals from RFC 3339 strings like "2026-02-26T12:00:00Z". The new
// proto-generated *timestamppb.Timestamp expects {"seconds":N, "nanos":N}.
// This method shadows timestamp fields with time.Time to accept the expected
// JSON format.
func (ai *AlertInstance) UnmarshalJSON(data []byte) error {
	type Alias AlertInstance
	aux := &struct {
		*Alias
		ActiveAt *time.Time `json:"activeAt,omitempty"`
	}{
		Alias: (*Alias)(ai),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.ActiveAt != nil {
		ai.ActiveAt = timestamppb.New(*aux.ActiveAt)
	}
	return nil
}

func (x *AlertState) UnmarshalJSON(entry []byte) error {
	fieldStr, err := strconv.Unquote(string(entry))
	if err != nil {
		return errors.Wrapf(err, "alertState: unquote %v", string(entry))
	}

	if fieldStr == "" {
		return errors.New("empty alertState")
	}

	state, ok := AlertState_value[strings.ToUpper(fieldStr)]
	if !ok {
		return errors.Errorf("unknown alertState: %v", string(entry))
	}
	*x = AlertState(state)
	return nil
}

func (x *AlertState) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(strings.ToLower(x.String()))), nil
}

// Compare compares alert state x and y and returns:
//
//	< 0 if x < y  (alert state x is more critical than alert state y)
//	  0 if x == y
//	> 0 if x > y  (alert state x is less critical than alert state y)
//
// For sorting this makes sure that more "critical" alert states come first.
func (x AlertState) Compare(y AlertState) int {
	return int(y) - int(x)
}

// Compare compares two equal alerting rules a1 and a2 and returns:
//
//	< 0 if a1 < a2  if rule a1 is lexically before rule a2
//	  0 if a1 == a2
//	> 0 if a1 > a2  if rule a1 is lexically after rule a2
//
// More formally, the ordering is determined in the following order:
//
// 1. alert state
// 2. alert last evaluation (earlier evaluation comes first)
//
// Note: This method assumes a1 and a2 are logically equal as per Rule#Compare.
func (a1 *Alert) Compare(a2 *Alert) int {
	if d := a1.State.Compare(a2.State); d != 0 {
		return d
	}

	t1, t2 := a1.LastEvaluation.AsTime(), a2.LastEvaluation.AsTime()
	if t1.Before(t2) {
		return 1
	}
	if t1.After(t2) {
		return -1
	}

	return 0
}
