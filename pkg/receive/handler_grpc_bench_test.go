// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package receive

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/thanos-io/thanos/pkg/store/labelpb"
	"github.com/thanos-io/thanos/pkg/store/storepb"
	"github.com/thanos-io/thanos/pkg/store/storepb/prompb"
	"github.com/thanos-io/thanos/pkg/tenancy"
)

type benchCluster struct {
	handlers  []*Handler
	servers   []*grpc.Server
	endpoints []Endpoint
	client    storepb.WriteableStoreClient
	conn      *grpc.ClientConn
}

func (c *benchCluster) close() {
	c.conn.Close()
	for _, h := range c.handlers {
		for _, ep := range c.endpoints {
			h.peers.close(ep)
		}
	}
	for _, srv := range c.servers {
		srv.GracefulStop()
	}
}

// setupBenchCluster creates a cluster of numNodes receive handlers, each
// running on a real gRPC server with an in-process TCP listener. The
// handlers are wired into a hashmod hashring and connected via real gRPC
// peer connections. A client connected to the first node is returned.
func setupBenchCluster(b *testing.B, numNodes int, rf uint64) *benchCluster {
	b.Helper()

	logger := log.NewNopLogger()

	listeners := make([]net.Listener, numNodes)
	addresses := make([]string, numNodes)
	for i := 0; i < numNodes; i++ {
		lis, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			b.Fatalf("listen: %v", err)
		}
		listeners[i] = lis
		addresses[i] = lis.Addr().String()
	}

	endpoints := make([]Endpoint, numNodes)
	for i, addr := range addresses {
		endpoints[i] = Endpoint{Address: addr}
	}

	limiter, _ := NewLimiter(NewNopConfig(), nil, RouterIngestor, log.NewNopLogger(), 1*time.Second)

	handlers := make([]*Handler, numNodes)
	for i := 0; i < numNodes; i++ {
		appendable := &fakeAppendable{appender: newFakeAppender(nil, nil, nil)}
		handlers[i] = NewHandler(logger, &Options{
			TenantHeader:        tenancy.DefaultTenantHeader,
			DefaultTenantID:     "bench-grpc",
			ReplicaHeader:       DefaultReplicaHeader,
			ReplicationFactor:   rf,
			ForwardTimeout:      5 * time.Minute,
			Writer:              NewWriter(log.NewNopLogger(), newFakeTenantAppendable(appendable), &WriterOptions{}),
			Limiter:             limiter,
			Endpoint:            addresses[i],
			ReplicationProtocol: ProtobufReplication,
			DialOpts: []grpc.DialOption{
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			},
		})
	}

	servers := make([]*grpc.Server, numNodes)
	for i := 0; i < numNodes; i++ {
		servers[i] = grpc.NewServer()
		storepb.RegisterWriteableStoreServer(servers[i], handlers[i])
		lis := listeners[i]
		go servers[i].Serve(lis)
	}

	hashring, err := NewMultiHashring(AlgorithmHashmod, rf, []HashringConfig{{
		Hashring:  "bench",
		Endpoints: endpoints,
	}}, prometheus.NewRegistry())
	if err != nil {
		b.Fatalf("hashring: %v", err)
	}
	for _, h := range handlers {
		h.Hashring(hashring)
	}

	conn, err := grpc.NewClient(
		addresses[0],
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		b.Fatalf("dial: %v", err)
	}

	return &benchCluster{
		handlers:  handlers,
		servers:   servers,
		endpoints: endpoints,
		client:    storepb.NewWriteableStoreClient(conn),
		conn:      conn,
	}
}

func BenchmarkHandlerRemoteWriteGRPC(b *testing.B) {
	type topology struct {
		name     string
		numNodes int
		rf       uint64
	}
	topologies := []topology{
		{"single_node", 1, 1},
		{"three_node_rf1", 3, 1},
		{"three_node_rf3", 3, 3},
	}

	type payload struct {
		name             string
		numSeries        int
		samplesPerSeries int
		labelsPerSeries  int
	}
	payloads := []payload{
		{"small_10s", 10, 1, 5},
		{"medium_500s", 500, 1, 10},
		{"large_5000s", 5000, 1, 10},
	}

	for _, topo := range topologies {
		b.Run(topo.name, func(b *testing.B) {
			cluster := setupBenchCluster(b, topo.numNodes, topo.rf)
			defer cluster.close()

			for _, pl := range payloads {
				req := buildWriteRequest(b, pl.numSeries, pl.samplesPerSeries, pl.labelsPerSeries)

				b.Run(pl.name, func(b *testing.B) {
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						resp, err := cluster.client.RemoteWrite(context.Background(), req)
						if err != nil {
							b.Fatal(err)
						}
						if resp == nil {
							b.Fatal("nil response")
						}
					}
				})
			}
		})
	}
}

func buildWriteRequest(b *testing.B, numSeries, samplesPerSeries, labelsPerSeries int) *storepb.WriteRequest {
	b.Helper()
	wreq := &storepb.WriteRequest{
		Tenant:     "bench-grpc",
		Timeseries: make([]*prompb.TimeSeries, numSeries),
	}
	for s := 0; s < numSeries; s++ {
		lbls := make([]*labelpb.Label, labelsPerSeries)
		lbls[0] = &labelpb.Label{Name: "__name__", Value: fmt.Sprintf("bench_metric_%d", s)}
		for l := 1; l < labelsPerSeries; l++ {
			lbls[l] = &labelpb.Label{
				Name:  fmt.Sprintf("label_%03d", l),
				Value: fmt.Sprintf("val_%03d_%06d", l, s),
			}
		}
		samples := make([]*prompb.Sample, samplesPerSeries)
		for i := 0; i < samplesPerSeries; i++ {
			samples[i] = &prompb.Sample{
				Value:     float64(i),
				Timestamp: int64(s*samplesPerSeries + i),
			}
		}
		wreq.Timeseries[s] = &prompb.TimeSeries{
			Labels:  lbls,
			Samples: samples,
		}
	}
	return wreq
}
