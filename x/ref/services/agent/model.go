// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package agent

import (
	"crypto/sha512"
	"io"

	"v.io/v23/security"
)

type Principal interface {
	security.Principal
	io.Closer
}

const PrincipalHandleByteSize = sha512.Size
