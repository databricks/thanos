// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package receive

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/efficientgo/core/testutil"
)

func TestValidateConfig(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		cfg  interface{}
		err  error
	}{
		{
			name: "<nil> config",
			cfg:  nil,
			err:  errEmptyConfigurationFile,
		},
		{
			name: "empty config",
			cfg:  []HashringConfig{},
			err:  errEmptyConfigurationFile,
		},
		{
			name: "unparsable config",
			cfg:  struct{}{},
			err:  errParseConfigurationFile,
		},
		{
			name: "valid config",
			cfg: []HashringConfig{
				{
					Endpoints: []Endpoint{{Address: "node1"}},
				},
			},
			err: nil, // means it's valid.
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content, err := json.Marshal(tc.cfg)
			testutil.Ok(t, err)

			tmpfile, err := os.CreateTemp("", "configwatcher_test.*.json")
			testutil.Ok(t, err)

			defer func() {
				testutil.Ok(t, os.Remove(tmpfile.Name()))
			}()

			_, err = tmpfile.Write(content)
			testutil.Ok(t, err)

			err = tmpfile.Close()
			testutil.Ok(t, err)

			cw, err := NewConfigWatcher(nil, nil, tmpfile.Name(), 1)
			testutil.Ok(t, err)
			defer cw.Stop()

			if err := cw.ValidateConfig(); err != nil && !errors.Is(err, tc.err) {
				t.Errorf("case %q: got unexpected error: %v", tc.name, err)
			}
		})
	}
}

func TestUnmarshalEndpointSlice(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		json      string
		endpoints []Endpoint
		expectErr bool
	}{
		{
			name:      "Endpoint with empty address",
			json:      `[{"az": "az-1"}]`,
			endpoints: []Endpoint{{Address: "node-1", CapNProtoAddress: "node-1:19391"}},
			expectErr: true,
		},
		{
			name:      "Endpoints as string slice",
			json:      `["node-1"]`,
			endpoints: []Endpoint{{Address: "node-1", CapNProtoAddress: "node-1:19391"}},
		},
		{
			name:      "Endpoints as endpoints slice",
			json:      `[{"address": "node-1", "az": "az-1"}]`,
			endpoints: []Endpoint{{Address: "node-1", CapNProtoAddress: "node-1:19391", AZ: "az-1"}},
		},
		{
			name:      "Endpoints as string slice with port",
			json:      `["node-1:80"]`,
			endpoints: []Endpoint{{Address: "node-1:80", CapNProtoAddress: "node-1:19391"}},
		},
		{
			name:      "Endpoints as string slice with capnproto port",
			json:      `[{"address": "node-1", "capnproto_address": "node-1:81"}]`,
			endpoints: []Endpoint{{Address: "node-1", CapNProtoAddress: "node-1:81"}},
		},
	}
	for _, tcase := range cases {
		t.Run(tcase.name, func(t *testing.T) {
			var endpoints []Endpoint
			err := json.Unmarshal([]byte(tcase.json), &endpoints)
			if tcase.expectErr {
				testutil.NotOk(t, err)
				return
			}
			testutil.Ok(t, err)
			testutil.Equals(t, tcase.endpoints, endpoints)
		})
	}
}

