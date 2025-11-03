// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package main

import (
	"path"
	"testing"

	"github.com/efficientgo/core/testutil"
	"github.com/go-kit/log"
	"github.com/thanos-io/objstore/client"
	"gopkg.in/yaml.v2"
)

func TestMultiTenancyBucketConfigMarshaling(t *testing.T) {
	tests := []struct {
		name             string
		config           MultiTenancyBucketConfig
		expectedPrefixes []string
	}{
		{
			name: "empty tenant prefixes",
			config: MultiTenancyBucketConfig{
				Type: client.FILESYSTEM,
				Config: map[string]interface{}{
					"directory": "/tmp/test",
				},
				Prefix:         "",
				TenantPrefixes: []string{},
			},
			expectedPrefixes: []string{},
		},
		{
			name: "single tenant prefix",
			config: MultiTenancyBucketConfig{
				Type: client.FILESYSTEM,
				Config: map[string]interface{}{
					"directory": "/tmp/test",
				},
				Prefix:         "",
				TenantPrefixes: []string{"tenant-a"},
			},
			expectedPrefixes: []string{"tenant-a"},
		},
		{
			name: "multiple tenant prefixes",
			config: MultiTenancyBucketConfig{
				Type: client.FILESYSTEM,
				Config: map[string]interface{}{
					"directory": "/tmp/test",
				},
				Prefix:         "",
				TenantPrefixes: []string{"tenant-a", "tenant-b", "tenant-c"},
			},
			expectedPrefixes: []string{"tenant-a", "tenant-b", "tenant-c"},
		},
		{
			name: "with base prefix",
			config: MultiTenancyBucketConfig{
				Type: client.FILESYSTEM,
				Config: map[string]interface{}{
					"directory": "/tmp/test",
				},
				Prefix:         "v1/raw/",
				TenantPrefixes: []string{"tenant-a", "tenant-b"},
			},
			expectedPrefixes: []string{"tenant-a", "tenant-b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to YAML
			yamlBytes, err := yaml.Marshal(tt.config)
			testutil.Ok(t, err)

			// Unmarshal back
			var unmarshaledConfig MultiTenancyBucketConfig
			err = yaml.Unmarshal(yamlBytes, &unmarshaledConfig)
			testutil.Ok(t, err)

			// Verify prefixes are preserved
			testutil.Equals(t, tt.expectedPrefixes, unmarshaledConfig.TenantPrefixes)
			testutil.Equals(t, tt.config.Prefix, unmarshaledConfig.Prefix)
			testutil.Equals(t, tt.config.Type, unmarshaledConfig.Type)
		})
	}
}

