// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flow

import (
	"strings"

	"v.io/v23/context"
	"v.io/v23/verror"
)

var noWrapPackages = []string{
	"v.io/v23/verror",
	"v.io/v23/flow",
	// TODO(mattr): We want to pass on the peerAuthorizedFailed error
	// from the peerAuthorizer passed to us from the client.  The
	// client detects that specific error to determine which error
	// to return to users.  We should probably have another way
	// to prevent this excessive wrapping, but this works for now.
	"v.io/x/ref/runtime/internal/rpc",
}

func MaybeWrapError(idAction verror.IDAction, ctx *context.T, err error) error {
	if !shouldWrap(err) {
		return err
	}
	return idAction.Errorf(ctx, "%v", err)
}

func shouldWrap(err error) bool {
	if err == nil || !verror.IsAny(err) {
		return true
	}
	id := verror.ErrorID(err)
	for _, pkg := range noWrapPackages {
		if strings.HasPrefix(string(id), pkg) {
			return false
		}
	}
	return true
}
