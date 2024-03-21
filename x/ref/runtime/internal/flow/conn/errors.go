// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import "v.io/v23/verror"

// These were previously defined in errors.vdl using the ID values
// below rather than the more conventional Err<Name>.
var (
	ErrMissingSetupOption       = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.MissingSetupOption")
	ErrUnexpectedMsg            = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.UnexpectedMsg")
	ErrConnectionClosed         = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.ConnectionClosed")
	ErrRemoteError              = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.RemoteError")
	ErrSend                     = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.Send")
	ErrRecv                     = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.Recv")
	ErrCounterOverflow          = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.CounterOverflow")
	ErrBlessingsFlowClosed      = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.BlessingsFlowClosed")
	ErrInvalidChannelBinding    = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.InvalidChannelBinding")
	ErrNoPublicKey              = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.NoPublicKey")
	ErrDialingNonServer         = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.DialingNonServer")
	ErrAcceptorBlessingsMissing = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.AcceptorBlessingsMissing")
	ErrDialerBlessingsMissing   = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.DialerBlessingsMissing")
	ErrBlessingsNotBound        = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.BlessingsNotBound")
	ErrInvalidPeerFlow          = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.InvalidPeerFlow")
	ErrChannelTimeout           = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.ChannelTimeout")
	ErrCannotDecryptBlessings   = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.CannotDecryptBlessings")
	ErrCannotDecryptDischarges  = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.CannotDecryptDischarges")
	ErrCannotEncryptBlessings   = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.CannotEncryptBlessings")
	ErrCannotEncryptDischarges  = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.CannotEncryptDischarges")
	ErrNoCrypter                = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.NoCrypter")
	ErrNoPrivateKey             = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.NoPrivateKey")
	ErrIdleConnKilled           = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.IdleConnKilled")
	ErrRPCVersionMismatch       = verror.NewID("v.io/x/ref/runtime/internal/flow/conn.RPCVersionMismatch")
)
