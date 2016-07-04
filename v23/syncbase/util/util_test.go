// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"v.io/v23/security/access"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/syncbase/util"
	"v.io/v23/verror"
	tu "v.io/x/ref/services/syncbase/testutil"
)

func TestEncodeDecode(t *testing.T) {
	strs := []string{}
	copy(strs, tu.OkRowKeys)
	strs = append(strs, tu.NotOkRowKeys...)
	for _, s := range strs {
		enc := util.Encode(s)
		dec, err := util.Decode(enc)
		if err != nil {
			t.Fatalf("decode failed: %q, %v", s, err)
		}
		if strings.ContainsAny(enc, "/") {
			t.Errorf("%q was escaped to %q, which contains bad chars", s, enc)
		}
		if !strings.ContainsAny(s, "%/") && s != enc {
			t.Errorf("%q should equal %q", s, enc)
		}
		if s != dec {
			t.Errorf("%q should equal %q", s, dec)
		}
	}
}

func TestEncodeIdDecodeId(t *testing.T) {
	for _, blessing := range tu.OkAppUserBlessings {
		for _, name := range tu.OkDbCxNames {
			id := wire.Id{blessing, name}
			enc := util.EncodeId(id)
			dec, err := util.DecodeId(enc)
			if err != nil {
				t.Fatalf("decode failed: %v, %v", id, err)
			}
			if id != dec {
				t.Errorf("%v should equal %v", id, dec)
			}
		}
	}
}

