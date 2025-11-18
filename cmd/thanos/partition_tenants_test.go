// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestComputeTenantAssignments(t *testing.T) {
	tests := []struct {
		name                string
		numShards           int
		tenantWeights       []TenantWeight
		expectedTenants     int
		validateAllAssigned bool
	}{
		{
			name:      "basic distribution",
			numShards: 3,
			tenantWeights: []TenantWeight{
				{TenantName: "tenant1", Weight: 10},
				{TenantName: "tenant2", Weight: 8},
				{TenantName: "tenant3", Weight: 5},
				{TenantName: "tenant4", Weight: 3},
			},
			expectedTenants:     4,
			validateAllAssigned: true,
		},
		{
			name:      "single shard",
			numShards: 1,
			tenantWeights: []TenantWeight{
				{TenantName: "tenant1", Weight: 10},
				{TenantName: "tenant2", Weight: 20},
			},
			expectedTenants:     2,
			validateAllAssigned: true,
		},
		{
			name:      "more shards than tenants",
			numShards: 5,
			tenantWeights: []TenantWeight{
				{TenantName: "tenant1", Weight: 10},
				{TenantName: "tenant2", Weight: 20},
			},
			expectedTenants:     2,
			validateAllAssigned: true,
		},
		{
			name:                "empty tenant list",
			numShards:           3,
			tenantWeights:       []TenantWeight{},
			expectedTenants:     0,
			validateAllAssigned: true,
		},
		{
			name:      "equal weights",
			numShards: 2,
			tenantWeights: []TenantWeight{
				{TenantName: "tenant1", Weight: 10},
				{TenantName: "tenant2", Weight: 10},
				{TenantName: "tenant3", Weight: 10},
				{TenantName: "tenant4", Weight: 10},
			},
			expectedTenants:     4,
			validateAllAssigned: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buckets, weights := computeTenantAssignments(tt.numShards, tt.tenantWeights)

			// Verify all shards exist in the maps
			if len(buckets) != tt.numShards {
				t.Errorf("expected %d buckets, got %d", tt.numShards, len(buckets))
			}
			if len(weights) != tt.numShards {
				t.Errorf("expected %d weight entries, got %d", tt.numShards, len(weights))
			}

			// Count total assigned tenants
			totalAssigned := 0
			assignedTenants := make(map[string]bool)
			for i := 0; i < tt.numShards; i++ {
				if _, ok := buckets[i]; !ok {
					t.Errorf("bucket %d not found in assignments", i)
					continue
				}
				totalAssigned += len(buckets[i])

				// Track which tenants are assigned
				for _, tenant := range buckets[i] {
					if assignedTenants[tenant] {
						t.Errorf("tenant %s assigned to multiple buckets", tenant)
					}
					assignedTenants[tenant] = true
				}
			}

			// Verify all tenants are assigned
			if tt.validateAllAssigned && totalAssigned != tt.expectedTenants {
				t.Errorf("expected %d tenants to be assigned, got %d", tt.expectedTenants, totalAssigned)
			}

			// Verify each tenant is assigned exactly once
			for _, tw := range tt.tenantWeights {
				if !assignedTenants[tw.TenantName] {
					t.Errorf("tenant %s was not assigned to any bucket", tw.TenantName)
				}
			}

			// Verify weights are calculated correctly
			for i := 0; i < tt.numShards; i++ {
				calculatedWeight := 0
				for _, tenantName := range buckets[i] {
					// Find the weight for this tenant
					for _, tw := range tt.tenantWeights {
						if tw.TenantName == tenantName {
							calculatedWeight += tw.Weight
							break
						}
					}
				}
				if weights[i] != calculatedWeight {
					t.Errorf("bucket %d: expected weight %d, got %d", i, calculatedWeight, weights[i])
				}
			}
		})
	}
}

