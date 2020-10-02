// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

// TODO(sadovsky): Check Resolve access on parent where applicable. Relatedly,
// convert ErrNoExist and ErrNoAccess to ErrNoExistOrNoAccess where needed to
// preserve privacy.

import (
	"errors"

	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/groups"
	"v.io/v23/verror"
	"v.io/x/lib/set"
	"v.io/x/ref/services/groups/internal/store"
)

type group struct {
	name string
	m    *manager
}

var _ groups.GroupServerMethods = (*group)(nil)

// TODO(sadovsky): Limit the number of groups that a particular user
// (v23/conventsions.GetClientUserId) can create?
func (g *group) Create(ctx *context.T, call rpc.ServerCall, perms access.Permissions, entries []groups.BlessingPatternChunk) error {
	if err := g.m.createAuthorizer.Authorize(ctx, call.Security()); err != nil {
		return err
	}
	if perms == nil {
		perms = access.Permissions{}
		blessings, _ := security.RemoteBlessingNames(ctx, call.Security())
		if len(blessings) == 0 {
			// The blessings presented by the caller do not give it a name for this
			// operation. We could create a world-accessible group, but it seems safer
			// to return an error.
			return groups.ErrNoBlessings.Errorf(ctx, "no blessings recognized; cannot create group Permissions")
		}
		for _, tag := range access.AllTypicalTags() {
			for _, b := range blessings {
				perms.Add(security.BlessingPattern(b), string(tag))
			}
		}
	}
	entrySet := map[groups.BlessingPatternChunk]struct{}{}
	for _, v := range entries {
		entrySet[v] = struct{}{}
	}
	gd := groupData{Perms: perms, Entries: entrySet}
	if err := g.m.st.Insert(g.name, gd); err != nil {
		// TODO(sadovsky): We are leaking the fact that this group exists. If the
		// client doesn't have access to this group, we should probably return an
		// opaque error. (Reserving buckets for users will help.)
		if errors.Is(err, store.ErrKeyExists) {
			return verror.ErrExist.Errorf(ctx, "already exists: %s", g.name)
		}
		return verror.ErrInternal.Errorf(ctx, "internal error: %v", err)
	}
	return nil
}

func (g *group) Delete(ctx *context.T, call rpc.ServerCall, version string) error {
	if err := g.readModifyWrite(ctx, call.Security(), version, func(gd *groupData, versionSt string) error {
		return g.m.st.Delete(g.name, versionSt)
	}); err != nil && !errors.Is(err, verror.ErrNoExist) {
		return err
	}
	return nil
}

func (g *group) Add(ctx *context.T, call rpc.ServerCall, entry groups.BlessingPatternChunk, version string) error {
	return g.update(ctx, call.Security(), version, func(gd *groupData) {
		if gd.Entries == nil {
			gd.Entries = map[groups.BlessingPatternChunk]struct{}{}
		}
		gd.Entries[entry] = struct{}{}
	})
}

func (g *group) Remove(ctx *context.T, call rpc.ServerCall, entry groups.BlessingPatternChunk, version string) error {
	return g.update(ctx, call.Security(), version, func(gd *groupData) {
		delete(gd.Entries, entry)
	})
}

func (g *group) Get(ctx *context.T, call rpc.ServerCall, req groups.GetRequest, reqVersion string) (groups.GetResponse, string, error) {
	gd, resVersion, err := g.getInternal(ctx, call.Security())
	if err != nil {
		return groups.GetResponse{}, "", err
	}

	// If version is set and matches the Group's current version,
	// send an empty response (the equivalent of "HTTP 304 Not Modified").
	if reqVersion == resVersion {
		return groups.GetResponse{}, resVersion, nil
	}

	return groups.GetResponse{Entries: gd.Entries}, resVersion, nil
}

func (g *group) Relate(ctx *context.T, call rpc.ServerCall, blessings map[string]struct{}, hint groups.ApproximationType, reqVersion string, visitedGroups map[string]struct{}) (map[string]struct{}, []groups.Approximation, string, error) {
	gd, resVersion, err := g.getInternal(ctx, call.Security())
	if err != nil {
		return nil, nil, "", err
	}

	// If version is set and matches the Group's current version,
	// send an empty response (the equivalent of "HTTP 304 Not Modified").
	if reqVersion == resVersion {
		return nil, nil, resVersion, nil
	}

	remainder := make(map[string]struct{})
	var approximations []groups.Approximation
	for p := range gd.Entries {
		rem, apprxs := groups.Match(ctx, security.BlessingPattern(p), hint, visitedGroups, blessings)
		set.String.Union(remainder, rem)
		approximations = append(approximations, apprxs...)
	}

	return remainder, approximations, resVersion, nil
}

func (g *group) SetPermissions(ctx *context.T, call rpc.ServerCall, perms access.Permissions, version string) error {
	return g.update(ctx, call.Security(), version, func(gd *groupData) {
		gd.Perms = perms
	})
}

func (g *group) GetPermissions(ctx *context.T, call rpc.ServerCall) (perms access.Permissions, version string, err error) {
	gd, version, err := g.getInternal(ctx, call.Security())
	if err != nil {
		return nil, "", err
	}
	return gd.Perms, version, nil
}

// Internal helpers

// Returns a VDL-compatible error.
func (g *group) authorize(ctx *context.T, call security.Call, perms access.Permissions) error {
	auth := access.TypicalTagTypePermissionsAuthorizer(perms)
	if err := auth.Authorize(ctx, call); err != nil {
		return verror.ErrNoAccess.Errorf(ctx, "access denied: %v", err)
	}
	return nil
}

// Returns a VDL-compatible error. Performs access check.
func (g *group) getInternal(ctx *context.T, call security.Call) (gd groupData, version string, err error) {
	version, err = g.m.st.Get(g.name, &gd)
	if err != nil {
		if errors.Is(err, store.ErrUnknownKey) {
			return groupData{}, "", verror.ErrNoExist.Errorf(ctx, "does not exist: %v", g.name)
		}
		return groupData{}, "", verror.ErrInternal.Errorf(ctx, "internal error: %v", err)
	}
	if err := g.authorize(ctx, call, gd.Perms); err != nil {
		return groupData{}, "", err
	}
	return gd, version, nil
}

// Returns a VDL-compatible error. Performs access check.
func (g *group) update(ctx *context.T, call security.Call, version string, fn func(gd *groupData)) error {
	return g.readModifyWrite(ctx, call, version, func(gd *groupData, versionSt string) error {
		fn(gd)
		return g.m.st.Update(g.name, *gd, versionSt)
	})
}

// Returns a VDL-compatible error. Performs access check.
// fn should perform the "modify, write" part of "read, modify, write", and
// should return a Store error.
func (g *group) readModifyWrite(ctx *context.T, call security.Call, version string, fn func(gd *groupData, versionSt string) error) error {
	// Transaction retry loop.
	for i := 0; i < 3; i++ {
		gd, versionSt, err := g.getInternal(ctx, call)
		if err != nil {
			return err
		}
		// Fail early if possible.
		if version != "" && version != versionSt {
			return verror.ErrBadVersion.Errorf(ctx, "version is out of date")
		}
		if err := fn(&gd, versionSt); err != nil {
			if errors.Is(err, verror.ErrBadVersion) {
				// Retry on version error if the original version was empty.
				if version != "" {
					return err
				}
			} else {
				// Abort on non-version error.
				return verror.ErrInternal.Errorf(ctx, "internal error: %v", err)
			}
		} else {
			return nil
		}
	}
	return groups.ErrExcessiveContention.Errorf(ctx, "gave up after encountering excessive contention; try again later")
}
