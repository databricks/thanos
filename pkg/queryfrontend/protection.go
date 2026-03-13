// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package queryfrontend

import (
	"context"
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
	Action    string // "log" or "block"
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
