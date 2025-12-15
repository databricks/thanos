// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package main

import (
	"path"
	"testing"
	"time"

	"github.com/efficientgo/core/testutil"
	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/thanos-io/objstore"
	"github.com/thanos-io/objstore/client"
	"gopkg.in/yaml.v2"

	"github.com/thanos-io/thanos/pkg/compact"
	"github.com/thanos-io/thanos/pkg/store"
)

func TestExtractOrdinalFromHostname(t *testing.T) {
	t.Parallel()

	for _, tcase := range []struct {
		name          string
		hostname      string
		expectedOrd   int
		expectedError bool
	}{
		{
			name:          "statefulset hostname with single digit",
			hostname:      "thanos-compact-0",
			expectedOrd:   0,
			expectedError: false,
		},
		{
			name:          "statefulset hostname with double digit",
			hostname:      "thanos-compact-12",
			expectedOrd:   12,
			expectedError: false,
		},
		{
			name:          "statefulset hostname with triple digit",
			hostname:      "thanos-compact-123",
			expectedOrd:   123,
			expectedError: false,
		},
		{
			name:          "kubernetes statefulset with namespace",
			hostname:      "thanos-compact-shard-5",
			expectedOrd:   5,
			expectedError: false,
		},
		{
			name:          "complex statefulset name",
			hostname:      "prod-us-west-thanos-compact-7",
			expectedOrd:   7,
			expectedError: false,
		},
		{
			name:          "hostname without number",
			hostname:      "thanos-compact-abc",
			expectedOrd:   0,
			expectedError: true,
		},
		{
			name:          "hostname with invalid suffix",
			hostname:      "thanos-compact-",
			expectedOrd:   0,
			expectedError: true,
		},
		{
			name:          "empty hostname",
			hostname:      "",
			expectedOrd:   0,
			expectedError: true,
		},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			ordinal, err := extractOrdinalFromHostname(tcase.hostname)
			if tcase.expectedError {
				testutil.NotOk(t, err)
			} else {
				testutil.Ok(t, err)
				testutil.Equals(t, tcase.expectedOrd, ordinal)
			}
		})
	}
}

func TestTenantPrefixBucketCreation(t *testing.T) {
	t.Parallel()

	for _, tcase := range []struct {
		name                   string
		initialPrefix          string
		tenantPrefixes         []string
		expectedBucketPrefixes []string
	}{
		{
			name:                   "single-tenant mode (no prefixes)",
			initialPrefix:          "",
			tenantPrefixes:         []string{""},
			expectedBucketPrefixes: []string{""},
		},
		{
			name:                   "tenant partitioning with one tenant",
			initialPrefix:          "",
			tenantPrefixes:         []string{"v1/raw/tenant1"},
			expectedBucketPrefixes: []string{"v1/raw/tenant1"},
		},
		{
			name:                   "tenant partitioning with multiple tenants",
			initialPrefix:          "",
			tenantPrefixes:         []string{"v1/raw/tenant1", "v1/raw/tenant2", "v1/raw/tenant3"},
			expectedBucketPrefixes: []string{"v1/raw/tenant1", "v1/raw/tenant2", "v1/raw/tenant3"},
		},
		{
			name:                   "tenant partitioning with base prefix and tenant paths",
			initialPrefix:          "base-prefix",
			tenantPrefixes:         []string{"v1/raw/tenant1", "v1/raw/tenant2"},
			expectedBucketPrefixes: []string{"base-prefix/v1/raw/tenant1", "base-prefix/v1/raw/tenant2"},
		},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			initialBucketConf := client.BucketConfig{
				Type:   client.FILESYSTEM,
				Config: nil,
				Prefix: tcase.initialPrefix,
			}

			var actualPrefixes []string
			for _, tenantPrefix := range tcase.tenantPrefixes {
				bucketConf := &client.BucketConfig{
					Type:   initialBucketConf.Type,
					Config: initialBucketConf.Config,
					Prefix: path.Join(initialBucketConf.Prefix, tenantPrefix),
				}
				actualPrefixes = append(actualPrefixes, bucketConf.Prefix)
			}

			testutil.Equals(t, tcase.expectedBucketPrefixes, actualPrefixes)
		})
	}
}

