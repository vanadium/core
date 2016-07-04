// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"fmt"
	"regexp"
	"strings"

	"v.io/v23/context"
	"v.io/v23/conventions"
	"v.io/v23/naming"
	"v.io/v23/query/pattern"
	"v.io/v23/security"
	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
)

// Encode escapes a component name for use in a Syncbase object name. In
// particular, it replaces bytes "%" and "/" with the "%" character followed by
// the byte's two-digit hex code. Clients using the client library need not
// escape names themselves; the client library does so on their behalf.
func Encode(s string) string {
	return naming.EncodeAsNameElement(s)
}

// Decode is the inverse of Encode.
func Decode(s string) (string, error) {
	res, ok := naming.DecodeFromNameElement(s)
	if !ok {
		return "", fmt.Errorf("failed to decode component name: %s", s)
	}
	return res, nil
}

// EncodeId encodes the given Id for use in a Syncbase object name.
func EncodeId(id wire.Id) string {
	return Encode(id.String())
}

// DecodeId is the inverse of EncodeId.
func DecodeId(s string) (wire.Id, error) {
	dec, err := Decode(s)
	if err != nil {
		return wire.Id{}, err
	}
	return wire.ParseId(dec)
}

// Currently we use \xff for perms index storage and \xfe as a component
// separator in storage engine keys. \xfc and \xfd are not used. In the future
// we might use \xfc followed by "c", "d", "e", or "f" as an order-preserving
// 2-byte encoding of our reserved bytes. Note that all valid UTF-8 byte
// sequences are allowed.
var reservedBytes = []string{"\xfc", "\xfd", "\xfe", "\xff"}

func containsAnyOf(s string, needles []string) bool {
	for _, v := range needles {
		if strings.Contains(s, v) {
			return true
		}
	}
	return false
}

// TODO(ivanpi): Consider relaxing this.
var idNameRegexp *regexp.Regexp = regexp.MustCompile("^[a-zA-Z][a-zA-Z0-9_]*$")

const (
	// maxBlessingLen is the max allowed number of bytes in an Id blessing.
	// TODO(ivanpi): Blessings are theoretically unbounded in length.
	maxBlessingLen = 1024
	// maxNameLen is the max allowed number of bytes in an Id name.
	maxNameLen = 64
)

// ValidateId returns nil iff the given Id is a valid database, collection, or
// syncgroup Id.
// TODO(ivanpi): Use verror.New instead of fmt.Errorf everywhere.
func ValidateId(id wire.Id) error {
	if x := len([]byte(id.Blessing)); x == 0 {
		return fmt.Errorf("Id blessing cannot be empty")
	} else if x > maxBlessingLen {
		return fmt.Errorf("Id blessing %q exceeds %d bytes", id.Blessing, maxBlessingLen)
	}
	if x := len([]byte(id.Name)); x == 0 {
		return fmt.Errorf("Id name cannot be empty")
	} else if x > maxNameLen {
		return fmt.Errorf("Id name %q exceeds %d bytes", id.Name, maxNameLen)
	}
	if bp := security.BlessingPattern(id.Blessing); bp == security.NoExtension {
		return fmt.Errorf("Id blessing %q cannot match any blessings, check blessing conventions", id.Blessing)
	} else if !bp.IsValid() {
		return fmt.Errorf("Id blessing %q is not a valid blessing pattern", id.Blessing)
	}
	if containsAnyOf(id.Blessing, reservedBytes) {
		return fmt.Errorf("Id blessing %q contains a reserved byte (one of %q)", id.Blessing, reservedBytes)
	}
	if !idNameRegexp.MatchString(id.Name) {
		return fmt.Errorf("Id name %q does not satisfy regex %q", id.Name, idNameRegexp.String())
	}
	return nil
}

// ValidateRowKey returns nil iff the given string is a valid row key.
func ValidateRowKey(s string) error {
	if s == "" {
		return fmt.Errorf("row key cannot be empty")
	}
	if containsAnyOf(s, reservedBytes) {
		return fmt.Errorf("row key %q contains a reserved byte (one of %q)", s, reservedBytes)
	}
	return nil
}

