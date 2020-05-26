// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"v.io/v23/security"
)

func TestImportMembers(t *testing.T) {
	workdir, err := ioutil.TempDir("", "test-role-server-")
	if err != nil {
		t.Fatalf("ioutil.TempDir failed: %v", err)
	}
	defer os.RemoveAll(workdir)
	os.Mkdir(filepath.Join(workdir, "sub"), 0700) //nolint:errcheck

	configs := map[string]Config{
		"role1":     {Members: []security.BlessingPattern{"A", "B", "C"}},
		"role2":     {Members: []security.BlessingPattern{"C", "D", "E"}},
		"sub/role3": {ImportMembers: []string{"../role2"}},
		"sub/role4": {ImportMembers: []string{"../role1", "../role2"}},
		"sub/role5": {ImportMembers: []string{"../role1", "../role6"}},
		"role6":     {ImportMembers: []string{"sub/role5"}, Members: []security.BlessingPattern{"F"}},
	}
	for role, config := range configs {
		WriteConfig(t, config, filepath.Join(workdir, role+".conf"))
	}

	testcases := []struct {
		role    string
		members []security.BlessingPattern
	}{
		{"role1", []security.BlessingPattern{"A:_role", "B:_role", "C:_role"}},
		{"role2", []security.BlessingPattern{"C:_role", "D:_role", "E:_role"}},
		{"sub/role3", []security.BlessingPattern{"C:_role", "D:_role", "E:_role"}},
		{"sub/role4", []security.BlessingPattern{"A:_role", "B:_role", "C:_role", "D:_role", "E:_role"}},
		{"sub/role5", []security.BlessingPattern{"A:_role", "B:_role", "C:_role", "F:_role"}},
		{"role6", []security.BlessingPattern{"A:_role", "B:_role", "C:_role", "F:_role"}},
	}
	for _, tc := range testcases {
		c, err := loadExpandedConfig(filepath.Join(workdir, tc.role+".conf"), nil)
		if err != nil {
			t.Errorf("unexpected error for %q: %v", tc.role, err)
			continue
		}
		sort.Sort(BlessingPatternSlice(c.Members))
		if !reflect.DeepEqual(tc.members, c.Members) {
			t.Errorf("unexpected results. Got %#v, expected %#v", c.Members, tc.members)
		}
	}
}

type BlessingPatternSlice []security.BlessingPattern

func (p BlessingPatternSlice) Len() int           { return len(p) }
func (p BlessingPatternSlice) Less(i, j int) bool { return p[i] < p[j] }
func (p BlessingPatternSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func WriteConfig(t *testing.T, config Config, fileName string) {
	mConf, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("json.MarshalIndent failed: %v", err)
	}
	if err := ioutil.WriteFile(fileName, mConf, 0644); err != nil {
		t.Fatalf("ioutil.WriteFile(%q, %q) failed: %v", fileName, string(mConf), err)
	}
}