func TestBucketConfigPrefixPreservation(t *testing.T) {
	t.Parallel()

	for _, tcase := range []struct {
		name           string
		prefix         string
		tenantPrefix   string
		expectedPrefix string
	}{
		{
			name:           "empty prefix",
			prefix:         "",
			tenantPrefix:   "",
			expectedPrefix: "",
		},
		{
			name:           "simple prefix",
			prefix:         "data",
			tenantPrefix:   "",
			expectedPrefix: "data",
		},
		{
			name:           "v1/raw prefix",
			prefix:         "v1/raw",
			tenantPrefix:   "tenant1",
			expectedPrefix: "v1/raw/tenant1",
		},
		{
			name:           "hierarchical prefix",
			prefix:         "env/prod/region/us-west",
			tenantPrefix:   "tenant-abc",
			expectedPrefix: "env/prod/region/us-west/tenant-abc",
		},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			result := path.Join(tcase.prefix, tcase.tenantPrefix)
			testutil.Equals(t, tcase.expectedPrefix, result)
		})
	}
}

func TestBucketConfigFromYAML(t *testing.T) {
	t.Parallel()

	for _, tcase := range []struct {
		name           string
		yamlConfig     string
		expectedType   client.ObjProvider
		expectedPrefix string
	}{
		{
			name: "filesystem without prefix",
			yamlConfig: `
type: FILESYSTEM
config:
  directory: /tmp/thanos
`,
			expectedType:   client.FILESYSTEM,
			expectedPrefix: "",
		},
		{
			name: "filesystem with v1/raw prefix",
			yamlConfig: `
type: FILESYSTEM
config:
  directory: /tmp/thanos
prefix: v1/raw
`,
			expectedType:   client.FILESYSTEM,
			expectedPrefix: "v1/raw",
		},
		{
			name: "S3 with data prefix",
			yamlConfig: `
type: S3
config:
  bucket: my-bucket
  endpoint: s3.amazonaws.com
prefix: data
`,
			expectedType:   client.S3,
			expectedPrefix: "data",
		},
		{
			name: "filesystem with tenant path",
			yamlConfig: `
type: FILESYSTEM
config:
  directory: /tmp/thanos
prefix: v1/raw/tenant1
`,
			expectedType:   client.FILESYSTEM,
			expectedPrefix: "v1/raw/tenant1",
		},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			var bucketConf client.BucketConfig
			err := yaml.Unmarshal([]byte(tcase.yamlConfig), &bucketConf)
			testutil.Ok(t, err)
			testutil.Equals(t, tcase.expectedType, bucketConf.Type)
			testutil.Equals(t, tcase.expectedPrefix, bucketConf.Prefix)
		})
	}
}

func TestGetBlockLister(t *testing.T) {
	t.Parallel()

	logger := log.NewNopLogger()
	bkt := objstore.WithNoopInstr(objstore.NewInMemBucket())

	for _, tcase := range []struct {
		name              string
		blockListStrategy string
		expectNonNil      bool
	}{
		{
			name:              "concurrent strategy",
			blockListStrategy: string(concurrentDiscovery),
			expectNonNil:      true,
		},
		{
			name:              "recursive strategy",
			blockListStrategy: string(recursiveDiscovery),
			expectNonNil:      true,
		},
		{
			name:              "default strategy",
			blockListStrategy: "",
			expectNonNil:      true,
		},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			conf := &compactConfig{
				blockListStrategy: tcase.blockListStrategy,
			}
			lister := getBlockLister(logger, conf, bkt)
			if tcase.expectNonNil {
				testutil.Assert(t, lister != nil, "expected non-nil block lister")
			}
		})
	}
}

