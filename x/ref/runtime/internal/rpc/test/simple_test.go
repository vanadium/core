// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"io"
	"testing"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/x/ref/test"
)

type simple struct {
	done <-chan struct{}
}

func (s *simple) Sleep(*context.T, rpc.ServerCall) error {
	select {
	case <-s.done:
	case <-time.After(time.Hour):
	}
	return nil
}

func (s *simple) Ping(_ *context.T, _ rpc.ServerCall) (string, error) {
	return "pong", nil
}

func (s *simple) Echo(_ *context.T, _ rpc.ServerCall, arg string) (string, error) {
	return arg, nil
}

func (s *simple) Source(_ *context.T, call rpc.StreamServerCall, start int) error {
	i := start
	backoff := 25 * time.Millisecond
	for {
		select {
		case <-s.done:
			return nil
		case <-time.After(backoff):
			call.Send(i)
			i++
		}
		backoff *= 2
	}
}

func (s *simple) Sink(_ *context.T, call rpc.StreamServerCall) (int, error) {
	i := 0
	for {
		if err := call.Recv(&i); err != nil {
			if err == io.EOF {
				return i, nil
			}
			return 0, err
		}
	}
}

func (s *simple) Inc(_ *context.T, call rpc.StreamServerCall, inc int) (int, error) {
	i := 0
	for {
		if err := call.Recv(&i); err != nil {
			if err == io.EOF {
				return i, nil
			}
			return 0, err
		}
		call.Send(i + inc)
	}
}

func startSimpleServer(t *testing.T, ctx *context.T) (string, func()) {
	done := make(chan struct{})
	ctx, server, err := v23.WithNewServer(ctx, "", &simple{done}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	return server.Status().Endpoints[0].Name(), func() { close(done) }
}

func TestSimpleRPC(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	name, fn := startSimpleServer(t, ctx)
	defer fn()

	client := v23.GetClient(ctx)
	call, err := client.StartCall(ctx, name, "Ping", nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	response := ""
	if err := call.Finish(&response); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if got, want := response, "pong"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSimpleStreaming(t *testing.T) {
	ctx, shutdown := test.V23InitWithMounttable()
	defer shutdown()
	name, fn := startSimpleServer(t, ctx)
	defer fn()

	inc := 1
	call, err := v23.GetClient(ctx).StartCall(ctx, name, "Inc", []interface{}{inc})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	want := 10
	for i := 0; i <= want; i++ {
		if err := call.Send(i); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		got := -1
		if err = call.Recv(&got); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if want := i + inc; got != want {
			t.Fatalf("got %d, want %d", got, want)
		}
	}
	call.CloseSend()
	final := -1
	err = call.Finish(&final)
	if err != nil {
		t.Errorf("unexpected error: %#v", err)
	}
	if got := final; got != want {
		t.Fatalf("got %d, want %d", got, want)
	}
}
