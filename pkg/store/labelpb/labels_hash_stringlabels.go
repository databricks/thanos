// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

//go:build stringlabels

// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package labelpb

import (
	"encoding/binary"

	"github.com/cespare/xxhash/v2"
)

// Hash computes a hash of the label set using the same encoding as
// Prometheus's stringlabels labels.Hash(): each label name and value
// is preceded by its length as a varint, producing the same byte
// sequence that Prometheus stores internally.
func Hash(ls []*Label) uint64 {
	b := make([]byte, 0, 1024)
	for i, v := range ls {
		needed := sizeVarint(len(v.Name)) + len(v.Name) +
			sizeVarint(len(v.Value)) + len(v.Value)
		if len(b)+needed > cap(b) {
			h := xxhash.New()
			_, _ = h.Write(b)
			var vbuf [binary.MaxVarintLen64]byte
			for _, v := range ls[i:] {
				n := binary.PutUvarint(vbuf[:], uint64(len(v.Name)))
				_, _ = h.Write(vbuf[:n])
				_, _ = h.WriteString(v.Name)
				n = binary.PutUvarint(vbuf[:], uint64(len(v.Value)))
				_, _ = h.Write(vbuf[:n])
				_, _ = h.WriteString(v.Value)
			}
			return h.Sum64()
		}
		b = binary.AppendUvarint(b, uint64(len(v.Name)))
		b = append(b, v.Name...)
		b = binary.AppendUvarint(b, uint64(len(v.Value)))
		b = append(b, v.Value...)
	}
	return xxhash.Sum64(b)
}

func sizeVarint(x int) int {
	n := 1
	for u := uint64(x); u >= 0x80; u >>= 7 {
		n++
	}
	return n
}
