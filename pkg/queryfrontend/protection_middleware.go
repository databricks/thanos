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
	parseLatency prometheus.Histogram
	evalLatency  prometheus.Histogram
	totalLatency prometheus.Histogram
}

// NewProtectionMiddleware creates a new middleware that applies protection rules to queries.
func NewProtectionMiddleware(engine *ProtectionEngine, logger log.Logger, reg prometheus.Registerer) queryrange.Middleware {
	durationBuckets := []float64{0.0001, 0.00025, 0.0005, 0.001, 0.005, 0.01, 0.1, 1.0, 10.0}

	parseLatency := promauto.With(reg).NewHistogram(prometheus.HistogramOpts{
		Namespace: "thanos",
		Subsystem: "query_frontend",
		Name:      "protection_parse_duration_seconds",
		Help:      "Duration of PromQL parsing in the protection middleware.",
		Buckets:   durationBuckets,
	})
	evalLatency := promauto.With(reg).NewHistogram(prometheus.HistogramOpts{
		Namespace: "thanos",
		Subsystem: "query_frontend",
		Name:      "protection_evaluate_duration_seconds",
		Help:      "Duration of rule evaluation in the protection middleware.",
		Buckets:   durationBuckets,
	})
	totalLatency := promauto.With(reg).NewHistogram(prometheus.HistogramOpts{
		Namespace: "thanos",
		Subsystem: "query_frontend",
		Name:      "protection_duration_seconds",
		Help:      "Total duration of the protection middleware, including parsing and evaluation.",
		Buckets:   durationBuckets,
	})

	return queryrange.MiddlewareFunc(func(next queryrange.Handler) queryrange.Handler {
		return &protectionMiddleware{
			next:         next,
			engine:       engine,
			logger:       logger,
			parseLatency: parseLatency,
			evalLatency:  evalLatency,
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

	// Parse PromQL and measure latency.
	parseStart := time.Now()
	parsed, err := extpromql.ParseExpr(query)
	m.parseLatency.Observe(time.Since(parseStart).Seconds())

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
	evalStart := time.Now()
	protectionResult, err := m.engine.Evaluate(ctx, req)
	m.evalLatency.Observe(time.Since(evalStart).Seconds())

	if err != nil {
		return nil, errors.Wrap(err, "protection engine evaluation failed")
	}

	if protectionResult != nil {
		ctx = WithProtectionResult(ctx, protectionResult)
	}

	// Return 400 Bad Request error code if a protection rule is triggered.
	if protectionResult != nil && protectionResult.Action == RuleActionBlock {
		return nil, httpgrpc.Errorf(http.StatusBadRequest, "query blocked by protection rule: %s", protectionResult.RuleName)
	}

	return m.next.Do(ctx, r)
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
