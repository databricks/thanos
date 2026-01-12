// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package v1

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/oklog/ulid"
	"github.com/prometheus/common/route"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/tsdb"
	"github.com/thanos-io/objstore"

	"github.com/efficientgo/core/testutil"
	baseAPI "github.com/thanos-io/thanos/pkg/api"
	"github.com/thanos-io/thanos/pkg/block"
	"github.com/thanos-io/thanos/pkg/block/metadata"
	"github.com/thanos-io/thanos/pkg/testutil/custom"
	"github.com/thanos-io/thanos/pkg/testutil/e2eutil"
)

func TestMain(m *testing.M) {
	custom.TolerantVerifyLeakMain(m)
}

type endpointTestCase struct {
	endpoint baseAPI.ApiFunc
	params   map[string]string
	query    url.Values
	method   string
	response interface{}
	errType  baseAPI.ErrorType
}
type responeCompareFunction func(interface{}, interface{}) bool

func testEndpoint(t *testing.T, test endpointTestCase, name string, responseCompareFunc responeCompareFunction) bool {
	return t.Run(name, func(t *testing.T) {
		// Build a context with the correct request params.
		ctx := context.Background()
		for p, v := range test.params {
			ctx = route.WithParam(ctx, p, v)
		}

		reqURL := "http://example.com"
		params := test.query.Encode()

		var body io.Reader
		if test.method == http.MethodPost {
			body = strings.NewReader(params)
		} else if test.method == "" {
			test.method = "ANY"
			reqURL += "?" + params
		}

		req, err := http.NewRequest(test.method, reqURL, body)
		if err != nil {
			t.Fatal(err)
		}

		if body != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}

		resp, _, apiErr, releaseResources := test.endpoint(req.WithContext(ctx))
		defer releaseResources()
		if apiErr != nil {
			if test.errType == baseAPI.ErrorNone {
				t.Fatalf("Unexpected error: %s", apiErr)
			}
			if test.errType != apiErr.Typ {
				t.Fatalf("Expected error of type %q but got type %q", test.errType, apiErr.Typ)
			}
			return
		}
		if test.errType != baseAPI.ErrorNone {
			t.Fatalf("Expected error of type %q but got none", test.errType)
		}

		if !responseCompareFunc(resp, test.response) {
			t.Fatalf("Response does not match, expected:\n%+v\ngot:\n%+v", test.response, resp)
		}
	})
}

func TestMarkBlockEndpoint(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// create block
	b1, err := e2eutil.CreateBlock(ctx, tmpDir, []labels.Labels{
		labels.FromStrings("a", "1"),
		labels.FromStrings("a", "2"),
		labels.FromStrings("a", "3"),
		labels.FromStrings("a", "4"),
		labels.FromStrings("b", "1"),
	}, 100, 0, 1000, labels.FromStrings("ext1", "val1"), 124, metadata.NoneFunc)
	testutil.Ok(t, err)

	// upload block
	bkt := objstore.WithNoopInstr(objstore.NewInMemBucket())
	logger := log.NewNopLogger()
	testutil.Ok(t, block.Upload(ctx, logger, bkt, path.Join(tmpDir, b1.String()), metadata.NoneFunc))

	now := time.Now()
	api := &BlocksAPI{
		baseAPI: &baseAPI.BaseAPI{
			Now: func() time.Time { return now },
		},
		logger: logger,
		globalBlocksInfo: &BlocksInfo{
			Blocks: []metadata.Meta{},
			Label:  "foo",
		},
		loadedBlocksInfo: &BlocksInfo{
			Blocks: []metadata.Meta{},
			Label:  "foo",
		},
		loadedBlocksByTenant: make(map[string][]metadata.Meta),
		disableCORS:          true,
		bkt:                  bkt,
		label:                "foo",
	}

	var tests = []endpointTestCase{
		// Empty ID
		{
			endpoint: api.markBlock,
			query: url.Values{
				"id": []string{""},
			},
			errType: baseAPI.ErrorBadData,
		},
		// Empty action
		{
			endpoint: api.markBlock,
			query: url.Values{
				"id":     []string{ulid.MustNew(1, nil).String()},
				"action": []string{""},
			},
			errType: baseAPI.ErrorBadData,
		},
		// invalid ULID
		{
			endpoint: api.markBlock,
			query: url.Values{
				"id":     []string{"invalid_id"},
				"action": []string{"DELETION"},
			},
			errType: baseAPI.ErrorBadData,
		},
		// invalid action
		{
			endpoint: api.markBlock,
			query: url.Values{
				"id":     []string{ulid.MustNew(2, nil).String()},
				"action": []string{"INVALID_ACTION"},
			},
			errType: baseAPI.ErrorBadData,
		},
		{
			endpoint: api.markBlock,
			query: url.Values{
				"id":     []string{b1.String()},
				"action": []string{"DELETION"},
			},
			response: nil,
		},
	}

	for i, test := range tests {
		if ok := testEndpoint(t, test, fmt.Sprintf("#%d %s", i, test.query.Encode()), reflect.DeepEqual); !ok {
			return
		}
	}

	file := path.Join(tmpDir, b1.String())
	_, err = os.Stat(file)
	testutil.Ok(t, err)
}

