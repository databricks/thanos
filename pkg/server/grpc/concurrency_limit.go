// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package grpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func NewUnaryServerConcurrencyLimitInterceptor(maxGoroutines int) grpc.UnaryServerInterceptor {
	var semaphore chan struct{}

	if maxGoroutines > 0 {
		semaphore = make(chan struct{}, maxGoroutines)
	}

	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if semaphore == nil {
			return handler(ctx, req)
		}

		select {
		case semaphore <- struct{}{}:
			defer func() { <-semaphore }()
			return handler(ctx, req)
		default:
			return nil, status.Errorf(codes.ResourceExhausted, "too many concurrent requests, please try again later")
		}
	}
}

func NewStreamServerConcurrencyLimitInterceptor(maxGoroutines int) grpc.StreamServerInterceptor {
	var semaphore chan struct{}

	if maxGoroutines > 0 {
		semaphore = make(chan struct{}, maxGoroutines)
	}

	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		if semaphore == nil {
			return handler(srv, ss)
		}
		select {
		case semaphore <- struct{}{}:
			defer func() { <-semaphore }()
			return handler(srv, ss)
		default:
			return status.Errorf(codes.ResourceExhausted, "too many concurrent requests")
		}
	}
}
