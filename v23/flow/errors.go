// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flow

import "v.io/v23/verror"

// Implementors of the flow interfaces guarantee to return one of the verror
// codes defined here.
// These were previously defined in errors.vdl using the ID values
// below rather than the more conventional Err<Name>.
var (
	ErrAuth          = verror.NewID("v.io/v23/flow.Auth")
	ErrNotTrusted    = verror.NewID("v.io/v23/flow.NotTrusted")
	ErrNetwork       = verror.NewID("v.io/v23/flow.Network")
	ErrDialFailed    = verror.NewID("v.io/v23/flow.DialFailed")
	ErrResolveFailed = verror.NewID("v.io/v23/flow.ResolveFailed")
	ErrProxy         = verror.NewID("v.io/v23/flow.Proxy")
	ErrBadArg        = verror.NewID("v.io/v23/flow.BadArg")
	ErrBadState      = verror.NewID("v.io/v23/flow.BadState")
	ErrAborted       = verror.NewID("v.io/v23/flow.Aborted")
)