func TestSetLoadedForTenant(t *testing.T) {
	logger := log.NewNopLogger()
	bkt := objstore.WithNoopInstr(objstore.NewInMemBucket())

	api := NewBlocksAPI(logger, true, "test-label", map[string]string{}, bkt)

	// Create some test block metadata
	block1 := metadata.Meta{
		BlockMeta: tsdb.BlockMeta{
			ULID:    ulid.MustNew(1, nil),
			MinTime: 0,
			MaxTime: 1000,
		},
	}
	block2 := metadata.Meta{
		BlockMeta: tsdb.BlockMeta{
			ULID:    ulid.MustNew(2, nil),
			MinTime: 1000,
			MaxTime: 2000,
		},
	}
	block3 := metadata.Meta{
		BlockMeta: tsdb.BlockMeta{
			ULID:    ulid.MustNew(3, nil),
			MinTime: 2000,
			MaxTime: 3000,
		},
	}

	// Test setting blocks for multiple tenants
	api.SetLoadedForTenant("tenant-a", []metadata.Meta{block1, block2}, nil)
	api.SetLoadedForTenant("tenant-b", []metadata.Meta{block3}, nil)

	// Verify blocks are stored per tenant
	testutil.Equals(t, 2, len(api.loadedBlocksByTenant["tenant-a"]))
	testutil.Equals(t, 1, len(api.loadedBlocksByTenant["tenant-b"]))

	// Test single-tenant mode (empty string tenant)
	api2 := NewBlocksAPI(logger, true, "test-label", map[string]string{}, bkt)
	api2.SetLoadedForTenant("", []metadata.Meta{block1, block2, block3}, nil)
	testutil.Equals(t, 3, len(api2.loadedBlocksByTenant[""]))
}

func TestBlocksEndpointMultiTenantAggregation(t *testing.T) {
	logger := log.NewNopLogger()
	bkt := objstore.WithNoopInstr(objstore.NewInMemBucket())

	api := NewBlocksAPI(logger, true, "test-label", map[string]string{}, bkt)

	// Create test block metadata for different tenants
	block1 := metadata.Meta{
		BlockMeta: tsdb.BlockMeta{
			ULID:    ulid.MustNew(1, nil),
			MinTime: 0,
			MaxTime: 1000,
		},
	}
	block2 := metadata.Meta{
		BlockMeta: tsdb.BlockMeta{
			ULID:    ulid.MustNew(2, nil),
			MinTime: 1000,
			MaxTime: 2000,
		},
	}
	block3 := metadata.Meta{
		BlockMeta: tsdb.BlockMeta{
			ULID:    ulid.MustNew(3, nil),
			MinTime: 2000,
			MaxTime: 3000,
		},
	}

	// Set blocks for multiple tenants
	api.SetLoadedForTenant("tenant-a", []metadata.Meta{block1}, nil)
	api.SetLoadedForTenant("tenant-b", []metadata.Meta{block2, block3}, nil)

	// Create a request for the loaded view
	req, err := http.NewRequest("GET", "http://example.com?view=loaded", nil)
	testutil.Ok(t, err)

	resp, _, apiErr, releaseResources := api.blocks(req)
	defer releaseResources()

	testutil.Equals(t, (*baseAPI.ApiError)(nil), apiErr)

	blocksInfo, ok := resp.(*BlocksInfo)
	testutil.Assert(t, ok, "response should be *BlocksInfo")
	testutil.Equals(t, "test-label", blocksInfo.Label)
	testutil.Equals(t, 3, len(blocksInfo.Blocks)) // Should aggregate all blocks from all tenants
}

func TestBlocksEndpointSingleTenantFallback(t *testing.T) {
	logger := log.NewNopLogger()
	bkt := objstore.WithNoopInstr(objstore.NewInMemBucket())

	api := NewBlocksAPI(logger, true, "test-label", map[string]string{}, bkt)

	// Create test block metadata
	block1 := metadata.Meta{
		BlockMeta: tsdb.BlockMeta{
			ULID:    ulid.MustNew(1, nil),
			MinTime: 0,
			MaxTime: 1000,
		},
	}

	// Use SetLoaded (single-tenant backward compatibility)
	api.SetLoaded([]metadata.Meta{block1}, nil)

	// Create a request for the loaded view
	req, err := http.NewRequest("GET", "http://example.com?view=loaded", nil)
	testutil.Ok(t, err)

	resp, _, apiErr, releaseResources := api.blocks(req)
	defer releaseResources()

	testutil.Equals(t, (*baseAPI.ApiError)(nil), apiErr)

	blocksInfo, ok := resp.(*BlocksInfo)
	testutil.Assert(t, ok, "response should be *BlocksInfo")
	testutil.Equals(t, 1, len(blocksInfo.Blocks)) // Should return blocks from loadedBlocksInfo
}

