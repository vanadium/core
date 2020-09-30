// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This is WIP to implement and test algorithms that use acls that
// involve groups. When we migrate this to v23/security/access, most
// of this code might not be needed.

package groups

import (
	"fmt"

	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/vdl"
)

func PermissionsAuthorizer(perms access.Permissions, tagType *vdl.Type) (security.Authorizer, error) {
	if tagType.Kind() != vdl.String {
		return nil, fmt.Errorf("tag type(%v) must be backed by a string not %v", tagType, tagType.Kind())
	}
	return &authorizer{perms, tagType}, nil
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
				return access.ErrorfMultipleTags(ctx, "authorizer on %v.%v cannot handle multiple tags of type %v; this is likely unintentional", call.Suffix(), call.Method(), a.tagType.String())
			}
			hasTag = true
			if acl, exists := a.perms[tag.RawString()]; !exists || !includes(ctx, acl, convertToSet(blessings...)) {
				return access.ErrorfNoPermissions(ctx, "%v does not have %[3]v access (rejected blessings: %[2]v)", blessings, invalid, tag.RawString())
			}
		}
	}
	if !hasTag {
		return access.ErrorfNoTags(ctx, "authorizer on %v.%v has no tags of type %v; this is likely unintentional", call.Suffix(), call.Method(), a.tagType.String())
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
