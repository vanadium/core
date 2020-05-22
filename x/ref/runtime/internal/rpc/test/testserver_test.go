// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package test

import (
	"fmt"
	"io"
	"sync"
	"time"

	"v.io/v23/context"
	"v.io/v23/i18n"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/uniqueid"
	"v.io/v23/vtrace"
)

// This file contains a test server and dispatcher which are used by other
// tests, especially those in full_test.
type userType string

type testServer struct{}

func (*testServer) Closure(*context.T, rpc.ServerCall) error {
	return nil
}

func (*testServer) Error(*context.T, rpc.ServerCall) error {
	return errMethod
}

func (*testServer) Echo(_ *context.T, call rpc.ServerCall, arg string) (string, error) {
	return fmt.Sprintf("method:%q,suffix:%q,arg:%q", "Echo", call.Suffix(), arg), nil
}

func (*testServer) EchoUser(_ *context.T, call rpc.ServerCall, arg string, u userType) (string, userType, error) {
	return fmt.Sprintf("method:%q,suffix:%q,arg:%q", "EchoUser", call.Suffix(), arg), u, nil
}

func (*testServer) EchoLang(ctx *context.T, call rpc.ServerCall) (string, error) {
	return string(i18n.GetLangID(ctx)), nil
}

func (*testServer) EchoBlessings(ctx *context.T, call rpc.ServerCall) (server, client string, _ error) {
	local := security.LocalBlessingNames(ctx, call.Security())
	remote, _ := security.RemoteBlessingNames(ctx, call.Security())
	return fmt.Sprintf("%v", local), fmt.Sprintf("%v", remote), nil
}

func (*testServer) EchoGrantedBlessings(_ *context.T, call rpc.ServerCall, arg string) (result, blessing string, _ error) {
	return arg, fmt.Sprintf("%v", call.GrantedBlessings()), nil
}

func (*testServer) EchoAndError(_ *context.T, call rpc.ServerCall, arg string) (string, error) {
	result := fmt.Sprintf("method:%q,suffix:%q,arg:%q", "EchoAndError", call.Suffix(), arg)
	if arg == "error" {
		return result, errMethod
	}
	return result, nil
}

func (*testServer) Stream(_ *context.T, call rpc.StreamServerCall, arg string) (string, error) {
	result := fmt.Sprintf("method:%q,suffix:%q,arg:%q", "Stream", call.Suffix(), arg)
	var u userType
	var err error
	for err = call.Recv(&u); err == nil; err = call.Recv(&u) {
		result += " " + string(u)
		if err := call.Send(u); err != nil {
			return "", err
		}
	}
	if err == io.EOF {
		err = nil
	}
	return result, err
}

func (*testServer) Unauthorized(*context.T, rpc.StreamServerCall) (string, error) {
	return "UnauthorizedResult", nil
}

type testServerAuthorizer struct{}

func (testServerAuthorizer) Authorize(ctx *context.T, call security.Call) error {
	// Verify that the Call object seen by the authorizer
	// has the necessary fields.
	lb := call.LocalBlessings()
	if lb.IsZero() {
		return fmt.Errorf("testServerAuthorzer: Call object %v has no LocalBlessings", call)
	}
	if tpcavs := lb.ThirdPartyCaveats(); len(tpcavs) > 0 && call.LocalDischarges() == nil {
		return fmt.Errorf("testServerAuthorzer: Call object %v has no LocalDischarges even when LocalBlessings have third-party caveats", call)

	}
	if call.LocalPrincipal() == nil {
		return fmt.Errorf("testServerAuthorzer: Call object %v has no LocalPrincipal", call)
	}
	if call.Method() == "" {
		return fmt.Errorf("testServerAuthorzer: Call object %v has no Method", call)
	}
	if call.LocalEndpoint().IsZero() {
		return fmt.Errorf("testServerAuthorzer: Call object %v has no LocalEndpoint", call)
	}
	if call.RemoteEndpoint().IsZero() {
		return fmt.Errorf("testServerAuthorzer: Call object %v has no RemoteEndpoint", call)
	}

	// Do not authorize the method "Unauthorized".
	if call.Method() == "Unauthorized" {
		return fmt.Errorf("testServerAuthorizer denied access")
	}
	return nil
}

type testServerDisp struct{ server interface{} }

func (t testServerDisp) Lookup(_ *context.T, suffix string) (interface{}, security.Authorizer, error) {
	// If suffix is "nilAuth" we use default authorization, if it is "aclAuth" we
	// use an AccessList-based authorizer, and otherwise we use the custom testServerAuthorizer.
	var authorizer security.Authorizer
	switch suffix {
	case "discharger":
		return &dischargeServer{}, testServerAuthorizer{}, nil
	case "nilAuth":
		authorizer = nil
	case "aclAuth":
		authorizer = &access.AccessList{
			In: []security.BlessingPattern{"test-blessing:client", "test-blessing:server"},
		}
	default:
		authorizer = testServerAuthorizer{}
	}
	return t.server, authorizer, nil
}

