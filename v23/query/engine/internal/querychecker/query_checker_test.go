// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package querychecker_test

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	v23 "v.io/v23"
	"v.io/v23/context"
	ds "v.io/v23/query/engine/datasource"
	"v.io/v23/query/engine/internal/querychecker"
	"v.io/v23/query/engine/internal/queryparser"
	"v.io/v23/query/pattern"
	"v.io/v23/query/syncql"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
)

type mockDB struct {
	ctx *context.T
}

func (db *mockDB) GetContext() *context.T {
	return db.ctx
}

type customerTable struct {
}

type invoiceTable struct {
}

func init() {
	var shutdown v23.Shutdown
	db.ctx, shutdown = test.V23Init()
	defer shutdown()
}

func (t invoiceTable) GetIndexFields() []ds.Index {
	return []ds.Index{}
}

func (t invoiceTable) Scan(indexRanges ...ds.IndexRanges) (ds.KeyValueStream, error) {
	return nil, errors.New("unimplemented")
}

func (t invoiceTable) Delete(k string) (bool, error) {
	return false, errors.New("unimplemented")
}

func (t customerTable) GetIndexFields() []ds.Index {
	return []ds.Index{}
}

func (t customerTable) Scan(indexRanges ...ds.IndexRanges) (ds.KeyValueStream, error) {
	return nil, errors.New("unimplemented")
}

func (t customerTable) Delete(k string) (bool, error) {
	return false, errors.New("unimplemented")
}

func (db *mockDB) GetTable(table string, writeAccessReq bool) (ds.Table, error) {
	if table == "Customer" {
		var t customerTable
		return t, nil
	} else if table == "Invoice" {
		var t invoiceTable
		return t, nil
	}
	return nil, fmt.Errorf("No such table: %s", table)
}

var db mockDB

type checkTest struct {
	query string
}

type keyRangesTest struct {
	query       string
	indexRanges *ds.IndexRanges
}

type indexRangesTest struct {
	query       string
	fieldNames  []string
	indexRanges []*ds.IndexRanges
}

type likePatternsTest struct {
	query      string
	matches    []string
	nonMatches []string
}

type checkErrorTest struct {
	query string
	err   error
}

func TestQueryChecker(t *testing.T) {
	basic := []checkTest{
		// Select
		{"select k, v from Customer"},
		{"select k, v.name from Customer"},
		{"select k, v.name from Customer limit 200"},
		{"select k, v.name from Customer offset 100"},
		{"select k, v.name from Customer where k = \"foo\""},
		{"select v.z from Customer where k = v.y"},
		{"select v.z from Customer where k <> v.y"},
		{"select v.z from Customer where k < v.y"},
		{"select v.z from Customer where k <= v.y"},
		{"select v.z from Customer where k > v.y"},
		{"select v.z from Customer where k >= v.y"},
		{"select v from Customer where k is nil"},
		{"select v from Customer where k is not nil"},
		{"select k, v.name from Customer where \"foo\" = k"},
		{"select v.z from Customer where v.y = k"},
		{"select v.z from Customer where v.y <> k"},
		{"select v.z from Customer where v.y < k"},
		{"select v.z from Customer where v.y <= k"},
		{"select v.z from Customer where v.y > k"},
		{"select v.z from Customer where \"abc%\" = k"},
		{"select v from Customer where k is nil"},
		{"select v from Customer where k is not nil"},
		{"select v from Customer where Type(v) = \"Foo.Bar\""},
		{"select v from Customer where Type(v) <> \"Foo.Bar\""},
		{"select v from Customer where Type(v) < \"Foo.Bar\""},
		{"select v from Customer where Type(v) <= \"Foo.Bar\""},
		{"select v from Customer where Type(v) > \"Foo.Bar\""},
		{"select v from Customer where Type(v) >= \"Foo.Bar\""},
		{"select v from Customer where Type(v) like \"%.Foo.Bar\""},
		{"select v from Customer where Type(v) not like \"%.Foo.Bar\""},
		{"select v from Customer where Type(v) is nil"},
		{"select v from Customer where Type(v) is not nil"},
		{"select v from Customer where \"Foo.Bar\" = Type(v)"},
		{"select v from Customer where \"Foo.Bar\" <> Type(v)"},
		{"select v from Customer where \"Foo.Bar\" < Type(v)"},
		{"select v from Customer where \"Foo.Bar\" > Type(v)"},
		{"select v from Customer where \"Foo.Bar\" <= Type(v)"},
		{"select v from Customer where \"Foo.Bar\" >= Type(v)"},
		{"select v.z from Customer where Type(v) = 2"},
		{"select v.z from Customer where Type(v) <> \"foo\""},
		{"select v.z from Customer where Type(v) < \"foo\""},
		{"select v.z from Customer where Type(v) <= \"foo\""},
		{"select v.z from Customer where Type(v) > \"foo\""},
		{"select v.z from Customer where Type(v) >= \"foo\""},
		{"select v.z from Customer where \"foo\" = Type(v)"},
		{"select k, v from Customer where Type(v) = \"Foo.Bar\" and k like \"abc%\" limit 100 offset 200"},
		{"select v from Customer where Type(v) is nil"},
		{"select v from Customer where Type(v) is not nil"},
		{"select v.z from Customer where k not like \"foo\""},
		{"select v.z from Customer where k not like \"foo%\""},
		{"select v from Customer where v.A = true"},
		{"select v from Customer where v.A <> true"},
		{"select v from Customer where false = v.A"},
		{"select v from Customer where false = false"},
		{"select v from Customer where true = true"},
		{"select v from Customer where false = true"},
		{"select v from Customer where true = false"},
		{"select v from Customer where false <> true"},
		{"select v from Customer where v.ZipCode is nil"},
		{"select v from Customer where v.ZipCode Is Nil"},
		{"select v from Customer where v.ZipCode is not nil"},
		{"select v from Customer where v.ZipCode IS NOT NIL"},
		{"select v from Customer where Now() < 10"},
		{"select Now() from Customer"},
		{"select Time(\"2006-01-02 MST\", \"2015-06-01 PST\"), Time(\"2006-01-02 15:04:05 MST\", \"2015-06-01 12:34:56 PST\"), Year(Now(), \"America/Los_Angeles\") from Customer"},
		// Delete
		{"delete from Customer"},
		{"delete from Customer where k = v.name"},
		{"delete from Customer limit 200"},
		{"delete from Customer where k = \"foo\""},
		{"delete from Customer where k = v.y"},
		{"delete from Customer where k <> v.y"},
		{"delete from Customer where k < v.y"},
		{"delete from Customer where k <= v.y"},
		{"delete from Customer where k > v.y"},
		{"delete from Customer where k >= v.y"},
		{"delete from Customer where k is nil"},
		{"delete from Customer where k is not nil"},
		{"delete from Customer where \"foo\" = k"},
		{"delete from Customer where v.y = k"},
		{"delete from Customer where v.y <> k"},
		{"delete from Customer where v.y < k"},
		{"delete from Customer where v.y <= k"},
		{"delete from Customer where v.y > k"},
		{"delete from Customer where \"abc%\" = k"},
		{"delete from Customer where k is nil"},
		{"delete from Customer where k is not nil"},
		{"delete from Customer where Type(v) = \"Foo.Bar\""},
		{"delete from Customer where Type(v) <> \"Foo.Bar\""},
		{"delete from Customer where Type(v) < \"Foo.Bar\""},
		{"delete from Customer where Type(v) <= \"Foo.Bar\""},
		{"delete from Customer where Type(v) > \"Foo.Bar\""},
		{"delete from Customer where Type(v) >= \"Foo.Bar\""},
		{"delete from Customer where Type(v) like \"%.Foo.Bar\""},
		{"delete from Customer where Type(v) not like \"%.Foo.Bar\""},
		{"delete from Customer where Type(v) is nil"},
		{"delete from Customer where Type(v) is not nil"},
		{"delete from Customer where \"Foo.Bar\" = Type(v)"},
		{"delete from Customer where \"Foo.Bar\" <> Type(v)"},
		{"delete from Customer where \"Foo.Bar\" < Type(v)"},
		{"delete from Customer where \"Foo.Bar\" > Type(v)"},
		{"delete from Customer where \"Foo.Bar\" <= Type(v)"},
		{"delete from Customer where \"Foo.Bar\" >= Type(v)"},
		{"delete from Customer where Type(v) = 2"},
		{"delete from Customer where Type(v) <> \"foo\""},
		{"delete from Customer where Type(v) < \"foo\""},
		{"delete from Customer where Type(v) <= \"foo\""},
		{"delete from Customer where Type(v) > \"foo\""},
		{"delete from Customer where Type(v) >= \"foo\""},
		{"delete from Customer where \"foo\" = Type(v)"},
		{"delete from Customer where Type(v) = \"Foo.Bar\" and k like \"abc%\" limit 100"},
		{"delete from Customer where Type(v) is nil"},
		{"delete from Customer where Type(v) is not nil"},
		{"delete from Customer where k not like \"foo\""},
		{"delete from Customer where k not like \"foo%\""},
		{"delete from Customer where v.A = true"},
		{"delete from Customer where v.A <> true"},
		{"delete from Customer where false = v.A"},
		{"delete from Customer where false = false"},
		{"delete from Customer where true = true"},
		{"delete from Customer where false = true"},
		{"delete from Customer where true = false"},
		{"delete from Customer where false <> true"},
		{"delete from Customer where v.ZipCode is nil"},
		{"delete from Customer where v.ZipCode Is Nil"},
		{"delete from Customer where v.ZipCode is not nil"},
		{"delete from Customer where v.ZipCode IS NOT NIL"},
		{"delete from Customer where Now() < 10"},
		{"delete from Customer where Now() > 0"},
		{"delete from Customer where Time(\"2006-01-02 MST\", \"2015-06-01 PST\") > 0 and Time(\"2006-01-02 15:04:05 MST\", \"2015-06-01 12:34:56 PST\") > 1 and Year(Now(), \"America/Los_Angeles\") > 3"},
	}

	for _, test := range basic {
		s, syntaxErr := queryparser.Parse(&db, test.query)
		if syntaxErr != nil {
			t.Errorf("query: %s; unexpected error: got %v, want nil", test.query, syntaxErr)
		}
		err := querychecker.Check(&db, s)
		if err != nil {
			t.Errorf("query: %s; got %v, want: nil", test.query, err)
		}
	}
}

