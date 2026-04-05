// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

// Tests for protection implementations in protection_impls.go.
package queryfrontend

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAlwaysMatchProtection_AlwaysMatches(t *testing.T) {
	p := &AlwaysMatchProtection{}
	matched, err := p.Run(context.Background(), thanosQueryReq{})
	require.NoError(t, err)
	require.True(t, matched)
}

func TestLookupProtection_AlwaysMatch(t *testing.T) {
	factory, err := LookupProtection("always-match")
	require.NoError(t, err)
	require.NotNil(t, factory)

	p, err := factory(nil)
	require.NoError(t, err)
	require.Equal(t, "always-match", p.Name())
}

func TestLookupProtection_Unknown(t *testing.T) {
	_, err := LookupProtection("unknown")
	require.Error(t, err)
}
