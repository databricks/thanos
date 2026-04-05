// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

// Tests for ProtectionEngine.Evaluate: verifies that rules are correctly
// evaluated, filtered, and prioritized against query requests.
package queryfrontend

import (
	"context"
	"regexp"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

// neverMatchProtection never matches (returns false).
type neverMatchProtection struct{}

func (p *neverMatchProtection) Name() string { return "never" }
func (p *neverMatchProtection) Run(_ context.Context, _ thanosQueryReq) (bool, error) {
	return false, nil
}

// errorProtection always matches and returns an error.
type errorProtection struct{}

func (p *errorProtection) Name() string { return "error" }
func (p *errorProtection) Run(_ context.Context, _ thanosQueryReq) (bool, error) {
	return true, errors.New("protection error")
}

func TestProtectionEngine_NoRules(t *testing.T) {
	engine := NewProtectionEngine(nil)
	protectionResult, err := engine.Evaluate(context.Background(), thanosQueryReq{})
	require.NoError(t, err)
	require.Nil(t, protectionResult)
}

func TestProtectionEngine_DisabledRuleSkipped(t *testing.T) {
	engine := NewProtectionEngine([]*Rule{
		NewRule("disabled", &AlwaysMatchProtection{}, RuleActionBlock, nil, false),
	})
	protectionResult, err := engine.Evaluate(context.Background(), thanosQueryReq{})
	require.NoError(t, err)
	require.Nil(t, protectionResult)
}

func TestProtectionEngine_ActorRegexNoMatch(t *testing.T) {
	engine := NewProtectionEngine([]*Rule{
		NewRule("filtered", &AlwaysMatchProtection{}, RuleActionBlock, regexp.MustCompile("^admin$"), true),
	})
	protectionResult, err := engine.Evaluate(context.Background(), thanosQueryReq{actor: "user"})
	require.NoError(t, err)
	require.Nil(t, protectionResult)
}

func TestProtectionEngine_ActorRegexMatch(t *testing.T) {
	engine := NewProtectionEngine([]*Rule{
		NewRule("filtered", &AlwaysMatchProtection{}, RuleActionBlock, regexp.MustCompile("^admin$"), true),
	})
	protectionResult, err := engine.Evaluate(context.Background(), thanosQueryReq{actor: "admin"})
	require.NoError(t, err)
	require.Equal(t, RuleActionBlock, protectionResult.Action)
	require.True(t, protectionResult.Triggered)
	require.Equal(t, "filtered", protectionResult.RuleName)
}

func TestProtectionEngine_FirstMatchingRuleWins(t *testing.T) {
	engine := NewProtectionEngine([]*Rule{
		NewRule("first", &neverMatchProtection{}, RuleActionBlock, nil, true),
		NewRule("second", &AlwaysMatchProtection{}, RuleActionLog, nil, true),
		NewRule("third", &AlwaysMatchProtection{}, RuleActionBlock, nil, true),
	})
	protectionResult, err := engine.Evaluate(context.Background(), thanosQueryReq{})
	require.NoError(t, err)
	require.Equal(t, RuleActionLog, protectionResult.Action)
	require.Equal(t, true, protectionResult.Triggered)
	require.Equal(t, "second", protectionResult.RuleName)
}

func TestProtectionEngine_RunError(t *testing.T) {
	engine := NewProtectionEngine([]*Rule{
		NewRule("error-rule", &errorProtection{}, RuleActionLog, nil, true),
	})
	_, err := engine.Evaluate(context.Background(), thanosQueryReq{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "protection error")
}

func TestProtectionEngine_UpdateRules(t *testing.T) {
	engine := NewProtectionEngine([]*Rule{
		NewRule("block-all", &AlwaysMatchProtection{}, RuleActionBlock, nil, true),
	})

	protectionResult, err := engine.Evaluate(context.Background(), thanosQueryReq{})
	require.NoError(t, err)
	require.Equal(t, RuleActionBlock, protectionResult.Action)
	require.Equal(t, true, protectionResult.Triggered)

	engine.UpdateRules(nil)

	protectionResult, err = engine.Evaluate(context.Background(), thanosQueryReq{})
	require.NoError(t, err)
	require.Nil(t, protectionResult)
}
