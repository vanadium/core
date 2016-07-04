// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pathperms

import (
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/security/access"
)

// hierarchicalAuthorizer contains the state needed to implement
// hierarchical authorization in the Authorize method.
type hierarchicalAuthorizer struct {
	rootDir, childDir string
	get               PermsGetter
}

// PermsGetter defines an abstract interface that a customer of
// NewHierarchicalAuthorizer can use to obtain the PermissionsAuthorizer
// instances that it needs to construct a hierarchicalAuthorizer.
type PermsGetter interface {
	// PermsForPath has two successful outcomes: either returning a valid
	// Permissions object or a boolean status true indicating that the
	// Permissions object is intentionally not present. Finally, it returns an
	// error if anything has gone wrong.
	PermsForPath(ctx *context.T, path string) (access.Permissions, bool, error)
}

// NewHierarchicalAuthorizer creates a new hierarchicalAuthorizer: one
// that implements a "root" like concept: admin rights at the root of
// a server can invoke RPCs regardless of permissions set on child objects.
//
// If the root permissions are not set, the authorizer behaves like the
// DefaultAuthorizer.
//
// If the child permissions are not set, the authorizer uses the permissions set
// on the root to restrict access to the child (including the admin override
// described above).
func NewHierarchicalAuthorizer(rootDir, childDir string, get PermsGetter) (security.Authorizer, error) {
	return &hierarchicalAuthorizer{
		rootDir:  rootDir,
		childDir: childDir,
		get:      get,
	}, nil
}

func (ha *hierarchicalAuthorizer) Authorize(ctx *context.T, call security.Call) error {
	rootPerms, intentionallyEmpty, err := ha.get.PermsForPath(ctx, ha.rootDir)
	if err != nil {
		return err
	} else if intentionallyEmpty {
		ctx.VI(2).Infof("PermsForPath(%s) is intentionally empty", ha.rootDir)
		return security.DefaultAuthorizer().Authorize(ctx, call)
	}

	// We are at the root so exit early.
	if ha.rootDir == ha.childDir {
		return adminCheckAuth(ctx, call, access.TypicalTagTypePermissionsAuthorizer(rootPerms), rootPerms)
	}

	// This is not fatal: the childDir may not exist if we are invoking
	// a method creating the object, so we only use the root Permissions.
	childPerms, intentionallyEmpty, err := ha.get.PermsForPath(ctx, ha.childDir)
	if err != nil {
		return err
	} else if intentionallyEmpty {
		return adminCheckAuth(ctx, call, access.TypicalTagTypePermissionsAuthorizer(rootPerms), rootPerms)
	}

	return adminCheckAuth(ctx, call, access.TypicalTagTypePermissionsAuthorizer(childPerms), rootPerms)
}

func adminCheckAuth(ctx *context.T, call security.Call, auth security.Authorizer, perms access.Permissions) error {
	err := auth.Authorize(ctx, call)
	if err == nil {
		return nil
	}

	// Maybe the invoking principal can invoke this method because
	// it has Admin permissions.
	names, _ := security.RemoteBlessingNames(ctx, call)
	if len(names) > 0 && perms[string(access.Admin)].Includes(names...) {
		return nil
	}

	return err
}
