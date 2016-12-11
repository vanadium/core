// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This is WIP to implement and test algorithms that use acls that
// involve groups. When we migrate this to v23/security/access, most
// of this code might not be needed.

package groups

import (
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/vdl"
	"v.io/v23/verror"
)

// Mostly copied code from security/access.
const pkgPath = "v.io/v23/services/groups"

var (
	errTagNeedsString     = verror.Register(pkgPath+".errTagNeedsString", verror.NoRetry, "{1:}{2:}tag type({3}) must be backed by a string not {4}{:_}")
	errNoMethodTags       = verror.Register(pkgPath+".errNoMethodTags", verror.NoRetry, "{1:}{2:}PermissionsAuthorizer.Authorize called on {3}.{4}, which has no tags of type {5}; this is likely unintentional{:_}")
	errMultipleMethodTags = verror.Register(pkgPath+".errMultipleMethodTags", verror.NoRetry, "{1:}{2:}PermissionsAuthorizer on {3}.{4} cannot handle multiple tags of type {5} ({6}); this is likely unintentional{:_}")
)

func PermissionsAuthorizer(perms access.Permissions, tagType *vdl.Type) (security.Authorizer, error) {
	if tagType.Kind() != vdl.String {
		return nil, errTagType(tagType)
	}
	return &authorizer{perms, tagType}, nil
}

func errTagType(tt *vdl.Type) error {
	return verror.New(errTagNeedsString, nil, verror.New(errTagNeedsString, nil, tt, tt.Kind()))
}

type authorizer struct {
	perms   access.Permissions
	tagType *vdl.Type
}

func (a *authorizer) Authorize(ctx *context.T, call security.Call) error {
	blessings, invalid := security.RemoteBlessingNames(ctx, call)
	hasTag := false
	for _, tag := range call.MethodTags() {
		if tag.Type() == a.tagType {
			if hasTag {
				return verror.New(errMultipleMethodTags, ctx, call.Suffix(), call.Method(), a.tagType, call.MethodTags())
			}
			hasTag = true
			if acl, exists := a.perms[tag.RawString()]; !exists || !includes(ctx, acl, convertToSet(blessings...)) {
				return access.NewErrNoPermissions(ctx, blessings, invalid, tag.RawString())
			}
		}
	}
	if !hasTag {
		return verror.New(errNoMethodTags, ctx, call.Suffix(), call.Method(), a.tagType)
	}
	return nil
}

func includes(ctx *context.T, acl access.AccessList, blessings map[string]struct{}) bool {
	pruneBlacklisted(ctx, acl, blessings)
	for _, pattern := range acl.In {
		rem, _ := Match(ctx, pattern, ApproximationTypeUnder, nil, blessings)
		// TODO(hpucha): Log errs.
		if len(rem) > 0 {
			return true
		}
	}

	return false
}

func pruneBlacklisted(ctx *context.T, acl access.AccessList, blessings map[string]struct{}) {
	for _, bp := range acl.NotIn {
		for b := range blessings {
			rem, _ := Match(ctx, security.BlessingPattern(bp), ApproximationTypeOver, nil, convertToSet(b))
			// TODO(hpucha): Log errs.
			if len(rem) > 0 {
				delete(blessings, b)
			}
		}
	}
}

func convertToSet(blessings ...string) map[string]struct{} {
	blessingSet := make(map[string]struct{})
	for _, b := range blessings {
		blessingSet[b] = struct{}{}
	}
	return blessingSet
}
