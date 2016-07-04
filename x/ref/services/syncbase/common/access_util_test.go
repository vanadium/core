// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package common_test

import (
	"fmt"
	"testing"

	"v.io/v23/security/access"
	"v.io/v23/verror"
	"v.io/x/ref/services/syncbase/common"
)

func TestValidatePerms(t *testing.T) {
	for _, test := range []struct {
		perms     access.Permissions
		allowTags []access.Tag
		wantErr   error
	}{
		{
			nil,
			access.AllTypicalTags(),
			common.NewErrPermsEmpty(nil),
		},
		{
			access.Permissions{},
			access.AllTypicalTags(),
			common.NewErrPermsEmpty(nil),
		},
		{
			access.Permissions{}.Add("alice", access.TagStrings(access.Read, access.Write, access.Admin, access.Resolve)...),
			access.AllTypicalTags(),
			nil,
		},
		{
			access.Permissions{}.Add("alice", access.TagStrings(access.Read, access.Write, access.Admin, access.Resolve)...),
			[]access.Tag{access.Read, access.Admin},
			common.NewErrPermsDisallowedTags(nil, access.TagStrings(access.Resolve, access.Write), access.TagStrings(access.Read, access.Admin)),
		},
		{
			access.Permissions{}.Add("alice", access.TagStrings(access.Read, access.Write, access.Resolve)...),
			access.AllTypicalTags(),
			common.NewErrPermsNoAdmin(nil),
		},
		{
			access.Permissions{}.Add("alice", access.TagStrings(access.Read)...).Blacklist("alice:phone", access.TagStrings(access.Admin)...),
			access.AllTypicalTags(),
			common.NewErrPermsNoAdmin(nil),
		},
		{
			access.Permissions{}.Add("alice", access.TagStrings(access.Read, access.Write)...).Add("bob", access.TagStrings(access.Read, access.Admin)...),
			access.AllTypicalTags(),
			nil,
		},
		{
			access.Permissions{}.Add("alice", access.TagStrings(access.Read, access.Admin)...).Blacklist("alice:phone", access.TagStrings(access.Write, access.Admin)...),
			access.AllTypicalTags(),
			nil,
		},
		{
			access.Permissions{}.Add("alice", access.TagStrings(access.Read, access.Admin)...).Blacklist("alice:phone", access.TagStrings(access.Write, access.Admin)...),
			[]access.Tag{access.Read, access.Admin},
			common.NewErrPermsDisallowedTags(nil, access.TagStrings(access.Write), access.TagStrings(access.Read, access.Admin)),
		},
	} {
		err := common.ValidatePerms(nil, test.perms, test.allowTags)
		if verror.ErrorID(err) != verror.ErrorID(test.wantErr) || fmt.Sprint(err) != fmt.Sprint(test.wantErr) {
			t.Errorf("ValidatePerms(%v, %v): got error %v, want %v", test.perms, test.allowTags, err, test.wantErr)
		}
	}
}
