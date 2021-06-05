// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// To build the proto file:
// sudo apt-get install protobuf-compiler
// GOPATH=$JIRI_ROOT/release/go go get github.com/golang/protobuf/proto
// GOPATH=$JIRI_ROOT/release/go go get github.com/golang/protobuf/protoc-gen-go
// GOPATH=$JIRI_ROOT/release/go go install github.com/golang/protobuf/protoc-gen-go
// PATH=$PATH:$JIRI_ROOT/release/go/bin protoc --go_out=. rpc.proto
// manually add copyright notice

package benchmark

import (
	"testing"

	"github.com/golang/protobuf/proto"
)

var echoStr = "Echo"
var langStr = "en-US.UTF-8"
var uint641 = uint64(1)
var boolTrue = true
var boolFalse = false
var int64100000 = int64(1000000)
var emptyString string = ""
var int320 = int32(0)

var protoReq = &RpcRequest{
	Suffix:           &emptyString,
	Method:           &echoStr,
	NumPosArgs:       &uint641,
	EndStreamArgs:    &boolFalse,
	Deadline:         &TimeWireDeadline{FromNow: &TimeDuration{Seconds: &int64100000, Nanos: &int64100000}, NoDeadline: &boolFalse},
	GrantedBlessings: &SecurityWireBlessings{},
	TraceRequest: &VtraceRequest{
		SpanId:   []byte{0x2a, 0xa2, 0xfd, 0xce, 0x42, 0xa7, 0xbd, 0x34, 0x88, 0x8, 0x11, 0x2f, 0x71, 0xce, 0x80, 0xa2},
		TraceId:  []byte{0x2a, 0xa2, 0xfd, 0xce, 0x42, 0xa7, 0xbd, 0x34, 0x88, 0x8, 0x11, 0x2f, 0x71, 0xce, 0x80, 0xa0},
		Flags:    &int320,
		LogLevel: &int320,
	},
	Language: &langStr,
}

var protoResp = &RpcResponse{
	EndStreamResults: &boolTrue,
	NumPosResults:    &uint641,
	TraceResponse: &VtraceResponse{
		TraceFlags: &int320,
		Trace: &VtraceTraceRecord{
			Id: []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		},
	},
	AckBlessings: &boolFalse,
}

func BenchmarkProtoEncodeRequest(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := proto.Marshal(protoReq); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProtoDecodeRequest(b *testing.B) {
	bytes, err := proto.Marshal(protoReq)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response := RpcResponse{}
		if err := proto.Unmarshal(bytes, &response); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProtoEncodeResponse(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := proto.Marshal(protoResp); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkProtoDecodeResponse(b *testing.B) {
	bytes, err := proto.Marshal(protoResp)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response := RpcResponse{}
		if err := proto.Unmarshal(bytes, &response); err != nil {
			b.Fatal(err)
		}
	}
}
