// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"v.io/v23/context"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/verror"

	"v.io/x/ref/lib/apilog"
)

var (
	// These errors are intended to be used as arguments to higher
	// level errors and hence {1}{2} is omitted from their format
	// strings to avoid repeating these n-times in the final error
	// message visible to the user.
	errAuthServerNotAllowed = reg(".errAuthServerNotAllowed",
		"server blessings {3} do not match any allowed server patterns {4}{:5}")
	errMultipleAuthorizationPolicies = reg(".errMultipleAuthorizationPolicies", "at most one ServerAuthorizer option can be provided")
)

type serverAuthorizer struct {
	auth         security.Authorizer
	extraPattern security.BlessingPattern
}

// newServerAuthorizer returns a security.Authorizer for authorizing the server
// during a flow. The authorization policy is based on options supplied to the
// call that initiated the flow.
//
// TODO(ashankar): Trace why we have the behavior in the following comment and
// consider removing it. It might be a relic from the early iterations of
// server authorization, but I suspect we can get rid of
// security.SplitPatternName and security.JoinPatternName? If we do, then this pattern argument
// goes away.
// If pattern is non-empty, then in addition, the server's blessing must satisfy the pattern.
//
// This method assumes that canCreateServerAuthorizer(opts) is nil.
func newServerAuthorizer(pattern security.BlessingPattern, opts ...rpc.CallOpt) security.Authorizer {
	if len(pattern) == 0 {
		return authorizerFromOpts(opts...)
	}
	return &serverAuthorizer{
		auth:         authorizerFromOpts(opts...),
		extraPattern: pattern,
	}
}

func authorizerFromOpts(opts ...rpc.CallOpt) security.Authorizer {
	for _, o := range opts {
		if v, ok := o.(options.ServerAuthorizer); ok {
			return v
		}
	}
	return security.EndpointAuthorizer()
}

func (a *serverAuthorizer) Authorize(ctx *context.T, call security.Call) error {
	defer apilog.LogCallf(ctx, "call=")(ctx, "") // gologcop: DO NOT EDIT, MUST BE FIRST STATEMENT
	if err := a.auth.Authorize(ctx, call); err != nil {
		return err
	}
	names, rejected := security.RemoteBlessingNames(ctx, call)
	if !a.extraPattern.MatchedBy(names...) {
		return verror.New(errAuthServerNotAllowed, ctx, names, a.extraPattern, rejected)
	}
	return nil
}