func TestBlocksEndpointGlobalView(t *testing.T) {
	logger := log.NewNopLogger()
	bkt := objstore.WithNoopInstr(objstore.NewInMemBucket())

	api := NewBlocksAPI(logger, true, "test-label", map[string]string{}, bkt)

	// Create test block metadata
	block1 := metadata.Meta{
		BlockMeta: tsdb.BlockMeta{
			ULID:    ulid.MustNew(1, nil),
			MinTime: 0,
			MaxTime: 1000,
		},
	}
	block2 := metadata.Meta{
		BlockMeta: tsdb.BlockMeta{
			ULID:    ulid.MustNew(2, nil),
			MinTime: 1000,
			MaxTime: 2000,
		},
	}

	// Set global blocks
	api.SetGlobal([]metadata.Meta{block1, block2}, nil)

	// Create a request for the global view (no view param)
	req, err := http.NewRequest("GET", "http://example.com", nil)
	testutil.Ok(t, err)

	resp, _, apiErr, releaseResources := api.blocks(req)
	defer releaseResources()

	testutil.Equals(t, (*baseAPI.ApiError)(nil), apiErr)

	blocksInfo, ok := resp.(*BlocksInfo)
	testutil.Assert(t, ok, "response should be *BlocksInfo")
	testutil.Equals(t, 2, len(blocksInfo.Blocks))
}

func TestBlocksEndpointTenantFilter(t *testing.T) {
	logger := log.NewNopLogger()
	bkt := objstore.WithNoopInstr(objstore.NewInMemBucket())

	api := NewBlocksAPI(logger, true, "test-label", map[string]string{}, bkt)

	// Create test block metadata for different tenants
	block1 := metadata.Meta{
		BlockMeta: tsdb.BlockMeta{
			ULID:    ulid.MustNew(1, nil),
			MinTime: 0,
			MaxTime: 1000,
		},
	}
	block2 := metadata.Meta{
		BlockMeta: tsdb.BlockMeta{
			ULID:    ulid.MustNew(2, nil),
			MinTime: 1000,
			MaxTime: 2000,
		},
	}
	block3 := metadata.Meta{
		BlockMeta: tsdb.BlockMeta{
			ULID:    ulid.MustNew(3, nil),
			MinTime: 2000,
			MaxTime: 3000,
		},
	}

	// Set blocks for multiple tenants
	api.SetLoadedForTenant("tenant-a", []metadata.Meta{block1}, nil)
	api.SetLoadedForTenant("tenant-b", []metadata.Meta{block2, block3}, nil)

	// Test filtering by tenant-a
	req, err := http.NewRequest("GET", "http://example.com?view=loaded&tenant=tenant-a", nil)
	testutil.Ok(t, err)

	resp, _, apiErr, releaseResources := api.blocks(req)
	testutil.Equals(t, (*baseAPI.ApiError)(nil), apiErr)

	blocksInfo, ok := resp.(*BlocksInfo)
	testutil.Assert(t, ok, "response should be *BlocksInfo")
	testutil.Equals(t, 1, len(blocksInfo.Blocks)) // Only tenant-a blocks
	releaseResources()

	// Test filtering by tenant-b
	req, err = http.NewRequest("GET", "http://example.com?view=loaded&tenant=tenant-b", nil)
	testutil.Ok(t, err)

	resp, _, apiErr, releaseResources = api.blocks(req)
	testutil.Equals(t, (*baseAPI.ApiError)(nil), apiErr)

	blocksInfo, ok = resp.(*BlocksInfo)
	testutil.Assert(t, ok, "response should be *BlocksInfo")
	testutil.Equals(t, 2, len(blocksInfo.Blocks)) // Only tenant-b blocks
	releaseResources()

	// Test filtering by non-existent tenant
	req, err = http.NewRequest("GET", "http://example.com?view=loaded&tenant=tenant-c", nil)
	testutil.Ok(t, err)

	resp, _, apiErr, releaseResources = api.blocks(req)
	testutil.Equals(t, (*baseAPI.ApiError)(nil), apiErr)

	blocksInfo, ok = resp.(*BlocksInfo)
	testutil.Assert(t, ok, "response should be *BlocksInfo")
	testutil.Equals(t, 0, len(blocksInfo.Blocks)) // No blocks for non-existent tenant
	releaseResources()
}
