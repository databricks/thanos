// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package storepb

// ReplicaInfo contains replica topology hints for GROUP_REPLICA partial response strategy.
// It unifies DNS-based grouping (legacy) with first-class StoreInfo fields.
type ReplicaInfo struct {
	// Group identifies stores that hold replicated data. Stores with the same Group
	// value are considered replicas of each other.
	// Examples: "pantheon-db", "long-range-store"
	Group string

	// Replica identifies this specific store within a group.
	// Examples: "pantheon-db-rep0", "pantheon-db-rep1"
	Replica string

	// Quorum is the minimum number of healthy stores required per group.
	// A value of 0 means "must-success" - the store must respond successfully.
	Quorum int
}

// IsMustSuccess returns true if this store must respond successfully (quorum=0).
func (ri ReplicaInfo) IsMustSuccess() bool {
	return ri.Quorum == 0
}
