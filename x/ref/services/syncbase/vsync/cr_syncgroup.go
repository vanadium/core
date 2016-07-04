// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

import (
	"v.io/v23/context"
	"v.io/v23/services/syncbase"
	"v.io/v23/verror"
	"v.io/x/ref/services/syncbase/server/interfaces"
)

// resolveSyncgroup performs fine-grained conflict resolution of Syncgroups.

// Immutable Fields: Id, Creator, DbId
// Fields where latest wins: none
// Custom logic: SpecVersion, Spec, Joiners, Status
//
// Joiners: a map of joinerSyncbaseName->SyncgroupMemberState. Each joiner is merged using latest
// wins according to the WhenUpdated field of the SyncgroupMemberState.
//
// Status: Running > Rejected > Pending
//
// Spec: if no conflict take remote or local depending on which one changed, otherwise
// perform a three-way merge of the Spec.
//
// SpecVersion: the version of the chosen Spec is used, and if the Spec was the result of a
// merge then the version is left as NoVersion and the caller is responsible for generating a
// new version when needed. Generally this happens when the syncgroup object is persisted.
//
//    preferLocal This is derived from the versions of the local, remote, and ancestor and
//                is true if the local is to be considered the latest, and false if the remote
//                is the latest.
func resolveSyncgroup(ctx *context.T, preferLocal bool, local, remote, ancestor *interfaces.Syncgroup) (*interfaces.Syncgroup, error) {
	// Validate the immutable fields.
	if ancestor == nil {
		ancestor = &interfaces.Syncgroup{}
	} else {
		if err := validateImmutableFields(ctx, ancestor, local); err != nil {
			return nil, err
		}
	}
	if err := validateImmutableFields(ctx, remote, local); err != nil {
		return nil, err
	}

	// Create a Syncgroup to hold the result of the merge.
	result := &interfaces.Syncgroup{Id: local.Id, Creator: local.Creator, DbId: local.DbId}
	result.Status = resolveStatus(local.Status, remote.Status, ancestor.Status)
	result.Joiners = resolveJoiners(preferLocal, local.Joiners, remote.Joiners, ancestor.Joiners)
	result.SpecVersion, result.Spec = resolveSpec(preferLocal,
		local.SpecVersion, remote.SpecVersion, ancestor.SpecVersion,
		local.Spec, remote.Spec, ancestor.Spec)

	return result, nil
}

func validateImmutableFields(ctx *context.T, a, b *interfaces.Syncgroup) error {
	if a.Id != b.Id {
		return verror.New(verror.ErrBadState, ctx, "illegal change to the Id")
	}
	if a.Creator != b.Creator {
		return verror.New(verror.ErrBadState, ctx, "illegal change to the Creator")
	}
	if a.DbId != b.DbId {
		return verror.New(verror.ErrBadState, ctx, "illegal change to the DbId")
	}
	if !sameCollections(a.Spec.Collections, b.Spec.Collections) {
		return verror.New(verror.ErrBadState, ctx, "illegal change to the Collections set")
	}
	return nil
}

// Spec:
//    Immutable: Prefixes
//    Latest Wins: Description, PublishSyncbaseName, IsPrivate
//    Custom: Perms, MountTables
//
//    Perms: standard Permissions 3-way merging
//    MountTables: standard slice 3-way merging
//
// If the SyncgroupSpec version is NoVersion then the caller is responsible for generating a
// new one if needed.
func resolveSpec(preferLocal bool, vlocal, vremote, vancestor string, local, remote, ancestor syncbase.SyncgroupSpec) (string, syncbase.SyncgroupSpec) {
	// If the versions are the same then pick any, local is fine.
	// If only local or remote changed then use it.
	// Otherwise merge the Spec.
	if vlocal == vremote {
		return vlocal, local
	}

	if vlocal == vancestor {
		return vremote, remote
	}

	if vremote == vancestor {
		return vlocal, local
	}

	// Both local and remote changed from the ancestor, perform a three-way merge and indicate that a new version number is needed.
	vresult := NoVersion
	result := syncbase.SyncgroupSpec{}
	result.Collections = local.Collections
	if preferLocal {
		result.Description = local.Description
		result.PublishSyncbaseName = local.PublishSyncbaseName
		result.IsPrivate = local.IsPrivate
	} else {
		result.Description = remote.Description
		result.PublishSyncbaseName = remote.PublishSyncbaseName
		result.IsPrivate = remote.IsPrivate
	}
	result.Perms = resolvePermissions(local.Perms, remote.Perms, ancestor.Perms)
	result.MountTables = resolveSlice(local.MountTables, remote.MountTables, ancestor.MountTables)
	return vresult, result
}

// resolveStatus merges Status using the rule: Running > Rejected > Pending. If any
// of the inputs is Running then pick Running, if any are Rejected then pick Rejected,
// otherwise pick Pending.
func resolveStatus(local, remote, ancestor interfaces.SyncgroupStatus) interfaces.SyncgroupStatus {
	if local == interfaces.SyncgroupStatusRunning ||
		remote == interfaces.SyncgroupStatusRunning ||
		ancestor == interfaces.SyncgroupStatusRunning {
		return interfaces.SyncgroupStatusRunning
	} else if local == interfaces.SyncgroupStatusPublishRejected ||
		remote == interfaces.SyncgroupStatusPublishRejected ||
		ancestor == interfaces.SyncgroupStatusPublishRejected {
		return interfaces.SyncgroupStatusPublishRejected
	} else {
		return interfaces.SyncgroupStatusPublishPending
	}
}

// resolveJoiners performs a granular merge of the members of a syncgroup, choosing the
// local or remote version of the syncgroup information based on latest wins.
// The new set of members is the union of those in the local and remote sets.
// Each member is then individually merged preferring the one with the later WhenUpdated timestamp.
// TODO(fredq): we need to garbage collect the members after they have left the group for a while.
func resolveJoiners(preferLocal bool, local, remote, ancestor map[string]interfaces.SyncgroupMemberState) (result map[string]interfaces.SyncgroupMemberState) {
	result = make(map[string]interfaces.SyncgroupMemberState)
	newMembers := map[string]bool{}
	for member, _ := range local {
		newMembers[member] = true
	}
	for member, _ := range remote {
		newMembers[member] = true
	}
	for key, _ := range newMembers {
		_, inL := local[key]
		_, inR := remote[key]
		if !inL {
			result[key] = remote[key]
		} else if !inR {
			result[key] = local[key]
		} else if local[key].WhenUpdated == remote[key].WhenUpdated {
			// The member is in both remote and local and the update times are the same,
			// choose one based on the value of preferLocal. The MemberInfo should
			// be the same in this case.
			if preferLocal {
				result[key] = local[key]
			} else {
				result[key] = remote[key]
			}
		} else {
			// The member is in both remote and local but the update times are
			// different, choose the one with the later update time.
			if local[key].WhenUpdated > remote[key].WhenUpdated {
				result[key] = local[key]
			} else {
				result[key] = remote[key]
			}
		}
	}
	return
}