func appendZeroByte(start string) string {
	limit := []byte(start)
	limit = append(limit, 0)
	return string(limit)

}

func TestKeyRanges(t *testing.T) {
	basic := []keyRangesTest{
		// Select
		{
			"select k, v from Customer",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: true,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "", Limit: ""},
				},
			},
		},
		{
			"select k, v from Customer where k = \"abc\" or k = \"def\"",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "abc", Limit: appendZeroByte("abc")},
					ds.StringFieldRange{Start: "def", Limit: appendZeroByte("def")},
				},
			},
		},
		{
			"select k, v from Customer where \"abc\" = k or \"def\" = k",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "abc", Limit: appendZeroByte("abc")},
					ds.StringFieldRange{Start: "def", Limit: appendZeroByte("def")},
				},
			},
		},
		{
			"select k, v from Customer where k >= \"foo\" and k < \"goo\"",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "foo", Limit: "goo"},
				},
			},
		},
		{
			"select k, v from Customer where \"foo\" <= k and \"goo\" >= k",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "foo", Limit: appendZeroByte("goo")},
				},
			},
		},
		{
			"select k, v from Customer where k <> \"foo\"",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "", Limit: "foo"},
					ds.StringFieldRange{Start: appendZeroByte("foo"), Limit: ""},
				},
			},
		},
		{
			"select k, v from Customer where k <> \"foo\" and k > \"bar\"",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: appendZeroByte("bar"), Limit: "foo"},
					ds.StringFieldRange{Start: appendZeroByte("foo"), Limit: ""},
				},
			},
		},
		{
			"select k, v from Customer where k <> \"foo\" or k > \"bar\"",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "", Limit: ""},
				},
			},
		},
		{
			"select k, v from Customer where k <> \"bar\" or k > \"foo\"",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "", Limit: "bar"},
					ds.StringFieldRange{Start: appendZeroByte("bar"), Limit: ""},
				},
			},
		},
		{
			"select v from Customer where Type(v) = \"Foo.Bar\" and k >= \"100\" and k < \"200\" and v.foo > 50 and v.bar <= 1000 and v.baz <> -20.7",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "100", Limit: "200"},
				},
			},
		},
		{
			"select k, v from Customer where k = \"abc\" and k = \"def\"",
			&ds.IndexRanges{
				FieldName:    "k",
				Kind:         vdl.String,
				NilAllowed:   false,
				StringRanges: &ds.StringFieldRanges{},
			},
		},
		{
			"select k, v from Customer where Type(v) = \"Foo.Bar\" and k like \"abc%\" limit 100 offset 200",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "abc", Limit: "abd"},
				},
			},
		},
		{
			"select  k,  v from \n  Customer where k like \"002%\" or k like \"001%\" or k like \"%\"",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "", Limit: ""},
				},
			},
		},
		{
			"select k, v from Customer where k = \"Foo.Bar\" and k like \"abc%\" limit 100 offset 200",
			&ds.IndexRanges{
				FieldName:    "k",
				Kind:         vdl.String,
				NilAllowed:   false,
				StringRanges: &ds.StringFieldRanges{},
			},
		},
		{
			"select k, v from Customer where k like \"foo%\" and k like \"bar%\"",
			&ds.IndexRanges{
				FieldName:    "k",
				Kind:         vdl.String,
				NilAllowed:   false,
				StringRanges: &ds.StringFieldRanges{},
			},
		},
		{
			"select k, v from Customer where k like \"foo%\" or k like \"bar%\"",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "bar", Limit: "bas"},
					ds.StringFieldRange{Start: "foo", Limit: "fop"},
				},
			},
		},
		{
			// Note: 'like "Foo"' is optimized to '= "Foo"'
			"select k, v from Customer where k = \"Foo.Bar\" or k like \"Foo\" or k like \"abc%\" limit 100 offset 200",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "Foo", Limit: appendZeroByte("Foo")},
					ds.StringFieldRange{Start: "Foo.Bar", Limit: appendZeroByte("Foo.Bar")},
					ds.StringFieldRange{Start: "abc", Limit: "abd"},
				},
			},
		},
		{
			"select k, v from Customer where k like \"Foo@%Bar\" or k like \"abc%\" limit 100 offset 200 escape '@'",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "Foo%Bar", Limit: appendZeroByte("Foo%Bar")},
					ds.StringFieldRange{Start: "abc", Limit: "abd"},
				},
			},
		},
		{
			"select k, v from Customer where k like \"Foo%Bar\" or k like \"abc%\" limit 100 offset 200",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "Foo", Limit: "Fop"},
					ds.StringFieldRange{Start: "abc", Limit: "abd"},
				},
			},
		},
		{
			"select k, v from Customer where k like \"Foo#%Bar\" or k like \"abc%\" escape '#' limit 100 offset 200",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "Foo%Bar", Limit: appendZeroByte("Foo%Bar")},
					ds.StringFieldRange{Start: "abc", Limit: "abd"},
				},
			},
		},
		{
			"select k, v from Customer where k not like \"002%\"",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "", Limit: "002"},
					ds.StringFieldRange{Start: "003", Limit: ""},
				},
			},
		},
		// Delete
		{
			"delete from Customer",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: true,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "", Limit: ""},
				},
			},
		},
		{
			"delete from Customer where k = \"abc\" or k = \"def\"",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "abc", Limit: appendZeroByte("abc")},
					ds.StringFieldRange{Start: "def", Limit: appendZeroByte("def")},
				},
			},
		},
		{
			"delete from Customer where \"abc\" = k or \"def\" = k",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "abc", Limit: appendZeroByte("abc")},
					ds.StringFieldRange{Start: "def", Limit: appendZeroByte("def")},
				},
			},
		},
		{
			"delete from Customer where k >= \"foo\" and k < \"goo\"",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "foo", Limit: "goo"},
				},
			},
		},
		{
			"delete from Customer where \"foo\" <= k and \"goo\" >= k",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "foo", Limit: appendZeroByte("goo")},
				},
			},
		},
		{
			"delete from Customer where k <> \"foo\"",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "", Limit: "foo"},
					ds.StringFieldRange{Start: appendZeroByte("foo"), Limit: ""},
				},
			},
		},
		{
			"delete from Customer where k <> \"foo\" and k > \"bar\"",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: appendZeroByte("bar"), Limit: "foo"},
					ds.StringFieldRange{Start: appendZeroByte("foo"), Limit: ""},
				},
			},
		},
		{
			"delete from Customer where k <> \"foo\" or k > \"bar\"",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "", Limit: ""},
				},
			},
		},
		{
			"delete from Customer where k <> \"bar\" or k > \"foo\"",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "", Limit: "bar"},
					ds.StringFieldRange{Start: appendZeroByte("bar"), Limit: ""},
				},
			},
		},
		{
			"select v from Customer where Type(v) = \"Foo.Bar\" and k >= \"100\" and k < \"200\" and v.foo > 50 and v.bar <= 1000 and v.baz <> -20.7",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "100", Limit: "200"},
				},
			},
		},
		{
			"delete from Customer where k = \"abc\" and k = \"def\"",
			&ds.IndexRanges{
				FieldName:    "k",
				Kind:         vdl.String,
				NilAllowed:   false,
				StringRanges: &ds.StringFieldRanges{},
			},
		},
		{
			"delete from Customer where Type(v) = \"Foo.Bar\" and k like \"abc%\" limit 100",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "abc", Limit: "abd"},
				},
			},
		},
		{
			"select  k,  v from \n  Customer where k like \"002%\" or k like \"001%\" or k like \"%\"",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "", Limit: ""},
				},
			},
		},
		{
			"delete from Customer where k = \"Foo.Bar\" and k like \"abc%\" limit 100",
			&ds.IndexRanges{
				FieldName:    "k",
				Kind:         vdl.String,
				NilAllowed:   false,
				StringRanges: &ds.StringFieldRanges{},
			},
		},
		{
			"delete from Customer where k like \"foo%\" and k like \"bar%\"",
			&ds.IndexRanges{
				FieldName:    "k",
				Kind:         vdl.String,
				NilAllowed:   false,
				StringRanges: &ds.StringFieldRanges{},
			},
		},
		{
			"delete from Customer where k like \"foo%\" or k like \"bar%\"",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "bar", Limit: "bas"},
					ds.StringFieldRange{Start: "foo", Limit: "fop"},
				},
			},
		},
		{
			// Note: 'like "Foo"' is optimized to '= "Foo"'
			"delete from Customer where k = \"Foo.Bar\" or k like \"Foo\" or k like \"abc%\" limit 100",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "Foo", Limit: appendZeroByte("Foo")},
					ds.StringFieldRange{Start: "Foo.Bar", Limit: appendZeroByte("Foo.Bar")},
					ds.StringFieldRange{Start: "abc", Limit: "abd"},
				},
			},
		},
		{
			"delete from Customer where k like \"Foo@%Bar\" or k like \"abc%\" limit 100 escape '@'",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "Foo%Bar", Limit: appendZeroByte("Foo%Bar")},
					ds.StringFieldRange{Start: "abc", Limit: "abd"},
				},
			},
		},
		{
			"delete from Customer where k like \"Foo%Bar\" or k like \"abc%\" limit 100",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "Foo", Limit: "Fop"},
					ds.StringFieldRange{Start: "abc", Limit: "abd"},
				},
			},
		},
		{
			"delete from Customer where k like \"Foo#%Bar\" or k like \"abc%\" escape '#' limit 100",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "Foo%Bar", Limit: appendZeroByte("Foo%Bar")},
					ds.StringFieldRange{Start: "abc", Limit: "abd"},
				},
			},
		},
		{
			"delete from Customer where k not like \"002%\"",
			&ds.IndexRanges{
				FieldName:  "k",
				Kind:       vdl.String,
				NilAllowed: false,
				StringRanges: &ds.StringFieldRanges{
					ds.StringFieldRange{Start: "", Limit: "002"},
					ds.StringFieldRange{Start: "003", Limit: ""},
				},
			},
		},
	}

	for _, test := range basic {
		s, syntaxErr := queryparser.Parse(&db, test.query)
		if syntaxErr != nil {
			t.Errorf("query: %s; unexpected error: got %v, want nil", test.query, syntaxErr)
		}
		err := querychecker.Check(&db, s)
		if err != nil {
			t.Errorf("query: %s; got %v, want: nil", test.query, err)
		}
		switch sel := (*s).(type) {
		case queryparser.SelectStatement:
			indexRanges := querychecker.CompileIndexRanges(&queryparser.Field{Segments: []queryparser.Segment{{Value: "k"}}}, vdl.String, sel.Where)
			if !reflect.DeepEqual(test.indexRanges, indexRanges) {
				t.Errorf("query: %s;\nGOT  %s\nWANT %s", test.query, indexRanges.String(), test.indexRanges.String())
			}
		case queryparser.DeleteStatement:
			indexRanges := querychecker.CompileIndexRanges(&queryparser.Field{Segments: []queryparser.Segment{{Value: "k"}}}, vdl.String, sel.Where)
			if !reflect.DeepEqual(test.indexRanges, indexRanges) {
				t.Errorf("query: %s;\nGOT  %s\nWANT %s", test.query, indexRanges.String(), test.indexRanges.String())
			}
		default:
			t.Errorf("query: %s;\nGOT  %v\nWANT select or delete statement", test.query, *s)
		}
	}
}