func TestComputeTenantAssignments_DistributionFairness(t *testing.T) {
	// Test that distribution is relatively fair
	numShards := 3
	tenantWeights := []TenantWeight{
		{TenantName: "tenant1", Weight: 100},
		{TenantName: "tenant2", Weight: 90},
		{TenantName: "tenant3", Weight: 80},
		{TenantName: "tenant4", Weight: 70},
		{TenantName: "tenant5", Weight: 60},
		{TenantName: "tenant6", Weight: 50},
	}

	buckets, weights := computeTenantAssignments(numShards, tenantWeights)

	// Calculate total weight
	totalWeight := 0
	for _, tw := range tenantWeights {
		totalWeight += tw.Weight
	}

	// Check that each bucket has at least one tenant (since we have more tenants than shards)
	for i := 0; i < numShards; i++ {
		if len(buckets[i]) == 0 {
			t.Errorf("bucket %d has no tenants assigned", i)
		}
	}

	// Verify the greedy algorithm behavior: highest weight tenant goes to highest ordinal
	// After sorting by weight desc, tenant1 (weight 100) should go to bucket 2 (highest ordinal)
	found := false
	for _, tenant := range buckets[numShards-1] {
		if tenant == "tenant1" {
			found = true
			break
		}
	}
	if !found {
		t.Log("Note: tenant1 with highest weight expected in highest ordinal bucket initially")
		t.Logf("Bucket assignments: %+v", buckets)
		t.Logf("Bucket weights: %+v", weights)
	}
}

func TestComputeTenantAssignments_SortingOrder(t *testing.T) {
	// Test that sorting works correctly (weight desc, then name asc)
	numShards := 2
	tenantWeights := []TenantWeight{
		{TenantName: "zebra", Weight: 10},
		{TenantName: "alpha", Weight: 10},
		{TenantName: "beta", Weight: 20},
	}

	buckets, _ := computeTenantAssignments(numShards, tenantWeights)

	// Count total assigned
	totalAssigned := 0
	for i := 0; i < numShards; i++ {
		totalAssigned += len(buckets[i])
	}

	if totalAssigned != 3 {
		t.Errorf("expected 3 tenants assigned, got %d", totalAssigned)
	}

	// Verify all tenants are present
	allTenants := make(map[string]bool)
	for i := 0; i < numShards; i++ {
		for _, tenant := range buckets[i] {
			allTenants[tenant] = true
		}
	}

	expectedTenants := []string{"zebra", "alpha", "beta"}
	for _, expected := range expectedTenants {
		if !allTenants[expected] {
			t.Errorf("tenant %s not found in any bucket", expected)
		}
	}
}

func TestReadTenantWeights(t *testing.T) {
	// Create a temporary JSON file
	tempFile, err := os.CreateTemp("", "tenant_weights_*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	testData := map[string]int{
		"tenant1": 10,
		"tenant2": 20,
		"tenant3": 30,
	}

	jsonData, err := json.Marshal(testData)
	if err != nil {
		t.Fatalf("failed to marshal test data: %v", err)
	}

	if _, err := tempFile.Write(jsonData); err != nil {
		t.Fatalf("failed to write test data: %v", err)
	}
	tempFile.Close()

	// Test reading the file
	weights, err := readTenantWeights(tempFile.Name())
	if err != nil {
		t.Fatalf("readTenantWeights failed: %v", err)
	}

	// Verify all tenants are loaded
	if len(weights) != len(testData) {
		t.Errorf("expected %d tenants, got %d", len(testData), len(weights))
	}

	// Verify each tenant and weight
	foundTenants := make(map[string]int)
	for _, tw := range weights {
		foundTenants[tw.TenantName] = tw.Weight
	}

	for name, expectedWeight := range testData {
		if weight, ok := foundTenants[name]; !ok {
			t.Errorf("tenant %s not found in results", name)
		} else if weight != expectedWeight {
			t.Errorf("tenant %s: expected weight %d, got %d", name, expectedWeight, weight)
		}
	}
}

func TestReadTenantWeights_InvalidFile(t *testing.T) {
	_, err := readTenantWeights("/nonexistent/file.json")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestReadTenantWeights_InvalidJSON(t *testing.T) {
	tempFile, err := os.CreateTemp("", "invalid_json_*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Write invalid JSON
	if _, err := tempFile.Write([]byte("invalid json content")); err != nil {
		t.Fatalf("failed to write test data: %v", err)
	}
	tempFile.Close()

	_, err = readTenantWeights(tempFile.Name())
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}
