// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mounttablelib

import (
	"encoding/json"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"v.io/v23/context"
	"v.io/v23/conventions"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/mounttable"
	"v.io/v23/verror"
)

// Blessings can't include a comma so we use them in made up user ids.  The following distinctions are
// made so that we can account for them differently.
//nolint:deadcode,unused,varcheck
const (
	localUser   = ",LOCAL,"   // a client that has our public key but no blessing from which we can extract a user name
	blessedUser = ",BLESSED," // a client with blessings we trust but from which we can't extract a user name
	unknownUser = ",UNKNOWN," // a client which presents no blessing we trust
)

// VersionedPermissions associates a Version with a Permissions
type VersionedPermissions struct {
	V int32
	P access.Permissions
}

func NewVersionedPermissions() *VersionedPermissions {
	return &VersionedPermissions{P: make(access.Permissions)}
}

// Set sets the Permissions iff Version matches the current Version.  If the set happens, the Version is advanced.
// If b is nil, this creates a new VersionedPermissions.
func (b *VersionedPermissions) Set(ctx *context.T, verstr string, perm access.Permissions) (*VersionedPermissions, error) {
	if b == nil {
		b = new(VersionedPermissions)
	}
	if len(verstr) > 0 {
		gen, err := strconv.ParseInt(verstr, 10, 32)
		if err != nil {
			return b, verror.ErrBadVersion.Errorf(ctx, "version is out of date")
		}
		if gen >= 0 && int32(gen) != b.V {
			return b, verror.ErrBadVersion.Errorf(ctx, "version is out of date")
		}
	}
	b.P = perm
	// Increment with possible wrap.
	b.V++
	if b.V < 0 {
		b.V = 0
	}
	return b, nil
}

// Get returns the current Version and Permissions.
func (b *VersionedPermissions) Get() (string, access.Permissions) {
	if b == nil {
		return "", nil
	}
	return strconv.FormatInt(int64(b.V), 10), b.P
}

// AccessListForTag returns the current access list for the given tag.
func (b *VersionedPermissions) AccessListForTag(tag string) (access.AccessList, bool) {
	al, exists := b.P[tag]
	return al, exists
}

// Copy copies the receiver.
func (b *VersionedPermissions) Copy() *VersionedPermissions {
	nt := new(VersionedPermissions)
	nt.P = b.P.Copy()
	nt.V = b.V
	return nt
}

// Add adds the blessing pattern to the tag in the receiver.
func (b *VersionedPermissions) Add(pattern security.BlessingPattern, tag string) {
	b.P.Add(pattern, tag)
}

// parsePermFile reads a file and parses the contained permissions.
func (mt *mountTable) parsePermFile(ctx *context.T, path string) error {
	ctx.VI(2).Infof("parsePermFile(%s)", path)
	if path == "" {
		return nil
	}
	// A map from node name to permissions.
	var pm map[string]access.Permissions
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	for {
		if err = decoder.Decode(&pm); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		cc := &callContext{ctx: ctx,
			create:       true,
			ignorePerms:  true,
			ignoreLimits: true,
		}
		// Sort the map shortest key first.  That way configs for nodes higher up in the
		// name tree happen first ensuring that lower nodes correctly inherit permissions.
		var keys []string
		for name := range pm {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		for _, name := range keys {
			perms := pm[name]
			var elems []string
			isPattern := false

			// The configuration file allows patterns and also will cause the superuser to
			// be set to the root's administrator.
			if len(name) == 0 {
				// If the config file has is an Admin tag on the root AccessList, the
				// list of Admin users is the equivalent of a super user for
				// the whole table.  Later SetPermissions do not update the set
				// of super users.
				if bp, exists := perms[string(mounttable.Admin)]; exists {
					mt.superUsers = bp
				}
			} else {
				// AccessList templates terminate with a %% element.  These are very
				// constrained matches, i.e., the trailing element of the name
				// is copied into every %% in the AccessList.
				elems = strings.Split(name, "/")
				if elems[len(elems)-1] == templateVar {
					isPattern = true
					elems = elems[:len(elems)-1]
				}
			}

			// Create name and add the Permissions map to it.
			cc.creator = mt.pickCreator(ctx, nil)
			n, err := mt.findNode(cc, elems, nil, nil)
			if err != nil {
				ctx.Errorf("skipping node for %v; error: %v", elems, err)
			}
			if n == nil {
				continue
			}
			if err == nil {
				ctx.VI(2).Infof("added perms %v to %s", perms, name)
				if isPattern {
					n.permsTemplate = perms
				} else {
					n.vPerms, _ = n.vPerms.Set(nil, "", perms)
					n.explicitPermissions = true
				}
			}
			n.parent.Unlock()
			n.Unlock()
		}
	}
	return nil
}

// pickCreator returns a string matching the blessing of the user performing the creation.
func (mt *mountTable) pickCreator(ctx *context.T, call security.Call) string {
	ids := conventions.GetClientUserIds(ctx, call)
	if len(ids) > 0 {
		// Replace the slashes with something else or we'll confuse the stats package.
		return strings.ReplaceAll(ids[0], "/", "\\")
	}
	return ""
}
