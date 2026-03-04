// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

// Labels provides a sorted label set type built on []*Label (proto-generated pointers).
// This replaces the role of github.com/prometheus/prometheus/model/labels.Labels
// with a type that embraces pointer semantics and vtproto pooling.

package labelpb

import (
	"bytes"
	"encoding/json"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/cespare/xxhash/v2"
	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
)

const (
	MetricName = "__name__"
	labelSep   = '\xff'
)

var labelSeps = []byte{labelSep}

// Labels is a sorted set of labels. Order has to be guaranteed upon
// instantiation. It is a slice of proto-generated *Label pointers,
// making it compatible with protobuf message fields and vtproto pooling.
type Labels []*Label

// sort.Interface implementation.

func (ls Labels) Len() int           { return len(ls) }
func (ls Labels) Swap(i, j int)      { ls[i], ls[j] = ls[j], ls[i] }
func (ls Labels) Less(i, j int) bool { return ls[i].Name < ls[j].Name }

// Compare compares two individual labels by name, then value.
func (m *Label) Compare(other *Label) int {
	if c := strings.Compare(m.Name, other.Name); c != 0 {
		return c
	}
	return strings.Compare(m.Value, other.Value)
}

// Hash is a free function that hashes a []*Label without requiring a type cast.
func Hash(ls []*Label) uint64 {
	b := make([]byte, 0, 1024)
	for i, v := range ls {
		if len(b)+len(v.Name)+len(v.Value)+2 >= cap(b) {
			h := xxhash.New()
			_, _ = h.Write(b)
			for _, v := range ls[i:] {
				_, _ = h.WriteString(v.Name)
				_, _ = h.Write(labelSeps)
				_, _ = h.WriteString(v.Value)
				_, _ = h.Write(labelSeps)
			}
			return h.Sum64()
		}
		b = append(b, v.Name...)
		b = append(b, labelSep)
		b = append(b, v.Value...)
		b = append(b, labelSep)
	}
	return xxhash.Sum64(b)
}

// Hash returns a hash value for the label set.
// Uses the same xxhash algorithm as Prometheus for compatibility.
func (ls Labels) Hash() uint64 { return Hash(ls) }

// HashWithoutLabels returns a hash value for all labels except those matching
// the provided names. __name__ is always excluded.
// 'names' must be sorted in ascending order.
func HashWithoutLabels(ls []*Label, b []byte, names ...string) (uint64, []byte) {
	b = b[:0]
	j := 0
	for _, l := range ls {
		for j < len(names) && names[j] < l.Name {
			j++
		}
		if l.Name == MetricName || (j < len(names) && l.Name == names[j]) {
			continue
		}
		b = append(b, l.Name...)
		b = append(b, labelSep)
		b = append(b, l.Value...)
		b = append(b, labelSep)
	}
	return xxhash.Sum64(b), b
}

// HashWithoutLabels calls the free function HashWithoutLabels.
func (ls Labels) HashWithoutLabels(b []byte, names ...string) (uint64, []byte) {
	return HashWithoutLabels(ls, b, names...)
}

// Copy returns a copy of the label set. The underlying *Label pointers
// are shared (shallow copy of the slice).
func (ls Labels) Copy() Labels {
	res := make(Labels, len(ls))
	copy(res, ls)
	return res
}

// DeepCopy returns a new Labels where each *Label is cloned via CloneVT.
// String references are shared (not byte-copied), which preserves vtproto
// string interning from unmarshal. Use this when you need an independent
// label set whose individual *Label pointers won't alias the originals.
func (ls Labels) DeepCopy() Labels {
	if ls == nil {
		return nil
	}
	res := make(Labels, len(ls))
	for i, l := range ls {
		res[i] = l.CloneVT()
	}
	return res
}

// DeepCopyPooled returns a new Labels where each *Label is obtained from
// the vtproto pool rather than freshly allocated. String references are
// shared (not byte-copied), preserving vtproto string interning.
//
// The caller is responsible for returning the labels via Labels.ReturnLabelsToPool
// when they are no longer needed.
func (ls Labels) DeepCopyPooled() Labels {
	if ls == nil {
		return nil
	}
	res := make(Labels, len(ls))
	for i, l := range ls {
		pooled := LabelFromVTPool()
		pooled.Name = l.Name
		pooled.Value = l.Value
		res[i] = pooled
	}
	return res
}

