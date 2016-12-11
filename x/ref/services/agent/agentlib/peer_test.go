// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package agentlib

import (
	"v.io/v23/security"
)

func NewUncachedPrincipalX(path string) (security.Principal, error) {
	return newUncachedPrincipalX(path, 0)
}
