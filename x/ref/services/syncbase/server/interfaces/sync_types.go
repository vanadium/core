// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package interfaces

import (
	"v.io/v23/security/access"
	"v.io/x/ref/services/syncbase/server/util"
)

func (in Knowledge) DeepCopy() Knowledge {
	out := make(Knowledge)
	for p, inpgv := range in {
		out[p] = inpgv.DeepCopy()
	}
	return out
}

func (in GenVector) DeepCopy() GenVector {
	out := make(GenVector)
	for id, gen := range in {
		out[id] = gen
	}
	return out
}

// Compare returns an integer comparing two generation vectors. The result will
// be 0 if a==b, -1 if a < b, +1 if a > b and +2 if a and b are uncomparable.
func (a GenVector) Compare(b GenVector) int {
	res := -2

	if len(a) == 0 && len(b) == 0 {
		return 0
	}

	for aid, agen := range a {
		bgen, ok := b[aid]

		resCur := 0
		if agen > bgen || !ok {
			resCur = 1
		} else if agen < bgen {
			resCur = -1
		}

		if res == -2 || res == 0 {
			// Initialize/overwrite safely with the curent result.
			res = resCur
		} else if res != resCur && resCur != 0 {
			// Uncomparable, since some elements are less and others
			// are greater.
			return 2
		}
	}

	for bid := range b {
		if _, ok := a[bid]; ok {
			continue
		}

		if res == 1 {
			// Missing elements. So a cannot be greater than b.
			return 2
		}
		return -1
	}

	return res
}

var (
	_ util.Permser = (*CollectionPerms)(nil)
)

func (perms *CollectionPerms) GetPerms() access.Permissions {
	return access.Permissions(*perms)
}
