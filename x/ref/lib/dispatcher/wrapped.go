// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dispatcher

import (
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
)

// DispatcherWrapper is used when a dispatcher can't be constructed at server
// creation time.  The most common use for this is when the dispatcher needs
// to know some information about the server to be constructed.  For example
// it is sometimes helpful to know the server's endpoints.
// In such cases you can construct a DispatcherWrapper which will simply block
// all lookups until the real dispatcher is set with SetDispatcher.
//nolint:golint // API change required.
type DispatcherWrapper struct {
	wrapped      rpc.Dispatcher
	wrappedIsSet chan struct{}
}

// Lookup will wait until SetDispatcher is called and then simply forward requests
// to the underlying dispatcher.
func (w *DispatcherWrapper) Lookup(ctx *context.T, suffix string) (interface{}, security.Authorizer, error) {
	<-w.wrappedIsSet
	return w.wrapped.Lookup(ctx, suffix)
}

// SetDispatcher sets the underlying dispatcher and allows Lookups to proceed.
func (w *DispatcherWrapper) SetDispatcher(d rpc.Dispatcher) {
	w.wrapped = d
	close(w.wrappedIsSet)
}

// NewDispatcherWrapper creates a new DispatcherWrapper.
func NewDispatcherWrapper() *DispatcherWrapper {
	return &DispatcherWrapper{
		wrappedIsSet: make(chan struct{}),
	}
}