type dischargeServer struct {
	mu    sync.Mutex
	count int
}

func (ds *dischargeServer) Discharge(ctx *context.T, call rpc.StreamServerCall, cav security.Caveat, _ security.DischargeImpetus) (security.Discharge, error) {
	ds.mu.Lock()
	ds.count++
	ds.mu.Unlock()
	tp := cav.ThirdPartyDetails()
	if tp == nil {
		return security.Discharge{}, fmt.Errorf("discharger: %v does not represent a third-party caveat", cav)
	}
	if err := tp.Dischargeable(ctx, call.Security()); err != nil {
		return security.Discharge{}, fmt.Errorf("third-party caveat %v cannot be discharged for this context: %v", cav, err)
	}
	// Add a fakeTimeCaveat to be able to control discharge expiration via 'clock'.
	expiry, err := security.NewCaveat(fakeTimeCaveat, clock.Now())
	if err != nil {
		return security.Discharge{}, fmt.Errorf("failed to create an expiration on the discharge: %v", err)
	}
	return call.Security().LocalPrincipal().MintDischarge(cav, expiry)
}

// granter implements rpc.Granter.
//
// It returns the specified (security.Blessings, error) pair if either the
// blessing or the error is specified. Otherwise it returns a blessing
// derived from the local blessings of the current call.
type granter struct {
	rpc.CallOpt
	b   security.Blessings
	err error
}

func (g granter) Grant(ctx *context.T, call security.Call) (security.Blessings, error) {
	if !g.b.IsZero() || g.err != nil {
		return g.b, g.err
	}
	return call.LocalPrincipal().Bless(
		call.RemoteBlessings().PublicKey(),
		call.LocalBlessings(),
		"blessed",
		security.UnconstrainedUse())
}

// dischargeTestServer implements the discharge service. Always fails to
// issue a discharge, but records the impetus and traceid of the RPC call.
type dischargeTestServer struct {
	impetus []security.DischargeImpetus
	traceid []uniqueid.Id
}

func (s *dischargeTestServer) Discharge(ctx *context.T, _ rpc.ServerCall, cav security.Caveat, impetus security.DischargeImpetus) (security.Discharge, error) {
	s.impetus = append(s.impetus, impetus)
	s.traceid = append(s.traceid, vtrace.GetSpan(ctx).Trace())
	return security.Discharge{}, fmt.Errorf("discharges not issued")
}

func (s *dischargeTestServer) Release() ([]security.DischargeImpetus, []uniqueid.Id) {
	impetus, traceid := s.impetus, s.traceid
	s.impetus, s.traceid = nil, nil
	return impetus, traceid
}

//nolint:deadcode,unused
type streamRecvInGoroutineServer struct{ c chan error }

func (s *streamRecvInGoroutineServer) RecvInGoroutine(_ *context.T, call rpc.StreamServerCall) error {
	// Spawn a goroutine to read streaming data from the client.
	go func() {
		var i interface{}
		for {
			err := call.Recv(&i)
			if err != nil {
				s.c <- err
				return
			}
		}
	}()
	// Imagine the server did some processing here and now that it is done,
	// it does not care to see what else the client has to say.
	return nil
}

type expiryDischarger struct {
	mu    sync.Mutex
	count int
}

func (ed *expiryDischarger) Discharge(ctx *context.T, call rpc.StreamServerCall, cav security.Caveat, _ security.DischargeImpetus) (security.Discharge, error) {
	tp := cav.ThirdPartyDetails()
	if tp == nil {
		return security.Discharge{}, fmt.Errorf("discharger: %v does not represent a third-party caveat", cav)
	}
	if err := tp.Dischargeable(ctx, call.Security()); err != nil {
		return security.Discharge{}, fmt.Errorf("third-party caveat %v cannot be discharged for this context: %v", cav, err)
	}
	expDur := 10 * time.Millisecond
	expiry, err := security.NewExpiryCaveat(time.Now().Add(expDur))
	if err != nil {
		return security.Discharge{}, fmt.Errorf("failed to create an expiration on the discharge: %v", err)
	}
	d, err := call.Security().LocalPrincipal().MintDischarge(cav, expiry)
	if err != nil {
		return security.Discharge{}, err
	}
	ed.mu.Lock()
	ed.count++
	ed.mu.Unlock()
	return d, nil
}
