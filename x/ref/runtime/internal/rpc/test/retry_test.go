// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"testing"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/x/ref/test"
)

var errRetryThis = verror.NewIDAction("retry_test.retryThis", verror.RetryBackoff)

type retryServer struct {
	called int // number of times TryAgain has been called
}

func (s *retryServer) TryAgain(ctx *context.T, _ rpc.ServerCall) error {
	// If this is the second time this method is being called, return success.
	if s.called > 0 {
		s.called++
		return nil
	}
	s.called++
	// otherwise, return a verror with action code RetryBackoff.
	return errRetryThis.Errorf(ctx, "retryable error")
}

func TestRetryCall(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	// Start the server.
	rs := retryServer{}
	_, server, err := v23.WithNewServer(ctx, "", &rs, security.AllowEveryone())
	if err != nil {
		t.Fatal(err)
	}
	name := server.Status().Endpoints[0].Name()

	client := v23.GetClient(ctx)
	// A traditional client.StartCall/call.Finish sequence should fail at
	// call.Finish since the error returned by the server won't be retried.
	call, err := client.StartCall(ctx, name, "TryAgain", nil)
	if err != nil {
		t.Errorf("client.StartCall failed: %v", err)
	}
	if err := call.Finish(); err == nil {
		t.Errorf("call.Finish should have failed")
	}
	rs.called = 0

	// A call to client.Call should succeed because the error return by the
	// server should be retried.
	if err := client.Call(ctx, name, "TryAgain", nil, nil); err != nil {
		t.Errorf("client.Call failed: %v", err)
	}
	// Ensure that client.Call retried the call exactly once.
	if rs.called != 2 {
		t.Errorf("retryServer should have been called twice, instead called %d times", rs.called)
	}
	rs.called = 0

	// The options.NoRetry option should be honored by client.Call, so the following
	// call should fail.
	if err := client.Call(ctx, name, "TryAgain", nil, nil, options.NoRetry{}); err == nil {
		t.Errorf("client.Call(..., options.NoRetry{}) should have failed")
	}
	// Ensure that client.Call did not retry the call.
	if rs.called != 1 {
		t.Errorf("retryServer have been called once, instead called %d times", rs.called)
	}
}
