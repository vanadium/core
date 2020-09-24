// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testutil

import (
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/verror"
)

// LeafDispatcher returns a dispatcher for a single object obj, using
// ReflectInvokerOrDie to invoke methods. Lookup only succeeds on the empty
// suffix.  The provided auth is returned for successful lookups.
func LeafDispatcher(obj interface{}, auth security.Authorizer) rpc.Dispatcher {
	return &leafDispatcher{rpc.ReflectInvokerOrDie(obj), auth}
}

type leafDispatcher struct {
	invoker rpc.Invoker
	auth    security.Authorizer
}

func (d leafDispatcher) Lookup(_ *context.T, suffix string) (interface{}, security.Authorizer, error) {
	if suffix != "" {
		return nil, nil, verror.ErrUnknownSuffix.Errorf(nil, "suffix does not exist: %v", suffix)
	}
	return d.invoker, d.auth, nil
}