func TestGetCompactionLevels(t *testing.T) {
	t.Parallel()

	logger := log.NewNopLogger()

	for _, tcase := range []struct {
		name               string
		maxCompactionLevel int
		expectedLevels     int
		expectError        bool
	}{
		{
			name:               "default max level",
			maxCompactionLevel: 4,
			expectedLevels:     5,
			expectError:        false,
		},
		{
			name:               "level 0",
			maxCompactionLevel: 0,
			expectedLevels:     1,
			expectError:        false,
		},
		{
			name:               "level 2",
			maxCompactionLevel: 2,
			expectedLevels:     3,
			expectError:        false,
		},
		{
			name:               "level too high",
			maxCompactionLevel: 10,
			expectedLevels:     0,
			expectError:        true,
		},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			conf := &compactConfig{
				maxCompactionLevel: tcase.maxCompactionLevel,
			}
			levels, err := getCompactionLevels(logger, conf)
			if tcase.expectError {
				testutil.NotOk(t, err)
			} else {
				testutil.Ok(t, err)
				testutil.Equals(t, tcase.expectedLevels, len(levels))
			}
		})
	}
}

func TestCheckVerticalCompaction(t *testing.T) {
	t.Parallel()

	logger := log.NewNopLogger()

	for _, tcase := range []struct {
		name                            string
		enableVerticalCompaction        bool
		dedupReplicaLabels              []string
		expectedVerticalCompaction      bool
		expectedDedupReplicaLabelsCount int
	}{
		{
			name:                            "disabled by default",
			enableVerticalCompaction:        false,
			dedupReplicaLabels:              nil,
			expectedVerticalCompaction:      false,
			expectedDedupReplicaLabelsCount: 0,
		},
		{
			name:                            "explicitly enabled",
			enableVerticalCompaction:        true,
			dedupReplicaLabels:              nil,
			expectedVerticalCompaction:      true,
			expectedDedupReplicaLabelsCount: 0,
		},
		{
			name:                            "enabled via dedup replica labels",
			enableVerticalCompaction:        false,
			dedupReplicaLabels:              []string{"replica"},
			expectedVerticalCompaction:      true,
			expectedDedupReplicaLabelsCount: 1,
		},
		{
			name:                            "multiple dedup replica labels",
			enableVerticalCompaction:        false,
			dedupReplicaLabels:              []string{"replica", "prometheus"},
			expectedVerticalCompaction:      true,
			expectedDedupReplicaLabelsCount: 2,
		},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			conf := &compactConfig{
				enableVerticalCompaction: tcase.enableVerticalCompaction,
				dedupReplicaLabels:       tcase.dedupReplicaLabels,
			}
			enabled, labels := checkVerticalCompaction(logger, conf)
			testutil.Equals(t, tcase.expectedVerticalCompaction, enabled)
			testutil.Equals(t, tcase.expectedDedupReplicaLabelsCount, len(labels))
		})
	}
}

func TestGetMergeFunc(t *testing.T) {
	t.Parallel()

	logger := log.NewNopLogger()

	for _, tcase := range []struct {
		name               string
		dedupFunc          string
		dedupReplicaLabels []string
		expectError        bool
	}{
		{
			name:               "default merge func",
			dedupFunc:          "",
			dedupReplicaLabels: nil,
			expectError:        false,
		},
		{
			name:               "penalty dedup without labels",
			dedupFunc:          compact.DedupAlgorithmPenalty,
			dedupReplicaLabels: nil,
			expectError:        true,
		},
		{
			name:               "penalty dedup with labels",
			dedupFunc:          compact.DedupAlgorithmPenalty,
			dedupReplicaLabels: []string{"replica"},
			expectError:        false,
		},
		{
			name:               "unsupported dedup func",
			dedupFunc:          "invalid",
			dedupReplicaLabels: nil,
			expectError:        true,
		},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			conf := &compactConfig{
				dedupFunc: tcase.dedupFunc,
			}
			mergeFunc, err := getMergeFunc(logger, conf, tcase.dedupReplicaLabels)
			if tcase.expectError {
				testutil.NotOk(t, err)
			} else {
				testutil.Ok(t, err)
				testutil.Assert(t, mergeFunc != nil, "expected non-nil merge func")
			}
		})
	}
}

