// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package common

import (
	"sort"

	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
)

// ValidatePerms does basic sanity checking on the provided perms:
// - Perms can contain only tags in the provided whitelist.
// - At least one admin must be included to avoid permanently losing access.
func ValidatePerms(ctx *context.T, perms access.Permissions, allowTags []access.Tag) error {
	// Perms cannot be empty or nil.
	if len(perms) == 0 {
		return NewErrPermsEmpty(ctx)
	}
	// Perms cannot contain any tags not in the allowTags whitelist.
	allowTagsSet := make(map[string]struct{}, len(allowTags))
	for _, tag := range allowTags {
		allowTagsSet[string(tag)] = struct{}{}
	}
	var disallowedTags []string
	for tag, _ := range perms {
		if _, ok := allowTagsSet[tag]; !ok {
			disallowedTags = append(disallowedTags, tag)
		}
	}
	if len(disallowedTags) > 0 {
		sort.Strings(disallowedTags)
		return NewErrPermsDisallowedTags(ctx, disallowedTags, access.TagStrings(allowTags...))
	}
	// Perms must include at least one Admin.
	// TODO(ivanpi): More sophisticated admin verification, e.g. check that NotIn
	// doesn't blacklist all possible In matches.
	if adminAcl, ok := perms[string(access.Admin)]; !ok || len(adminAcl.In) == 0 {
		return NewErrPermsNoAdmin(ctx)
	}
	// TODO(ivanpi): Check that perms are enforceable? It would make validation
	// context-dependent.
	return nil
}

// CheckImplicitPerms performs an authorization check against the implicit
// permissions derived from the blessing pattern in the Id. It returns the
// generated implicit perms or an authorization error.
// TODO(ivanpi): Change to check against the specific blessing used for signing
// instead of any blessing in call.Security().
func CheckImplicitPerms(ctx *context.T, call rpc.ServerCall, id wire.Id, allowedTags []access.Tag) (access.Permissions, error) {
	implicitPerms := access.Permissions{}.Add(security.BlessingPattern(id.Blessing), access.TagStrings(allowedTags...)...)
	// Note, allowedTags is expected to contain access.Admin.
	if err := implicitPerms[string(access.Admin)].Authorize(ctx, call.Security()); err != nil {
		return nil, verror.New(wire.ErrUnauthorizedCreateId, ctx, id.Blessing, id.Name, err)
	}
	return implicitPerms, nil
}
