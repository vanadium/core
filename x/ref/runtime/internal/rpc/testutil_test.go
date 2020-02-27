// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/vdl"
	"v.io/v23/vtrace"
	"v.io/x/ref/lib/flags"
	ivtrace "v.io/x/ref/runtime/internal/vtrace"
	"v.io/x/ref/test"
)

func initForTest() (*context.T, v23.Shutdown) {
	ctx, shutdown := test.V23Init()
	ctx, err := ivtrace.Init(ctx, flags.VtraceFlags{})
	if err != nil {
		panic(err)
	}
	ctx, _ = vtrace.WithNewTrace(ctx)
	return ctx, shutdown
}

// mockCall implements security.Call
type mockCall struct {
	p        security.Principal
	l, r     security.Blessings
	m        string
	ld, rd   security.Discharge
	lep, rep naming.Endpoint
}

var _ security.Call = (*mockCall)(nil)

func (c *mockCall) Timestamp() (t time.Time) { return }
func (c *mockCall) Method() string           { return c.m }
func (c *mockCall) MethodTags() []*vdl.Value { return nil }
func (c *mockCall) Suffix() string           { return "" }
func (c *mockCall) LocalDischarges() map[string]security.Discharge {
	return map[string]security.Discharge{c.ld.ID(): c.ld}
}
func (c *mockCall) RemoteDischarges() map[string]security.Discharge {
	return map[string]security.Discharge{c.rd.ID(): c.rd}
}
func (c *mockCall) LocalEndpoint() naming.Endpoint      { return c.lep }
func (c *mockCall) RemoteEndpoint() naming.Endpoint     { return c.rep }
func (c *mockCall) LocalPrincipal() security.Principal  { return c.p }
func (c *mockCall) LocalBlessings() security.Blessings  { return c.l }
func (c *mockCall) RemoteBlessings() security.Blessings { return c.r }
