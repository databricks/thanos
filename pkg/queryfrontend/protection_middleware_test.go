// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

// Tests for protectionMiddleware.Do: verifies routing behavior based on
// query validity, rule evaluation results, and actor filtering.
package queryfrontend

import (
	"context"
	"net/http"
	"regexp"
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"github.com/thanos-io/thanos/internal/cortex/querier/queryrange"
	"github.com/weaveworks/common/httpgrpc"
)

// makeRequest builds a ThanosQueryRangeRequest with the given query and optional X-Source header.
func makeRequest(query, source string) *ThanosQueryRangeRequest {
	r := &ThanosQueryRangeRequest{
		Path:  "/api/v1/query_range",
		Start: 0,
		End:   3600000,
		Step:  60000,
		Query: query,
	}
	if source != "" {
		r.Headers = []*RequestHeader{
			{Name: "x-source", Values: []string{source}},
		}
	}
	return r
}

func newTestMiddleware(engine *ProtectionEngine) queryrange.Middleware {
	return NewProtectionMiddleware(engine, log.NewNopLogger(), prometheus.NewRegistry())
}

func TestProtectionMiddleware_EmptyQuery_PassesThrough(t *testing.T) {
	called := false
	next := queryrange.HandlerFunc(func(ctx context.Context, r queryrange.Request) (queryrange.Response, error) {
		called = true
		return &queryrange.PrometheusResponse{Status: "success"}, nil
	})

	handler := newTestMiddleware(NewProtectionEngine([]*Rule{
		NewRule("block-all", &alwaysMatchProtection{}, RuleActionBlock, nil, true),
	})).Wrap(next)

	_, err := handler.Do(context.Background(), makeRequest("", ""))
	require.NoError(t, err)
	require.True(t, called, "next should be called for empty query")
}

func TestProtectionMiddleware_InvalidPromQL_PassesThrough(t *testing.T) {
	called := false
	next := queryrange.HandlerFunc(func(ctx context.Context, r queryrange.Request) (queryrange.Response, error) {
		called = true
		return &queryrange.PrometheusResponse{Status: "success"}, nil
	})

	handler := newTestMiddleware(NewProtectionEngine([]*Rule{
		NewRule("block-all", &alwaysMatchProtection{}, RuleActionBlock, nil, true),
	})).Wrap(next)

	_, err := handler.Do(context.Background(), makeRequest("this is not valid promql!!!", ""))
	require.NoError(t, err)
	require.True(t, called, "next should be called when PromQL parsing fails")
}

func TestProtectionMiddleware_RuleActionLog_PassesThrough(t *testing.T) {
	called := false
	next := queryrange.HandlerFunc(func(ctx context.Context, r queryrange.Request) (queryrange.Response, error) {
		called = true
		return &queryrange.PrometheusResponse{Status: "success"}, nil
	})

	handler := newTestMiddleware(NewProtectionEngine([]*Rule{
		NewRule("log-all", &alwaysMatchProtection{}, RuleActionLog, nil, true),
	})).Wrap(next)

	_, err := handler.Do(context.Background(), makeRequest("up", ""))
	require.NoError(t, err)
	require.True(t, called)
}

func TestProtectionMiddleware_RuleActionBlock_Returns400(t *testing.T) {
	called := false
	next := queryrange.HandlerFunc(func(ctx context.Context, r queryrange.Request) (queryrange.Response, error) {
		called = true
		return &queryrange.PrometheusResponse{Status: "success"}, nil
	})

	handler := newTestMiddleware(NewProtectionEngine([]*Rule{
		NewRule("block-all", &alwaysMatchProtection{}, RuleActionBlock, nil, true),
	})).Wrap(next)

	_, err := handler.Do(context.Background(), makeRequest("up", ""))
	require.Error(t, err)
	require.False(t, called, "next should NOT be called when query is blocked")

	httpErr, ok := httpgrpc.HTTPResponseFromError(err)
	require.True(t, ok)
	require.Equal(t, int32(http.StatusBadRequest), httpErr.Code)
}

func TestProtectionMiddleware_ProtectionResultInContext(t *testing.T) {
	var capturedCtx context.Context
	next := queryrange.HandlerFunc(func(ctx context.Context, r queryrange.Request) (queryrange.Response, error) {
		capturedCtx = ctx
		return &queryrange.PrometheusResponse{Status: "success"}, nil
	})

	handler := newTestMiddleware(NewProtectionEngine([]*Rule{
		NewRule("log-rule", &alwaysMatchProtection{}, RuleActionLog, nil, true),
	})).Wrap(next)

	_, err := handler.Do(context.Background(), makeRequest("up", "actor1"))
	require.NoError(t, err)

	result := GetProtectionResult(capturedCtx)
	require.NotNil(t, result)
	require.True(t, result.Triggered)
	require.Equal(t, "log-rule", result.RuleName)
	require.Equal(t, RuleActionLog, result.Action)
}

func TestProtectionMiddleware_EvaluationError_ReturnsError(t *testing.T) {
	called := false
	next := queryrange.HandlerFunc(func(ctx context.Context, r queryrange.Request) (queryrange.Response, error) {
		called = true
		return &queryrange.PrometheusResponse{Status: "success"}, nil
	})

	handler := newTestMiddleware(NewProtectionEngine([]*Rule{
		NewRule("error-rule", &errorProtection{}, RuleActionLog, nil, true),
	})).Wrap(next)

	_, err := handler.Do(context.Background(), makeRequest("up", ""))
	require.Error(t, err)
	require.False(t, called, "next should NOT be called when engine evaluation fails")
}

func TestProtectionMiddleware_ActorFiltering(t *testing.T) {
	called := 0
	next := queryrange.HandlerFunc(func(ctx context.Context, r queryrange.Request) (queryrange.Response, error) {
		called++
		return &queryrange.PrometheusResponse{Status: "success"}, nil
	})

	handler := newTestMiddleware(NewProtectionEngine([]*Rule{
		NewRule("block-admin", &alwaysMatchProtection{}, RuleActionBlock, regexp.MustCompile("^admin$"), true),
	})).Wrap(next)

	// Non-admin should pass through.
	_, err := handler.Do(context.Background(), makeRequest("up", "user"))
	require.NoError(t, err)
	require.Equal(t, 1, called)

	// Admin should be blocked.
	_, err = handler.Do(context.Background(), makeRequest("up", "admin"))
	require.Error(t, err)
	require.Equal(t, 1, called, "next should not be called for blocked admin request")
}
