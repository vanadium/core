// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

import (
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/x/lib/set"
)

// resolvePermissions performs fine-grained conflict resolution of Permissions.
func resolvePermissions(local, remote, ancestor access.Permissions) access.Permissions {
	result := access.Permissions{}
	if ancestor == nil {
		ancestor = access.Permissions{}
	}

	tagsSet := map[string]bool{}
	for tag, _ := range local {
		tagsSet[tag] = true
	}
	for tag, _ := range remote {
		tagsSet[tag] = true
	}
	for tag, _ := range tagsSet {
		in := toBlessingPatterns(resolveSlice(
			fromBlessingPatterns(local[tag].In),
			fromBlessingPatterns(remote[tag].In),
			fromBlessingPatterns(ancestor[tag].In)))
		notIn := resolveSlice(
			local[tag].NotIn,
			remote[tag].NotIn,
			ancestor[tag].NotIn)
		// TODO(fredq): remove this when it is added to Normalize
		if len(in) != 0 || len(notIn) != 0 {
			result[tag] = access.AccessList{In: in, NotIn: notIn}
		}
	}
	return result.Normalize()
}

// resolveSlice is passed two new versions plus an ancestor and returns the merge of
// the three by performing the following set calculation:
//    A - (A - L) - (A - R) + (L - A) + (R - A)
// The resultant set will be the ancestor minus the elements that right and left each removed,
// plus the elements that the right and left added.
//
// Since a side can only express adds and removals (and not, e.g., keep element), it isn't
// possible for one side to add an element that the other side removed, and vice versa. So if the
// initial set is {A} and one side adds {B} to it, it is not possible for the other side of
// the conflict to remove {B} during the conflict, since {B} wasn't yet in the set
// to be removed.
func resolveSlice(local, remote, ancestor []string) []string {
	ls := set.String.FromSlice(local)
	rs := set.String.FromSlice(remote)
	as := set.String.FromSlice(ancestor)

	// We may be adding things to ls later, and since it will be nil if local is empty,
	// let's be sure to allocate a map for it now.
	if ls == nil {
		ls = map[string]struct{}{}
	}

	// ls now has all the local elements in it.

	// Add in the elements from remote that are not in ancestor.
	for _, el := range remote {
		if _, exists := as[el]; !exists {
			ls[el] = struct{}{}
		}
	}
	// Remove the elements that are in ancestor but not in remote.
	for _, el := range ancestor {
		if _, exists := rs[el]; !exists {
			delete(ls, el)
		}
	}

	// Convert the map back to a slice, sort it, and return it.
	return set.String.ToSlice(ls)
}

// fromBlessingPatterns takes a slice of BlessingPatterns, which is a type alias of string, and
// returns a slice of strings, casting each BlessingPattern to a string.
func fromBlessingPatterns(in []security.BlessingPattern) []string {
	result := make([]string, len(in))
	for i, value := range in {
		result[i] = string(value)
	}
	return result
}

// toBlessingPatterns takes a slice of strings and returns it as a new slice of
// BlessingPatterns, casting each string to a BlessingPattern.
func toBlessingPatterns(in []string) []security.BlessingPattern {
	result := make([]security.BlessingPattern, len(in))
	for i, value := range in {
		result[i] = security.BlessingPattern(value)
	}
	return result
}