// ReturnLabelsToPool returns every *Label in the set to the vtproto pool.
func (ls Labels) ReturnToVTPool() {
	for _, l := range ls {
		l.ReturnToVTPool()
	}
}

// FindValue returns the value for the label with the given name
// in a []*Label without requiring a type cast.
// Returns an empty string if the label doesn't exist.
func FindValue(ls []*Label, name string) string {
	for _, l := range ls {
		if l.Name == name {
			return l.Value
		}
	}
	return ""
}

// Get calls the free function FindValue.
func (ls Labels) Get(name string) string {
	return FindValue(ls, name)
}

// Has returns true if the label with the given name is present.
func (ls Labels) Has(name string) bool {
	for _, l := range ls {
		if l.Name == name {
			return true
		}
	}
	return false
}

// Equal returns whether two label sets are equal.
func Equal(a, b Labels) bool {
	if len(a) != len(b) {
		return false
	}
	for i, l := range a {
		if l.Name != b[i].Name || l.Value != b[i].Value {
			return false
		}
	}
	return true
}

// EmptyLabels returns an empty Labels value.
// deprecated: this is an unnecessary indirection and is not idiomatic go; use nil instead.
func EmptyLabels() Labels {
	return Labels{}
}

// NewLabels returns a sorted Labels from the given label pointers.
// The caller must guarantee that all label names are unique.
func NewLabels(ls ...*Label) Labels {
	set := make(Labels, len(ls))
	copy(set, ls)
	sort.Sort(set)
	return set
}

// FromStrings creates new sorted Labels from pairs of strings.
// Panics if an odd number of strings is provided.
func FromStrings(ss ...string) Labels {
	if len(ss)%2 != 0 {
		panic("invalid number of strings")
	}
	res := make(Labels, 0, len(ss)/2)
	for i := 0; i < len(ss); i += 2 {
		res = append(res, &Label{Name: ss[i], Value: ss[i+1]})
	}
	slices.SortFunc(res, func(a, b *Label) int { return strings.Compare(a.Name, b.Name) })
	return res
}

// Compare compares two label sets lexicographically.
// Returns 0 if a==b, <0 if a < b, and >0 if a > b.
func Compare(a, b []*Label) int {
	l := len(a)
	if len(b) < l {
		l = len(b)
	}
	for i := 0; i < l; i++ {
		if c := a[i].Compare(b[i]); c != 0 {
			return c
		}
	}
	return len(a) - len(b)
}

// IsEmpty returns true if the label set has no labels.
func (ls Labels) IsEmpty() bool {
	return len(ls) == 0
}

// Range calls f on each label.
func (ls Labels) Range(f func(l *Label)) {
	for _, l := range ls {
		f(l)
	}
}

// Validate calls f on each label. If f returns a non-nil error, that
// error is returned immediately, cancelling the iteration.
func (ls Labels) Validate(f func(l *Label) error) error {
	for _, l := range ls {
		if err := f(l); err != nil {
			return err
		}
	}
	return nil
}

// DropMetricName returns Labels with the __name__ label removed.
func (ls Labels) DropMetricName() Labels {
	for i, l := range ls {
		if l.Name == MetricName {
			if i == 0 {
				return ls[1:]
			}
			return append(ls[:i:i], ls[i+1:]...)
		}
	}
	return ls
}

// ToPromLabels converts a slice of *Label to prometheus/model/labels.Labels.
// This must ONLY be used at the boundary of Prometheus PromQL or Prometheus TSDB.
func ToPromLabels(ls []*Label) labels.Labels {
	result := make(labels.Labels, len(ls))
	for i, l := range ls {
		result[i] = labels.Label{Name: l.Name, Value: l.Value}
	}
	return result
}

// PromLabels converts a LabelSet to prometheus/model/labels.Labels.
// Nil-safe: returns empty labels for a nil receiver.
// This must ONLY be used at the boundary of Prometheus PromQL or Prometheus TSDB.
func (m *LabelSet) PromLabels() labels.Labels {
	return ToPromLabels(m.GetLabels())
}

