// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package groups

import (
	"errors"
	"reflect"
	"testing"

	"v.io/v23/context"
	"v.io/v23/security"
)

var (
	errGrpNotFound = errors.New("group not found")
)

func TestSplitPattern(t *testing.T) {
	valid := []struct {
		Pattern security.BlessingPattern
		Parts   []string
	}{
		{
			Pattern: "a:b:c",
			Parts:   []string{"a", "b", "c"},
		},
		{
			Pattern: "a",
			Parts:   []string{"a"},
		},
		{
			Pattern: "a:<grp:1/2/3>:c",
			Parts:   []string{"a", "<grp:1/2/3>", "c"},
		},
		{
			Pattern: "a:<grp:1/2/3>:c:<grp:4/5/6>",
			Parts:   []string{"a", "<grp:1/2/3>", "c", "<grp:4/5/6>"},
		},
		{
			Pattern: "a:<grp:1/2/3>:<grp:4/5/6>:<grp:/7/8/9>",
			Parts:   []string{"a", "<grp:1/2/3>", "<grp:4/5/6>", "<grp:/7/8/9>"},
		},
		{
			Pattern: "<grp:1/2/3>",
			Parts:   []string{"<grp:1/2/3>"},
		},
		{
			Pattern: "<grp:1/2/3<grp:>",
			Parts:   []string{"<grp:1/2/3<grp:>"},
		},
	}

	invalid := []security.BlessingPattern{"a:UV<grp:1/2/3>:c:<grp:4/5/6>WXY", "a:<grp:1/2/3><grp:4/5/6>", "a:<grp:1/2/3<>grp:4/5/6>", "<grp:1/2/3>:", "a:UV<grp:1/2/3>", "a:<grp:>:b"}

	for _, test := range valid {
		want := test.Parts
		got, err := splitPattern(test.Pattern)
		if err != nil {
			t.Errorf("%q.splitPattern() return err %v", test.Pattern, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q.splitPattern() got %v want %v", test.Pattern, got, want)
		}
	}

	for _, test := range invalid {
		if got, err := splitPattern(test); err == nil {
			t.Errorf("%q.splitPattern() got %v want nil", test, got)
		}
	}
}

// Portion of test copied from v23/security/pattern_test.go.
func TestIsValid(t *testing.T) {
	var (
		valid = []security.BlessingPattern{security.AllPrincipals, "alice", "al$ice", "alice:$", "alice.jones:$", "alice@google:$", "v23:alice@google:$", "v23:alice@google:bob:$", "alice", "alice:bob", "alice:bob:<grp:vndm/bob/allmydevs>:app1", "alice:bob:<grp:vndm/bob/allmydevs>:<grp:vndm/bob/corpapps>", "<grp:vndm/bob/allmydevs>", "<grp:vndm/bob/allmydevs>:app1"}

		invalid = []security.BlessingPattern{"", "alice...", "...alice", "alice...bob", ":alice", "alice:", "...alice:bob", "alice...:bob", "alice:...:bob", "alice:$:bob", "alice:$:$", "alice:...:$", "alice:...", "alice<", "alice<>", "alice>", "alice<grp:>", "alice:<grp:vndm/bob/allmydevs<>grp:vndm/bob/corpapps>", "alice:UV<grp:vndm/bob/allmydevs>:c:<grp:vndm/bob/corpapps>WXY", "a:<grp:vndm/bob/corpapps><grp:vndm/bob/allmydevs>", "alice:<grp:>"}
	)
	for _, p := range valid {
		if _, err := splitPattern(p); err != nil {
			t.Errorf("%q.splitPattern() returned error %v", p, err)
		}
	}
	for _, p := range invalid {
		if _, err := splitPattern(p); err == nil {
			t.Errorf("%q.splitPattern() returned nil", p)
		}
	}
}

// Test copied from v23/security/pattern_test.go.
func TestMatch(t *testing.T) { //nolint:gocyclo
	type v []string
	valid := []struct {
		Pattern      security.BlessingPattern
		Matches      v
		Remainder    v // Remainder[i] is the result of match on Pattern and Matches[i].
		DoesNotMatch v
	}{
		{
			Pattern:      "",
			DoesNotMatch: v{"", "ann", "bob", "ann:friend"},
		},
		{
			Pattern:      "$",
			DoesNotMatch: v{"$", "ann", "bob", "ann:friend"},
		},
		{
			Pattern:   security.AllPrincipals,
			Matches:   v{"", "ann", "bob", "ann:friend"},
			Remainder: v{"", "", "", ""},
		},
		{
			Pattern:      "ann",
			Matches:      v{"ann", "ann:friend", "ann:enemy", "ann:friend:spouse"},
			Remainder:    v{"", "friend", "enemy", "friend:spouse"},
			DoesNotMatch: v{"", "bob", "bob:ann"},
		},
		{
			Pattern:      "ann:friend",
			Matches:      v{"ann:friend", "ann:friend:spouse"},
			Remainder:    v{"", "spouse"},
			DoesNotMatch: v{"", "ann", "ann:enemy", "bob", "bob:ann"},
		},
		{
			Pattern:      "ann:friend:$",
			Matches:      v{"ann:friend"},
			Remainder:    v{""},
			DoesNotMatch: v{"", "ann", "ann:enemy", "ann:friend:spouse", "bob", "bob:friend", "bob:ann"},
		},
	}

	g := &grpClient{}
	for _, test := range valid {
		if len(test.Matches) != len(test.Remainder) {
			t.Errorf("invalid test case %v", test)
		}

		for i := 0; i < len(test.Matches); i++ {
			if remainder := g.match(nil, test.Pattern, convertToSet(test.Matches[i])); len(remainder) != 1 || !contains(remainder, test.Remainder[i]) || g.apprxs != nil {
				t.Errorf("%q.match(%v) got %v, want %v, errs %v", test.Pattern, test.Matches[i], remainder, test.Remainder[i], g.apprxs)
			}
		}

		if len(test.Matches) != 0 {
			remainderWant := convertToSet(test.Remainder...)
			if remainderGot := g.match(nil, test.Pattern, convertToSet(test.Matches...)); !reflect.DeepEqual(remainderGot, remainderWant) || g.apprxs != nil {
				t.Errorf("%q.match(%v) got %v, want %v, errs %v", test.Pattern, test.Matches, remainderGot, remainderWant, g.apprxs)
			}
		}

		for i := 0; i < len(test.DoesNotMatch); i++ {
			if remainder := g.match(nil, test.Pattern, convertToSet(test.DoesNotMatch[i])); len(remainder) != 0 || g.apprxs != nil {
				t.Errorf("%q.match(%v) returned remainder %v, errs %v", test.Pattern, test.DoesNotMatch[i], remainder, g.apprxs)
			}
		}

		if len(test.DoesNotMatch) != 0 {
			if remainder := g.match(nil, test.Pattern, convertToSet(test.DoesNotMatch...)); len(remainder) != 0 || g.apprxs != nil {
				t.Errorf("%q.match(%v) returned remainder %v, errs %v", test.Pattern, test.DoesNotMatch, remainder, g.apprxs)
			}
		}
	}

	invalid := []struct {
		Pattern      security.BlessingPattern
		Matches      v
		Remainder    v // Remainder[i] is the result of match on Pattern and Matches[i].
		DoesNotMatch v
	}{
		{
			Pattern:      "ann:$:$",
			DoesNotMatch: v{"", "ann", "bob", "ann:friend", "ann:friend:spouse"},
		},
	}

	for _, test := range invalid {
		for i := 0; i < len(test.DoesNotMatch); i++ {
			if remainder := g.match(nil, test.Pattern, convertToSet(test.DoesNotMatch[i])); len(remainder) != 0 || g.apprxs == nil {
				t.Errorf("%q.match(%v) returned remainder %v, errs %v", test.Pattern, test.DoesNotMatch[i], remainder, g.apprxs)
			}
		}

		if len(test.DoesNotMatch) != 0 {
			if remainder := g.match(nil, test.Pattern, convertToSet(test.DoesNotMatch...)); len(remainder) != 0 || g.apprxs == nil {
				t.Errorf("%q.match(%v) returned remainder %v, errs %v", test.Pattern, test.DoesNotMatch, remainder, g.apprxs)
			}
		}
	}
}

// Test copied from v23/security/pattern_test.go.
func TestMatchCornerCases(t *testing.T) {
	g := &grpClient{}
	emptyBlessing := convertToSet("")

	if remainder := g.match(nil, security.AllPrincipals, emptyBlessing); remainder == nil || g.apprxs != nil {
		t.Errorf("%q.match(%q) failed, errs %v", security.AllPrincipals, "", g.apprxs)
	}
	/*if remainder := match(security.NoExtension, "", nil); remainder != nil {
		t.Errorf("%q.match(%q) returned remainder %v", security.NoExtension, "", remainder)
	}*/
	if remainder := g.match(nil, security.BlessingPattern("ann:$"), emptyBlessing); len(remainder) != 0 || g.apprxs != nil {
		t.Errorf("%q.match(%q) returned remainder %v, errs %v", "ann:$", "", remainder, g.apprxs)
	}
	if remainder := g.match(nil, security.BlessingPattern("ann"), emptyBlessing); len(remainder) != 0 || g.apprxs != nil {
		t.Errorf("%q.match(%q) returned remainder %v, errs %v", "ann", "", remainder, g.apprxs)
	}
}

// TODO(hpucha): Add more tests for multiple groups in a pattern,
// nested groups, groups with cycles, inaccessible groups.
func TestMatchWithGrps(t *testing.T) {
	var noMatch []string
	exactMatch := []string{""}

	grpTests := []struct {
		Pattern   security.BlessingPattern
		Blessing  string
		Groups    groupClientRPCTest
		Remainder []string
	}{
		{
			Pattern:  "a:b:<grp:v/g1>",
			Blessing: "a:b:c",
			Groups: groupClientRPCTest{
				"v/g1": []string{"c:d"},
			},
			Remainder: noMatch,
		},
		{
			Pattern:  "a:b:<grp:v/g1>",
			Blessing: "a:b:c",
			Groups: groupClientRPCTest{
				"v/g1": []string{"d:e:f", "c:d"},
			},
			Remainder: noMatch,
		},
		{
			Pattern:  "a:b:<grp:v/g1>",
			Blessing: "a:b:c",
			Groups: groupClientRPCTest{
				"v/g1": []string{"c"},
			},
			Remainder: exactMatch,
		},
		{
			Pattern:  "a:b:<grp:v/g1>",
			Blessing: "a:b:c",
			Groups: groupClientRPCTest{
				"v/g1": []string{"c:d", "c"},
			},
			Remainder: exactMatch,
		},
		{
			Pattern:  "a:b:<grp:v/g1>",
			Blessing: "a:b:c:d:e",
			Groups: groupClientRPCTest{
				"v/g1": []string{"c"},
			},
			Remainder: []string{"d:e"},
		},
		{
			Pattern:  "a:b:<grp:v/g1>",
			Blessing: "a:b:c:d:e",
			Groups: groupClientRPCTest{
				"v/g1": []string{"c:d"},
			},
			Remainder: []string{"e"},
		},
		{
			Pattern:  "a:b:<grp:v/g1>",
			Blessing: "a:b:c:d:e",
			Groups: groupClientRPCTest{
				"v/g1": []string{"c:d", "c"},
			},
			Remainder: []string{"e", "d:e"},
		},
		{
			Pattern:  "a:b:<grp:v/g1>",
			Blessing: "a:b:c:d:e",
			Groups: groupClientRPCTest{
				"v/g1": []string{"c:d:e", "c:d", "c"},
			},
			Remainder: []string{"", "e", "d:e"},
		},
		{
			Pattern:  "a:b:<grp:v/g1>:$",
			Blessing: "a:b:c:d:e",
			Groups: groupClientRPCTest{
				"v/g1": []string{"c:d", "c"},
			},
			Remainder: noMatch,
		},
		{
			Pattern:  "a:b:<grp:v/g1>:$",
			Blessing: "a:b:c:d:e",
			Groups: groupClientRPCTest{
				"v/g1": []string{"c:d:e", "c:d", "c"},
			},
			Remainder: exactMatch,
		},
		{
			Pattern:  "a:b:<grp:v/g1>:e",
			Blessing: "a:b:c:d:e",
			Groups: groupClientRPCTest{
				"v/g1": []string{"c"},
			},
			Remainder: noMatch,
		},
		{
			Pattern:  "a:b:<grp:v/g1>:e",
			Blessing: "a:b:c:d:e",
			Groups: groupClientRPCTest{
				"v/g1": []string{"c:d", "c"},
			},
			Remainder: exactMatch,
		},
		{
			Pattern:  "a:b:<grp:v/g1>:<grp:v/g2>",
			Blessing: "a:b:c:d:e",
			Groups: groupClientRPCTest{
				"v/g1": []string{"c:d", "c"},
				"v/g2": []string{"e"},
			},
			Remainder: exactMatch,
		},
		{
			Pattern:  "a:b:<grp:v/g1>:<grp:v/g2>",
			Blessing: "a:b:c:d:e",
			Groups: groupClientRPCTest{
				"v/g1": []string{"c:d", "c"},
				"v/g2": []string{"d:e"},
			},
			Remainder: exactMatch,
		},
		{
			Pattern:  "a:b:<grp:v/g1>:<grp:v/g2>",
			Blessing: "a:b:c:d:e",
			Groups: groupClientRPCTest{
				"v/g1": []string{"c:d", "c"},
				"v/g2": []string{"d", "e"},
			},
			Remainder: []string{"", "e"},
		},
		{
			Pattern:  "a:b:<grp:v/g1>:<grp:v/g2>",
			Blessing: "a:b:c:d:e",
			Groups: groupClientRPCTest{
				"v/g1": []string{"c:d", "c"},
				"v/g2": []string{"d:e", "d", "e"},
			},
			Remainder: []string{"", "e"},
		},
		{
			Pattern:  "a:b:<grp:v/g1>:<grp:v/g2>:e",
			Blessing: "a:b:c:d:e",
			Groups: groupClientRPCTest{
				"v/g1": []string{"c:d", "c"},
				"v/g2": []string{"d:e", "d", "e"},
			},
			Remainder: exactMatch,
		},
		{
			Pattern:  "a:b:<grp:v/g1>",
			Blessing: "a:b:c:d:e",
			Groups: groupClientRPCTest{
				"v/g1": []string{"c:d:<grp:v/g2>"},
				"v/g2": []string{"e"},
			},
			Remainder: exactMatch,
		},
	}
	for _, test := range grpTests {
		want := convertToSet(test.Remainder...)
		g := &grpClient{rpcHandler: test.Groups}
		got := g.match(nil, test.Pattern, convertToSet(test.Blessing))
		if g.apprxs != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%q.match(%q) groupClientRPCTest %v got %v want %v, errs %v", test.Pattern, test.Blessing, test.Groups, got, want, g.apprxs)
		}
	}
}

