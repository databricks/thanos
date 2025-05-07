// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package grpc

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeUnaryHandler is a test handler that we can use for unary requests.
func fakeUnaryHandler(ctx context.Context, req interface{}) (interface{}, error) {
	return "response", nil
}

// blockingUnaryHandler returns a handler that blocks until the returned channel is closed.
// This lets us control concurrency by pausing the handler execution.
func blockingUnaryHandler(done chan struct{}) grpc.UnaryHandler {
	return func(ctx context.Context, req interface{}) (interface{}, error) {
		<-done
		return "response", nil
	}
}

// fakeServerStream is a mock that implements grpc.ServerStream interface for testing.
type fakeServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (f *fakeServerStream) Context() context.Context {
	return f.ctx
}

// fakeStreamHandler is a test handler for streaming requests.
func fakeStreamHandler(srv interface{}, stream grpc.ServerStream) error {
	return nil
}

// blockingStreamHandler is a handler that blocks until the returned channel is closed.
func blockingStreamHandler(done chan struct{}) grpc.StreamHandler {
	return func(srv interface{}, stream grpc.ServerStream) error {
		<-done
		return nil
	}
}

func TestNewUnaryServerConcurrencyLimitInterceptor_NoLimit(t *testing.T) {
	// maxGoroutines <= 0 means no concurrency limit.
	interceptor := NewUnaryServerConcurrencyLimitInterceptor(0)

	// Should just call the handler without limitation.
	resp, err := interceptor(context.Background(), "req", &grpc.UnaryServerInfo{}, fakeUnaryHandler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "response" {
		t.Fatalf("expected response 'response', got %v", resp)
	}
}

func TestNewUnaryServerConcurrencyLimitInterceptor_LimitNotExceeded(t *testing.T) {
	interceptor := NewUnaryServerConcurrencyLimitInterceptor(2)

	resp, err := interceptor(context.Background(), "req", &grpc.UnaryServerInfo{}, fakeUnaryHandler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "response" {
		t.Fatalf("expected 'response', got %v", resp)
	}
}

func TestNewUnaryServerConcurrencyLimitInterceptor_LimitExceeded(t *testing.T) {
	// Set a limit of 1 concurrent request.
	interceptor := NewUnaryServerConcurrencyLimitInterceptor(1)

	done := make(chan struct{})
	handler := blockingUnaryHandler(done)

	// First request starts and blocks.
	ctx1 := context.Background()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = interceptor(ctx1, "req1", &grpc.UnaryServerInfo{}, handler)
	}()

	// Give some time for the first request to block inside the handler.
	time.Sleep(50 * time.Millisecond)

	// Second request should fail because the first one is still running and limit=1.
	ctx2 := context.Background()
	resp2, err2 := interceptor(ctx2, "req2", &grpc.UnaryServerInfo{}, fakeUnaryHandler)
	if err2 == nil {
		t.Fatalf("expected ResourceExhausted error, got nil error with response: %v", resp2)
	}
	st, _ := status.FromError(err2)
	if st.Code() != codes.ResourceExhausted {
		t.Fatalf("expected ResourceExhausted error, got %v", st.Code())
	}

	// Unblock the first handler so goroutines can clean up.
	close(done)
	wg.Wait()
}

func TestNewStreamServerConcurrencyLimitInterceptor_NoLimit(t *testing.T) {
	interceptor := NewStreamServerConcurrencyLimitInterceptor(0)
	stream := &fakeServerStream{ctx: context.Background()}
	err := interceptor(nil, stream, &grpc.StreamServerInfo{}, fakeStreamHandler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewStreamServerConcurrencyLimitInterceptor_LimitNotExceeded(t *testing.T) {
	interceptor := NewStreamServerConcurrencyLimitInterceptor(2)
	stream := &fakeServerStream{ctx: context.Background()}
	err := interceptor(nil, stream, &grpc.StreamServerInfo{}, fakeStreamHandler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewStreamServerConcurrencyLimitInterceptor_LimitExceeded(t *testing.T) {
	interceptor := NewStreamServerConcurrencyLimitInterceptor(1)
	done := make(chan struct{})
	handler := blockingStreamHandler(done)

	stream1 := &fakeServerStream{ctx: context.Background()}
	stream2 := &fakeServerStream{ctx: context.Background()}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = interceptor(nil, stream1, &grpc.StreamServerInfo{}, handler)
	}()

	// Wait a bit for the first stream to block inside the handler.
	time.Sleep(50 * time.Millisecond)

	err2 := interceptor(nil, stream2, &grpc.StreamServerInfo{}, fakeStreamHandler)
	if err2 == nil {
		t.Fatalf("expected ResourceExhausted error, got nil")
	}
	st, _ := status.FromError(err2)
	if st.Code() != codes.ResourceExhausted {
		t.Fatalf("expected ResourceExhausted, got %v", st.Code())
	}

	// Unblock the first handler.
	close(done)
	wg.Wait()
}

func TestNewUnaryServerConcurrencyLimitInterceptor_HandlerErrorPropagation(t *testing.T) {
	// Test that if the handler returns an error, it's propagated correctly.
	interceptor := NewUnaryServerConcurrencyLimitInterceptor(2)
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, errors.New("handler error")
	}

	_, err := interceptor(context.Background(), "req", &grpc.UnaryServerInfo{}, handler)
	if err == nil || err.Error() != "handler error" {
		t.Fatalf("expected 'handler error', got %v", err)
	}
}

func TestNewStreamServerConcurrencyLimitInterceptor_HandlerErrorPropagation(t *testing.T) {
	interceptor := NewStreamServerConcurrencyLimitInterceptor(2)
	handler := func(srv interface{}, stream grpc.ServerStream) error {
		return errors.New("handler error")
	}
	stream := &fakeServerStream{ctx: context.Background()}

	err := interceptor(nil, stream, &grpc.StreamServerInfo{}, handler)
	if err == nil || err.Error() != "handler error" {
		t.Fatalf("expected 'handler error', got %v", err)
	}
}
