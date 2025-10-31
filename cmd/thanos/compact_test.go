// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package main

import (
	"testing"

	"github.com/efficientgo/core/testutil"
	"github.com/go-kit/log"
	"github.com/thanos-io/objstore/client"
	"gopkg.in/yaml.v2"
)

func TestAccessTenantPrefixes(t *testing.T) {
	tests := []struct {
		name             string
		bucketConfig     client.BucketConfig
		expectedPrefixes []string
		expectError      bool
	}{
		{
			name: "bucket config with tenant_prefixes",
			bucketConfig: client.BucketConfig{
				Type: client.FILESYSTEM,
				Config: map[string]interface{}{
					"directory":       "/tmp/test",
					"tenant_prefixes": []interface{}{"tenant1", "tenant2", "tenant3"},
				},
			},
			expectedPrefixes: []string{"tenant1", "tenant2", "tenant3"},
			expectError:      false,
		},
		{
			name: "bucket config without tenant_prefixes",
			bucketConfig: client.BucketConfig{
				Type: client.FILESYSTEM,
				Config: map[string]interface{}{
					"directory": "/tmp/test",
				},
			},
			expectedPrefixes: nil,
			expectError:      false,
		},
		{
			name: "bucket config with empty tenant_prefixes",
			bucketConfig: client.BucketConfig{
				Type: client.FILESYSTEM,
				Config: map[string]interface{}{
					"directory":       "/tmp/test",
					"tenant_prefixes": []interface{}{},
				},
			},
			expectedPrefixes: []string{},
			expectError:      false,
		},
		{
			name: "bucket config with single tenant_prefix",
			bucketConfig: client.BucketConfig{
				Type: client.FILESYSTEM,
				Config: map[string]interface{}{
					"directory":       "/tmp/test",
					"tenant_prefixes": []interface{}{"tenant-a"},
				},
			},
			expectedPrefixes: []string{"tenant-a"},
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefixes, err := accessTenantPrefixes(tt.bucketConfig)

			if tt.expectError {
				testutil.NotOk(t, err)
			} else {
				testutil.Ok(t, err)
				testutil.Equals(t, tt.expectedPrefixes, prefixes)
			}
		})
	}
}

func TestTenantBucketConfigMarshaling(t *testing.T) {
	tests := []struct {
		name             string
		config           TenantBucketConfig
		expectedPrefixes []string
	}{
		{
			name: "empty tenant prefixes",
			config: TenantBucketConfig{
				TenantPrefixes: []string{},
			},
			expectedPrefixes: []string{},
		},
		{
			name: "single tenant prefix",
			config: TenantBucketConfig{
				TenantPrefixes: []string{"tenant-a"},
			},
			expectedPrefixes: []string{"tenant-a"},
		},
		{
			name: "multiple tenant prefixes",
			config: TenantBucketConfig{
				TenantPrefixes: []string{"tenant-a", "tenant-b", "tenant-c"},
			},
			expectedPrefixes: []string{"tenant-a", "tenant-b", "tenant-c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to YAML
			yamlBytes, err := yaml.Marshal(tt.config)
			testutil.Ok(t, err)

			// Unmarshal back
			var unmarshaledConfig TenantBucketConfig
			err = yaml.Unmarshal(yamlBytes, &unmarshaledConfig)
			testutil.Ok(t, err)

			// Verify prefixes are preserved
			testutil.Equals(t, tt.expectedPrefixes, unmarshaledConfig.TenantPrefixes)
		})
	}
}