func TestMatchWithApproxGrps(t *testing.T) {
	var noMatch []string
	exactMatch := []string{""}

	grpTests := []struct {
		Pattern        security.BlessingPattern
		Blessing       string
		Groups         groupClientRPCTest
		RemainderOver  []string
		RemainderUnder []string
	}{
		{
			Pattern:        "a:b:<grp:v/g1>",
			Blessing:       "a:b:c",
			Groups:         groupClientRPCTest{},
			RemainderOver:  exactMatch,
			RemainderUnder: noMatch,
		},
		{
			Pattern:        "a:b:<grp:v/g1>",
			Blessing:       "a:b:c:d:e",
			Groups:         groupClientRPCTest{},
			RemainderOver:  []string{"", "e", "d:e"},
			RemainderUnder: noMatch,
		},
		{
			Pattern:  "a:b:<grp:v/g1>:<grp:v/g2>",
			Blessing: "a:b:c:d:e",
			Groups: groupClientRPCTest{
				"v/g1": []string{"c:d", "c"},
			},
			RemainderOver:  []string{"", "e"},
			RemainderUnder: noMatch,
		},
		{
			Pattern:  "a:b:<grp:v/g1>",
			Blessing: "a:b:c:d:e",
			Groups: groupClientRPCTest{
				"v/g1": []string{"c:d:<grp:v/g2>"},
			},
			RemainderOver:  exactMatch,
			RemainderUnder: noMatch,
		},
		{
			Pattern:  "a:b:<grp:v/g1>",
			Blessing: "a:b:c:d:e",
			Groups: groupClientRPCTest{
				"v/g1": []string{"c:d:<grp:v/g2>", "c"},
			},
			RemainderOver:  []string{"", "d:e"},
			RemainderUnder: []string{"d:e"},
		},
	}

	for _, test := range grpTests {
		want := convertToSet(test.RemainderOver...)
		g := &grpClient{hint: ApproximationTypeOver, rpcHandler: test.Groups}
		got := g.match(nil, test.Pattern, convertToSet(test.Blessing))
		if g.apprxs == nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%q.match(%q) groupClient {Over, %v} got %v want %v, errs %v", test.Pattern, test.Blessing, test.Groups, got, want, g.apprxs)
		}
	}

	for _, test := range grpTests {
		want := convertToSet(test.RemainderUnder...)
		g := &grpClient{hint: ApproximationTypeUnder, rpcHandler: test.Groups}
		got := g.match(nil, test.Pattern, convertToSet(test.Blessing))
		if g.apprxs == nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%q.match(%q) groupClient {Under, %v} got %v want %v, errs %v", test.Pattern, test.Blessing, test.Groups, got, want, g.apprxs)
		}
	}
}