// FromPromLabels converts prometheus/model/labels.Labels to labelpb.Labels.
// This must ONLY be used at the boundary of Prometheus PromQL or Prometheus TSDB.
func FromPromLabels(ls labels.Labels) Labels {
	result := make(Labels, len(ls))
	for i, l := range ls {
		result[i] = &Label{Name: l.Name, Value: l.Value}
	}
	return result
}

// String returns the label set in the form {name="value", ...}.
func (ls Labels) String() string {
	var bytea [1024]byte // On stack to avoid memory allocation while building the output.
	b := bytes.NewBuffer(bytea[:0])

	b.WriteByte('{')
	for i, l := range ls {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(l.Name)
		b.WriteByte('=')
		b.Write(strconv.AppendQuote(b.AvailableBuffer(), l.Value))
	}
	b.WriteByte('}')
	return b.String()
}

// MarshalJSON implements json.Marshaler. Encodes as a JSON object
// mapping label names to values: {"name":"value",...}.
func (ls Labels) MarshalJSON() ([]byte, error) {
	return json.Marshal(ls.Map())
}

// UnmarshalJSON implements json.Unmarshaler. Expects a JSON object
// mapping label names to values: {"name":"value",...}.
func (ls *Labels) UnmarshalJSON(b []byte) error {
	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	*ls = FromMap(m)
	return nil
}

// MarshalJSON implements json.Marshaler for LabelSet.
// Encodes the labels as a flat JSON object {"name":"value",...} for
// backward compatibility with the Prometheus API JSON format.
func (ls *LabelSet) MarshalJSON() ([]byte, error) {
	return Labels(ls.GetLabels()).MarshalJSON()
}

// UnmarshalJSON implements json.Unmarshaler for LabelSet.
// Expects a flat JSON object {"name":"value",...}.
func (ls *LabelSet) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	var parsed Labels
	if err := parsed.UnmarshalJSON(b); err != nil {
		return err
	}
	ls.Labels = parsed
	return nil
}

// MarshalYAML implements yaml.Marshaler.
func (ls Labels) MarshalYAML() (interface{}, error) {
	return ls.Map(), nil
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (ls *Labels) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var m map[string]string
	if err := unmarshal(&m); err != nil {
		return err
	}
	*ls = FromMap(m)
	return nil
}

// IsValid checks if the metric name or label names are valid.
func (ls Labels) IsValid(validationScheme model.ValidationScheme) bool {
	err := ls.Validate(func(l *Label) error {
		if l.Name == model.MetricNameLabel {
			if validationScheme == model.LegacyValidation && model.NameValidationScheme == model.UTF8Validation && !model.IsValidLegacyMetricName(string(model.LabelValue(l.Value))) {
				return strconv.ErrSyntax
			}
			if !model.IsValidMetricName(model.LabelValue(l.Value)) {
				return strconv.ErrSyntax
			}
		}
		if validationScheme == model.LegacyValidation && model.NameValidationScheme == model.UTF8Validation {
			if !model.LabelName(l.Name).IsValidLegacy() || !model.LabelValue(l.Value).IsValid() {
				return strconv.ErrSyntax
			}
		} else if !model.LabelName(l.Name).IsValid() || !model.LabelValue(l.Value).IsValid() {
			return strconv.ErrSyntax
		}
		return nil
	})
	return err == nil
}

// Map returns a string map of the labels.
func (ls Labels) Map() map[string]string {
	m := make(map[string]string, len(ls))
	ls.Range(func(l *Label) {
		m[l.Name] = l.Value
	})
	return m
}

// FromMap returns new sorted Labels from the given map.
func FromMap(m map[string]string) Labels {
	res := make(Labels, 0, len(m))
	for k, v := range m {
		res = append(res, &Label{Name: k, Value: v})
	}
	slices.SortFunc(res, func(a, b *Label) int { return strings.Compare(a.Name, b.Name) })
	return res
}

// LabelSetFromStrings creates a *LabelSet from key-value string pairs.
func LabelSetFromStrings(ss ...string) *LabelSet {
	return &LabelSet{Labels: FromStrings(ss...)}
}

var (
	ErrOutOfOrderLabels = errors.New("out of order labels")
	ErrEmptyLabels      = errors.New("label set contains a label with empty name or value")
	ErrDuplicateLabels  = errors.New("label set contains duplicate label names")
)

