// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"time"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
)

// PreferredProtocols instructs the Runtime implementation to select
// endpoints with the specified protocols when a Client makes a call
// and to order them in the specified order.
type PreferredProtocols []string

func (PreferredProtocols) RPCClientOpt() {
}

// This option is used to sort and filter the endpoints when resolving the
// proxy name from a mounttable.
type PreferredServerResolveProtocols []string

func (PreferredServerResolveProtocols) RPCServerOpt() {
}

// ReservedNameDispatcher specifies the dispatcher that controls access
// to framework managed portion of the namespace.
type ReservedNameDispatcher struct {
	Dispatcher rpc.Dispatcher
}

func (ReservedNameDispatcher) RPCServerOpt() {
}

// IdleConnectionExpiry is the amount of the time after which the connection cache
// will close idle connections.
type IdleConnectionExpiry time.Duration

func (IdleConnectionExpiry) RPCClientOpt() {
}
func (IdleConnectionExpiry) RPCServerOpt() {
}

type connectionOpts struct {
	connDeadline   time.Time
	channelTimeout time.Duration
	useOnlyCached  bool
	noRetry        bool
}

func getConnectionOptions(ctx *context.T, opts []rpc.CallOpt) *connectionOpts {
	now := time.Now()
	var copts connectionOpts
	for _, o := range opts {
		switch t := o.(type) {
		case options.ConnectionTimeout:
			if dur := time.Duration(t); dur == 0 {
				copts.useOnlyCached = true
			} else if dl := now.Add(dur); dl.Before(copts.connDeadline) || copts.connDeadline.IsZero() {
				// Use the minimum of all the timeouts passed in.
				copts.connDeadline = dl

			}
		case options.ChannelTimeout:
			// Use the minimum channel timeout.
			if dur := time.Duration(t); dur < copts.channelTimeout || copts.channelTimeout == 0 {
				copts.channelTimeout = time.Duration(t)
			}
		case options.NoRetry:
			copts.noRetry = true
		}
	}
	// If the context deadline is sooner than connection deadline, use it instead.
	if dl, hasDl := ctx.Deadline(); hasDl && (dl.Before(copts.connDeadline) || copts.connDeadline.IsZero()) {
		copts.connDeadline = dl
	}
	// If no deadline has been set yet, use the default call timeout.
	if copts.connDeadline.IsZero() {
		copts.connDeadline = now.Add(defaultCallTimeout)
	}
	return &copts
}

func getNoNamespaceOpt(opts []rpc.CallOpt) bool {
	for _, o := range opts {
		if _, ok := o.(options.Preresolved); ok {
			return true
		}
	}
	return false
}

func getNamespaceOpts(opts []rpc.CallOpt) (resolveOpts []naming.NamespaceOpt) {
	for _, o := range opts {
		if r, ok := o.(naming.NamespaceOpt); ok {
			resolveOpts = append(resolveOpts, r)
		}
	}
	return
}
