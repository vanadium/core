// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase

import (
	"sync"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/vom"
)

type resultStream struct {
	// cancel cancels the RPC resultStream.
	cancel context.CancelFunc

	mu sync.Mutex
	// call is the RPC resultStream object.
	call wire.DatabaseExecClientCall
	// curr is the currently staged result, or nil if nothing is staged.
	curr []*vom.RawBytes
	// err is the first error encountered during streaming. It may also be
	// populated by a call to Cancel.
	err error
	// finished records whether we have called call.Finish().
	finished bool
}

var _ ResultStream = (*resultStream)(nil)

func newResultStream(cancel context.CancelFunc, call wire.DatabaseExecClientCall) *resultStream {
	return &resultStream{
		cancel: cancel,
		call:   call,
	}
}

func (rs *resultStream) Advance() bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if rs.finished {
		return false
	}
	// Advance never blocks if the context has been cancelled.
	if !rs.call.RecvStream().Advance() {
		// TODO(ivanpi): Query tests rely on simpler error formatting in RecvStream
		// error vs Finish, so keep the redundant error assignment for now.
		rs.err = rs.call.RecvStream().Err()
		err := rs.call.Finish()
		if rs.err == nil {
			rs.err = err
		}
		rs.cancel()
		rs.finished = true
		return false
	}
	rs.curr = rs.call.RecvStream().Value()
	return true
}

func (rs *resultStream) ResultCount() int {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return len(rs.curr)
}

func (rs *resultStream) Result(i int, value interface{}) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.curr[i].ToValue(value)
}

func (rs *resultStream) Err() error {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.err
}

func (rs *resultStream) Cancel() {
	rs.cancel()
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if !rs.finished {
		// TODO(ivanpi): Query tests rely on simpler error formatting in RecvStream
		// error vs Finish, so keep the redundant error assignment for now.
		rs.err = rs.call.RecvStream().Err()
		err := rs.call.Finish()
		if rs.err == nil {
			rs.err = err
		}
		rs.finished = true
	}
}
