//go:build !stringlabels

// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package labelpb

import "github.com/cespare/xxhash/v2"

// Hash computes a hash of the label set using the same encoding as
// Prometheus's default (non-stringlabels) labels.Hash():
// name + \xff + value + \xff for each label.
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
