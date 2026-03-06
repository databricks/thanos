// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

// protoc-gen-go-grpc-vtpool is a protoc plugin that generates pool-aware
// gRPC server handler functions for messages that have both vtprotobuf's
// mempool option and a vtpool.pool_return option set. It produces a
// supplementary _grpc_vtpool.pb.go file that patches the ServiceDesc at
// init() time to use handlers that acquire request messages from the VT
// pool instead of heap-allocating.
//
// Messages must opt in explicitly via proto options:
//
//	message WriteRequest {
//	  option (vtproto.mempool) = true;
//	  option (vtpool.pool_return) = POOL_RETURN_DEFER;
//	  ...
//	}
//
// Pool return modes (set per-message via vtpool.pool_return):
//   - POOL_RETURN_DEFER:  generates defer in.ReturnToVTPool() after acquisition
//   - POOL_RETURN_CALLER: no return generated; the RPC handler takes ownership
//
// Usage:
//
//	protoc --go-grpc-vtpool_out=. --go-grpc-vtpool_opt=paths=source_relative foo.proto
package main

import (
	"flag"
	"fmt"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

const version = "0.1.0"

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("protoc-gen-go-grpc-vtpool %v\n", version)
		return
	}

	var flags flag.FlagSet

	protogen.Options{
		ParamFunc: flags.Set,
	}.Run(func(gen *protogen.Plugin) error {
		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			generateFile(gen, f)
		}
		return nil
	})
}