// ParseCollectionRowPair splits the "<encCxId>/<rowKey>" part of a Syncbase
// object name into the collection id and the row key or prefix.
func ParseCollectionRowPair(ctx *context.T, pattern string) (wire.Id, string, error) {
	parts := strings.SplitN(pattern, "/", 2)
	if len(parts) != 2 { // require both collection and row parts
		return wire.Id{}, "", verror.New(verror.ErrBadArg, ctx, pattern)
	}
	collectionEnc, row := parts[0], parts[1]
	collection, err := DecodeId(parts[0])
	if err == nil {
		err = ValidateId(collection)
	}
	if err != nil {
		return wire.Id{}, "", verror.New(wire.ErrInvalidName, ctx, collectionEnc, err)
	}
	if row != "" {
		if err := ValidateRowKey(row); err != nil {
			return wire.Id{}, "", verror.New(wire.ErrInvalidName, ctx, row, err)
		}
	}
	return collection, row, nil
}

// RowPrefixPattern returns a CollectionRowPattern matching a single key prefix
// in a single collection.
func RowPrefixPattern(cxId wire.Id, keyPrefix string) wire.CollectionRowPattern {
	return wire.CollectionRowPattern{
		CollectionBlessing: pattern.Escape(cxId.Blessing),
		CollectionName:     pattern.Escape(cxId.Name),
		RowKey:             pattern.Escape(keyPrefix) + "%",
	}
}

// PrefixRangeStart returns the start of the row range for the given prefix.
func PrefixRangeStart(p string) string {
	return p
}

// PrefixRangeLimit returns the limit of the row range for the given prefix.
func PrefixRangeLimit(p string) string {
	// A string is a []byte, i.e. can be thought of as a base-256 number. The code
	// below effectively adds 1 to this number, then chops off any trailing \x00
	// bytes. If the input string consists entirely of \xff bytes, we return an
	// empty string.
	x := []byte(p)
	for len(x) > 0 {
		if x[len(x)-1] == 255 {
			x = x[:len(x)-1] // chop off trailing \x00
		} else {
			x[len(x)-1] += 1 // add 1
			break            // no carry
		}
	}
	return string(x)
}

// IsPrefix returns true if start and limit together represent a prefix range.
// If true, start is the represented prefix.
func IsPrefix(start, limit string) bool {
	return PrefixRangeLimit(start) == limit
}

// AccessController provides access control for various syncbase objects.
type AccessController interface {
	// SetPermissions replaces the current Permissions for an object.
	// For detailed documentation, see Object.SetPermissions.
	SetPermissions(ctx *context.T, perms access.Permissions, version string) error

	// GetPermissions returns the current Permissions for an object.
	// For detailed documentation, see Object.GetPermissions.
	GetPermissions(ctx *context.T) (perms access.Permissions, version string, err error)
}

// AppAndUserPatternFromBlessings infers the app and user blessing pattern from
// the given set of blessing names.
// <idp>:o:<app>:<user> blessings are preferred, with a fallback to
// <idp>:u:<user> and unrestricted app. Returns an error and no-match patterns
// if the inferred pattern is ambiguous (multiple blessings for different apps
// or users are found), or if no blessings matching conventions are found.
// TODO(ivanpi): Allow caller to restrict format to app:user or user instead of
// automatic fallback?
func AppAndUserPatternFromBlessings(blessings ...string) (app, user security.BlessingPattern, err error) {
	pbs := conventions.ParseBlessingNames(blessings...)
	found := false
	// Find a blessing of the form app:user; ensure there is only one app:user pair.
	for _, b := range pbs {
		a, au := b.AppPattern(), b.AppUserPattern()
		if a != security.NoExtension && au != security.NoExtension {
			if found && (a != app || au != user) {
				return security.NoExtension, security.NoExtension, NewErrFoundMultipleAppUserBlessings(nil, string(user), string(au))
			}
			app, user, found = a, au, true
		}
	}
	if found {
		return app, user, nil
	}
	// Fall back to a user blessing; ensure there is only one user.
	for _, b := range pbs {
		u := b.UserPattern()
		if u != security.NoExtension {
			if found && (u != user) {
				return security.NoExtension, security.NoExtension, NewErrFoundMultipleUserBlessings(nil, string(user), string(u))
			}
			app, user, found = security.AllPrincipals, u, true
		}
	}
	if found {
		return app, user, nil
	}
	// No app:user or user blessings found.
	return security.NoExtension, security.NoExtension, NewErrFoundNoConventionalBlessings(nil)
}

// FilterTags returns a copy of the provided perms, filtered to include only
// entries for tags allowed by the allowTags whitelist.
func FilterTags(perms access.Permissions, allowTags ...access.Tag) (filtered access.Permissions) {
	filtered = access.Permissions{}
	for _, allowTag := range allowTags {
		if acl, ok := perms[string(allowTag)]; ok {
			filtered[string(allowTag)] = acl
		}
	}
	// Copy to make sure lists in ACLs don't share backing arrays.
	return filtered.Copy()
}
