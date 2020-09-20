// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import "v.io/v23/verror"

// These were previously defined in errors.vdl using the ID values
// below rather than the more conventional Err<Name>.
var (
	ErrMissingSetupOption       = verror.NewID("MissingSetupOption")
	ErrUnexpectedMsg            = verror.NewID("UnexpectedMsg")
	ErrConnectionClosed         = verror.NewID("ConnectionClosed")
	ErrRemoteError              = verror.NewID("RemoteError")
	ErrSend                     = verror.NewID("Send")
	ErrRecv                     = verror.NewID("Recv")
	ErrCounterOverflow          = verror.NewID("CounterOverflow")
	ErrBlessingsFlowClosed      = verror.NewID("BlessingsFlowClosed")
	ErrInvalidChannelBinding    = verror.NewID("InvalidChannelBinding")
	ErrNoPublicKey              = verror.NewID("NoPublicKey")
	ErrDialingNonServer         = verror.NewID("DialingNonServer")
	ErrAcceptorBlessingsMissing = verror.NewID("AcceptorBlessingsMissing")
	ErrDialerBlessingsMissing   = verror.NewID("DialerBlessingsMissing")
	ErrBlessingsNotBound        = verror.NewID("BlessingsNotBound")
	ErrInvalidPeerFlow          = verror.NewID("InvalidPeerFlow")
	ErrChannelTimeout           = verror.NewID("ChannelTimeout")
	ErrCannotDecryptBlessings   = verror.NewID("CannotDecryptBlessings")
	ErrCannotDecryptDischarges  = verror.NewID("CannotDecryptDischarges")
	ErrCannotEncryptBlessings   = verror.NewID("CannotEncryptBlessings")
	ErrCannotEncryptDischarges  = verror.NewID("CannotEncryptDischarges")
	ErrNoCrypter                = verror.NewID("NoCrypter")
	ErrNoPrivateKey             = verror.NewID("NoPrivateKey")
	ErrIdleConnKilled           = verror.NewID("IdleConnKilled")
)
