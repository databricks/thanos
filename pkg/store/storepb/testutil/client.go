// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package storetestutil

import (
	"github.com/prometheus/prometheus/model/labels"

	"github.com/thanos-io/thanos/pkg/info/infopb"
	"github.com/thanos-io/thanos/pkg/store/storepb"
)

type TestClient struct {
	storepb.StoreClient

	Name string

	ExtLset                     []labels.Labels
	MinTime, MaxTime            int64
	Shardable                   bool
	WithoutReplicaLabelsEnabled bool
	IsLocalStore                bool
	StoreTSDBInfos              []infopb.TSDBInfo
	StoreFilterNotMatches       bool

	// ReplicaInfo fields for GROUP_REPLICA strategy testing.
	// GroupKeyStr is used as Group when ReplicaGroupStr is empty (DNS-based fallback).
	// ReplicaGroupStr is the first-class StoreInfo field - if set, it takes precedence.
	// ReplicaKeyStr is used as Replica in ReplicaInfo.
	// QuorumValue is used as Quorum when ReplicaGroupStr is set (0 = must-success).
	GroupKeyStr     string
	ReplicaKeyStr   string
	ReplicaGroupStr string // First-class StoreInfo field (takes precedence over GroupKeyStr)
	QuorumValue     int
}

func (c TestClient) LabelSets() []labels.Labels         { return c.ExtLset }
func (c TestClient) TimeRange() (mint, maxt int64)      { return c.MinTime, c.MaxTime }
func (c TestClient) TSDBInfos() []infopb.TSDBInfo       { return c.StoreTSDBInfos }
func (c TestClient) SupportsSharding() bool             { return c.Shardable }
func (c TestClient) SupportsWithoutReplicaLabels() bool { return c.WithoutReplicaLabelsEnabled }
func (c TestClient) String() string                     { return c.Name }
func (c TestClient) Addr() (string, bool)               { return c.Name, c.IsLocalStore }

func (c TestClient) Matches(matches []*labels.Matcher) bool { return !c.StoreFilterNotMatches }

// ReplicaInfo returns replica topology hints for GROUP_REPLICA partial response strategy.
// It mimics the unification logic of endpointRef.ReplicaInfo():
// - If ReplicaGroupStr is set (first-class StoreInfo field), use it as Group with QuorumValue.
// - Otherwise fall back to GroupKeyStr (DNS-based) with Quorum=0 (must-success).
func (c TestClient) ReplicaInfo() storepb.ReplicaInfo {
	// Prefer first-class ReplicaGroupStr if set
	if c.ReplicaGroupStr != "" {
		return storepb.ReplicaInfo{
			Group:   c.ReplicaGroupStr,
			Replica: c.ReplicaKeyStr,
			Quorum:  c.QuorumValue,
		}
	}
	// Fall back to DNS-based grouping
	return storepb.ReplicaInfo{
		Group:   c.GroupKeyStr,
		Replica: c.ReplicaKeyStr,
		Quorum:  0, // Must-success for DNS-based stores
	}
}
