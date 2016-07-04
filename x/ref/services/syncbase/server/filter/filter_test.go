// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package filter_test

import (
	"testing"

	wire "v.io/v23/services/syncbase"
	"v.io/x/ref/services/syncbase/server/filter"
)

type cxRow struct {
	cxId   wire.Id
	rowKey string
}

type filterCollectionsTest struct {
	rawFilter   interface{}
	matchCxs    []wire.Id
	noMatchCxs  []wire.Id
	matchRows   []cxRow
	noMatchRows []cxRow
}

func runFilterTests(t *testing.T, tests []filterCollectionsTest, compileFilter func(raw interface{}) (filter.CollectionRowFilter, error)) {
	for _, test := range tests {
		f, err := compileFilter(test.rawFilter)
		if err != nil {
			t.Fatalf("filter %#v failed to parse: %v", test.rawFilter, err)
		}
		// Note, tests may include invalid collection ids, for example blessings
		// with a leading or trailing ':', as valid matches.
		for _, cx := range test.matchCxs {
			if !f.CollectionMatches(cx) {
				t.Errorf("filter %#v should match %v", test.rawFilter, cx)
			}
		}
		for _, cx := range test.noMatchCxs {
			if f.CollectionMatches(cx) {
				t.Errorf("filter %#v should not match %v", test.rawFilter, cx)
			}
		}
		for _, r := range test.matchRows {
			if !f.RowMatches(r.cxId, r.rowKey) {
				t.Errorf("filter %#v should match %v", test.rawFilter, r)
			}
		}
		for _, r := range test.noMatchRows {
			if f.RowMatches(r.cxId, r.rowKey) {
				t.Errorf("filter %#v should not match %v", test.rawFilter, r)
			}
		}
	}
}

func TestPatternFilter(t *testing.T) {
	runFilterTests(t, []filterCollectionsTest{
		{
			rawFilter: wire.CollectionRowPattern{
				CollectionBlessing: "%",
				CollectionName:     "foo",
				RowKey:             "bar%",
			},
			matchCxs: []wire.Id{
				{"alice", "foo"},
				{"bob", "foo"},
			},
			noMatchCxs: []wire.Id{
				{"alice", "bar"},
				{"bob", "foobar"},
			},
			matchRows: []cxRow{
				{wire.Id{"", "foo"}, "bar"},
				{wire.Id{"alice", "foo"}, "bar"},
				{wire.Id{"alice", "foo"}, "barbaz"},
				{wire.Id{"bob", "foo"}, "bart"},
			},
			noMatchRows: []cxRow{
				{wire.Id{"alice", "foobar"}, "bar"},
				{wire.Id{"alice", "f"}, "barracks"},
				{wire.Id{"alice", "f"}, "pie"},
				{wire.Id{"bob", "bar"}, "bart"},
			},
		},
		{
			rawFilter: wire.CollectionRowPattern{
				CollectionBlessing: "root:foo:%",
				CollectionName:     "_\\_%",
				RowKey:             "\\%%\\%",
			},
			matchCxs: []wire.Id{
				{"root:foo:alice", "w_"},
				{"root:foo:a", "z_abc"},
				{"root:foo:", "a_12"},
				{"root:foo:bob:friend:carol", "___"},
			},
			noMatchCxs: []wire.Id{
				{"alice", "x_123"},
				{"root:bar:alice", "x_abc"},
				{"root:foo:bob", "zabc"},
				{"root:foo:carol", "yy_abc"},
			},
			matchRows: []cxRow{
				{wire.Id{"root:foo:alice", "x_123"}, "%%"},
				{wire.Id{"root:foo:a", "z_abc"}, "%foo_bar%"},
				{wire.Id{"root:foo:bob", "3_"}, "%钒%同步%数据库%"},
				{wire.Id{"root:foo:carol", "x_123"}, "%%%"},
			},
			noMatchRows: []cxRow{
				{wire.Id{"alice", "x_123"}, ""},
				{wire.Id{"root:foo:alice", "zabc"}, "%a%b"},
				{wire.Id{"root:foo:bob", "t_xyz"}, "bar%"},
			},
		},
		{
			rawFilter: wire.CollectionRowPattern{
				CollectionBlessing: "%:alice%",
				CollectionName:     "foo%",
				RowKey:             "settings/%-main",
			},
			matchCxs: []wire.Id{
				{":alice", "foo"},
				{"root:alice", "foo_xyz"},
				{"foo:alice:phone", "foobar"},
			},
			noMatchCxs: []wire.Id{
				{"alice", "foobar"},
				{"root:alice:friend", ""},
				{"root:alice,foo", "bar"},
			},
			matchRows: []cxRow{
				{wire.Id{"root:alice", "foo"}, "settings/control-main"},
				{wire.Id{"root:bob:friend:alice", "foo_xyz"}, "settings/-main"},
				{wire.Id{"foo:alice", "foofoobar"}, "settings/panel/switch-main"},
			},
			noMatchRows: []cxRow{
				{wire.Id{"root:alice", "foobar/settings/"}, "foo-main"},
				{wire.Id{"root:alice:phone,foo", "bar/set"}, "tings/bar-main"},
				{wire.Id{"foo:alice", "foo"}, "settings/foo-main/"},
			},
		},
		{
			rawFilter: wire.CollectionRowPattern{
				CollectionBlessing: "_%",
				CollectionName:     "foo",
				RowKey:             "",
			},
			matchCxs: []wire.Id{
				{"alice", "foo"},
				{"bob", "foo"},
			},
			noMatchCxs: []wire.Id{
				{"", "foo"},
				{"bob", "foobar"},
			},
			matchRows: []cxRow{
				// Note, the filter matches no valid rows - empty keys are invalid.
				{wire.Id{"bob", "foo"}, ""},
			},
			noMatchRows: []cxRow{
				{wire.Id{"alice", "foo"}, "bar"},
				{wire.Id{"alice", "f"}, "b"},
			},
		},
	}, func(raw interface{}) (filter.CollectionRowFilter, error) {
		return filter.NewPatternFilter(raw.(wire.CollectionRowPattern))
	})
}