func TestIndexRanges(t *testing.T) {
	basic := []indexRangesTest{
		// Select
		{
			"select k, v from Customer",
			[]string{"v.InterfaceName"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.InterfaceName",
					Kind:       vdl.String,
					NilAllowed: true,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "", Limit: ""},
					},
				},
			},
		},
		{
			"select k, v from Customer where v.Foo = 12",
			[]string{"v.InterfaceName"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.InterfaceName",
					Kind:       vdl.String,
					NilAllowed: true,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "", Limit: ""},
					},
				},
			},
		},
		{
			"select k, v from Customer where v.InterfaceName = \"FooBar\"",
			[]string{"v.InterfaceName"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.InterfaceName",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "FooBar", Limit: appendZeroByte("FooBar")},
					},
				},
			},
		},
		{
			"select k, v from Customer where v.InterfaceName = \"Foo\" or v.InterfaceName = \"Bar\"",
			[]string{"v.InterfaceName"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.InterfaceName",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "Bar", Limit: appendZeroByte("Bar")},
						ds.StringFieldRange{Start: "Foo", Limit: appendZeroByte("Foo")},
					},
				},
			},
		},
		{
			"select k, v from Customer where v.InterfaceName = \"Foo\" and v.InterfaceName = \"Bar\"",
			[]string{"v.InterfaceName"},
			[]*ds.IndexRanges{
				{
					FieldName:    "v.InterfaceName",
					Kind:         vdl.String,
					NilAllowed:   false,
					StringRanges: &ds.StringFieldRanges{},
				},
			},
		},
		{
			"select k, v from Customer where v.InterfaceName like \"Foo%\"",
			[]string{"v.InterfaceName"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.InterfaceName",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "Foo", Limit: "Fop"},
					},
				},
			},
		},
		{
			"select k, v from Customer where v.InterfaceName like \"Foo%\" and v.Address like \"Bar%\"",
			[]string{"v.InterfaceName", "v.Address"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.InterfaceName",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "Foo", Limit: "Fop"},
					},
				},
				{
					FieldName:  "v.Address",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "Bar", Limit: "Bas"},
					},
				},
			},
		},
		{
			"select k, v from Customer where \"abc\" = v.Foo or \"def\" = v.Foo",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "abc", Limit: appendZeroByte("abc")},
						ds.StringFieldRange{Start: "def", Limit: appendZeroByte("def")},
					},
				},
			},
		},
		{
			"select k, v from Customer where v.Foo >= \"foo\" and v.Foo < \"goo\"",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "foo", Limit: "goo"},
					},
				},
			},
		},
		{
			"select k, v from Customer where \"foo\" <= v.Foo and \"goo\" >= v.Foo",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "foo", Limit: appendZeroByte("goo")},
					},
				},
			},
		},
		{
			"select k, v from Customer where v.Foo <> \"foo\"",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "", Limit: "foo"},
						ds.StringFieldRange{Start: appendZeroByte("foo"), Limit: ""},
					},
				},
			},
		},
		{
			"select k, v from Customer where v.Foo <> \"foo\" and v.Foo > \"bar\"",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: appendZeroByte("bar"), Limit: "foo"},
						ds.StringFieldRange{Start: appendZeroByte("foo"), Limit: ""},
					},
				},
			},
		},
		{
			"select k, v from Customer where v.Foo <> \"foo\" or v.Foo > \"bar\"",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "", Limit: ""},
					},
				},
			},
		},
		{
			"select k, v from Customer where v.Foo <> \"bar\" or v.Foo > \"foo\"",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "", Limit: "bar"},
						ds.StringFieldRange{Start: appendZeroByte("bar"), Limit: ""},
					},
				},
			},
		},
		{
			"select v from Customer where Type(v) = \"Foo.Bar\" and v.Foo >= \"100\" and v.Foo < \"200\" and v.foo > 50 and v.bar <= 1000 and v.baz <> -20.7",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "100", Limit: "200"},
					},
				},
			},
		},
		{
			"select k, v from Customer where v.Foo = \"abc\" and v.Foo = \"def\"",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:    "v.Foo",
					Kind:         vdl.String,
					NilAllowed:   false,
					StringRanges: &ds.StringFieldRanges{},
				},
			},
		},
		{
			"select k, v from Customer where Type(v) = \"Foo.Bar\" and v.Foo like \"abc%\" limit 100 offset 200",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "abc", Limit: "abd"},
					},
				},
			},
		},
		{
			"select  k,  v from \n  Customer where v.Foo like \"002%\" or v.Foo like \"001%\" or v.Foo like \"%\"",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "", Limit: ""},
					},
				},
			},
		},
		{
			"select k, v from Customer where v.Foo = \"Foo.Bar\" and v.Foo like \"abc%\" limit 100 offset 200",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:    "v.Foo",
					Kind:         vdl.String,
					NilAllowed:   false,
					StringRanges: &ds.StringFieldRanges{},
				},
			},
		},
		{
			"select k, v from Customer where v.Foo like \"foo%\" and v.Foo like \"bar%\"",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:    "v.Foo",
					Kind:         vdl.String,
					NilAllowed:   false,
					StringRanges: &ds.StringFieldRanges{},
				},
			},
		},
		{
			"select k, v from Customer where v.Foo like \"foo%\" or v.Foo like \"bar%\"",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "bar", Limit: "bas"},
						ds.StringFieldRange{Start: "foo", Limit: "fop"},
					},
				},
			},
		},
		{
			// Note: 'like "Foo"' is optimized to '= "Foo"'
			"select k, v from Customer where v.Foo = \"Foo.Bar\" or v.Foo like \"Foo\" or v.Foo like \"abc%\" limit 100 offset 200",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "Foo", Limit: appendZeroByte("Foo")},
						ds.StringFieldRange{Start: "Foo.Bar", Limit: appendZeroByte("Foo.Bar")},
						ds.StringFieldRange{Start: "abc", Limit: "abd"},
					},
				},
			},
		},
		{
			"select k, v from Customer where v.Foo like \"Foo@%Bar\" or v.Foo like \"abc%\" limit 100 offset 200 escape '@'",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "Foo%Bar", Limit: appendZeroByte("Foo%Bar")},
						ds.StringFieldRange{Start: "abc", Limit: "abd"},
					},
				},
			},
		},
		{
			"select k, v from Customer where v.Foo like \"Foo%Bar\" or v.Foo like \"abc%\" limit 100 offset 200",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "Foo", Limit: "Fop"},
						ds.StringFieldRange{Start: "abc", Limit: "abd"},
					},
				},
			},
		},
		{
			"select k, v from Customer where v.Foo like \"Foo#%Bar\" or v.Foo like \"abc%\" escape '#' limit 100 offset 200",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "Foo%Bar", Limit: appendZeroByte("Foo%Bar")},
						ds.StringFieldRange{Start: "abc", Limit: "abd"},
					},
				},
			},
		},
		{
			"select k, v from Customer where v.Foo not like \"002%\"",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "", Limit: "002"},
						ds.StringFieldRange{Start: "003", Limit: ""},
					},
				},
			},
		},
		// delete
		{
			"delete from Customer",
			[]string{"v.InterfaceName"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.InterfaceName",
					Kind:       vdl.String,
					NilAllowed: true,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "", Limit: ""},
					},
				},
			},
		},
		{
			"delete from Customer where v.Foo = 12",
			[]string{"v.InterfaceName"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.InterfaceName",
					Kind:       vdl.String,
					NilAllowed: true,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "", Limit: ""},
					},
				},
			},
		},
		{
			"delete from Customer where v.InterfaceName = \"FooBar\"",
			[]string{"v.InterfaceName"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.InterfaceName",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "FooBar", Limit: appendZeroByte("FooBar")},
					},
				},
			},
		},
		{
			"delete from Customer where v.InterfaceName = \"Foo\" or v.InterfaceName = \"Bar\"",
			[]string{"v.InterfaceName"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.InterfaceName",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "Bar", Limit: appendZeroByte("Bar")},
						ds.StringFieldRange{Start: "Foo", Limit: appendZeroByte("Foo")},
					},
				},
			},
		},
		{
			"delete from Customer where v.InterfaceName = \"Foo\" and v.InterfaceName = \"Bar\"",
			[]string{"v.InterfaceName"},
			[]*ds.IndexRanges{
				{
					FieldName:    "v.InterfaceName",
					Kind:         vdl.String,
					NilAllowed:   false,
					StringRanges: &ds.StringFieldRanges{},
				},
			},
		},
		{
			"delete from Customer where v.InterfaceName like \"Foo%\"",
			[]string{"v.InterfaceName"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.InterfaceName",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "Foo", Limit: "Fop"},
					},
				},
			},
		},
		{
			"delete from Customer where v.InterfaceName like \"Foo%\" and v.Address like \"Bar%\"",
			[]string{"v.InterfaceName", "v.Address"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.InterfaceName",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "Foo", Limit: "Fop"},
					},
				},
				{
					FieldName:  "v.Address",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "Bar", Limit: "Bas"},
					},
				},
			},
		},
		{
			"delete from Customer where \"abc\" = v.Foo or \"def\" = v.Foo",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "abc", Limit: appendZeroByte("abc")},
						ds.StringFieldRange{Start: "def", Limit: appendZeroByte("def")},
					},
				},
			},
		},
		{
			"delete from Customer where v.Foo >= \"foo\" and v.Foo < \"goo\"",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "foo", Limit: "goo"},
					},
				},
			},
		},
		{
			"delete from Customer where \"foo\" <= v.Foo and \"goo\" >= v.Foo",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "foo", Limit: appendZeroByte("goo")},
					},
				},
			},
		},
		{
			"delete from Customer where v.Foo <> \"foo\"",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "", Limit: "foo"},
						ds.StringFieldRange{Start: appendZeroByte("foo"), Limit: ""},
					},
				},
			},
		},
		{
			"delete from Customer where v.Foo <> \"foo\" and v.Foo > \"bar\"",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: appendZeroByte("bar"), Limit: "foo"},
						ds.StringFieldRange{Start: appendZeroByte("foo"), Limit: ""},
					},
				},
			},
		},
		{
			"delete from Customer where v.Foo <> \"foo\" or v.Foo > \"bar\"",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "", Limit: ""},
					},
				},
			},
		},
		{
			"delete from Customer where v.Foo <> \"bar\" or v.Foo > \"foo\"",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "", Limit: "bar"},
						ds.StringFieldRange{Start: appendZeroByte("bar"), Limit: ""},
					},
				},
			},
		},
		{
			"select v from Customer where Type(v) = \"Foo.Bar\" and v.Foo >= \"100\" and v.Foo < \"200\" and v.foo > 50 and v.bar <= 1000 and v.baz <> -20.7",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "100", Limit: "200"},
					},
				},
			},
		},
		{
			"delete from Customer where v.Foo = \"abc\" and v.Foo = \"def\"",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:    "v.Foo",
					Kind:         vdl.String,
					NilAllowed:   false,
					StringRanges: &ds.StringFieldRanges{},
				},
			},
		},
		{
			"delete from Customer where Type(v) = \"Foo.Bar\" and v.Foo like \"abc%\" limit 100",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "abc", Limit: "abd"},
					},
				},
			},
		},
		{
			"select  k,  v from \n  Customer where v.Foo like \"002%\" or v.Foo like \"001%\" or v.Foo like \"%\"",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "", Limit: ""},
					},
				},
			},
		},
		{
			"delete from Customer where v.Foo = \"Foo.Bar\" and v.Foo like \"abc%\" limit 100",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:    "v.Foo",
					Kind:         vdl.String,
					NilAllowed:   false,
					StringRanges: &ds.StringFieldRanges{},
				},
			},
		},
		{
			"delete from Customer where v.Foo like \"foo%\" and v.Foo like \"bar%\"",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:    "v.Foo",
					Kind:         vdl.String,
					NilAllowed:   false,
					StringRanges: &ds.StringFieldRanges{},
				},
			},
		},
		{
			"delete from Customer where v.Foo like \"foo%\" or v.Foo like \"bar%\"",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "bar", Limit: "bas"},
						ds.StringFieldRange{Start: "foo", Limit: "fop"},
					},
				},
			},
		},
		{
			// Note: 'like "Foo"' is optimized to '= "Foo"'
			"delete from Customer where v.Foo = \"Foo.Bar\" or v.Foo like \"Foo\" or v.Foo like \"abc%\" limit 100",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "Foo", Limit: appendZeroByte("Foo")},
						ds.StringFieldRange{Start: "Foo.Bar", Limit: appendZeroByte("Foo.Bar")},
						ds.StringFieldRange{Start: "abc", Limit: "abd"},
					},
				},
			},
		},
		{
			"delete from Customer where v.Foo like \"Foo@%Bar\" or v.Foo like \"abc%\" limit 100 escape '@'",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "Foo%Bar", Limit: appendZeroByte("Foo%Bar")},
						ds.StringFieldRange{Start: "abc", Limit: "abd"},
					},
				},
			},
		},
		{
			"delete from Customer where v.Foo like \"Foo%Bar\" or v.Foo like \"abc%\" limit 100",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "Foo", Limit: "Fop"},
						ds.StringFieldRange{Start: "abc", Limit: "abd"},
					},
				},
			},
		},
		{
			"delete from Customer where v.Foo like \"Foo#%Bar\" or v.Foo like \"abc%\" escape '#' limit 100",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "Foo%Bar", Limit: appendZeroByte("Foo%Bar")},
						ds.StringFieldRange{Start: "abc", Limit: "abd"},
					},
				},
			},
		},
		{
			"delete from Customer where v.Foo not like \"002%\"",
			[]string{"v.Foo"},
			[]*ds.IndexRanges{
				{
					FieldName:  "v.Foo",
					Kind:       vdl.String,
					NilAllowed: false,
					StringRanges: &ds.StringFieldRanges{
						ds.StringFieldRange{Start: "", Limit: "002"},
						ds.StringFieldRange{Start: "003", Limit: ""},
					},
				},
			},
		},
	}

	for _, test := range basic {
		s, syntaxErr := queryparser.Parse(&db, test.query)
		if syntaxErr != nil {
			t.Errorf("query: %s; unexpected error: got %v, want nil", test.query, syntaxErr)
		}
		err := querychecker.Check(&db, s)
		if err != nil {
			t.Errorf("query: %s; got %v, want: nil", test.query, err)
		}
		switch sel := (*s).(type) {
		case queryparser.SelectStatement:
			for i, fieldName := range test.fieldNames {
				idxField, err := queryparser.ParseIndexField(&db, fieldName, "Services")
				if err != nil {
					t.Errorf("query: %s;\nGOT  %v\nWANT nil", test.query, err)
				}
				indexRanges := querychecker.CompileIndexRanges(idxField, vdl.String, sel.Where)
				if !reflect.DeepEqual(test.indexRanges[i], indexRanges) {
					t.Errorf("query: %s;\nGOT  %v\nWANT %v", test.query, indexRanges, test.indexRanges[i])
				}
			}
		case queryparser.DeleteStatement:
			for i, fieldName := range test.fieldNames {
				idxField, err := queryparser.ParseIndexField(&db, fieldName, "Services")
				if err != nil {
					t.Errorf("query: %s;\nGOT  %v\nWANT nil", test.query, err)
				}
				indexRanges := querychecker.CompileIndexRanges(idxField, vdl.String, sel.Where)
				if !reflect.DeepEqual(test.indexRanges[i], indexRanges) {
					t.Errorf("query: %s;\nGOT  %v\nWANT %v", test.query, indexRanges, test.indexRanges[i])
				}
			}
		default:
			t.Errorf("query: %s;\nGOT  %v\nWANT select or delete statement", test.query, *s)
		}
	}
}