func TestMatchWithCyclicGroups(t *testing.T) {
	var noMatch []string
	exactMatch := []string{""}

	grpTests := []struct {
		Pattern        security.BlessingPattern
		Blessing       string
		Groups         groupClientRPCTest
		RemainderOver  []string
		RemainderUnder []string
		lenErrors      int
	}{
		{
			Pattern:  "a:b:<grp:v/g1>",
			Blessing: "a:b:c",
			Groups: groupClientRPCTest{
				"v/g1": []string{"<grp:v/g1>"},
			},
			RemainderOver:  exactMatch,
			RemainderUnder: noMatch,
			lenErrors:      1,
		},
		{
			Pattern:  "a:b:<grp:v/g1>",
			Blessing: "a:b:c:d:e",
			Groups: groupClientRPCTest{
				"v/g1": []string{"<grp:v:g2>"},
				"v/g2": []string{"<grp:v:g1>"},
			},
			RemainderOver:  []string{"", "e", "d:e"},
			RemainderUnder: noMatch,
			lenErrors:      1,
		},
		{
			Pattern:  "a:b:<grp:v/g1>:<grp:v/g3>",
			Blessing: "a:b:c:d:e",
			Groups: groupClientRPCTest{
				"v/g1": []string{"<grp:v/g2>"},
				"v/g2": []string{"<grp:v/g1>"},
				"v/g3": []string{"d"},
			},
			RemainderOver:  []string{"e"},
			RemainderUnder: noMatch,
			lenErrors:      1,
		},
		{
			Pattern:  "a:b:<grp:v/g1>:<grp:v/g3>",
			Blessing: "a:b:c:d:e",
			Groups: groupClientRPCTest{
				"v/g1": []string{"f:<grp:v/g2>"},
				"v/g2": []string{"<grp:v/g1>"},
				"v/g3": []string{"d"},
			},
			RemainderOver:  noMatch,
			RemainderUnder: noMatch,
			lenErrors:      0,
		},
		{
			Pattern:  "a:b:<grp:v/g1>",
			Blessing: "a:b:c:d:e",
			Groups: groupClientRPCTest{
				"v/g1": []string{"f:<grp:v/g2>", "<grp:v/g2>:<grp:v/g3>", "c"},
				"v/g2": []string{"<grp:v/g1>"},
				"v/g3": []string{"d"},
			},
			RemainderOver:  []string{"e", "d:e"},
			RemainderUnder: []string{"d:e"},
			lenErrors:      1,
		},
		{
			Pattern:  "a:b:<grp:v/g1>",
			Blessing: "a:b:c:d:e",
			Groups: groupClientRPCTest{
				"v/g1": []string{"<grp:v/g2>", "<grp:v/g1>"},
				"v/g2": []string{"<grp:v/g1>"},
			},
			RemainderOver:  []string{"", "e", "d:e"},
			RemainderUnder: noMatch,
			lenErrors:      2,
		},
	}

	for _, test := range grpTests {
		want := convertToSet(test.RemainderOver...)
		g := &grpClient{hint: ApproximationTypeOver, rpcHandler: test.Groups}
		got := g.match(nil, test.Pattern, convertToSet(test.Blessing))
		if len(g.apprxs) != test.lenErrors || !reflect.DeepEqual(got, want) {
			t.Errorf("%q.match(%q) groupClientRPCTest %v got %v want %v, errs %v (got len %v, want len %v)", test.Pattern, test.Blessing, test.Groups, got, want, g.apprxs, len(g.apprxs), test.lenErrors)
		}
	}

	for _, test := range grpTests {
		want := convertToSet(test.RemainderUnder...)
		g := &grpClient{hint: ApproximationTypeUnder, rpcHandler: test.Groups}
		got := g.match(nil, test.Pattern, convertToSet(test.Blessing))
		if len(g.apprxs) != test.lenErrors || !reflect.DeepEqual(got, want) {
			t.Errorf("%q.match(%q) groupClientRPCTest %v got %v want %v, errs %v (got len %v, want len %v)", test.Pattern, test.Blessing, test.Groups, got, want, g.apprxs, len(g.apprxs), test.lenErrors)
		}
	}
}

// Helpers
// =======

type groupClientRPCTest map[string][]string

func (c groupClientRPCTest) relate(ctx *context.T, group string, blessingChunks map[string]struct{}, hint ApproximationType, version string, vGrps map[string]struct{}) (map[string]struct{}, []Approximation, string, error) {
	if _, ok := c[group]; !ok {
		return nil, nil, "", errGrpNotFound
	}

	var remainder = make(map[string]struct{})
	var approximations []Approximation
	for _, p := range c[group] {
		g := &grpClient{
			hint:       hint,
			version:    version,
			visited:    vGrps,
			rpcHandler: c,
		}

		rem := g.match(ctx, security.BlessingPattern(p), blessingChunks)
		remainder = union(remainder, rem)
		approximations = append(approximations, g.apprxs...)
	}

	return remainder, approximations, "", nil
}

func contains(in map[string]struct{}, str string) bool {
	_, ok := in[str]
	return ok
}