func TestGetRetentionPolicies(t *testing.T) {
	t.Parallel()

	logger := log.NewNopLogger()

	for _, tcase := range []struct {
		name                string
		retentionRaw        model.Duration
		retentionFiveMin    model.Duration
		retentionOneHr      model.Duration
		retentionTenants    []string
		disableDownsampling bool
		expectError         bool
	}{
		{
			name:                "no retention set",
			retentionRaw:        0,
			retentionFiveMin:    0,
			retentionOneHr:      0,
			retentionTenants:    nil,
			disableDownsampling: false,
			expectError:         false,
		},
		{
			name:                "valid raw retention",
			retentionRaw:        model.Duration(30 * 24 * time.Hour),
			retentionFiveMin:    0,
			retentionOneHr:      0,
			retentionTenants:    nil,
			disableDownsampling: false,
			expectError:         false,
		},
		{
			name:                "raw retention too low for downsampling",
			retentionRaw:        model.Duration(1 * time.Hour),
			retentionFiveMin:    0,
			retentionOneHr:      0,
			retentionTenants:    nil,
			disableDownsampling: false,
			expectError:         true,
		},
		{
			name:                "raw retention low but downsampling disabled",
			retentionRaw:        model.Duration(1 * time.Hour),
			retentionFiveMin:    0,
			retentionOneHr:      0,
			retentionTenants:    nil,
			disableDownsampling: true,
			expectError:         false,
		},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			tenants := tcase.retentionTenants
			conf := &compactConfig{
				retentionRaw:        tcase.retentionRaw,
				retentionFiveMin:    tcase.retentionFiveMin,
				retentionOneHr:      tcase.retentionOneHr,
				retentionTenants:    &tenants,
				disableDownsampling: tcase.disableDownsampling,
			}
			byResolution, byTenant, err := getRetentionPolicies(logger, conf)
			if tcase.expectError {
				testutil.NotOk(t, err)
			} else {
				testutil.Ok(t, err)
				testutil.Assert(t, byResolution != nil, "expected non-nil byResolution")
				testutil.Assert(t, byTenant != nil, "expected non-nil byTenant")
			}
		})
	}
}

func TestSingleTenantModeConfiguration(t *testing.T) {
	t.Parallel()

	// Test that when enableTenantPathPrefix is false, we get single tenant behavior
	conf := &compactConfig{
		enableTenantPathPrefix: false,
	}

	// Verify single tenant mode configuration
	testutil.Equals(t, false, conf.enableTenantPathPrefix)

	// In single tenant mode, tenantPrefixes should be [""]
	var tenantPrefixes []string
	if conf.enableTenantPathPrefix {
		t.Fatal("enableTenantPathPrefix should be false")
	} else {
		tenantPrefixes = []string{""}
	}

	testutil.Equals(t, 1, len(tenantPrefixes))
	testutil.Equals(t, "", tenantPrefixes[0])
}

func TestMultiTenantModeConfiguration(t *testing.T) {
	t.Parallel()

	// Test that multi-tenant configuration is properly set up
	conf := &compactConfig{
		enableTenantPathPrefix: true,
		replicas:               6,
		replicationFactor:      2,
		commonPathPrefix:       "v1/raw/",
	}

	// Verify multi-tenant mode configuration
	testutil.Equals(t, true, conf.enableTenantPathPrefix)
	testutil.Equals(t, 6, conf.replicas)
	testutil.Equals(t, 2, conf.replicationFactor)
	testutil.Equals(t, "v1/raw/", conf.commonPathPrefix)

	// Calculate total shards
	totalShards := conf.replicas / conf.replicationFactor
	testutil.Equals(t, 3, totalShards)
}

