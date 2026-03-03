// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package receive

import (
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/efficientgo/core/testutil"
	"github.com/stretchr/testify/require"
	"github.com/thanos-io/thanos/pkg/store/labelpb"
	"github.com/thanos-io/thanos/pkg/store/storepb/prompb"
)

// podDNS creates a DNS-like string for testing endpoint addresses.
func podDNS(name string, shard int) string {
	return name + "-" + strconv.Itoa(shard) + ".test-svc.test-namespace.svc.cluster.local"
}

func TestGroupByAZ(t *testing.T) {
	// Test setup endpoints.
	ep0a := Endpoint{Address: podDNS("pod", 0), AZ: "zone-a", Shard: 0}
	ep1a := Endpoint{Address: podDNS("pod", 1), AZ: "zone-a", Shard: 1}
	ep2a := Endpoint{Address: podDNS("pod", 2), AZ: "zone-a", Shard: 2}
	ep0b := Endpoint{Address: podDNS("pod", 0), AZ: "zone-b", Shard: 0}
	ep1b := Endpoint{Address: podDNS("pod", 1), AZ: "zone-b", Shard: 1}
	ep0c := Endpoint{Address: podDNS("pod", 0), AZ: "zone-c", Shard: 0}
	ep1c := Endpoint{Address: podDNS("pod", 1), AZ: "zone-c", Shard: 1}
	duplicateEp0a := Endpoint{Address: podDNS("anotherpod", 0), AZ: "zone-a", Shard: 0} // Same shard (0) as ep0a in zone-a.

	testCases := map[string]struct {
		inputEndpoints []Endpoint
		expectedResult [][]Endpoint
		expectError    bool
		errorContains  string
	}{
		"error on empty input": {
			inputEndpoints: []Endpoint{},
			expectedResult: nil,
			expectError:    true,
			errorContains:  "no endpoints provided",
		},
		"single AZ, multiple endpoints": {
			inputEndpoints: []Endpoint{ep1a, ep0a, ep2a},
			expectedResult: [][]Endpoint{
				{ep0a, ep1a, ep2a},
			},
			expectError: false,
		},
		"multiple AZs, balanced and ordered": {
			inputEndpoints: []Endpoint{ep1a, ep0b, ep0a, ep1b},
			expectedResult: [][]Endpoint{
				{ep0a, ep1a},
				{ep0b, ep1b},
			},
			expectError: false,
		},
		"multiple AZs, different counts, stops at first missing shard > 0": {
			inputEndpoints: []Endpoint{ep1a, ep0b, ep0a, ep1b, ep2a, ep0c},
			expectedResult: [][]Endpoint{
				{ep0a},
				{ep0b},
				{ep0c},
			},
			expectError: false,
		},
		"error if shard 0 missing in any AZ": {
			inputEndpoints: []Endpoint{ep1a, ep2a, ep1b},
			expectedResult: nil,
			expectError:    true,
			errorContains:  "missing endpoint with shard 0",
		},
		"error if shard 0 missing in only one AZ": {
			inputEndpoints: []Endpoint{ep0a, ep1a, ep1b},
			expectedResult: nil,
			expectError:    true,
			errorContains:  `AZ "zone-b" is missing endpoint with shard 0`,
		},
		"error on duplicate shard within an AZ": {
			inputEndpoints: []Endpoint{ep0a, ep1a, ep0b, duplicateEp0a},
			expectedResult: nil,
			expectError:    true,
			errorContains:  "duplicate endpoint shard 0 for address " + duplicateEp0a.Address + " in AZ zone-a",
		},
		"AZ sorting check": {
			inputEndpoints: []Endpoint{ep0b, ep0c, ep0a},
			expectedResult: [][]Endpoint{
				{ep0a},
				{ep0b},
				{ep0c},
			},
			expectError: false,
		},
		"multiple AZs, stops correctly when next shard missing everywhere": {
			inputEndpoints: []Endpoint{ep1a, ep0b, ep0a, ep1b, ep0c, ep1c},
			expectedResult: [][]Endpoint{
				{ep0a, ep1a},
				{ep0b, ep1b},
				{ep0c, ep1c},
			},
			expectError: false,
		},
	}

	for tcName, tc := range testCases {
		t.Run(tcName, func(t *testing.T) {
			result, err := groupByAZ(tc.inputEndpoints)

			if tc.expectError {
				testutil.NotOk(t, err)
				if tc.errorContains != "" {
					testutil.Assert(t, strings.Contains(err.Error(), tc.errorContains), "Expected error message to contain '%s', but got: %v", tc.errorContains, err)
				}
				testutil.Assert(t, result == nil, "Expected nil result on error, got: %v", result)
			} else {
				testutil.Ok(t, err)
				testutil.Equals(t, tc.expectedResult, result)

				// Verify outer slice (AZs) is sorted alphabetically.
				if err == nil && len(result) > 1 {
					azOrderCorrect := sort.SliceIsSorted(result, func(i, j int) bool {
						return result[i][0].AZ < result[j][0].AZ
					})
					testutil.Assert(t, azOrderCorrect, "Outer slice is not sorted by AZ")
				}
			}
		})
	}
}

