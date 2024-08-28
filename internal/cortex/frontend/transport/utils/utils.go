// Copyright (c) The Cortex Authors.
// Licensed under the Apache License 2.0.

// Package utils Monitoring platform team helper resources for frontend
package utils

import (
	"fmt"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	cacheableResponseCodes = []int{http.StatusRequestTimeout, http.StatusGatewayTimeout, http.StatusBadRequest}
)

// FailedQueryCache Handler holds an instance of FailedQueryCache and calls its methods
type FailedQueryCache struct {
	regex         *regexp.Regexp
	errorExtract  *regexp.Regexp
	lruCache      *expirable.LRU[string, int]
	cachedHits    prometheus.Counter
	cachedQueries prometheus.Gauge
}

func NewFailedQueryCache(capacity int, ttlDuration time.Duration, reg prometheus.Registerer) *FailedQueryCache {
	regex := regexp.MustCompile(`[\s\n\t]+`)
	errorExtract := regexp.MustCompile(`Code\((\d+)\)`)
	lruCacheWithTTL := expirable.NewLRU[string, int](capacity, nil, ttlDuration)

	cachedHits := promauto.With(reg).NewCounter(prometheus.CounterOpts{
		Namespace: "cortex",
		Name:      "cached_failed_queries_count",
		Help:      "Total number of queries that hit the failed query cache.",
	})
	cachedQueries := promauto.With(reg).NewGauge(prometheus.GaugeOpts{
		Namespace: "cortex",
		Name:      "failed_query_cache_size",
		Help:      "How many queries are cached in the failed query cache.",
	})
	cachedQueries.Set(0)

	return &FailedQueryCache{
		regex:         regex,
		errorExtract:  errorExtract,
		lruCache:      lruCacheWithTTL,
		cachedHits:    cachedHits,
		cachedQueries: cachedQueries,
	}
}

// UpdateFailedQueryCache returns true if query is cached so that callsite can increase counter, returns message as a string for callsite to log outcome
func (f *FailedQueryCache) updateFailedQueryCache(err error, queryExpressionNormalized string, queryExpressionRangeLength int) (bool, string) {
	// Extracting error code from error string.
	codeExtract := f.errorExtract.FindStringSubmatch(err.Error())

	// Checking if error code extracted successfully.
	if codeExtract == nil || len(codeExtract) < 2 {
		message := createLogMessage("String to regex conversion error", queryExpressionNormalized, -1, queryExpressionRangeLength, err)
		return false, message
	}

	// Converting error code to int.
	errCode, strConvError := strconv.Atoi(codeExtract[1])

	// Checking if error code extracted properly from string.
	if strConvError != nil {
		message := createLogMessage("String to int conversion error", queryExpressionNormalized, -1, queryExpressionRangeLength, err)
		return false, message
	}

	// If error should be cached, store it in cache.
	if !isCacheableError(errCode) {
		message := createLogMessage("Query not cached due to non-cacheable error code", queryExpressionNormalized, -1, queryExpressionRangeLength, err)
		return false, message
	}
	f.addCacheEntry(queryExpressionNormalized, queryExpressionRangeLength)
	message := createLogMessage("Cached a failed query", queryExpressionNormalized, -1, queryExpressionRangeLength, err)
	return true, message
}

func (f *FailedQueryCache) addCacheEntry(queryExpressionNormalized string, queryExpressionRangeLength int) {
	// Checks if queryExpression is already in cache, and updates time range length value to min of stored and new value.
	if contains := f.lruCache.Contains(queryExpressionNormalized); contains {
		if oldValue, ok := f.lruCache.Get(queryExpressionNormalized); ok {
			queryExpressionRangeLength = min(queryExpressionRangeLength, oldValue)
		}
	}
	f.lruCache.Add(queryExpressionNormalized, queryExpressionRangeLength)

	f.cachedQueries.Set(float64(f.lruCache.Len()))
}

// QueryHitCache checks if the lru cache is hit and returns whether to increment counter for cache hits along with appropriate message.
func queryHitCache(queryExpressionNormalized string, queryExpressionRangeLength int, lruCache *expirable.LRU[string, int], cachedHits prometheus.Counter) (bool, string) {
	if value, ok := lruCache.Get(queryExpressionNormalized); ok && value <= queryExpressionRangeLength {
		cachedQueryRangeSeconds := value
		message := createLogMessage("Retrieved query from cache", queryExpressionNormalized, cachedQueryRangeSeconds, queryExpressionRangeLength, nil)
		cachedHits.Inc()
		return true, message
	}
	return false, ""
}

// isCacheableError Returns true if response code is in pre-defined cacheable errors list, else returns false.
func isCacheableError(statusCode int) bool {
	for _, errStatusCode := range cacheableResponseCodes {
		if errStatusCode == statusCode {
			return true
		}
	}
	return false
}

// GetQueryRangeSeconds Time range length for queries, if either of "start" or "end" are not present, return 0.
func getQueryRangeSeconds(query url.Values) int {
	start, err := strconv.Atoi(query.Get("start"))
	if err != nil {
		return 0
	}
	end, err := strconv.Atoi(query.Get("end"))
	if err != nil {
		return 0
	}
	return end - start
}

func (f *FailedQueryCache) normalizeQueryString(query url.Values) string {
	return f.regex.ReplaceAllString(query.Get("query"), " ")
}

func createLogMessage(message string, queryExpressionNormalized string, cachedQueryRangeSeconds int, queryExpressionRangeLength int, err error) string {
	if err == nil {
		return fmt.Sprintf(
			`%s, %s: %s, %s: %d, %s: %d`, message,
			"cached_query", queryExpressionNormalized,
			"cached_range_seconds", cachedQueryRangeSeconds,
			"query_range_seconds", queryExpressionRangeLength)
	}
	return fmt.Sprintf(
		`%s, %s: %s, %s: %d, %s: %s`, message,
		"cached_query", queryExpressionNormalized,
		"query_range_seconds", queryExpressionRangeLength,
		"cached_error", err)
}

func (f *FailedQueryCache) UpdateFailedQueryCache(err error, query url.Values, queryResponseTime time.Duration) (bool, string) {
	queryExpressionNormalized := f.normalizeQueryString(query)
	queryExpressionRangeLength := getQueryRangeSeconds(query)
	// TODO(hc.zhu): add a flag for the threshold
	// The current gateway timeout is 5 minutes, so we cache the failed query running longer than 5 minutes - 10 seconds.
	if queryResponseTime > time.Second*(60*5-10) {
		// Cache long running queries regardless of the error code. The most common case is "context canceled".
		f.addCacheEntry(queryExpressionNormalized, queryExpressionRangeLength)
		message := createLogMessage("Cached a failed long running query", queryExpressionNormalized, -1, queryExpressionRangeLength, err)
		return true, message
	}
	if queryExpressionNormalized == "" {
		// Other APIs don't have "query" parameter e.g., /api/v1/series, /api/v1/labels
		return false, "Query parameter is empty"
	}
	success, message := f.updateFailedQueryCache(err, queryExpressionNormalized, queryExpressionRangeLength)
	return success, message
}

func (f *FailedQueryCache) QueryHitCache(query url.Values) (bool, string) {
	queryExpressionNormalized := f.normalizeQueryString(query)
	if queryExpressionNormalized == "" {
		// Other APIs don't have "query" parameter e.g., /api/v1/series, /api/v1/labels
		return false, "Query parameter is empty"
	}
	queryExpressionRangeLength := getQueryRangeSeconds(query)
	cached, message := queryHitCache(queryExpressionNormalized, queryExpressionRangeLength, f.lruCache, f.cachedHits)
	return cached, message
}
