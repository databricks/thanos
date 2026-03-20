// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package queryfrontend

import (
	"context"
	"net/http"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/thanos-io/thanos/internal/cortex/querier/queryrange"
	"github.com/thanos-io/thanos/pkg/extpromql"
	"github.com/weaveworks/common/httpgrpc"
)

type protectionMiddleware struct {
	next         queryrange.Handler
	engine       *ProtectionEngine
	logger       log.Logger
	totalLatency prometheus.Histogram
}

// NewProtectionMiddleware creates a new middleware that applies protection rules to queries.
func NewProtectionMiddleware(engine *ProtectionEngine, logger log.Logger, reg prometheus.Registerer) queryrange.Middleware {
	totalLatency := promauto.With(reg).NewHistogram(prometheus.HistogramOpts{
		Namespace: "thanos",
		Subsystem: "query_frontend",
		Name:      "protection_duration_seconds",
		Help:      "Total duration of the protection middleware, including parsing and evaluation.",
		Buckets:   []float64{0.0001, 0.00025, 0.0005, 0.001, 0.005, 0.01, 0.1, 1.0, 10.0},
	})

	return queryrange.MiddlewareFunc(func(next queryrange.Handler) queryrange.Handler {
		return &protectionMiddleware{
			next:         next,
			engine:       engine,
			logger:       logger,
			totalLatency: totalLatency,
		}
	})
}

func (m *protectionMiddleware) Do(ctx context.Context, r queryrange.Request) (queryrange.Response, error) {
	query := r.GetQuery()
	if query == "" {
		return m.next.Do(ctx, r)
	}

	totalStart := time.Now()
	defer func() { m.totalLatency.Observe(time.Since(totalStart).Seconds()) }()

	// Parse PromQL.
	parsed, err := extpromql.ParseExpr(query)
	if err != nil {
		// Malformed query: let downstream handle it, don't block.
		level.Debug(m.logger).Log("msg", "protection middleware: failed to parse query, skipping", "err", err)
		return m.next.Do(ctx, r)
	}

	// Extract actor from X-Source header.
	actor := extractActor(r)

	req := thanosQueryReq{
		inner:  r,
		parsed: parsed,
		actor:  actor,
	}

	// Evaluate all protection rules.
	protectionResult, err := m.engine.Evaluate(ctx, req)

	if err != nil {
		return nil, errors.Wrap(err, "protection engine evaluation failed")
	}

	if protectionResult != nil {
		ctx = WithProtectionResult(ctx, protectionResult)
		if err := m.applyProtectionResult(protectionResult, query); err != nil {
			return nil, err
		}
	}

	return m.next.Do(ctx, r)
}

// applyProtectionResult performs the action indicated by the protection result.
// Returns an error if the query should be blocked, nil otherwise.
func (m *protectionMiddleware) applyProtectionResult(result *ProtectionResult, query string) error {
	switch result.Action {
	case RuleActionBlock:
		return httpgrpc.Errorf(http.StatusBadRequest, "query blocked by protection rule: %s", result.RuleName)
	case RuleActionLog:
		level.Info(m.logger).Log("msg", "protection rule triggered", "rule", result.RuleName, "action", "log", "query", query)
	}
	return nil
}

// extractActor returns the actor identifier from the request headers.
// Uses X-Source header as the actor identifier.
func extractActor(r queryrange.Request) string {
	headers := getHeaders(r)
	userInfo := ExtractUserInfoFromHeaders(headers)
	return userInfo.Source
}

// getHeaders extracts headers from a queryrange.Request.
func getHeaders(r queryrange.Request) []*RequestHeader {
	switch req := r.(type) {
	case *ThanosQueryRangeRequest:
		return req.Headers
	case *ThanosQueryInstantRequest:
		return req.Headers
	default:
		return nil
	}
}