func TestShardSizeUnmarshalJSON(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		input     string
		expected  ShardSize
		expectErr bool
	}{
		{
			name:     "integer value",
			input:    `6`,
			expected: ShardSize{Value: 6},
		},
		{
			name:     "zero integer",
			input:    `0`,
			expected: ShardSize{Value: 0},
		},
		{
			name:     "percentage string",
			input:    `"50%"`,
			expected: ShardSize{Percent: 0.5, IsPercent: true},
		},
		{
			name:     "zero percentage",
			input:    `"0%"`,
			expected: ShardSize{Percent: 0, IsPercent: true},
		},
		{
			name:     "100 percentage",
			input:    `"100%"`,
			expected: ShardSize{Percent: 1.0, IsPercent: true},
		},
		{
			name:     "25 percentage",
			input:    `"25%"`,
			expected: ShardSize{Percent: 0.25, IsPercent: true},
		},
		{
			name:      "invalid string without percent",
			input:     `"50"`,
			expectErr: true,
		},
		{
			name:      "negative percentage",
			input:     `"-10%"`,
			expectErr: true,
		},
		{
			name:      "over 100 percentage",
			input:     `"150%"`,
			expectErr: true,
		},
		{
			name:      "invalid type",
			input:     `true`,
			expectErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var s ShardSize
			err := json.Unmarshal([]byte(tc.input), &s)
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expected, s)
		})
	}
}

func TestShardSizeMarshalJSON(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		input    ShardSize
		expected string
	}{
		{
			name:     "integer value",
			input:    ShardSize{Value: 6},
			expected: `6`,
		},
		{
			name:     "zero value",
			input:    ShardSize{},
			expected: `0`,
		},
		{
			name:     "percentage",
			input:    ShardSize{Percent: 0.5, IsPercent: true},
			expected: `"50%"`,
		},
		{
			name:     "100 percentage",
			input:    ShardSize{Percent: 1.0, IsPercent: true},
			expected: `"100%"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.expected, string(data))
		})
	}
}

func TestShardSizeIsZero(t *testing.T) {
	t.Parallel()

	require.True(t, ShardSize{}.IsZero())
	require.True(t, ShardSize{Value: 0}.IsZero())
	require.True(t, ShardSize{Percent: 0, IsPercent: true}.IsZero())
	require.False(t, ShardSize{Value: 1}.IsZero())
	require.False(t, ShardSize{Percent: 0.5, IsPercent: true}.IsZero())
}

func TestShardSizeResolveCount(t *testing.T) {
	t.Parallel()

	// Absolute value: returns Value directly regardless of total.
	require.Equal(t, 6, ShardSize{Value: 6}.ResolveCount(100))
	require.Equal(t, 6, ShardSize{Value: 6}.ResolveCount(4))

	// Percentage: max(1, total * pct).
	require.Equal(t, 2, ShardSize{Percent: 0.5, IsPercent: true}.ResolveCount(4))
	require.Equal(t, 1, ShardSize{Percent: 0.25, IsPercent: true}.ResolveCount(4))
	require.Equal(t, 4, ShardSize{Percent: 1.0, IsPercent: true}.ResolveCount(4))
	// Very small percentage still returns at least 1.
	require.Equal(t, 1, ShardSize{Percent: 0.01, IsPercent: true}.ResolveCount(4))
}

func TestShardSizeRoundTripJSON(t *testing.T) {
	t.Parallel()

	// Test that ShardSize round-trips through full config JSON parsing.
	cfgJSON := `[{
		"hashring": "test",
		"endpoints": [{"address": "node1"}],
		"shuffle_sharding_config": {
			"shard_size": "50%",
			"overrides": [
				{"shard_size": 6, "tenants": ["t1"]},
				{"shard_size": "25%", "tenants": ["t2"]}
			]
		}
	}]`

	configs, err := ParseConfig([]byte(cfgJSON))
	require.NoError(t, err)
	require.Len(t, configs, 1)

	ssc := configs[0].ShuffleShardingConfig
	require.True(t, ssc.ShardSize.IsPercent)
	require.InDelta(t, 0.5, ssc.ShardSize.Percent, 0.001)

	require.Len(t, ssc.Overrides, 2)
	require.False(t, ssc.Overrides[0].ShardSize.IsPercent)
	require.Equal(t, 6, ssc.Overrides[0].ShardSize.Value)
	require.True(t, ssc.Overrides[1].ShardSize.IsPercent)
	require.InDelta(t, 0.25, ssc.Overrides[1].ShardSize.Percent, 0.001)
}