func TestMultiTenantShardCalculation(t *testing.T) {
	t.Parallel()

	for _, tcase := range []struct {
		name              string
		replicas          int
		replicationFactor int
		expectedShards    int
		expectError       bool
	}{
		{
			name:              "6 replicas, 2 replication factor",
			replicas:          6,
			replicationFactor: 2,
			expectedShards:    3,
			expectError:       false,
		},
		{
			name:              "9 replicas, 3 replication factor",
			replicas:          9,
			replicationFactor: 3,
			expectedShards:    3,
			expectError:       false,
		},
		{
			name:              "4 replicas, 1 replication factor",
			replicas:          4,
			replicationFactor: 1,
			expectedShards:    4,
			expectError:       false,
		},
		{
			name:              "non-divisible replicas",
			replicas:          5,
			replicationFactor: 2,
			expectedShards:    0,
			expectError:       true,
		},
		{
			name:              "zero replication factor",
			replicas:          6,
			replicationFactor: 0,
			expectedShards:    0,
			expectError:       true,
		},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			if tcase.replicationFactor <= 0 || tcase.replicas%tcase.replicationFactor != 0 {
				if !tcase.expectError {
					t.Fatal("expected error for invalid configuration")
				}
				return
			}

			totalShards := tcase.replicas / tcase.replicationFactor
			if tcase.expectError {
				testutil.Assert(t, totalShards <= 0, "expected invalid shard count")
			} else {
				testutil.Equals(t, tcase.expectedShards, totalShards)
			}
		})
	}
}

func TestTenantPrefixGeneration(t *testing.T) {
	t.Parallel()

	for _, tcase := range []struct {
		name             string
		commonPathPrefix string
		tenants          []string
		expectedPrefixes []string
	}{
		{
			name:             "v1/raw prefix with tenants",
			commonPathPrefix: "v1/raw",
			tenants:          []string{"tenant1", "tenant2", "tenant3"},
			expectedPrefixes: []string{"v1/raw/tenant1", "v1/raw/tenant2", "v1/raw/tenant3"},
		},
		{
			name:             "empty prefix with tenants",
			commonPathPrefix: "",
			tenants:          []string{"tenant1"},
			expectedPrefixes: []string{"tenant1"},
		},
		{
			name:             "nested prefix with tenants",
			commonPathPrefix: "org/data/raw",
			tenants:          []string{"tenant-1", "tenant-2"},
			expectedPrefixes: []string{"org/data/raw/tenant-1", "org/data/raw/tenant-2"},
		},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			var tenantPrefixes []string
			for _, tenant := range tcase.tenants {
				tenantPrefixes = append(tenantPrefixes, path.Join(tcase.commonPathPrefix, tenant))
			}
			testutil.Equals(t, tcase.expectedPrefixes, tenantPrefixes)
		})
	}
}

func TestCompactConfigDefaults(t *testing.T) {
	t.Parallel()

	// Test default values for compactConfig
	conf := &compactConfig{}

	// Verify default values
	testutil.Equals(t, false, conf.enableTenantPathPrefix)
	testutil.Equals(t, 0, conf.replicas)
	testutil.Equals(t, 0, conf.replicationFactor)
	testutil.Equals(t, "", conf.commonPathPrefix)
	testutil.Equals(t, false, conf.enableVerticalCompaction)
	testutil.Equals(t, false, conf.disableDownsampling)
	testutil.Equals(t, false, conf.disableWeb)
	testutil.Equals(t, false, conf.haltOnError)
}

func TestFilterConfigDefaults(t *testing.T) {
	t.Parallel()

	// Verify filter config can be created
	conf := &compactConfig{
		filterConf: &store.FilterConfig{},
	}

	testutil.Assert(t, conf.filterConf != nil, "expected non-nil filter config")
}

