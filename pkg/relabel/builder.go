// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package relabel

import (
	"slices"
	"strings"

	"github.com/thanos-io/thanos/pkg/store/labelpb"
)

// builder allows modifying Labels by adding and removing labels
// from a base set, producing a new sorted result.
// This mimics the Prometheus labels.Builder which is not necessary except for
// here, in the relabel package, thus it is not exported.
type builder struct {
	base labelpb.Labels
	del  []string
	add  []*labelpb.Label
}

func newBuilder(base labelpb.Labels) *builder {
	return &builder{
		base: base,
		del:  make([]string, 0, 5),
		add:  make([]*labelpb.Label, 0, 5),
	}
}

func (b *builder) Del(ns ...string) *builder {
	for _, n := range ns {
		for i, a := range b.add {
			if a.Name == n {
				b.add = append(b.add[:i], b.add[i+1:]...)
			}
		}
		b.del = append(b.del, n)
	}
	return b
}

// Set adds or updates a label. A value of "" deletes the label.
func (b *builder) Set(n, v string) *builder {
	if v == "" {
		return b.Del(n)
	}
	for i, a := range b.add {
		if a.Name == n {
			b.add[i].Value = v
			return b
		}
	}
	b.add = append(b.add, &labelpb.Label{Name: n, Value: v})
	return b
}

// Get returns the value for the label with the given name, reflecting
// any pending adds/deletes that haven't been materialized yet.
func (b *builder) Get(name string) string {
	for _, a := range b.add {
		if a.Name == name {
			return a.Value
		}
	}
	if slices.Contains(b.del, name) {
		return ""
	}
	return b.base.Get(name)
}

// Range calls f on each label in the effective set (base minus deletes,
// plus adds). Order is not guaranteed. The set is snapshotted before
// iteration so mutations via Set/Del inside f are safe.
func (b *builder) Range(f func(l *labelpb.Label)) {
	adds := b.add // snapshot, since things may be 'added' during iteration
	for _, l := range b.base {
		if slices.Contains(b.del, l.Name) || containsName(adds, l.Name) {
			continue
		}
		f(l)
	}
	for _, l := range adds {
		f(l)
	}
}

// Labels returns the new sorted label set with all modifications applied.
func (b *builder) Labels() labelpb.Labels {
	if len(b.del) == 0 && len(b.add) == 0 {
		return b.base
	}

	expectedSize := len(b.base) + len(b.add) - len(b.del)
	if expectedSize < 1 {
		expectedSize = 1
	}
	res := make(labelpb.Labels, 0, expectedSize)
	for _, l := range b.base {
		if slices.Contains(b.del, l.Name) || containsName(b.add, l.Name) {
			continue
		}
		res = append(res, l)
	}
	if len(b.add) > 0 {
		res = append(res, b.add...)
		slices.SortFunc(res, func(a, b *labelpb.Label) int { return strings.Compare(a.Name, b.Name) })
	}
	return res
}

func containsName(ls []*labelpb.Label, name string) bool {
	for _, l := range ls {
		if l.Name == name {
			return true
		}
	}
	return false
}
