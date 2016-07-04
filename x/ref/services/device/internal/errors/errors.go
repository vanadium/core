// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO(caprita): Consider moving these to v23 if they're meant to be public
// beyond the deviced and device tool implementations.

// Package errors defines the error ids that are shared between server and
// client-side.
package errors

import "v.io/v23/verror"

// TODO(caprita): the value of pkgPath corresponds to the previous package where
// the error ids were defined.  Updating error ids needs to be carefully
// coordinated between clients and servers, so we should do it when we settle on
// the final location for these error definitions.
const pkgPath = "v.io/x/ref/services/device/internal/impl"

var (
	ErrInvalidSuffix        = verror.Register(pkgPath+".InvalidSuffix", verror.NoRetry, "{1:}{2:} invalid suffix{:_}")
	ErrOperationFailed      = verror.Register(pkgPath+".OperationFailed", verror.NoRetry, "{1:}{2:} operation failed{:_}")
	ErrOperationInProgress  = verror.Register(pkgPath+".OperationInProgress", verror.NoRetry, "{1:}{2:} operation in progress{:_}")
	ErrAppTitleMismatch     = verror.Register(pkgPath+".AppTitleMismatch", verror.NoRetry, "{1:}{2:} app title mismatch{:_}")
	ErrUpdateNoOp           = verror.Register(pkgPath+".UpdateNoOp", verror.NoRetry, "{1:}{2:} update is no op{:_}")
	ErrInvalidOperation     = verror.Register(pkgPath+".InvalidOperation", verror.NoRetry, "{1:}{2:} invalid operation{:_}")
	ErrInvalidBlessing      = verror.Register(pkgPath+".InvalidBlessing", verror.NoRetry, "{1:}{2:} invalid blessing{:_}")
	ErrInvalidPairingToken  = verror.Register(pkgPath+".InvalidPairingToken", verror.NoRetry, "{1:}{2:} pairing token mismatch{:_}")
	ErrUnclaimedDevice      = verror.Register(pkgPath+".UnclaimedDevice", verror.NoRetry, "{1:}{2:} device needs to be claimed first")
	ErrDeviceAlreadyClaimed = verror.Register(pkgPath+".AlreadyClaimed", verror.NoRetry, "{1:}{2:} device has already been claimed")
)
