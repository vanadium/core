// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build mojo

// The evolving story of identification and security in syncbase
// running in mojo:
// (Last Updated: November 5, 2015).
//
// SHORT STORY:
// The syncbase mojo service gets a blessing by exchanging an OAuth2 token for
// a blessing. This allows syncbases to identify each other when synchronizing
// data between each other and enables basic security policies like "all my
// devices sync all my data with each other". However, different mojo "clients"
// (that communicate with the local syncbase via mojo IPC) are not isolated
// from each other - so each mojo app has complete access to all the data
// written to the local syncbase service (modulo any filesystem isolation via
// the --root-dir flag). This is something that we intend to change.
//
// LONGER VERSION:
// A "mojo app" consists of a set of interrelated components, accessed by a
// URL. Communication between these components is via mojo IPC. One of those
// components is syncbase (this package), which is a store that synchronizes
// between devices via Vanadium RPCs. See:
// https://docs.google.com/a/google.com/document/d/1TyxPYIhj9VBCtY7eAXu_MEV9y0dtRx7n7UY4jm76Qq4/edit?usp=sharing
//
// Any process communicating via Vanadium RPCs needs to have an identity
// represented as a Blessing
// (https://vanadium.github.io/concepts/security.html).
//
// So, the current story is this:
// 1 The mojo syncbase app/service fetches an OAuth2 token from the
//   mojo authentication service
// 2 It exchanges this token for a blessing bound to the public
//   key of the principal associated with this service
//   (this is essentially a copy of what the mojo principal service
//   implementation does)
// 3 And all RPCs use the blessing obtained above
// 4 Other mojo components communicate with their local syncbase via
//   Mojo IPC (see v.io/x/ref/services/syncbase/server/mojo_call.go)
//   and DO NOT AUTHENTICATE with it
// 5 The resulting security model is that all mojo apps on the same
//   device have access to whatever is stored in the local syncbase
//
// Clearly (5) is not ideal. And as this evolves, the plan is likely to be
// something involving each "client" of this syncbase mojo service being
// identified using the principal service
// (https://github.com/domokit/mojo/blob/master/mojo/services/vanadium/security/interfaces/principal.mojom).
// But more on that when we get there.

package bridge_mojo

import (
	"fmt"
	"runtime"

	"mojo/public/go/application"
	"mojo/public/go/bindings"
	"mojo/services/authentication/interfaces/authentication"

	"v.io/v23/context"
	"v.io/x/ref/services/syncbase/bridge"
)

type selectAccountFailed struct {
	error
}

func SeekAndSetBlessings(v23ctx *context.T, appctx application.Context) error {
	// Get an OAuth2 token from the mojo authentication service.
	// At the time of this writing, the mojo authentication
	// service was implemented only for Android, so in absence
	// of that abort. Which means that the syncbases cannot
	// communicate with each other, unless --v23.credentials was
	// supplied as a flag.
	if runtime.GOOS != "android" {
		v23ctx.Infof("Using default blessings for non-Android OS (%v)", runtime.GOOS)
		return nil
	}
	// Get an OAuth2 token from the mojo authentication service.
	// TODO(ashankar,ataly): This is almost a duplicate of:
	// https://github.com/domokit/mojo/blob/master/services/vanadium/security/principal_service.go
	// At some point this needs to be cleared up - the syncbase mojo service
	// should talk to the principal mojo service.
	token, err := oauthToken(appctx)
	if err != nil {
		if _, ok := err.(selectAccountFailed); ok {
			// TODO(sadovsky): This behavior is convenient for development, but
			// probably ought to be disabled in production.
			v23ctx.Infof("SelectAccount failed; using default blessings")
			return nil
		}
		return err
	}
	ctx.Infof("Obtained OAuth2 token, will exchange for blessings")
	return bridge.SeekAndSetBlessings(v23ctx, "google", token)
}

func oauthToken(ctx application.Context) (string, error) {
	req, ptr := authentication.CreateMessagePipeForAuthenticationService()
	ctx.ConnectToApplication("mojo:authentication").ConnectToService(&req)
	proxy := authentication.NewAuthenticationServiceProxy(ptr, bindings.GetAsyncWaiter())
	name, errstr, _ := proxy.SelectAccount(true /*return last selected*/)
	if name == nil || errstr != nil {
		return "", selectAccountFailed{error: fmt.Errorf("failed to select an account for user: %v", errstr)}
	}
	token, errstr, _ := proxy.GetOAuth2Token(*name, []string{"email"})
	if token == nil || errstr != nil {
		return "", fmt.Errorf("failed to obtain an OAuth2 token for %q: %v", *name, errstr)
	}
	return *token, nil
}
