// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"v.io/v23/security"
	"v.io/v23/security/access"
)

func NewAuthorizer(fname string) security.Authorizer {
	a, err := access.PermissionsAuthorizerFromFile(fname, access.TypicalTagType())
	if err != nil {
		panic(err)
	}
	return a
}
