// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package appd

import (
	"testing"

	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/services/application"

	"v.io/x/ref/services/binary/tidy/binaryd"
	"v.io/x/ref/services/internal/servicetest"
)

type mockAppdInvoker struct {
	binaryd.MockBinarydInvoker
}

type MatchStimulus struct {
	Name     string
	Suffix   string
	Profiles []string
}

type MatchResult struct {
	Env application.Envelope
	Err error
}

func (mdi *mockAppdInvoker) Match(ctx *context.T, _ rpc.ServerCall, profiles []string) (application.Envelope, error) {
	ir := mdi.Tape.Record(MatchStimulus{"Match", mdi.Suffix, profiles})
	r := ir.(MatchResult)
	return r.Env, r.Err
}

func (mdi *mockAppdInvoker) TidyNow(ctx *context.T, _ rpc.ServerCall) error {
	return mdi.SimpleCore("TidyNow", "TidyNow")
}

type dispatcher struct {
	tape *servicetest.Tape
	t    *testing.T
}

func NewDispatcher(t *testing.T, tape *servicetest.Tape) rpc.Dispatcher {
	return &dispatcher{tape: tape, t: t}
}

func (d *dispatcher) Lookup(p *context.T, suffix string) (interface{}, security.Authorizer, error) {
	return &mockAppdInvoker{binaryd.NewMockBinarydInvoker(suffix, d.tape, d.t)}, nil, nil
}