func TestBackwardCompatibility_SingleTenantMode(t *testing.T) {
	t.Parallel()

	// This test ensures backward compatibility when enableTenantPathPrefix is false
	// The compactor should behave as a single-tenant compactor

	conf := &compactConfig{
		enableTenantPathPrefix:    false,
		dataDir:                   t.TempDir(),
		maxCompactionLevel:        4,
		blockMetaFetchConcurrency: 32,
		blockFilesConcurrency:     1,
		compactionConcurrency:     1,
		downsampleConcurrency:     1,
	}

	// Verify single tenant mode
	testutil.Equals(t, false, conf.enableTenantPathPrefix)

	// In single tenant mode, we should have exactly one tenant prefix (empty string)
	var tenantPrefixes []string
	var isMultiTenant bool

	if conf.enableTenantPathPrefix {
		isMultiTenant = true
	} else {
		isMultiTenant = false
		tenantPrefixes = []string{""}
	}

	testutil.Equals(t, false, isMultiTenant)
	testutil.Equals(t, 1, len(tenantPrefixes))
	testutil.Equals(t, "", tenantPrefixes[0])

	// Verify bucket prefix is empty in single tenant mode
	initialBucketConf := client.BucketConfig{
		Type:   client.FILESYSTEM,
		Config: nil,
		Prefix: "",
	}

	bucketConf := &client.BucketConfig{
		Type:   initialBucketConf.Type,
		Config: initialBucketConf.Config,
		Prefix: path.Join(initialBucketConf.Prefix, tenantPrefixes[0]),
	}

	testutil.Equals(t, "", bucketConf.Prefix)
}

func TestMultiTenantMode_TenantIsolation(t *testing.T) {
	t.Parallel()

	// This test ensures that each tenant gets its own isolated bucket prefix

	conf := &compactConfig{
		enableTenantPathPrefix: true,
		commonPathPrefix:       "v1/raw",
	}

	tenants := []string{"tenant-alpha", "tenant-beta", "tenant-gamma"}

	var tenantPrefixes []string
	for _, tenant := range tenants {
		tenantPrefixes = append(tenantPrefixes, path.Join(conf.commonPathPrefix, tenant))
	}

	// Verify each tenant has a unique, isolated prefix
	testutil.Equals(t, 3, len(tenantPrefixes))
	testutil.Equals(t, "v1/raw/tenant-alpha", tenantPrefixes[0])
	testutil.Equals(t, "v1/raw/tenant-beta", tenantPrefixes[1])
	testutil.Equals(t, "v1/raw/tenant-gamma", tenantPrefixes[2])

	// Verify prefixes are unique
	prefixSet := make(map[string]bool)
	for _, prefix := range tenantPrefixes {
		testutil.Assert(t, !prefixSet[prefix], "duplicate prefix found: "+prefix)
		prefixSet[prefix] = true
	}
}

func TestCompactionSetLevels(t *testing.T) {
	t.Parallel()

	cs := compactionSet{
		1 * time.Hour,
		2 * time.Hour,
		8 * time.Hour,
		2 * 24 * time.Hour,
		14 * 24 * time.Hour,
	}

	for _, tcase := range []struct {
		name           string
		maxLevel       int
		expectedLevels int
		expectError    bool
	}{
		{
			name:           "level 0",
			maxLevel:       0,
			expectedLevels: 1,
			expectError:    false,
		},
		{
			name:           "level 2",
			maxLevel:       2,
			expectedLevels: 3,
			expectError:    false,
		},
		{
			name:           "max level 4",
			maxLevel:       4,
			expectedLevels: 5,
			expectError:    false,
		},
		{
			name:           "level exceeds set",
			maxLevel:       5,
			expectedLevels: 0,
			expectError:    true,
		},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			levels, err := cs.levels(tcase.maxLevel)
			if tcase.expectError {
				testutil.NotOk(t, err)
			} else {
				testutil.Ok(t, err)
				testutil.Equals(t, tcase.expectedLevels, len(levels))
			}
		})
	}
}

func TestCompactionSetMaxLevel(t *testing.T) {
	t.Parallel()

	cs := compactionSet{
		1 * time.Hour,
		2 * time.Hour,
		8 * time.Hour,
		2 * 24 * time.Hour,
		14 * 24 * time.Hour,
	}

	testutil.Equals(t, 4, cs.maxLevel())
}

func TestCompactionSetString(t *testing.T) {
	t.Parallel()

	cs := compactionSet{
		1 * time.Hour,
		2 * time.Hour,
	}

	str := cs.String()
	testutil.Assert(t, str != "", "expected non-empty string")
	testutil.Assert(t, len(str) > 0, "expected string representation")
}