// ExtendSortedLabels returns a new label set that is the result of merging two sorted label sets.
// Labels in 'extend' override labels with the same name in 'lset'. Both inputs must be sorted.
func ExtendSortedLabels(lset, extend Labels) Labels {
	if extend.IsEmpty() {
		return lset.Copy()
	}
	if lset.IsEmpty() {
		return extend.Copy()
	}

	res := make(Labels, 0, len(lset)+len(extend))
	i, j := 0, 0
	for i < len(lset) && j < len(extend) {
		switch strings.Compare(lset[i].Name, extend[j].Name) {
		case -1:
			res = append(res, lset[i])
			i++
		case 0:
			res = append(res, extend[j])
			i++
			j++
		case 1:
			res = append(res, extend[j])
			j++
		}
	}
	res = append(res, lset[i:]...)
	res = append(res, extend[j:]...)
	return res
}

// RmLabels returns a new Labels slice without the labels that are in the remove set.
// The input slice is never modified.
// Use when the caller still needs the original slice (e.g. a struct field
// or a variable referenced later).
func RmLabels(lset Labels, remove map[string]struct{}) Labels {
	if len(remove) == 0 {
		return lset.Copy()
	}
	res := make(Labels, 0, len(lset))
	for _, l := range lset {
		if _, ok := remove[l.Name]; !ok {
			res = append(res, l)
		}
	}
	return res
}

// RmLabelsInPlace filters lset in place, removing labels that are in the remove set.
// in the remove set. It re-uses the underlying array and returns the
// shortened slice. The caller must not use the original slice header after
// this call — always reassign: lset = RmLabelsInPlace(lset, remove).
func RmLabelsInPlace(lset Labels, remove map[string]struct{}) Labels {
	if len(remove) == 0 {
		return lset
	}
	n := 0
	for _, l := range lset {
		if _, ok := remove[l.Name]; !ok {
			lset[n] = l
			n++
		}
	}
	// Nil out the tail so removed *Label pointers can be GC'd.
	clear(lset[n:])
	return lset[:n]
}

func LabelSetsToString(lsets []Labels) string {
	return LabelSetsToStringN(lsets, 200)
}

func LabelSetsToStringN(lsets []Labels, maxLength int) string {
	if len(lsets) == 0 {
		return ""
	}
	s := []string{}
	for _, ls := range lsets {
		str := ls.String()
		s = append(s, str)
		maxLength -= len(str)
		if maxLength <= 0 {
			break
		}
	}
	sort.Strings(s)
	return strings.Join(s, ",")
}

// HashWithPrefix returns a hash for the given prefix and labels.
func HashWithPrefix(prefix string, lbls Labels) uint64 {
	b := make([]byte, 0, 1024)
	b = append(b, prefix...)
	b = append(b, labelSep)

	for i, v := range lbls {
		if len(b)+len(v.Name)+len(v.Value)+2 >= cap(b) {
			h := xxhash.New()
			_, _ = h.Write(b)
			for _, v := range lbls[i:] {
				_, _ = h.WriteString(v.Name)
				_, _ = h.Write(labelSeps)
				_, _ = h.WriteString(v.Value)
				_, _ = h.Write(labelSeps)
			}
			return h.Sum64()
		}
		b = append(b, v.Name...)
		b = append(b, labelSep)
		b = append(b, v.Value...)
		b = append(b, labelSep)
	}
	return xxhash.Sum64(b)
}

// ValidateLabels validates label names and values (checks for empty
// names and values, out of order labels and duplicate label names).
// Returns appropriate error if validation fails on a label.
func ValidateLabels(lbls Labels) error {
	if len(lbls) == 0 {
		return ErrEmptyLabels
	}

	l0 := lbls[0]
	if l0.Name == "" || l0.Value == "" {
		return ErrEmptyLabels
	}

	for _, l := range lbls[1:] {
		if l.Name == "" || l.Value == "" {
			return ErrEmptyLabels
		}

		if l.Name == l0.Name {
			return ErrDuplicateLabels
		}

		if l.Name < l0.Name {
			return ErrOutOfOrderLabels
		}
		l0 = l
	}

	return nil
}
