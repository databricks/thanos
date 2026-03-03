// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package receive

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/cespare/xxhash"
	"github.com/pkg/errors"
)

// groupByAZ groups endpoints by Availability Zone and sorts them by shard.
// It returns a 2D slice where each inner slice represents an AZ (sorted alphabetically)
// and contains endpoints sorted by shard. All inner slices are truncated to the
// length of the largest common sequence of shards starting from 0 across all AZs.
func groupByAZ(endpoints []Endpoint) ([][]Endpoint, error) {
	if len(endpoints) == 0 {
		return nil, errors.New("no endpoints provided")
	}

	// Group endpoints by AZ and then by shard.
	azEndpoints := make(map[string]map[int]Endpoint)
	for _, ep := range endpoints {
		if _, ok := azEndpoints[ep.AZ]; !ok {
			azEndpoints[ep.AZ] = make(map[int]Endpoint)
		}
		if _, exists := azEndpoints[ep.AZ][ep.Shard]; exists {
			return nil, fmt.Errorf("duplicate endpoint shard %d for address %s in AZ %s", ep.Shard, ep.Address, ep.AZ)
		}
		azEndpoints[ep.AZ][ep.Shard] = ep
	}

	// Get sorted list of AZ names.
	sortedAZs := make([]string, 0, len(azEndpoints))
	for az := range azEndpoints {
		sortedAZs = append(sortedAZs, az)
	}
	sort.Strings(sortedAZs)

	// Determine the maximum common shard across all AZs.
	maxCommonShard := -1
	for i := 0; ; i++ {
		presentInAllAZs := true
		for _, az := range sortedAZs {
			if _, ok := azEndpoints[az][i]; !ok {
				presentInAllAZs = false
				if i == 0 {
					return nil, fmt.Errorf("AZ %q is missing endpoint with shard 0", az)
				}
				break
			}
		}
		if !presentInAllAZs {
			maxCommonShard = i - 1
			break
		}
	}
	if maxCommonShard < 0 {
		return nil, errors.New("no common endpoints with shard 0 found across all AZs")
	}
	numAZs := len(sortedAZs)
	result := make([][]Endpoint, numAZs)
	for i, az := range sortedAZs {
		result[i] = make([]Endpoint, 0, maxCommonShard+1)
		for j := 0; j <= maxCommonShard; j++ {
			result[i] = append(result[i], azEndpoints[az][j])
		}
	}
	return result, nil
}

// newAlignedKetamaHashring creates a Ketama hash ring where replicas are strictly aligned across Availability Zones.
// Each section on the hash ring corresponds to a primary endpoint (taken from the first AZ) and its
// aligned replicas in other AZs (endpoints with the same shard). The hash for a section is calculated
// based *only* on the primary endpoint's address.
func newAlignedKetamaHashring(endpoints []Endpoint, sectionsPerNode int, replicationFactor uint64) (*ketamaHashring, error) {
	if replicationFactor == 0 {
		return nil, errors.New("replication factor cannot be zero")
	}
	if sectionsPerNode <= 0 {
		return nil, errors.New("sections per node must be positive")
	}
	groupedEndpoints, err := groupByAZ(endpoints)
	if err != nil {
		return nil, errors.Wrap(err, "failed to group endpoints by AZ")
	}
	numAZs := len(groupedEndpoints)
	if numAZs == 0 {
		return nil, errors.New("no endpoint groups found after grouping by AZ")
	}
	if uint64(numAZs) != replicationFactor {
		return nil, fmt.Errorf("number of AZs (%d) must equal replication factor (%d)", numAZs, replicationFactor)
	}
	numEndpointsPerAZ := len(groupedEndpoints[0])
	if numEndpointsPerAZ == 0 {
		return nil, errors.New("AZ groups are empty after grouping")
	}
	totalEndpoints := numAZs * numEndpointsPerAZ
	flatEndpoints := make([]Endpoint, 0, totalEndpoints)
	for azIndex := 0; azIndex < numAZs; azIndex++ {
		flatEndpoints = append(flatEndpoints, groupedEndpoints[azIndex]...)
	}
	hasher := xxhash.New()
	ringSections := make(sections, 0, numEndpointsPerAZ*sectionsPerNode)

	// Iterate through primary endpoints (those in the first AZ) to define sections.
	for primaryShardIndex := 0; primaryShardIndex < numEndpointsPerAZ; primaryShardIndex++ {
		primaryEndpoint := groupedEndpoints[0][primaryShardIndex]
		for sectionIndex := 1; sectionIndex <= sectionsPerNode; sectionIndex++ {
			hasher.Reset()
			_, _ = hasher.Write([]byte(primaryEndpoint.Address + ":" + strconv.Itoa(sectionIndex)))
			sectionHash := hasher.Sum64()
			sec := &section{
				hash:          sectionHash,
				az:            primaryEndpoint.AZ,
				endpointIndex: uint64(primaryShardIndex),
				replicas:      make([]uint64, 0, replicationFactor),
			}

			// Find indices of all replicas (including primary) in the flat list.
			for azIndex := 0; azIndex < numAZs; azIndex++ {
				replicaFlatIndex := azIndex*numEndpointsPerAZ + primaryShardIndex
				sec.replicas = append(sec.replicas, uint64(replicaFlatIndex))
			}
			ringSections = append(ringSections, sec)
		}
	}
	sort.Sort(ringSections)
	return &ketamaHashring{
		endpoints:    flatEndpoints,
		sections:     ringSections,
		numEndpoints: uint64(totalEndpoints),
	}, nil
}
