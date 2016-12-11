// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package securityflag implements utilities for creating security objects based
// on flags.
package securityflag

import (
	"bytes"
	"flag"
	"os"

	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/verror"
	"v.io/x/ref/lib/flags"
)

const pkgPath = "v.io/x/ref/lib/security/securityflag"

var (
	errCantOpenPermissionsFile = verror.Register(pkgPath+".errCantOpenPermissionsFile", verror.NoRetry, "{1:}{2:} cannot open argument to --v23.permissions.file {3}{:_}")
)

var authFlags *flags.Flags

func init() {
	authFlags = flags.CreateAndRegister(flag.CommandLine, flags.Permissions)
}

// NewAuthorizerOrDie constructs an Authorizer based on the provided
// "--v23.permissions.literal" or "--v23.permissions.file" flags. Otherwise it
// creates a default Authorizer.
func NewAuthorizerOrDie() security.Authorizer {
	flags := authFlags.PermissionsFlags()
	fname := flags.PermissionsFile("runtime")
	literal := flags.PermissionsLiteral()

	if fname == "" && literal == "" {
		return nil
	}
	var a security.Authorizer
	var err error
	if literal == "" {
		a, err = access.PermissionsAuthorizerFromFile(fname, access.TypicalTagType())
	} else {
		var perms access.Permissions
		if perms, err = access.ReadPermissions(bytes.NewBufferString(literal)); err == nil {
			a = access.TypicalTagTypePermissionsAuthorizer(perms)
		}
	}
	if err != nil {
		panic(err)
	}
	return a
}

// TODO(rjkroege): Refactor these two functions into one by making an Authorizer
// use a Permissions accessor interface.
// PermissionsFromFlag reads the same flags as NewAuthorizerOrDie but produces a
// Permissions for callers that need more control of how Permissions are
// managed.
func PermissionsFromFlag() (access.Permissions, error) {
	flags := authFlags.PermissionsFlags()
	fname := flags.PermissionsFile("runtime")
	literal := flags.PermissionsLiteral()

	if fname == "" && literal == "" {
		return nil, nil
	}

	if literal == "" {
		file, err := os.Open(fname)
		if err != nil {
			return nil, verror.New(errCantOpenPermissionsFile, nil, fname)
		}
		defer file.Close()
		return access.ReadPermissions(file)
	} else {
		return access.ReadPermissions(bytes.NewBufferString(literal))
	}
}
