// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"v.io/v23/context"
	"v.io/v23/rpc"

	"v.io/x/ref/runtime/internal/rpc/benchmark"
)

type impl struct {
}

func NewService() benchmark.BenchmarkServerStub {
	return benchmark.BenchmarkServer(&impl{})
}

func (i *impl) Echo(_ *context.T, _ rpc.ServerCall, payload []byte) ([]byte, error) {
	return payload, nil
}

func (i *impl) EchoStream(_ *context.T, call benchmark.BenchmarkEchoStreamServerCall) error {
	rStream := call.RecvStream()
	sStream := call.SendStream()
	for rStream.Advance() {
		sStream.Send(rStream.Value()) //nolint:errcheck
	}
	if err := rStream.Err(); err != nil {
		return err
	}
	return nil
}
