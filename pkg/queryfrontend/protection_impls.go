// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package queryfrontend

import (
	"context"

	"github.com/pkg/errors"
)

// This file contains all protection rule implementations.
// Each protection implements the Protection interface defined in protection.go.

// NoopProtection is a protection that always matches but never blocks any query.
// Used in M2 to verify the protection engine framework without affecting queries.
type NoopProtection struct{}

func (n *NoopProtection) Name() string { return "noop" }

func (n *NoopProtection) Run(_ context.Context, _ thanosQueryReq) (bool, error) {
	return true, nil
}

// protectionRegistry maps protection names (as used in config) to their factory functions.
var protectionRegistry = map[string]ProtectionFactory{
	"noop": func(_ map[string]string) (Protection, error) {
		return &NoopProtection{}, nil
	},
}

// LookupProtection returns the factory for the given protection name.
func LookupProtection(name string) (ProtectionFactory, error) {
	factory, ok := protectionRegistry[name]
	if !ok {
		return nil, errors.Errorf("unknown protection %q", name)
	}
	return factory, nil
}
