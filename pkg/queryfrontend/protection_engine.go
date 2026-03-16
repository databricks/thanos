// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package queryfrontend

import (
	"context"
	"sync"
)

// ProtectionEngine evaluates a list of rules against a query request.
type ProtectionEngine struct {
	mu    sync.RWMutex
	rules []*Rule
}

// NewProtectionEngine creates a new ProtectionEngine with the given rules.
func NewProtectionEngine(rules []*Rule) *ProtectionEngine {
	return &ProtectionEngine{rules: rules}
}

// UpdateRules replaces the current rule set. Safe for concurrent use.
func (e *ProtectionEngine) UpdateRules(rules []*Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = rules
}

// Evaluate runs all applicable rules against the request.
// Rules are evaluated in order; the first matching rule wins.
// Returns the updated context (with ProtectionResult if a rule triggered),
// the action to take, and any error.
func (e *ProtectionEngine) Evaluate(ctx context.Context, req thanosQueryReq) (*ProtectionResult, error) {
	e.mu.RLock()
	rules := e.rules
	e.mu.RUnlock()

	for _, rule := range rules {
		// Skip disabled rules.
		if !rule.enabled {
			continue
		}

		// Check actor filter.
		if rule.actorRegex != nil && !rule.actorRegex.MatchString(req.actor) {
			continue
		}

		matched, err := rule.protection.Run(ctx, req)
		if !matched {
			continue
		}

		return &ProtectionResult{
			Triggered: true,
			RuleName:  rule.name,
			Action:    rule.action,
		}, err
	}
	return nil, nil
}