func TestInvalidPatternFilter(t *testing.T) {
	invalid := []wire.CollectionRowPattern{
		// empty collection blessing and/or name
		{"", "", ""},
		{"", "bar", "baz"},
		{"foo", "", "baz"},
		// escape followed by nothing
		{"foo", "bar\\", "baz"},
		// escape followed by non-wildcard
		{"%", "bar", "b\\az"},
		{"foo\\%ba\\r", "%", "baz"},
		// multiple violations
		{"fooba\\r", "%\\", "b\\az\\_"},
	}

	for _, wp := range invalid {
		if _, err := filter.NewPatternFilter(wp); err == nil {
			t.Errorf("filter %#v should have failed to parse", wp)
		}
	}
}

func TestMultiPatternFilter(t *testing.T) {
	runFilterTests(t, []filterCollectionsTest{
		{
			rawFilter: []wire.CollectionRowPattern{
				{
					CollectionBlessing: "%",
					CollectionName:     "foo",
					RowKey:             "bar\\%%",
				},
			},
			matchCxs: []wire.Id{
				{"alice", "foo"},
				{"bob", "foo"},
			},
			noMatchCxs: []wire.Id{
				{"alice", "bar"},
				{"bob", "foobar"},
			},
			matchRows: []cxRow{
				{wire.Id{"alice", "foo"}, "bar%"},
				{wire.Id{"alice", "foo"}, "bar%n"},
				{wire.Id{"bob", "foo"}, "bar%t"},
			},
			noMatchRows: []cxRow{
				{wire.Id{"alice", "foo"}, "bar"},
				{wire.Id{"alice", "foobar"}, "bar%"},
				{wire.Id{"alice", "f"}, "bar%racks"},
				{wire.Id{"alice", "f"}, "pie"},
				{wire.Id{"bob", "bar"}, "bar%k"},
			},
		},
		{
			rawFilter: []wire.CollectionRowPattern{
				{
					CollectionBlessing: "root:%",
					CollectionName:     "foo",
					RowKey:             "one%",
				},
				{
					CollectionBlessing: "%:alice",
					CollectionName:     "%",
					RowKey:             "two%",
				},
				{
					CollectionBlessing: "%:bob",
					CollectionName:     "foo%",
					RowKey:             "three%",
				},
			},
			matchCxs: []wire.Id{
				{"root:", "foo"},
				{"root:alice", "foo"},
				{"root:bob", "foo"},
				{"root:carol", "foo"},
				{"root:alice", "bar"},
				{"root:bob", "foo"},
				{"foo:alice", "baz"},
				{"foo:bob", "foobar"},
			},
			noMatchCxs: []wire.Id{
				{"foo:alice:bar", "foo"},
				{"root:carol", "foobar"},
				{"root:bob", "bar"},
				{"foo:bob", "barfoo"},
			},
			matchRows: []cxRow{
				{wire.Id{"root:alice", "foo"}, "one"},
				{wire.Id{"root:alice", "foo"}, "two"},
				{wire.Id{"root:alice", "bar"}, "two22"},
				{wire.Id{"root:bob", "foo"}, "one1"},
				{wire.Id{"root:bob", "foo"}, "three"},
				{wire.Id{"root:bob", "foobar"}, "three333"},
				{wire.Id{"root:carol", "foo"}, "one"},
				{wire.Id{"foo:bar:alice", "xyz"}, "two"},
				{wire.Id{"foo:bat:bob", "foo"}, "three"},
			},
			noMatchRows: []cxRow{
				{wire.Id{"root:alice:phone", "bar"}, "one"},
				{wire.Id{"root:alice", "foobar"}, "one1"},
				{wire.Id{"root:bob", "foobar"}, "one"},
				{wire.Id{"foo:alice", "foobar"}, "three"},
				{wire.Id{"foo:bob", "bar"}, "three333"},
			},
		},
		{
			rawFilter: []wire.CollectionRowPattern{
				{
					CollectionBlessing: "root:%",
					CollectionName:     "foo",
					RowKey:             "%",
				},
				{
					CollectionBlessing: "root:%",
					CollectionName:     "foo",
					RowKey:             "bar%",
				},
				{
					CollectionBlessing: "root:%",
					CollectionName:     "foo%",
					RowKey:             "bar%",
				},
				{
					CollectionBlessing: "%:carol",
					CollectionName:     "foo",
					RowKey:             "",
				},
			},
			matchCxs: []wire.Id{
				{"root:alice", "foo"},
				{"root:bob:phone", "foobar"},
				{"root:carol", "foo"},
				{"xyz:carol", "foo"},
			},
			noMatchCxs: []wire.Id{
				{"xyz:bob", "foo"},
				{"root:alice", "barfoo"},
				{"xyz:carol", "foobar"},
			},
			matchRows: []cxRow{
				{wire.Id{"root:alice", "foo"}, "foobar"},
				{wire.Id{"root:bob", "foo"}, "xyz"},
				{wire.Id{"root:carol", "foo"}, "barfoo"},
				{wire.Id{"root:dave", "foofoo"}, "barfoo"},
			},
			noMatchRows: []cxRow{
				{wire.Id{"root:alice", "foobar"}, "foofoo"},
				{wire.Id{"root:bob", "barfoo"}, "barfoo"},
				{wire.Id{"foo:alice", "foo"}, "barbaz"},
				{wire.Id{"root:alice", "foobar"}, "%"},
				{wire.Id{"xyz:carol", "foo"}, "bar"},
			},
		},
	}, func(raw interface{}) (filter.CollectionRowFilter, error) {
		return filter.NewMultiPatternFilter(raw.([]wire.CollectionRowPattern))
	})
}