func TestLikePatterns(t *testing.T) {
	basic := []likePatternsTest{
		// Select
		{
			"select v from Customer where v like \"abc%\"",
			[]string{"abc", "abcd", "abcabc"},
			[]string{"xabcd"},
		},
		{
			"select v from Customer where v like \"abc_\"",
			[]string{"abcd", "abc1"},
			[]string{"abc", "xabcd", "abcde"},
		},
		{
			"select v from Customer where v like \"*%*_%\" escape '*'",
			[]string{"%_", "%_abc"},
			[]string{"%a", "abc%_abc"},
		},
		{
			"select v from Customer where v like \"abc^_\" escape '^'",
			[]string{"abc_"},
			[]string{"abc", "xabcd", "abcde"},
		},
		{
			"select v from Customer where v like \"abc^%\" escape '^'",
			[]string{"abc%"},
			[]string{"abc", "xabcd", "abcde"},
		},
		{
			"select v from Customer where v like \"abc_efg\"",
			[]string{"abcdefg"},
			[]string{"abc", "xabcd", "abcde", "abcdefgh"},
		},
		{
			"select v from Customer where v like \"abc%def\"",
			[]string{"abcdefdef", "abcdef", "abcdefghidef"},
			[]string{"abcdefg", "abcdefde"},
		},
		{
			"select v from Customer where v like \"[0-9]*abc%def\"",
			[]string{"[0-9]*abcdefdef", "[0-9]*abcdef", "[0-9]*abcdefghidef"},
			[]string{"0abcdefg", "9abcdefde", "[0-9]abcdefg", "[0-9]abcdefg", "[0-9]abcdefg"},
		},
		// Delete
		{
			"delete from Customer where v like \"abc%\"",
			[]string{"abc", "abcd", "abcabc"},
			[]string{"xabcd"},
		},
		{
			"delete from Customer where v like \"abc_\"",
			[]string{"abcd", "abc1"},
			[]string{"abc", "xabcd", "abcde"},
		},
		{
			"delete from Customer where v like \"*%*_%\" escape '*'",
			[]string{"%_", "%_abc"},
			[]string{"%a", "abc%_abc"},
		},
		{
			"delete from Customer where v like \"abc^_\" escape '^'",
			[]string{"abc_"},
			[]string{"abc", "xabcd", "abcde"},
		},
		{
			"delete from Customer where v like \"abc^%\" escape '^'",
			[]string{"abc%"},
			[]string{"abc", "xabcd", "abcde"},
		},
		{
			"delete from Customer where v like \"abc_efg\"",
			[]string{"abcdefg"},
			[]string{"abc", "xabcd", "abcde", "abcdefgh"},
		},
		{
			"delete from Customer where v like \"abc%def\"",
			[]string{"abcdefdef", "abcdef", "abcdefghidef"},
			[]string{"abcdefg", "abcdefde"},
		},
		{
			"delete from Customer where v like \"[0-9]*abc%def\"",
			[]string{"[0-9]*abcdefdef", "[0-9]*abcdef", "[0-9]*abcdefghidef"},
			[]string{"0abcdefg", "9abcdefde", "[0-9]abcdefg", "[0-9]abcdefg", "[0-9]abcdefg"},
		},
	}

	for _, test := range basic {
		s, syntaxErr := queryparser.Parse(&db, test.query)
		if syntaxErr != nil {
			t.Errorf("query: %s; unexpected error: got %v, want nil", test.query, syntaxErr)
		}
		err := querychecker.Check(&db, s)
		if err != nil {
			t.Errorf("query: %s; got %v, want: nil", test.query, err)
		}
		switch sel := (*s).(type) {
		case queryparser.SelectStatement:
			// We know there is exactly one like expression and operand2 contains
			// a prefix and compiled pattern.
			p := sel.Where.Expr.Operand2.Pattern
			// Make sure all matches actually match.
			for _, m := range test.matches {
				if !p.MatchString(m) {
					t.Errorf("query: %s;Expected match: %s; \nGOT  false\nWANT true", test.query, m)
				}
			}
			// Make sure all nonMatches actually don't match.
			for _, n := range test.nonMatches {
				if p.MatchString(n) {
					t.Errorf("query: %s;Expected nonMatch: %s; \nGOT  true\nWANT false", test.query, n)
				}
			}
		case queryparser.DeleteStatement:
			// We know there is exactly one like expression and operand2 contains
			// a prefix and compiled pattern.
			p := sel.Where.Expr.Operand2.Pattern
			// Make sure all matches actually match.
			for _, m := range test.matches {
				if !p.MatchString(m) {
					t.Errorf("query: %s;Expected match: %s; \nGOT  false\nWANT true", test.query, m)
				}
			}
			// Make sure all nonMatches actually don't match.
			for _, n := range test.nonMatches {
				if p.MatchString(n) {
					t.Errorf("query: %s;Expected nonMatch: %s; \nGOT  true\nWANT false", test.query, n)
				}
			}
		}
	}
}