func TestTenantPrefixBucketCreation(t *testing.T) {
	tests := []struct {
		name                      string
		multiTenancyConfig        MultiTenancyBucketConfig
		expectedPrefixes          []string
		expectedEffectivePrefixes []string
	}{
		{
			name: "single-tenant mode (no prefixes)",
			multiTenancyConfig: MultiTenancyBucketConfig{
				Type: client.FILESYSTEM,
				Config: map[string]interface{}{
					"directory": "/tmp/test",
				},
				Prefix:         "",
				TenantPrefixes: []string{},
			},
			expectedPrefixes:          []string{""},
			expectedEffectivePrefixes: []string{""},
		},
		{
			name: "multi-tenant mode with one tenant",
			multiTenancyConfig: MultiTenancyBucketConfig{
				Type: client.FILESYSTEM,
				Config: map[string]interface{}{
					"directory": "/tmp/test",
				},
				Prefix:         "",
				TenantPrefixes: []string{"tenant1"},
			},
			expectedPrefixes:          []string{"tenant1"},
			expectedEffectivePrefixes: []string{"tenant1"},
		},
		{
			name: "multi-tenant mode with multiple tenants",
			multiTenancyConfig: MultiTenancyBucketConfig{
				Type: client.FILESYSTEM,
				Config: map[string]interface{}{
					"directory": "/tmp/test",
				},
				Prefix:         "",
				TenantPrefixes: []string{"tenant1", "tenant2", "tenant3"},
			},
			expectedPrefixes:          []string{"tenant1", "tenant2", "tenant3"},
			expectedEffectivePrefixes: []string{"tenant1", "tenant2", "tenant3"},
		},
		{
			name: "multi-tenant mode with base prefix",
			multiTenancyConfig: MultiTenancyBucketConfig{
				Type: client.FILESYSTEM,
				Config: map[string]interface{}{
					"directory": "/tmp/test",
				},
				Prefix:         "v1/raw/",
				TenantPrefixes: []string{"tenant1", "tenant2"},
			},
			expectedPrefixes:          []string{"tenant1", "tenant2"},
			expectedEffectivePrefixes: []string{"v1/raw/tenant1", "v1/raw/tenant2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate what runCompact does
			tenantPrefixes := tt.multiTenancyConfig.TenantPrefixes
			if len(tenantPrefixes) == 0 {
				tenantPrefixes = []string{""}
			}

			testutil.Equals(t, tt.expectedPrefixes, tenantPrefixes)

			// Simulate bucket creation for each tenant
			var actualEffectivePrefixes []string
			for _, tenantPrefix := range tenantPrefixes {
				bucketConf := &client.BucketConfig{
					Type:   tt.multiTenancyConfig.Type,
					Config: tt.multiTenancyConfig.Config,
					Prefix: path.Join(tt.multiTenancyConfig.Prefix, tenantPrefix),
				}
				actualEffectivePrefixes = append(actualEffectivePrefixes, bucketConf.Prefix)

				// Verify the bucket can be created
				// Use a real temp directory for filesystem buckets
				if bucketConf.Type == client.FILESYSTEM {
					configMap := make(map[string]interface{})
					for k, v := range bucketConf.Config.(map[string]interface{}) {
						configMap[k] = v
					}
					configMap["directory"] = t.TempDir()
					bucketConf.Config = configMap
				}

				tenantYaml, err := yaml.Marshal(bucketConf)
				testutil.Ok(t, err)

				bkt, err := client.NewBucket(log.NewNopLogger(), tenantYaml, "test", nil)
				testutil.Ok(t, err)
				testutil.Ok(t, bkt.Close())
			}

			testutil.Equals(t, tt.expectedEffectivePrefixes, actualEffectivePrefixes)
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
	tests := []struct {
		name                   string
		yamlConfig             string
		expectedTenantPrefixes []string
		expectedPrefix         string
		expectedType           client.ObjProvider
	}{
		{
			name: "multi-tenant config without base prefix",
			yamlConfig: `
type: FILESYSTEM
config:
  directory: /tmp/test
prefix: ""
tenant_prefixes:
  - tenant-alpha
  - tenant-beta
  - tenant-gamma
`,
			expectedTenantPrefixes: []string{"tenant-alpha", "tenant-beta", "tenant-gamma"},
			expectedPrefix:         "",
			expectedType:           client.FILESYSTEM,
		},
		{
			name: "multi-tenant config with base prefix",
			yamlConfig: `
type: FILESYSTEM
config:
  directory: /tmp/test
prefix: "v1/raw/"
tenant_prefixes:
  - tenant-a
  - tenant-b
`,
			expectedTenantPrefixes: []string{"tenant-a", "tenant-b"},
			expectedPrefix:         "v1/raw/",
			expectedType:           client.FILESYSTEM,
		},
		{
			name: "single-tenant config (no tenant_prefixes)",
			yamlConfig: `
type: FILESYSTEM
config:
  directory: /tmp/test
prefix: ""
`,
			expectedTenantPrefixes: nil,
			expectedPrefix:         "",
			expectedType:           client.FILESYSTEM,
		},
		{
			name: "S3 multi-tenant config",
			yamlConfig: `
type: S3
config:
  bucket: test-bucket
  endpoint: s3.amazonaws.com
prefix: "data/"
tenant_prefixes:
  - org1
  - org2
`,
			expectedTenantPrefixes: []string{"org1", "org2"},
			expectedPrefix:         "data/",
			expectedType:           client.S3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var multiTenancyConfig MultiTenancyBucketConfig
			err := yaml.Unmarshal([]byte(tt.yamlConfig), &multiTenancyConfig)
			testutil.Ok(t, err)

			// Verify all fields are correctly parsed
			// For TenantPrefixes, both nil and empty slice are equivalent
			if tt.expectedTenantPrefixes == nil && len(multiTenancyConfig.TenantPrefixes) == 0 {
				// Both nil and [] are acceptable for "no tenant prefixes"
			} else {
				testutil.Equals(t, tt.expectedTenantPrefixes, multiTenancyConfig.TenantPrefixes)
			}
			testutil.Equals(t, tt.expectedPrefix, multiTenancyConfig.Prefix)
			testutil.Equals(t, tt.expectedType, multiTenancyConfig.Type)

			// Verify we can marshal back
			yamlBytes, err := yaml.Marshal(multiTenancyConfig)
			testutil.Ok(t, err)

			// Verify we can unmarshal again and get the same result
			var roundtripConfig MultiTenancyBucketConfig
			err = yaml.Unmarshal(yamlBytes, &roundtripConfig)
			testutil.Ok(t, err)

			// For roundtrip, verify lengths match (handles nil vs [] equivalence)
			testutil.Equals(t, len(multiTenancyConfig.TenantPrefixes), len(roundtripConfig.TenantPrefixes))
			if len(multiTenancyConfig.TenantPrefixes) > 0 {
				testutil.Equals(t, multiTenancyConfig.TenantPrefixes, roundtripConfig.TenantPrefixes)
			}
			testutil.Equals(t, multiTenancyConfig.Prefix, roundtripConfig.Prefix)
			testutil.Equals(t, multiTenancyConfig.Type, roundtripConfig.Type)
		})
	}
}
