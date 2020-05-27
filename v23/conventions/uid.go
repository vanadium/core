// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package conventions implements unenforced conventions for Vanadium.
package conventions

import (
	"reflect"
	"strings"

	"v.io/v23/context"
	"v.io/v23/security"
)

// Special case user ids.  They all have the identity provider "self" meaning that the server is poviding
// the identity and type "?" meaning that we don't know what type of Id this is.
const (
	ServerUser          = "self:?:ServerUser"      // a client that has our public key
	UnauthenticatedUser = "self:?:Unauthenticated" // a client which presents no blessing we trust
)

// GetClientUserIds returns a slice of ids for the client.  Each Id is one of the special ones above or
// the string <identity provider>:<single letter type>:<actor>.   Examples of types are "u" for user
// and "r" for role and "?" for unknown.
func GetClientUserIds(ctx *context.T, call security.Call) []string {
	// If there is no call or context, we must be the user.
	if ctx == nil || call == nil {
		return []string{ServerUser}
	}
	// The convention is: the first 3 components of a blessing name are a user name
	// if the second component is a single character.  Otherwise, use just the first
	// component.
	var ids []string
	rbn, _ := security.RemoteBlessingNames(ctx, call)
	for _, b := range rbn {
		if c := ParseUserId(b); c != nil {
			ids = append(ids, strings.Join(c, security.ChainSeparator))
		}
	}
	// If the client has our public key, we assume identity.
	if l, r := call.LocalBlessings().PublicKey(), call.RemoteBlessings().PublicKey(); l != nil && reflect.DeepEqual(l, r) {
		ids = append(ids, ServerUser)
	}
	if len(ids) > 0 {
		return ids
	}
	return []string{UnauthenticatedUser}
}

// Parse the userId components from a blessing name or a userId string.  Returns nil on failure.
//nolint:golint // API change required.
func ParseUserId(s string) []string {
	c := strings.Split(s, security.ChainSeparator)
	if len(c) >= 3 && len(c[1]) == 1 {
		// Identity provider conforms to conventions.
		return c[0:3]
	} else if len(c) >= 1 {
		// Identity provider doesn't conform.  Treat all his users as a single resource group.
		//
		// This has the side effect of making tests a bit easier to write since you can use
		// "bob" and "alice" rather than "self:u:bob" and "self:u:alice" or some such.
		return c[0:1]
	}
	return nil
}
