// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vtrace_test

import (
	"strings"
	"testing"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/vtrace"
	"v.io/x/ref/test/testutil"
)

func (c *testServer) Log(ctx *context.T, _ rpc.ServerCall) error {
	ctx.VI(1).Info("logging")
	return nil
}

// TestLogging tests to make sure that ctx.Infof comments are added to the trace
func TestLogging(t *testing.T) {
	ctx, shutdown, _ := initForTest(t)
	defer shutdown()

	ctx, _ = vtrace.WithNewTrace(ctx)
	vtrace.ForceCollect(ctx, 0)
	ctx, span := vtrace.WithNewSpan(ctx, "foo")
	ctx.Info("logging ", "from ", "info")
	ctx.Infof("logging from %s", "infof")
	ctx.InfoDepth(0, "logging from info depth")

	ctx.Error("logging ", "from ", "error")
	ctx.Errorf("logging from %s", "errorf")
	ctx.ErrorDepth(0, "logging from error depth")

	span.Finish()
	record := vtrace.GetStore(ctx).TraceRecord(span.Trace())
	messages := []string{
		"vtrace_logging_test.go:31] logging from info",
		"vtrace_logging_test.go:32] logging from infof",
		"vtrace_logging_test.go:33] logging from info depth",
		"vtrace_logging_test.go:35] logging from error",
		"vtrace_logging_test.go:36] logging from errorf",
		"vtrace_logging_test.go:37] logging from error depth",
	}
	expectSequence(t, *record, []string{
		"foo: " + strings.Join(messages, ", "),
	})
}

func startLoggingServer(ctx *context.T, idp *testutil.IDProvider) error {
	principal := testutil.NewPrincipal()
	if err := idp.Bless(principal, "server"); err != nil {
		return err
	}
	_, _, err := makeTestServer(ctx, principal, "logger")
	if err != nil {
		return err
	}
	// Make sure the server is mounted to avoid any retries in when StartCall
	// is invoked in runCallChain which complicate the span comparisons.
	verifyMount(ctx, "logger") //nolint:errcheck
	return nil
}

func runLoggingCall(ctx *context.T) (*vtrace.TraceRecord, error) {
	ctx, span := vtrace.WithNewSpan(ctx, "logging")
	call, err := v23.GetClient(ctx).StartCall(ctx, "logger", "Log", nil)
	if err != nil {
		return nil, err
	}
	if err := call.Finish(); err != nil {
		return nil, err
	}
	span.Finish()

	return vtrace.GetStore(ctx).TraceRecord(span.Trace()), nil
}

func TestVIRPCWithNoLogging(t *testing.T) {
	ctx, shutdown, idp := initForTest(t)
	defer shutdown()

	if err := startLoggingServer(ctx, idp); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	vtrace.ForceCollect(ctx, 0)
	record, err := runLoggingCall(ctx)

	if err != nil {
		t.Fatalf("logging call failed: %v", err)
	}
	expectSequence(t, *record, []string{
		"logging",
		"<rpc.Client>\"logger\".Log",
		"\"\".Log",
	})
}

func TestVIRPCWithLogging(t *testing.T) {
	ctx, shutdown, idp := initForTest(t)
	defer shutdown()

	if err := startLoggingServer(ctx, idp); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	vtrace.ForceCollect(ctx, 1)
	record, err := runLoggingCall(ctx)

	if err != nil {
		t.Fatalf("logging call failed: %v", err)
	}
	expectedSpanRegex := "\"\".Log: vtrace_logging_test.go:19] [a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}logging"
	expectSequence(t, *record, []string{expectedSpanRegex})
}
