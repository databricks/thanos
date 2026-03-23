// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package queryfrontend

import (
	"context"
	"regexp"

	"github.com/prometheus/prometheus/promql/parser"
	"github.com/thanos-io/thanos/internal/cortex/querier/queryrange"
)

// RuleAction represents the action to take when a protection rule is triggered.
type RuleAction int

const (
	RuleActionLog   RuleAction = iota
	RuleActionBlock RuleAction = iota
)

// ProtectionResult holds the result of evaluating protection rules against a query.
// It is stored in the context by the protection middleware and read by the logging middleware.
type ProtectionResult struct {
	Triggered bool
	RuleName  string
	Action    RuleAction
}

type protectionContextKey int

const protectionResultKey protectionContextKey = iota

// WithProtectionResult stores a ProtectionResult in the context.
func WithProtectionResult(ctx context.Context, result *ProtectionResult) context.Context {
	return context.WithValue(ctx, protectionResultKey, result)
}

// GetProtectionResult retrieves a ProtectionResult from the context.
// Returns nil if no result is stored.
func GetProtectionResult(ctx context.Context) *ProtectionResult {
	result, _ := ctx.Value(protectionResultKey).(*ProtectionResult)
	return result
}

// RuleActionToString converts a RuleAction to a string.
func RuleActionToString(action RuleAction) string {
	switch action {
	case RuleActionLog:
		return "Log"
	case RuleActionBlock:
		return "Block"
	default:
		return "Unknown"
	}
}

// thanosQueryReq wraps a query request with parsed PromQL and actor information.
type thanosQueryReq struct {
	inner  queryrange.Request
	parsed parser.Expr
	actor  string
}

// Protection is the interface that all protection implementations must satisfy.
// It only determines whether a query matches — the action (log/block) is configured in Rule.
type Protection interface {
	Name() string
	Run(ctx context.Context, req thanosQueryReq) (bool, error)
}

// ProtectionFactory constructs a Protection from a map of args parsed from config.
type ProtectionFactory func(args map[string]string) (Protection, error)

// Rule combines a Protection with its configured action, actor filter, and enabled state.
type Rule struct {
	name       string
	protection Protection
	action     RuleAction
	actorRegex *regexp.Regexp
	enabled    bool
}

// NewRule creates a Rule. actorRegex may be nil to match all actors.
func NewRule(name string, protection Protection, action RuleAction, actorRegex *regexp.Regexp, enabled bool) *Rule {
	return &Rule{
		name:       name,
		protection: protection,
		action:     action,
		actorRegex: actorRegex,
		enabled:    enabled,
	}
}