func TestInvalidMultiPatternFilter(t *testing.T) {
	invalid := [][]wire.CollectionRowPattern{
		// no patterns
		{},
		// empty collection blessing and/or name
		{{"", "bar", "baz"}},
		{{"foo", "bar", "baz"}, {"", "", ""}},
		{{"%", "%", "%"}, {"", "bar", "baz"}},
		{{"foo", "", "baz"}, {"foo", "bar", "baz"}},
		// escape followed by nothing
		{{"abc", "def", "xyz"}, {"foo", "bar\\", "baz"}, {"%", "foo", "bar%"}},
		// escape followed by non-wildcard
		{{"%", "bar", "b\\az"}, {"root:alice", "foo%", "\\_bar%"}},
		{{"%", "bar", "foo-_-baz"}, {"foo\\%ba\\r", "%", "baz"}},
		// multiple violations
		{{"foo", "bar", "\\\\baz"}, {"fooba\\r", "%\\", "b\\az\\_"}},
	}

	for _, wps := range invalid {
		if _, err := filter.NewMultiPatternFilter(wps); err == nil {
			t.Errorf("filter %#v should have failed to parse", wps)
		}
	}
}

func TestGlobFilter(t *testing.T) {
	runFilterTests(t, []filterCollectionsTest{
		{
			rawFilter: "*,foo/bar/*",
			matchCxs: []wire.Id{
				{"alice", "foo"},
				{"bob", "foo"},
			},
			noMatchCxs: []wire.Id{
				{"alice", "bar"},
				{"bob", "foobar"},
			},
			matchRows: []cxRow{
				{wire.Id{"alice", "foo"}, "bar/bat"},
				{wire.Id{"bob", "foo"}, "bar/baz"},
				{wire.Id{"carol", "foo"}, "bar/"},
			},
			noMatchRows: []cxRow{
				{wire.Id{"alice", "foo"}, "bar"},
				{wire.Id{"bob", "foo"}, "bar/baz/bat"},
				{wire.Id{"alice", "foobar"}, "bar"},
				{wire.Id{"alice", "f"}, "foo/bar"},
				{wire.Id{"alice", "bar"}, "pie/bar"},
			},
		},
		{
			rawFilter: "root:*,foo[a-z]/bar",
			matchCxs: []wire.Id{
				{"root:alice", "fooo"},
				{"root:bob", "fooa"},
				{"root:carol:tablet", "foob"},
				{"root:", "foox"},
			},
			noMatchCxs: []wire.Id{
				{"foo:alice:bar", "foow"},
				{"root:bob", "bar"},
				{"root:carol", "fooxy"},
				{"root:dave:phone", "foo"},
			},
			matchRows: []cxRow{
				{wire.Id{"root:alice", "food"}, "bar"},
				{wire.Id{"root:bob", "foox"}, "bar"},
			},
			noMatchRows: []cxRow{
				{wire.Id{"root:alice:phone", "foowq"}, "baz"},
				{wire.Id{"root:bob", "foob"}, "bart"},
				{wire.Id{"root:carol", "foom"}, "bar/baz"},
			},
		},
		{
			rawFilter: "*/foo/***",
			matchCxs: []wire.Id{
				{"root:alice", "foo"},
				{"root:bob:phone", "barfoo"},
				{"", ""},
				{"alice", ""},
				{"", "foobar"},
				{"alice", "foo%2Fbar"},
			},
			noMatchCxs: []wire.Id{},
			matchRows: []cxRow{
				{wire.Id{"root:alice", "abc"}, "foo"},
				{wire.Id{"root:bob", "def"}, "foo/bar"},
				{wire.Id{"root:carol", "ghi"}, "foo/bar/baz"},
				{wire.Id{"root:dave", "foo"}, "foo//baz"},
			},
			noMatchRows: []cxRow{
				{wire.Id{"root:alice", "foobar"}, "foofoo"},
				{wire.Id{"root:bob", "barfoo"}, "barfoo"},
				{wire.Id{"foo:alice", "foo"}, "barbaz"},
				{wire.Id{"root:alice", "foobar"}, "%"},
			},
		},
		{
			rawFilter: "root:alice,*/foo*bar/bat\\*/***",
			matchCxs: []wire.Id{
				{"root:alice", "foo"},
				{"root:alice", "barfoo"},
				{"root:alice", ""},
				{"root:alice", "*"},
			},
			noMatchCxs: []wire.Id{
				{"root:alice:bob", "foo"},
				{"root:bob", "foobar"},
				{"", "foo*bar"},
			},
			matchRows: []cxRow{
				{wire.Id{"root:alice", "abc"}, "foobar/bat*/xyz/*$"},
				{wire.Id{"root:alice", "def"}, "foofoobar/bat*/baz"},
				{wire.Id{"root:alice", "ghi"}, "foobar/bat*"},
			},
			noMatchRows: []cxRow{
				{wire.Id{"root:alice", "foo"}, "foobar/bat"},
				{wire.Id{"root:alice", "foo"}, "foobar/batxyz/baz"},
				{wire.Id{"root:alice", "foo"}, "foo/bar/bat*"},
				{wire.Id{"root:alice", "bar"}, "foofoobar/baz/bat"},
				{wire.Id{"root:alice:bob", "barfoo"}, "foobarbar/bat*"},
			},
		},
	}, func(raw interface{}) (filter.CollectionRowFilter, error) {
		return filter.NewGlobFilter(raw.(string))
	})
}

func TestInvalidGlobFilter(t *testing.T) {
	invalid := []string{
		"foo,bar/[a-z",
		"foo,bar[/[a-z]",
		"foo//bar",
	}

	for _, p := range invalid {
		if _, err := filter.NewGlobFilter(p); err == nil {
			t.Errorf("filter %#v should have failed to parse", p)
		}
	}
}