func TestTenantPrefixBucketCreation(t *testing.T) {
	tests := []struct {
		name             string
		tenantPrefixes   []string
		expectedPrefixes []string
	}{
		{
			name:             "single-tenant mode (no prefixes)",
			tenantPrefixes:   []string{},
			expectedPrefixes: []string{""},
		},
		{
			name:             "multi-tenant mode with one tenant",
			tenantPrefixes:   []string{"tenant1"},
			expectedPrefixes: []string{"v1/raw/tenant1"},
		},
		{
			name:             "multi-tenant mode with multiple tenants",
			tenantPrefixes:   []string{"tenant1", "tenant2", "tenant3"},
			expectedPrefixes: []string{"v1/raw/tenant1", "v1/raw/tenant2", "v1/raw/tenant3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create base bucket config
			baseBucketConf := client.BucketConfig{
				Type: client.FILESYSTEM,
				Config: map[string]interface{}{
					"directory": t.TempDir(),
				},
				Prefix: "",
			}

			// Add tenant_prefixes if provided
			if len(tt.tenantPrefixes) > 0 {
				configMap := baseBucketConf.Config.(map[string]interface{})
				configMap["tenant_prefixes"] = tt.tenantPrefixes
				baseBucketConf.Config = configMap
			}

			baseYaml, err := yaml.Marshal(baseBucketConf)
			testutil.Ok(t, err)

			var bucketConf client.BucketConfig
			err = yaml.Unmarshal(baseYaml, &bucketConf)
			testutil.Ok(t, err)

			// Get tenant prefixes (simulating what runCompact does)
			tenantPrefixes := []string{""}
			extractedPrefixes, err := accessTenantPrefixes(bucketConf)
			if err == nil && len(extractedPrefixes) > 0 {
				tenantPrefixes = extractedPrefixes
			}

			// Simulate what the code does: create buckets for each tenant
			var actualPrefixes []string
			for _, tenantPrefix := range tenantPrefixes {
				if tenantPrefix != "" {
					bucketConf.Prefix = "v1/raw/" + tenantPrefix
				}
				actualPrefixes = append(actualPrefixes, bucketConf.Prefix)

				// Verify the bucket can be created
				tenantYaml, err := yaml.Marshal(bucketConf)
				testutil.Ok(t, err)

				bkt, err := client.NewBucket(log.NewNopLogger(), tenantYaml, "test", nil)
				testutil.Ok(t, err)
				testutil.Ok(t, bkt.Close())
			}

			testutil.Equals(t, tt.expectedPrefixes, actualPrefixes)
		})
	}
}

func TestBucketConfigPrefixPreservation(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
	}{
		{
			name:   "empty prefix",
			prefix: "",
		},
		{
			name:   "simple prefix",
			prefix: "tenant1",
		},
		{
			name:   "v1/raw prefix",
			prefix: "v1/raw/tenant1",
		},
		{
			name:   "hierarchical prefix",
			prefix: "org/team/tenant1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a bucket config with prefix
			originalConf := client.BucketConfig{
				Type: client.FILESYSTEM,
				Config: map[string]interface{}{
					"directory": "/tmp/test",
				},
				Prefix: tt.prefix,
			}

			// Marshal to YAML
			yamlBytes, err := yaml.Marshal(originalConf)
			testutil.Ok(t, err)

			// Unmarshal back
			var unmarshaledConf client.BucketConfig
			err = yaml.Unmarshal(yamlBytes, &unmarshaledConf)
			testutil.Ok(t, err)

			// Verify prefix is preserved
			testutil.Equals(t, tt.prefix, unmarshaledConf.Prefix)
		})
	}
}

func TestTenantPrefixesFromYAML(t *testing.T) {
	yamlConfig := `
type: FILESYSTEM
config:
  directory: /tmp/test
  tenant_prefixes:
    - tenant-alpha
    - tenant-beta
    - tenant-gamma
prefix: ""
`

	var bucketConf client.BucketConfig
	err := yaml.Unmarshal([]byte(yamlConfig), &bucketConf)
	testutil.Ok(t, err)

	// Extract tenant prefixes
	tenantPrefixes, err := accessTenantPrefixes(bucketConf)
	testutil.Ok(t, err)

	expected := []string{"tenant-alpha", "tenant-beta", "tenant-gamma"}
	testutil.Equals(t, expected, tenantPrefixes)
}
