// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package store

const EngineForTest = "leveldb"

// TODO(sadovsky): Maybe define verrors for these.
const (
	ErrMsgClosedStore     = "closed store"
	ErrMsgAbortedSnapshot = "aborted snapshot"
	ErrMsgCanceledStream  = "canceled stream"
	ErrMsgCommittedTxn    = "already called commit"
	ErrMsgAbortedTxn      = "already called abort"
	ErrMsgExpiredTxn      = "expired transaction"
)
