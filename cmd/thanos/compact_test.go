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

func TestTenantConfigMarshaling(t *testing.T) {
	tests := []struct {
		name             string
		config           TenantConfig
		expectedPrefixes []string
	}{
		{
			name: "empty tenant prefixes",
			config: TenantConfig{
				TenantPrefixes: []string{},
			},
			expectedPrefixes: []string{},
		},
		{
			name: "single tenant prefix",
			config: TenantConfig{
				TenantPrefixes: []string{"tenant-a"},
			},
			expectedPrefixes: []string{"tenant-a"},
		},
		{
			name: "multiple tenant prefixes",
			config: TenantConfig{
				TenantPrefixes: []string{"tenant-a", "tenant-b", "tenant-c"},
			},
			expectedPrefixes: []string{"tenant-a", "tenant-b", "tenant-c"},
		},
		{
			name: "with full path prefixes",
			config: TenantConfig{
				TenantPrefixes: []string{"v1/raw/tenant-a", "v1/raw/tenant-b"},
			},
			expectedPrefixes: []string{"v1/raw/tenant-a", "v1/raw/tenant-b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to YAML
			yamlBytes, err := yaml.Marshal(tt.config)
			testutil.Ok(t, err)

			// Unmarshal back
			var unmarshaledConfig TenantConfig
			err = yaml.Unmarshal(yamlBytes, &unmarshaledConfig)
			testutil.Ok(t, err)

			// Verify prefixes are preserved (UnmarshalYAML converts empty to [""])
			testutil.Equals(t, tt.expectedPrefixes, unmarshaledConfig.TenantPrefixes)
		})
	}
}