func TestValidateNameFuncs(t *testing.T) {
	for _, a := range tu.OkAppUserBlessings {
		for _, d := range tu.OkDbCxNames {
			id := wire.Id{Blessing: a, Name: d}
			if util.ValidateId(id) != nil {
				t.Errorf("%v should be valid", id)
			}
		}
	}
	for _, a := range tu.NotOkAppUserBlessings {
		for _, d := range append(tu.OkDbCxNames, tu.NotOkDbCxNames...) {
			id := wire.Id{Blessing: a, Name: d}
			if util.ValidateId(id) == nil {
				t.Errorf("%v should be invalid", id)
			}
		}
	}
	for _, d := range tu.NotOkDbCxNames {
		for _, a := range append(tu.OkAppUserBlessings, tu.NotOkAppUserBlessings...) {
			id := wire.Id{Blessing: a, Name: d}
			if util.ValidateId(id) == nil {
				t.Errorf("%v should be invalid", id)
			}
		}
	}
	for _, s := range tu.OkRowKeys {
		if util.ValidateRowKey(s) != nil {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range tu.NotOkRowKeys {
		if util.ValidateRowKey(s) == nil {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestPrefixRange(t *testing.T) {
	tests := []struct {
		prefix string
		start  string
		limit  string
	}{
		{"", "", ""},
		{"a", "a", "b"},
		{"aa", "aa", "ab"},
		{"\xfe", "\xfe", "\xff"},
		{"a\xfe", "a\xfe", "a\xff"},
		{"aa\xfe", "aa\xfe", "aa\xff"},
		{"a\xff", "a\xff", "b"},
		{"aa\xff", "aa\xff", "ab"},
		{"a\xff\xff", "a\xff\xff", "b"},
		{"aa\xff\xff", "aa\xff\xff", "ab"},
		{"\xff", "\xff", ""},
		{"\xff\xff", "\xff\xff", ""},
	}
	for _, test := range tests {
		start, limit := util.PrefixRangeStart(test.prefix), util.PrefixRangeLimit(test.prefix)
		if start != test.start || limit != test.limit {
			t.Errorf("%q: got {%q, %q}, want {%q, %q}", test.prefix, start, limit, test.start, test.limit)
		}
	}
}

func TestIsPrefix(t *testing.T) {
	tests := []struct {
		isPrefix bool
		start    string
		limit    string
	}{
		{true, "", ""},
		{true, "a", "b"},
		{true, "aa", "ab"},
		{true, "\xfe", "\xff"},
		{true, "a\xfe", "a\xff"},
		{true, "aa\xfe", "aa\xff"},
		{true, "a\xff", "b"},
		{true, "aa\xff", "ab"},
		{true, "a\xff\xff", "b"},
		{true, "aa\xff\xff", "ab"},
		{true, "\xff", ""},
		{true, "\xff\xff", ""},

		{false, "", "\x00"},
		{false, "a", "aa"},
		{false, "aa", "aa"},
		{false, "\xfe", "\x00"},
		{false, "a\xfe", "b\xfe"},
		{false, "aa\xfe", "aa\x00"},
		{false, "a\xff", "b\x00"},
		{false, "aa\xff", "ab\x00"},
		{false, "a\xff\xff", "a\xff\xff\xff"},
		{false, "aa\xff\xff", "a"},
		{false, "\xff", "\x00"},
	}
	for _, test := range tests {
		result := util.IsPrefix(test.start, test.limit)
		if result != test.isPrefix {
			t.Errorf("%q, %q: got %v, want %v", test.start, test.limit, result, test.isPrefix)
		}
	}
}

func TestAppAndUserPatternFromBlessings(t *testing.T) {
	for _, test := range []struct {
		blessings []string
		wantApp   string
		wantUser  string
		wantErr   error
	}{
		{
			[]string{},
			"$",
			"$",
			util.NewErrFoundNoConventionalBlessings(nil),
		},
		{
			[]string{"foo", "bar:x:baz"},
			"$",
			"$",
			util.NewErrFoundNoConventionalBlessings(nil),
		},
		// non-conventional blessings are ignored if a conventional blessing is present
		{
			[]string{"foo", "root:o:angrybirds:alice", "bar:x:baz"},
			"root:o:angrybirds",
			"root:o:angrybirds:alice",
			nil,
		},
		// user blessings are ignored when an app:user blessing is present
		{
			[]string{"foo", "root:u:alice", "root:o:angrybirds:alice:device:phone", "bar:x:baz"},
			"root:o:angrybirds",
			"root:o:angrybirds:alice",
			nil,
		},
		// user blessings are ignored when an app:user blessing is present, even if multiple
		{
			[]string{"foo", "root:u:bob", "root:o:todos:dave:friend:alice", "root:u:carol", "bar:x:baz"},
			"root:o:todos",
			"root:o:todos:dave",
			nil,
		},
		// multiple blessings for the same app:user are allowed
		{
			[]string{"foo", "root:u:bob", "root:o:todos:dave:friend:alice", "root:u:carol", "root:o:todos:dave:device:phone", "bar:x:baz"},
			"root:o:todos",
			"root:o:todos:dave",
			nil,
		},
		// multiple blessings for different apps, users, or identity providers are not allowed
		{
			[]string{"foo", "root:u:bob", "root:o:todos:dave:friend:alice", "root:o:angrybirds:dave", "root:o:todos:dave:device:phone", "bar:x:baz"},
			"$",
			"$",
			util.NewErrFoundMultipleAppUserBlessings(nil, "root:o:todos:dave", "root:o:angrybirds:dave"),
		},
		{
			[]string{"foo", "root:u:bob", "root:o:todos:dave:friend:alice", "root:o:todos:fred"},
			"$",
			"$",
			util.NewErrFoundMultipleAppUserBlessings(nil, "root:o:todos:dave", "root:o:todos:fred"),
		},
		{
			[]string{"foo", "root:u:bob", "root:o:todos:dave:friend:alice", "google:o:todos:dave"},
			"$",
			"$",
			util.NewErrFoundMultipleAppUserBlessings(nil, "root:o:todos:dave", "google:o:todos:dave"),
		},
		// non-conventional blessings are ignored if a conventional blessing is present
		{
			[]string{"foo", "root:u:bob", "bar:x:baz"},
			"...",
			"root:u:bob",
			nil,
		},
		// multiple blessings for the same user are allowed
		{
			[]string{"foo", "root:u:bob:angrybirds", "root:u:bob:todos:phone", "bar:x:baz"},
			"...",
			"root:u:bob",
			nil,
		},
		// multiple blessings for different users or identity providers are not allowed
		{
			[]string{"foo", "root:u:bob:angrybirds", "root:u:bob:todos:phone", "root:u:carol", "root:u:dave", "bar:x:baz"},
			"$",
			"$",
			util.NewErrFoundMultipleUserBlessings(nil, "root:u:bob", "root:u:carol"),
		},
		{
			[]string{"foo", "root:u:bob:angrybirds", "google:u:bob:todos:phone", "bar:x:baz"},
			"$",
			"$",
			util.NewErrFoundMultipleUserBlessings(nil, "root:u:bob", "google:u:bob"),
		},
	} {
		app, user, err := util.AppAndUserPatternFromBlessings(test.blessings...)
		if verror.ErrorID(err) != verror.ErrorID(test.wantErr) || fmt.Sprint(err) != fmt.Sprint(test.wantErr) {
			t.Errorf("AppAndUserPatternFromBlessings(%v): got error %v, want %v", test.blessings, err, test.wantErr)
		}
		if string(app) != test.wantApp {
			t.Errorf("AppAndUserPatternFromBlessings(%v): got app %s, want %s", test.blessings, app, test.wantApp)
		}
		if string(user) != test.wantUser {
			t.Errorf("AppAndUserPatternFromBlessings(%v): got user %s, want %s", test.blessings, user, test.wantUser)
		}
	}
}

func TestFilterTags(t *testing.T) {
	for _, test := range []struct {
		input     access.Permissions
		allowTags []access.Tag
		want      access.Permissions
	}{
		{
			access.Permissions{}.
				Add("root", access.TagStrings(access.Read, access.Write, access.Admin)...).
				Add("root:alice", access.TagStrings(access.Read, access.Write)...),
			[]access.Tag{},
			access.Permissions{},
		},
		{
			access.Permissions{}.
				Add("root", access.TagStrings(access.Read, access.Write, access.Admin)...).
				Add("root:alice", access.TagStrings(access.Read, access.Write)...),
			[]access.Tag{access.Admin, access.Read},
			access.Permissions{}.
				Add("root", access.TagStrings(access.Read, access.Admin)...).
				Add("root:alice", access.TagStrings(access.Read)...),
		},
		{
			access.Permissions{}.
				Add("alice", access.TagStrings(access.Admin)...).
				Add("bob", access.TagStrings(access.Read, access.Write)...).
				Add("carol", access.TagStrings(access.Write, access.Admin)...),
			[]access.Tag{access.Read, access.Write},
			access.Permissions{}.
				Add("bob", access.TagStrings(access.Read, access.Write)...).
				Add("carol", access.TagStrings(access.Write)...),
		},
		{
			access.Permissions{},
			[]access.Tag{access.Read, access.Write, access.Admin},
			access.Permissions{},
		},
		{
			access.Permissions{}.
				Add("alice", access.TagStrings(access.Admin)...).
				Add("bob", access.TagStrings(access.Read, access.Write, access.Admin)...).
				Blacklist("bob:tablet", access.TagStrings(access.Write, access.Admin)...),
			[]access.Tag{access.Read, access.Write},
			access.Permissions{}.
				Add("bob", access.TagStrings(access.Read, access.Write)...).
				Blacklist("bob:tablet", access.TagStrings(access.Write)...),
		},
		{
			access.Permissions{}.
				Add("alice", access.TagStrings(access.Admin)...).
				Add("bob", access.TagStrings(access.Read, access.Write)...).
				Blacklist("bob:tablet", access.TagStrings(access.Write)...),
			[]access.Tag{access.Admin, access.Read},
			access.Permissions{}.
				Add("alice", access.TagStrings(access.Admin)...).
				Add("bob", access.TagStrings(access.Read)...),
		},
	} {
		inputCopy := test.input.Copy()
		filtered := util.FilterTags(inputCopy, test.allowTags...)
		// Modify the input perms to ensure the filtered copy is unaffected.
		if adminAcl, ok := inputCopy[string(access.Admin)]; ok && len(adminAcl.In) > 0 {
			adminAcl.In[0] = "mallory"
		}
		if got, want := filtered, test.want.Normalize(); !reflect.DeepEqual(got, want) {
			t.Errorf("FilterTags(%v, %v): got %v, want %v", test.input, test.allowTags, got, want)
		}
	}
}
