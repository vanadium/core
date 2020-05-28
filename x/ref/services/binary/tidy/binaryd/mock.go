// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package binaryd

import (
	"log"
	"testing"

	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/services/binary"
	"v.io/v23/services/repository"

	"v.io/x/ref/services/internal/servicetest"
)

type MockBinarydInvoker struct {
	Suffix string
	Tape   *servicetest.Tape
	t      *testing.T
}

// simpleCore implements the core of all mock methods that take
// arguments and return error.
func (mdi *MockBinarydInvoker) SimpleCore(callRecord interface{}, name string) error {
	ri := mdi.Tape.Record(callRecord)
	switch r := ri.(type) {
	case nil:
		return nil
	case error:
		return r
	}
	log.Fatalf("%s (mock) response %v is of bad type", name, ri)
	return nil
}

type DeleteStimulus struct {
	Op     string
	Suffix string
}

func (mdi *MockBinarydInvoker) Delete(ctx *context.T, _ rpc.ServerCall) error {
	return mdi.SimpleCore(DeleteStimulus{"Delete", mdi.Suffix}, "Delete")
}

type StatStimulus struct {
	Op     string
	Suffix string
}

func (mdi *MockBinarydInvoker) Stat(ctx *context.T, _ rpc.ServerCall) ([]binary.PartInfo, repository.MediaInfo, error) {
	// Only the presence or absence of the error is necessary.
	if err := mdi.SimpleCore(StatStimulus{"Stat", mdi.Suffix}, "Stat"); err != nil {
		return nil, repository.MediaInfo{}, err
	}
	return nil, repository.MediaInfo{}, nil
}

type GlobStimulus struct {
	Pattern string
}

type GlobResponse struct {
	Results []string
	Err     error
}

//nolint:golint // API change required.
func (mdi *MockBinarydInvoker) Glob__(p *context.T, call rpc.GlobServerCall, g *glob.Glob) error {
	gs := GlobStimulus{g.String()}
	gr := mdi.Tape.Record(gs).(GlobResponse)
	for _, r := range gr.Results {
		//nolint:errcheck
		call.SendStream().Send(naming.GlobReplyEntry{Value: naming.MountEntry{Name: r}})
	}
	return gr.Err
}

type dispatcher struct {
	tape *servicetest.Tape
	t    *testing.T
}

func NewDispatcher(t *testing.T, tape *servicetest.Tape) rpc.Dispatcher {
	return &dispatcher{tape: tape, t: t}
}

func NewMockBinarydInvoker(suffix string, tape *servicetest.Tape, t *testing.T) MockBinarydInvoker {
	return MockBinarydInvoker{Suffix: suffix, Tape: tape, t: t}
}

func (d *dispatcher) Lookup(p *context.T, suffix string) (interface{}, security.Authorizer, error) {
	v := NewMockBinarydInvoker(suffix, d.tape, d.t)
	return &v, nil, nil
}