func TestTenantPrefixBucketCreation(t *testing.T) {
	tests := []struct {
		name                      string
		bucketConfig              client.BucketConfig
		tenantConfig              TenantConfig
		expectedPrefixes          []string
		expectedEffectivePrefixes []string
	}{
		{
			name: "single-tenant mode (no prefixes)",
			bucketConfig: client.BucketConfig{
				Type: client.FILESYSTEM,
				Config: map[string]interface{}{
					"directory": "/tmp/test",
				},
				Prefix: "",
			},
			tenantConfig: TenantConfig{
				TenantPrefixes: []string{},
			},
			expectedPrefixes:          []string{""},
			expectedEffectivePrefixes: []string{""},
		},
		{
			name: "multi-tenant mode with one tenant",
			bucketConfig: client.BucketConfig{
				Type: client.FILESYSTEM,
				Config: map[string]interface{}{
					"directory": "/tmp/test",
				},
				Prefix: "",
			},
			tenantConfig: TenantConfig{
				TenantPrefixes: []string{"tenant1"},
			},
			expectedPrefixes:          []string{"tenant1"},
			expectedEffectivePrefixes: []string{"tenant1"},
		},
		{
			name: "multi-tenant mode with multiple tenants",
			bucketConfig: client.BucketConfig{
				Type: client.FILESYSTEM,
				Config: map[string]interface{}{
					"directory": "/tmp/test",
				},
				Prefix: "",
			},
			tenantConfig: TenantConfig{
				TenantPrefixes: []string{"tenant1", "tenant2", "tenant3"},
			},
			expectedPrefixes:          []string{"tenant1", "tenant2", "tenant3"},
			expectedEffectivePrefixes: []string{"tenant1", "tenant2", "tenant3"},
		},
		{
			name: "multi-tenant mode with base prefix",
			bucketConfig: client.BucketConfig{
				Type: client.FILESYSTEM,
				Config: map[string]interface{}{
					"directory": "/tmp/test",
				},
				Prefix: "v1/raw/",
			},
			tenantConfig: TenantConfig{
				TenantPrefixes: []string{"tenant1", "tenant2"},
			},
			expectedPrefixes:          []string{"tenant1", "tenant2"},
			expectedEffectivePrefixes: []string{"v1/raw/tenant1", "v1/raw/tenant2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate what runCompact does
			tenantPrefixes := tt.tenantConfig.TenantPrefixes
			if len(tenantPrefixes) == 0 {
				tenantPrefixes = []string{""}
			}

			testutil.Equals(t, tt.expectedPrefixes, tenantPrefixes)

			// Simulate bucket creation for each tenant
			var actualEffectivePrefixes []string
			for _, tenantPrefix := range tenantPrefixes {
				bucketConf := &client.BucketConfig{
					Type:   tt.bucketConfig.Type,
					Config: tt.bucketConfig.Config,
					Prefix: path.Join(tt.bucketConfig.Prefix, tenantPrefix),
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
		tenantYamlConfig       string
		bucketYamlConfig       string
		expectedTenantPrefixes []string
		expectedPrefix         string
		expectedType           client.ObjProvider
	}{
		{
			name: "multi-tenant config without base prefix",
			tenantYamlConfig: `
tenant_prefixes:
  - tenant-alpha
  - tenant-beta
  - tenant-gamma
`,
			bucketYamlConfig: `
type: FILESYSTEM
config:
  directory: /tmp/test
prefix: ""
`,
			expectedTenantPrefixes: []string{"tenant-alpha", "tenant-beta", "tenant-gamma"},
			expectedPrefix:         "",
			expectedType:           client.FILESYSTEM,
		},
		{
			name: "multi-tenant config with base prefix",
			tenantYamlConfig: `
tenant_prefixes:
  - tenant-a
  - tenant-b
`,
			bucketYamlConfig: `
type: FILESYSTEM
config:
  directory: /tmp/test
prefix: "v1/raw/"
`,
			expectedTenantPrefixes: []string{"tenant-a", "tenant-b"},
			expectedPrefix:         "v1/raw/",
			expectedType:           client.FILESYSTEM,
		},
		{
			name:             "single-tenant config (empty YAML)",
			tenantYamlConfig: `{}`,
			bucketYamlConfig: `
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
			tenantYamlConfig: `
tenant_prefixes:
  - org1
  - org2
`,
			bucketYamlConfig: `
type: S3
config:
  bucket: test-bucket
  endpoint: s3.amazonaws.com
prefix: "data/"
`,
			expectedTenantPrefixes: []string{"org1", "org2"},
			expectedPrefix:         "data/",
			expectedType:           client.S3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tenantConfig TenantConfig
			err := yaml.Unmarshal([]byte(tt.tenantYamlConfig), &tenantConfig)
			testutil.Ok(t, err)

			var bucketConfig client.BucketConfig
			err = yaml.Unmarshal([]byte(tt.bucketYamlConfig), &bucketConfig)
			testutil.Ok(t, err)

			// Verify all fields are correctly parsed
			testutil.Equals(t, tt.expectedTenantPrefixes, tenantConfig.TenantPrefixes)
			testutil.Equals(t, tt.expectedPrefix, bucketConfig.Prefix)
			testutil.Equals(t, tt.expectedType, bucketConfig.Type)

			// Verify we can marshal back
			tenantYamlBytes, err := yaml.Marshal(tenantConfig)
			testutil.Ok(t, err)

			bucketYamlBytes, err := yaml.Marshal(bucketConfig)
			testutil.Ok(t, err)

			// Verify we can unmarshal again and get the same result
			var roundtripTenantConfig TenantConfig
			err = yaml.Unmarshal(tenantYamlBytes, &roundtripTenantConfig)
			testutil.Ok(t, err)

			var roundtripBucketConfig client.BucketConfig
			err = yaml.Unmarshal(bucketYamlBytes, &roundtripBucketConfig)
			testutil.Ok(t, err)

			// For roundtrip, verify configs match
			// Note: nil and empty slice are equivalent for our purposes
			if len(tenantConfig.TenantPrefixes) == 0 && len(roundtripTenantConfig.TenantPrefixes) == 0 {
				// Both are empty (nil or []string{}), which is fine
			} else {
				testutil.Equals(t, tenantConfig.TenantPrefixes, roundtripTenantConfig.TenantPrefixes)
			}
			testutil.Equals(t, bucketConfig.Prefix, roundtripBucketConfig.Prefix)
			testutil.Equals(t, bucketConfig.Type, roundtripBucketConfig.Type)
		})
	}
}