func TestQueryCheckerErrors(t *testing.T) {
	basic := []checkErrorTest{
		// Select
		{"select a from Customer", syncql.ErrorfInvalidSelectField(db.GetContext(), "[%v]select field must be 'k' or 'v[{.<ident>}...]'", 7)},
		{"select v from Bob", syncql.ErrorfTableCantAccess(db.GetContext(), "[%v]table %v does not exist (or cannot be accessed): %v", 14, "Bob", errors.New("No such table: Bob"))},
		{"select k.a from Customer", syncql.ErrorfDotNotationDisallowedForKey(db.GetContext(), "[%v]dot notation may not be used on a key field", 9)},
		{"select k from Customer where t.a = \"Foo.Bar\"", syncql.ErrorfBadFieldInWhere(db.GetContext(), "[%v]where field must be 'k' or 'v[{.<ident>}...]'", 29)},
		{"select v from Customer where a=1", syncql.ErrorfBadFieldInWhere(db.GetContext(), "[%v]where field must be 'k' or 'v[{.<ident>}...]'", 29)},
		{"select v from Customer limit 0", syncql.ErrorfLimitMustBeGt0(db.GetContext(), "[%v]limit must be > 0", 29)},
		{"select v.z from Customer where v.x like v.y", syncql.ErrorfLikeExpressionsRequireRhsString(db.GetContext(), "[%v]like expressions require right operand of type <string-literal>", 40)},
		{"select v.z from Customer where k like \"a^bc%\" escape '^'", syncql.ErrorfInvalidLikePattern(db.GetContext(), "[%v]Invalid like pattern: %v", 38, pattern.ErrorfInvalidEscape(nil, "[%v]'%v' is not a valid escape character", "b"))},
		{"select v from Customer where v.A > false", syncql.ErrorfBoolInvalidExpression(db.GetContext(), "[%v]boolean operands may only be used in equals and not equals expressions", 33)},
		{"select v from Customer where true <= v.A", syncql.ErrorfBoolInvalidExpression(db.GetContext(), "[%v]boolean operands may only be used in equals and not equals expressions", 34)},
		{"select v from Customer where Foo(\"2015/07/22\", true, 3.14157) = true", syncql.ErrorfFunctionNotFound(db.GetContext(), "[%v]function '%v' not found", 29, "Foo")},
		{"select v from Customer where nil is v.ZipCode", syncql.ErrorfIsIsNotRequireLhsValue(db.GetContext(), "[%v]'is/is not' expressions require left operand to be a value operand", 29)},
		{"select v from Customer where v.ZipCode is \"94303\"", syncql.ErrorfIsIsNotRequireRhsNil(db.GetContext(), "[%v]'is/is not' expressions require right operand to be nil", 42)},
		{"select v from Customer where v.ZipCode is 94303", syncql.ErrorfIsIsNotRequireRhsNil(db.GetContext(), "[%v]'is/is not' expressions require right operand to be nil", 42)},
		{"select v from Customer where v.ZipCode is true", syncql.ErrorfIsIsNotRequireRhsNil(db.GetContext(), "[%v]'is/is not' expressions require right operand to be nil", 42)},
		{"select v from Customer where v.ZipCode is 943.03", syncql.ErrorfIsIsNotRequireRhsNil(db.GetContext(), "[%v]'is/is not' expressions require right operand to be nil", 42)},
		{"select v from Customer where nil is not v.ZipCode", syncql.ErrorfIsIsNotRequireLhsValue(db.GetContext(), "[%v]'is/is not' expressions require left operand to be a value operand", 29)},
		{"select v from Customer where v.ZipCode is not \"94303\"", syncql.ErrorfIsIsNotRequireRhsNil(db.GetContext(), "[%v]'is/is not' expressions require right operand to be nil", 46)},
		{"select v from Customer where v.ZipCode is not 94303", syncql.ErrorfIsIsNotRequireRhsNil(db.GetContext(), "[%v]'is/is not' expressions require right operand to be nil", 46)},
		{"select v from Customer where v.ZipCode is not true", syncql.ErrorfIsIsNotRequireRhsNil(db.GetContext(), "[%v]'is/is not' expressions require right operand to be nil", 46)},
		{"select v from Customer where v.ZipCode is not 943.03", syncql.ErrorfIsIsNotRequireRhsNil(db.GetContext(), "[%v]'is/is not' expressions require right operand to be nil", 46)},
		{"select v from Customer where Type(v) = \"Customer\" and Year(v.InvoiceDate, \"ABC\") = 2015", syncql.ErrorfLocationConversionError(db.GetContext(), "[%v]can't convert to location: %v", 74, errors.New("unknown time zone ABC"))},
		{"select v from Customer where Type(v) = \"Customer\" and Month(v.InvoiceDate, \"ABC\") = 2015", syncql.ErrorfLocationConversionError(db.GetContext(), "[%v]can't convert to location: %v", 75, errors.New("unknown time zone ABC"))},
		{"select v from Customer where Type(v) = \"Customer\" and Day(v.InvoiceDate, \"ABC\") = 2015", syncql.ErrorfLocationConversionError(db.GetContext(), "[%v]can't convert to location: %v", 73, errors.New("unknown time zone ABC"))},
		{"select v from Customer where Type(v) = \"Customer\" and Hour(v.InvoiceDate, \"ABC\") = 2015", syncql.ErrorfLocationConversionError(db.GetContext(), "[%v]can't convert to location: %v", 74, errors.New("unknown time zone ABC"))},
		{"select v from Customer where Type(v) = \"Customer\" and Minute(v.InvoiceDate, \"ABC\") = 2015", syncql.ErrorfLocationConversionError(db.GetContext(), "[%v]can't convert to location: %v", 76, errors.New("unknown time zone ABC"))},
		{"select v from Customer where Type(v) = \"Customer\" and Second(v.InvoiceDate, \"ABC\") = 2015", syncql.ErrorfLocationConversionError(db.GetContext(), "[%v]can't convert to location: %v", 76, errors.New("unknown time zone ABC"))},
		{"select v from Customer where Type(v) = \"Customer\" and Now(v.InvoiceDate, \"ABC\") = 2015", syncql.ErrorfFunctionArgCount(db.GetContext(), "[%v]Function '%v' expects %v args, found: %v", 54, "Now", 0, 2)},
		{"select v from Customer where Type(v) = \"Customer\" and Lowercase(v.Name, 2) = \"smith\"", syncql.ErrorfFunctionArgCount(db.GetContext(), "[%v]Function '%v' expects %v args, found: %v", 54, "Lowercase", 1, 2)},
		{"select v from Customer where Type(v) = \"Customer\" and Uppercase(v.Name, 2) = \"SMITH\"", syncql.ErrorfFunctionArgCount(db.GetContext(), "[%v]Function '%v' expects %v args, found: %v", 54, "Uppercase", 1, 2)},
		{"select Time() from Customer", syncql.ErrorfFunctionArgCount(db.GetContext(), "[%v]Function '%v' expects %v args, found: %v", 7, "Time", 2, 0)},
		{"select Year(v.InvoiceDate, \"Foo\") from Customer where Type(v) = \"Invoice\"",
			syncql.ErrorfLocationConversionError(db.GetContext(), "[%v]can't convert to location: %v", 27, errors.New("unknown time zone Foo"))},
		{"select K from Customer where Type(v) = \"Invoice\"", syncql.ErrorfDidYouMeanLowercaseK(db.GetContext(), "[%v]did you mean: 'k'?", 7)},
		{"select V from Customer where Type(v) = \"Invoice\"", syncql.ErrorfDidYouMeanLowercaseV(db.GetContext(), "[%v]did you mean: 'v'?", 7)},
		{"select k from Customer where K = \"001\"", syncql.ErrorfDidYouMeanLowercaseK(db.GetContext(), "[%v]did you mean: 'k'?", 29)},
		{"select v from Customer where Type(V) = \"Invoice\"", syncql.ErrorfDidYouMeanLowercaseV(db.GetContext(), "[%v]did you mean: 'v'?", 34)},
		{"select K, V from Customer where Type(V) = \"Invoice\" and K = \"001\"", syncql.ErrorfDidYouMeanLowercaseK(db.GetContext(), "[%v]did you mean: 'k'?", 7)},
		// Delete
		{"delete from Bob", syncql.ErrorfTableCantAccess(db.GetContext(), "[%v]table %v does not exist (or cannot be accessed): %v", 12, "Bob", errors.New("No such table: Bob"))},
		{"delete from Customer where k.a = \"a\"", syncql.ErrorfDotNotationDisallowedForKey(db.GetContext(), "[%v]dot notation may not be used on a key field", 29)},
		{"delete from Customer where t.a = \"Foo.Bar\"", syncql.ErrorfBadFieldInWhere(db.GetContext(), "[%v]where field must be 'k' or 'v[{.<ident>}...]'", 27)},
		{"delete from Customer where a=1", syncql.ErrorfBadFieldInWhere(db.GetContext(), "[%v]where field must be 'k' or 'v[{.<ident>}...]'", 27)},
		{"delete from Customer limit 0", syncql.ErrorfLimitMustBeGt0(db.GetContext(), "[%v]limit must be > 0", 27)},
		{"delete from Customer where v.x like v.y", syncql.ErrorfLikeExpressionsRequireRhsString(db.GetContext(), "[%v]like expressions require right operand of type <string-literal>", 36)},
		{"delete from Customer where k like \"a^bc%\" escape '^'", syncql.ErrorfInvalidLikePattern(db.GetContext(), "[%v]Invalid like pattern: %v", 34, pattern.ErrorfInvalidEscape(nil, "[%v]'%v' is not a valid escape character", "b"))},
		{"delete from Customer where v.A > false", syncql.ErrorfBoolInvalidExpression(db.GetContext(), "[%v]boolean operands may only be used in equals and not equals expressions", 31)},
		{"delete from Customer where true <= v.A", syncql.ErrorfBoolInvalidExpression(db.GetContext(), "[%v]boolean operands may only be used in equals and not equals expressions", 32)},
		{"delete from Customer where Foo(\"2015/07/22\", true, 3.14157) = true", syncql.ErrorfFunctionNotFound(db.GetContext(), "[%v]function '%v' not found", 27, "Foo")},
		{"delete from Customer where nil is v.ZipCode", syncql.ErrorfIsIsNotRequireLhsValue(db.GetContext(), "[%v]'is/is not' expressions require left operand to be a value operand", 27)},
		{"delete from Customer where v.ZipCode is \"94303\"", syncql.ErrorfIsIsNotRequireRhsNil(db.GetContext(), "[%v]'is/is not' expressions require right operand to be nil", 40)},
		{"delete from Customer where v.ZipCode is 94303", syncql.ErrorfIsIsNotRequireRhsNil(db.GetContext(), "[%v]'is/is not' expressions require right operand to be nil", 40)},
		{"delete from Customer where v.ZipCode is true", syncql.ErrorfIsIsNotRequireRhsNil(db.GetContext(), "[%v]'is/is not' expressions require right operand to be nil", 40)},
		{"delete from Customer where v.ZipCode is 943.03", syncql.ErrorfIsIsNotRequireRhsNil(db.GetContext(), "[%v]'is/is not' expressions require right operand to be nil", 40)},
		{"delete from Customer where nil is not v.ZipCode", syncql.ErrorfIsIsNotRequireLhsValue(db.GetContext(), "[%v]'is/is not' expressions require left operand to be a value operand", 27)},
		{"delete from Customer where v.ZipCode is not \"94303\"", syncql.ErrorfIsIsNotRequireRhsNil(db.GetContext(), "[%v]'is/is not' expressions require right operand to be nil", 44)},
		{"delete from Customer where v.ZipCode is not 94303", syncql.ErrorfIsIsNotRequireRhsNil(db.GetContext(), "[%v]'is/is not' expressions require right operand to be nil", 44)},
		{"delete from Customer where v.ZipCode is not true", syncql.ErrorfIsIsNotRequireRhsNil(db.GetContext(), "[%v]'is/is not' expressions require right operand to be nil", 44)},
		{"delete from Customer where v.ZipCode is not 943.03", syncql.ErrorfIsIsNotRequireRhsNil(db.GetContext(), "[%v]'is/is not' expressions require right operand to be nil", 44)},
		{"delete from Customer where Type(v) = \"Customer\" and Year(v.InvoiceDate, \"ABC\") = 2015", syncql.ErrorfLocationConversionError(db.GetContext(), "[%v]can't convert to location: %v", 72, errors.New("unknown time zone ABC"))},
		{"delete from Customer where Type(v) = \"Customer\" and Month(v.InvoiceDate, \"ABC\") = 2015", syncql.ErrorfLocationConversionError(db.GetContext(), "[%v]can't convert to location: %v", 73, errors.New("unknown time zone ABC"))},
		{"delete from Customer where Type(v) = \"Customer\" and Day(v.InvoiceDate, \"ABC\") = 2015", syncql.ErrorfLocationConversionError(db.GetContext(), "[%v]can't convert to location: %v", 71, errors.New("unknown time zone ABC"))},
		{"delete from Customer where Type(v) = \"Customer\" and Hour(v.InvoiceDate, \"ABC\") = 2015", syncql.ErrorfLocationConversionError(db.GetContext(), "[%v]can't convert to location: %v", 72, errors.New("unknown time zone ABC"))},
		{"delete from Customer where Type(v) = \"Customer\" and Minute(v.InvoiceDate, \"ABC\") = 2015", syncql.ErrorfLocationConversionError(db.GetContext(), "[%v]can't convert to location: %v", 74, errors.New("unknown time zone ABC"))},
		{"delete from Customer where Type(v) = \"Customer\" and Second(v.InvoiceDate, \"ABC\") = 2015", syncql.ErrorfLocationConversionError(db.GetContext(), "[%v]can't convert to location: %v", 74, errors.New("unknown time zone ABC"))},
		{"delete from Customer where Type(v) = \"Customer\" and Now(v.InvoiceDate, \"ABC\") = 2015", syncql.ErrorfFunctionArgCount(db.GetContext(), "[%v]Function '%v' expects %v args, found: %v", 52, "Now", 0, 2)},
		{"delete from Customer where Type(v) = \"Customer\" and Lowercase(v.Name, 2) = \"smith\"", syncql.ErrorfFunctionArgCount(db.GetContext(), "[%v]Function '%v' expects %v args, found: %v", 52, "Lowercase", 1, 2)},
		{"delete from Customer where Type(v) = \"Customer\" and Uppercase(v.Name, 2) = \"SMITH\"", syncql.ErrorfFunctionArgCount(db.GetContext(), "[%v]Function '%v' expects %v args, found: %v", 52, "Uppercase", 1, 2)},
		{"delete from Customer where Time() > 0", syncql.ErrorfFunctionArgCount(db.GetContext(), "[%v]Function '%v' expects %v args, found: %v", 27, "Time", 2, 0)},
		{"delete from Customer where Year(v.InvoiceDate, \"Foo\") > 1 and Type(v) = \"Invoice\"",
			syncql.ErrorfLocationConversionError(db.GetContext(), "[%v]can't convert to location: %v", 47, errors.New("unknown time zone Foo"))},
		{"delete from Customer where K = \"1\" and Type(v) = \"Invoice\"", syncql.ErrorfDidYouMeanLowercaseK(db.GetContext(), "[%v]did you mean: 'k'?", 27)},
		{"delete from Customer where V = \"abc\" and Type(v) = \"Invoice\"", syncql.ErrorfDidYouMeanLowercaseV(db.GetContext(), "[%v]did you mean: 'v'?", 27)},
		{"delete from Customer where K = \"001\"", syncql.ErrorfDidYouMeanLowercaseK(db.GetContext(), "[%v]did you mean: 'k'?", 27)},
		{"delete from Customer where Type(V) = \"Invoice\"", syncql.ErrorfDidYouMeanLowercaseV(db.GetContext(), "[%v]did you mean: 'v'?", 32)},
		{"delete from Customer where Type(V) = \"Invoice\" and K = \"001\"", syncql.ErrorfDidYouMeanLowercaseV(db.GetContext(), "[%v]did you mean: 'v'?", 32)},
	}

	for _, test := range basic {
		s, syntaxErr := queryparser.Parse(&db, test.query)
		if syntaxErr != nil {
			t.Errorf("query: %s; unexpected error: got %v, want nil", test.query, syntaxErr)
		} else {
			err := querychecker.Check(&db, s)
			// Test both that the ID and offset are equal.
			testErrOff, _ := syncql.SplitError(test.err)
			errOff, _ := syncql.SplitError(err)
			if verror.ErrorID(test.err) != verror.ErrorID(err) || testErrOff != errOff {
				t.Errorf("query: %s; got %v, want %v", test.query, err, test.err)
			}
		}
	}
}