func TestAlignedKetamaHashringGet(t *testing.T) {
	t.Parallel()

	ep0a := Endpoint{Address: podDNS("pod", 0), AZ: "zone-a", Shard: 0}
	ep1a := Endpoint{Address: podDNS("pod", 1), AZ: "zone-a", Shard: 1}
	ep0b := Endpoint{Address: podDNS("pod", 0), AZ: "zone-b", Shard: 0}
	ep1b := Endpoint{Address: podDNS("pod", 1), AZ: "zone-b", Shard: 1}
	ep0c := Endpoint{Address: podDNS("pod", 0), AZ: "zone-c", Shard: 0}
	ep1c := Endpoint{Address: podDNS("pod", 1), AZ: "zone-c", Shard: 1}
	duplicateEp0a := Endpoint{Address: podDNS("anotherpod", 0), AZ: "zone-a", Shard: 0}

	tsForReplicaTest := &prompb.TimeSeries{
		Labels: []labelpb.ZLabel{{Name: "test", Value: "replica-routing"}},
	}

	testCases := map[string]struct {
		inputEndpoints    []Endpoint
		replicationFactor uint64
		sectionsPerNode   int

		tenant           string
		ts               *prompb.TimeSeries
		n                uint64
		expectedEndpoint Endpoint

		expectConstructorError   bool
		constructorErrorContains string
		expectGetNError          bool
		getNErrorContains        string
	}{
		"valid 2 AZs, RF=2, get replica 0": {
			inputEndpoints:         []Endpoint{ep1a, ep0b, ep0a, ep1b},
			replicationFactor:      2,
			sectionsPerNode:        SectionsPerNode,
			tenant:                 "tenant1",
			ts:                     tsForReplicaTest,
			n:                      0,
			expectedEndpoint:       ep0a,
			expectConstructorError: false,
			expectGetNError:        false,
		},
		"valid 2 AZs, RF=2, get replica 1": {
			inputEndpoints:         []Endpoint{ep1a, ep0b, ep0a, ep1b},
			replicationFactor:      2,
			sectionsPerNode:        SectionsPerNode,
			tenant:                 "tenant1",
			ts:                     tsForReplicaTest,
			n:                      1,
			expectedEndpoint:       ep0b,
			expectConstructorError: false,
			expectGetNError:        false,
		},
		"valid 3 AZs, RF=3, get replica 0": {
			inputEndpoints:         []Endpoint{ep1a, ep0b, ep0a, ep1b, ep0c, ep1c},
			replicationFactor:      3,
			sectionsPerNode:        SectionsPerNode,
			tenant:                 "tenant1",
			ts:                     tsForReplicaTest,
			n:                      0,
			expectedEndpoint:       ep0a,
			expectConstructorError: false,
			expectGetNError:        false,
		},
		"valid 3 AZs, RF=3, get replica 1": {
			inputEndpoints:         []Endpoint{ep1a, ep0b, ep0a, ep1b, ep0c, ep1c},
			replicationFactor:      3,
			sectionsPerNode:        SectionsPerNode,
			tenant:                 "tenant1",
			ts:                     tsForReplicaTest,
			n:                      1,
			expectedEndpoint:       ep0b,
			expectConstructorError: false,
			expectGetNError:        false,
		},
		"valid 3 AZs, RF=3, get replica 2": {
			inputEndpoints:         []Endpoint{ep1a, ep0b, ep0a, ep1b, ep0c, ep1c},
			replicationFactor:      3,
			sectionsPerNode:        SectionsPerNode,
			tenant:                 "tenant1",
			ts:                     tsForReplicaTest,
			n:                      2,
			expectedEndpoint:       ep0c,
			expectConstructorError: false,
			expectGetNError:        false,
		},
		"error: empty input": {
			inputEndpoints:           []Endpoint{},
			replicationFactor:        1,
			sectionsPerNode:          SectionsPerNode,
			expectConstructorError:   true,
			constructorErrorContains: "no endpoints provided",
		},
		"error: duplicate shard": {
			inputEndpoints:           []Endpoint{ep0a, ep1a, ep0b, duplicateEp0a},
			replicationFactor:        2,
			sectionsPerNode:          SectionsPerNode,
			expectConstructorError:   true,
			constructorErrorContains: "duplicate endpoint",
		},
		"error: missing shard 0": {
			inputEndpoints:           []Endpoint{ep1a, ep1b},
			replicationFactor:        2,
			sectionsPerNode:          SectionsPerNode,
			expectConstructorError:   true,
			constructorErrorContains: "failed to group endpoints by AZ: AZ \"zone-a\" is missing endpoint with shard 0",
		},
		"error: AZ count != RF (too few AZs)": {
			inputEndpoints:           []Endpoint{ep0a, ep1a},
			replicationFactor:        2,
			sectionsPerNode:          SectionsPerNode,
			expectConstructorError:   true,
			constructorErrorContains: "number of AZs (1) must equal replication factor (2)",
		},
		"error: AZ count != RF (too many AZs)": {
			inputEndpoints:           []Endpoint{ep0a, ep1a, ep0b, ep1b, ep0c, ep1c},
			replicationFactor:        2,
			sectionsPerNode:          SectionsPerNode,
			expectConstructorError:   true,
			constructorErrorContains: "number of AZs (3) must equal replication factor (2)",
		},
		"constructor success with unbalanced AZs (uses common subset)": {
			inputEndpoints:         []Endpoint{ep0a, ep1a, ep0b},
			replicationFactor:      2,
			sectionsPerNode:        SectionsPerNode,
			expectConstructorError: false,
		},
		"error: GetN index out of bounds (n >= numEndpoints)": {
			inputEndpoints:         []Endpoint{ep1a, ep0b, ep0a, ep1b},
			replicationFactor:      2,
			sectionsPerNode:        SectionsPerNode,
			tenant:                 "tenant1",
			ts:                     tsForReplicaTest,
			n:                      4,
			expectConstructorError: false,
			expectGetNError:        true,
			getNErrorContains:      "insufficient nodes; have 4, want 5",
		},
	}

	for tcName, tc := range testCases {
		t.Run(tcName, func(t *testing.T) {
			hashRing, err := newAlignedKetamaHashring(tc.inputEndpoints, tc.sectionsPerNode, tc.replicationFactor)

			if tc.expectConstructorError {
				require.Error(t, err, "Expected constructor error")
				require.Nil(t, hashRing, "Hashring should be nil on constructor error")
				if tc.constructorErrorContains != "" {
					require.Contains(t, err.Error(), tc.constructorErrorContains, "Constructor error message mismatch")
				}
				return
			}

			require.NoError(t, err, "Expected constructor to succeed")
			require.NotNil(t, hashRing, "Hashring should not be nil on successful construction")

			if tc.ts == nil && !tc.expectGetNError {
				return
			}
			if tc.ts == nil && tc.expectGetNError {
				tc.ts = &prompb.TimeSeries{Labels: []labelpb.ZLabel{{Name: "dummy", Value: "dummy"}}}
			}

			result, getNErr := hashRing.GetN(tc.tenant, tc.ts, tc.n)
			if tc.expectGetNError {
				require.Error(t, getNErr, "Expected GetN error")
				if tc.getNErrorContains != "" {
					require.Contains(t, getNErr.Error(), tc.getNErrorContains, "GetN error message mismatch")
				}
			} else {
				require.NoError(t, getNErr, "Expected GetN to succeed")
				testutil.Equals(t, tc.expectedEndpoint, result, "GetN returned unexpected endpoint")
			}
		})
	}
}

