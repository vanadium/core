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
	ErrAuth          = verror.NewID("Auth")
	ErrNotTrusted    = verror.NewID("NotTrusted")
	ErrNetwork       = verror.NewID("Network")
	ErrDialFailed    = verror.NewID("DialFailed")
	ErrResolveFailed = verror.NewID("ResolveFailed")
	ErrProxy         = verror.NewID("Proxy")
	ErrBadArg        = verror.NewID("BadArg")
	ErrBadState      = verror.NewID("BadState")
	ErrAborted       = verror.NewID("Aborted")
)
