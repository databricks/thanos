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

func TestExtractOrdinalFromHostname(t *testing.T) {
	tests := []struct {
		name            string
		hostname        string
		expectedOrdinal int
		expectError     bool
	}{
		{
			name:            "statefulset hostname with single digit",
			hostname:        "pantheon-compactor-0",
			expectedOrdinal: 0,
			expectError:     false,
		},
		{
			name:            "statefulset hostname with double digit",
			hostname:        "pantheon-compactor-15",
			expectedOrdinal: 15,
			expectError:     false,
		},
		{
			name:            "statefulset hostname with triple digit",
			hostname:        "pantheon-compactor-999",
			expectedOrdinal: 999,
			expectError:     false,
		},
		{
			name:            "kubernetes statefulset with namespace",
			hostname:        "pantheon-compactor-2",
			expectedOrdinal: 2,
			expectError:     false,
		},
		{
			name:            "complex statefulset name",
			hostname:        "pantheon-compactor-7",
			expectedOrdinal: 7,
			expectError:     false,
		},
		{
			name:        "hostname without number",
			hostname:    "pantheoncompactor",
			expectError: true,
		},
		{
			name:        "hostname with invalid suffix",
			hostname:    "pantheon-compactor-abc",
			expectError: true,
		},
		{
			name:        "empty hostname",
			hostname:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ordinal, err := extractOrdinalFromHostname(tt.hostname)
			if tt.expectError {
				testutil.NotOk(t, err)
			} else {
				testutil.Ok(t, err)
				testutil.Equals(t, tt.expectedOrdinal, ordinal)
			}
		})
	}
}

func TestTenantPrefixBucketCreation(t *testing.T) {
	tests := []struct {
		name                      string
		bucketConfig              client.BucketConfig
		tenantPrefixes            []string
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
			tenantPrefixes:            []string{""},
			expectedEffectivePrefixes: []string{""},
		},
		{
			name: "tenant partitioning with one tenant",
			bucketConfig: client.BucketConfig{
				Type: client.FILESYSTEM,
				Config: map[string]interface{}{
					"directory": "/tmp/test",
				},
				Prefix: "",
			},
			tenantPrefixes:            []string{"v1/raw/tenant1"},
			expectedEffectivePrefixes: []string{"v1/raw/tenant1"},
		},
		{
			name: "tenant partitioning with multiple tenants",
			bucketConfig: client.BucketConfig{
				Type: client.FILESYSTEM,
				Config: map[string]interface{}{
					"directory": "/tmp/test",
				},
				Prefix: "",
			},
			tenantPrefixes:            []string{"v1/raw/tenant1", "v1/raw/tenant2", "v1/raw/tenant3"},
			expectedEffectivePrefixes: []string{"v1/raw/tenant1", "v1/raw/tenant2", "v1/raw/tenant3"},
		},
		{
			name: "tenant partitioning with base prefix and tenant paths",
			bucketConfig: client.BucketConfig{
				Type: client.FILESYSTEM,
				Config: map[string]interface{}{
					"directory": "/tmp/test",
				},
				Prefix: "base/",
			},
			tenantPrefixes:            []string{"v1/raw/tenant1", "v1/raw/tenant2"},
			expectedEffectivePrefixes: []string{"base/v1/raw/tenant1", "base/v1/raw/tenant2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate bucket creation for each tenant prefix
			var actualEffectivePrefixes []string
			for _, tenantPrefix := range tt.tenantPrefixes {
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

func TestBucketConfigFromYAML(t *testing.T) {
	tests := []struct {
		name             string
		bucketYamlConfig string
		expectedPrefix   string
		expectedType     client.ObjProvider
	}{
		{
			name: "filesystem without prefix",
			bucketYamlConfig: `
type: FILESYSTEM
config:
  directory: /tmp/test
prefix: ""
`,
			expectedPrefix: "",
			expectedType:   client.FILESYSTEM,
		},
		{
			name: "filesystem with v1/raw prefix",
			bucketYamlConfig: `
type: FILESYSTEM
config:
  directory: /tmp/test
prefix: "v1/raw/"
`,
			expectedPrefix: "v1/raw/",
			expectedType:   client.FILESYSTEM,
		},
		{
			name: "S3 with data prefix",
			bucketYamlConfig: `
type: S3
config:
  bucket: test-bucket
  endpoint: s3.amazonaws.com
prefix: "data/"
`,
			expectedPrefix: "data/",
			expectedType:   client.S3,
		},
		{
			name: "filesystem with tenant path",
			bucketYamlConfig: `
type: FILESYSTEM
config:
  directory: /tmp/test
prefix: "v1/raw/tenant-alpha"
`,
			expectedPrefix: "v1/raw/tenant-alpha",
			expectedType:   client.FILESYSTEM,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var bucketConfig client.BucketConfig
			err := yaml.Unmarshal([]byte(tt.bucketYamlConfig), &bucketConfig)
			testutil.Ok(t, err)

			// Verify all fields are correctly parsed
			testutil.Equals(t, tt.expectedPrefix, bucketConfig.Prefix)
			testutil.Equals(t, tt.expectedType, bucketConfig.Type)

			// Verify we can marshal back
			bucketYamlBytes, err := yaml.Marshal(bucketConfig)
			testutil.Ok(t, err)

			// Verify we can unmarshal again and get the same result
			var roundtripBucketConfig client.BucketConfig
			err = yaml.Unmarshal(bucketYamlBytes, &roundtripBucketConfig)
			testutil.Ok(t, err)

			testutil.Equals(t, bucketConfig.Prefix, roundtripBucketConfig.Prefix)
			testutil.Equals(t, bucketConfig.Type, roundtripBucketConfig.Type)
		})
	}
}