func TestAlignedKetamaHashringReplicaShards(t *testing.T) {
	t.Parallel()

	var endpoints []Endpoint
	for i := 0; i < 20; i++ {
		endpoints = append(endpoints, Endpoint{Address: podDNS("pod", i), AZ: "zone-a", Shard: i})
	}
	for i := 0; i < 20; i++ {
		endpoints = append(endpoints, Endpoint{Address: podDNS("pod", i), AZ: "zone-b", Shard: i})
	}
	for i := 0; i < 20; i++ {
		endpoints = append(endpoints, Endpoint{Address: podDNS("pod", i), AZ: "zone-c", Shard: i})
	}
	replicationFactor := uint64(3)
	sectionsPerNode := 10

	hashRing, err := newAlignedKetamaHashring(endpoints, sectionsPerNode, replicationFactor)
	require.NoError(t, err, "Aligned hashring constructor failed")
	require.NotNil(t, hashRing, "Hashring should not be nil")
	require.NotEmpty(t, hashRing.sections, "Hashring should contain sections")

	// Verify that all replicas within a section have the same shard.
	for i, s := range hashRing.sections {
		if len(s.replicas) == 0 {
			continue
		}

		expectedShard := -1

		for replicaNum, replicaIndex := range s.replicas {
			require.Less(t, int(replicaIndex), len(hashRing.endpoints),
				"Section %d (hash %d), Replica %d: index %d out of bounds for endpoints list (len %d)",
				i, s.hash, replicaNum, replicaIndex, len(hashRing.endpoints))

			endpoint := hashRing.endpoints[replicaIndex]

			if expectedShard == -1 {
				expectedShard = endpoint.Shard
			} else {
				require.Equal(t, expectedShard, endpoint.Shard,
					"Section %d (hash %d), Replica %d (%s): Mismatched shard. Expected %d, got %d. Replicas in section: %v",
					i, s.hash, replicaNum, endpoint.Address, expectedShard, endpoint.Shard, s.replicas)
			}
		}
		if len(s.replicas) > 0 {
			require.NotEqual(t, -1, expectedShard, "Section %d (hash %d): Failed to determine expected shard for replicas %v", i, s.hash, s.replicas)
		}
	}
}
