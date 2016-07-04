// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build mojo

package roaming

import (
	"os"

	"mojo/public/go/application"
)

func init() {
	// musllibc does not sent an Arg[0] when dlopening() a shared library.  We
	// must set Arg[0] manually if it does not exist.
	if len(os.Args) < 1 {
		os.Args = []string{""}
	}
}

// TODO(caprita): Once v.io/i/891 is resolved, this should be handled via the
// runtime factory constructor.

// SetArgs initializes os.Args from the mojo context.  Must be called before
// v23.Init.
func SetArgs(mctx application.Context) {
	// mctx.Args() is a slice that contains the url of this mojo service
	// followed by all arguments passed to the mojo service via the
	// "--args-for" flag. Since the v23 runtime factories parse arguments
	// from os.Args, we must overwrite os.Args with mctx.Args().
	//
	// Note that os.Args must be set before calling internalInit().
	os.Args = mctx.Args()
	if len(os.Args) == 0 {
		// TODO(jhahn): mojo_run doesn't pass the service url when there is
		// no flags. See https://github.com/domokit/mojo/issues/586.
		os.Args = []string{mctx.URL()}
	}
}
