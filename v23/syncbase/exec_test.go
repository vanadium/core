// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncbase_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/query/pattern"
	"v.io/v23/query/syncql"
	"v.io/v23/syncbase"
	"v.io/v23/syncbase/testdata"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	_ "v.io/x/ref/runtime/factories/roaming"
	tu "v.io/x/ref/services/syncbase/testutil"
)

var ctx *context.T
var db syncbase.Database
var cleanup func()

// In addition to populating store, populate these arrays to make
// specifying the wanted values in the tests easier.
type kv struct {
	key   string
	value *vdl.Value
}

var customerEntries []kv
var numbersEntries []kv
var fooEntries []kv
var keyIndexDataEntries []kv

var t2015 time.Time

var t2015_04 time.Time
var t2015_04_12 time.Time
var t2015_04_12_22 time.Time
var t2015_04_12_22_16 time.Time
var t2015_04_12_22_16_06 time.Time

var t2015_07 time.Time
var t2015_07_01 time.Time
var t2015_07_01_01_23_45 time.Time

var customerCollection syncbase.Collection
var numbersCollection syncbase.Collection
var fooCollection syncbase.Collection
var keyIndexDataCollection syncbase.Collection
var bigCollection syncbase.Collection

func setup(t *testing.T) {
	var sName string
	ctx, sName, cleanup = tu.SetupOrDie(nil)
	db = tu.CreateDatabase(t, ctx, syncbase.NewService(sName), "d")
	customerCollection = tu.CreateCollection(t, ctx, db, "Customer")
	numbersCollection = tu.CreateCollection(t, ctx, db, "Numbers")
	fooCollection = tu.CreateCollection(t, ctx, db, "Foo")
	keyIndexDataCollection = tu.CreateCollection(t, ctx, db, "KeyIndexData")
	bigCollection = tu.CreateCollection(t, ctx, db, "BigCollection")
}

func initCollections(t *testing.T) {
	t20150122131101, _ := time.Parse("Jan 2 2006 15:04:05 -0700 MST", "Jan 22 2015 13:11:01 -0800 PST")
	t20150210161202, _ := time.Parse("Jan 2 2006 15:04:05 -0700 MST", "Feb 10 2015 16:12:02 -0800 PST")
	t20150311101303, _ := time.Parse("Jan 2 2006 15:04:05 -0700 MST", "Mar 11 2015 10:13:03 -0700 PDT")
	t20150317111404, _ := time.Parse("Jan 2 2006 15:04:05 -0700 MST", "Mar 17 2015 11:14:04 -0700 PDT")
	t20150317131505, _ := time.Parse("Jan 2 2006 15:04:05 -0700 MST", "Mar 17 2015 13:15:05 -0700 PDT")
	t20150412221606, _ := time.Parse("Jan 2 2006 15:04:05 -0700 MST", "Apr 12 2015 22:16:06 -0700 PDT")
	t20150413141707, _ := time.Parse("Jan 2 2006 15:04:05 -0700 MST", "Apr 13 2015 14:17:07 -0700 PDT")

	t2015, _ = time.Parse("2006 MST", "2015 PST")

	t2015_04, _ = time.Parse("Jan 2006 MST", "Apr 2015 PDT")
	t2015_07, _ = time.Parse("Jan 2006 MST", "Jul 2015 PDT")

	t2015_04_12, _ = time.Parse("Jan 2 2006 MST", "Apr 12 2015 PDT")
	t2015_07_01, _ = time.Parse("Jan 2 2006 MST", "Jul 01 2015 PDT")

	t2015_04_12_22, _ = time.Parse("Jan 2 2006 15 MST", "Apr 12 2015 22 PDT")
	t2015_04_12_22_16, _ = time.Parse("Jan 2 2006 15:04 MST", "Apr 12 2015 22:16 PDT")
	t2015_04_12_22_16_06, _ = time.Parse("Jan 2 2006 15:04:05 MST", "Apr 12 2015 22:16:06 PDT")
	t2015_07_01_01_23_45, _ = time.Parse("Jan 2 2006 15:04:05 MST", "Jul 01 2015 01:23:45 PDT")

	k := "001"
	c := testdata.Customer{"John Smith", 1, true, testdata.AddressInfo{"1 Main St.", "Palo Alto", "CA", "94303"}, testdata.CreditReport{Agency: testdata.CreditAgencyEquifax, Report: testdata.AgencyReportEquifaxReport{testdata.EquifaxCreditReport{'A'}}}}
	customerEntries = append(customerEntries, kv{k, vdl.ValueOf(c)})
	if err := customerCollection.Put(ctx, k, c); err != nil {
		t.Fatalf("customerCollection.Put() failed: %v", err)
	}

	k = "001001"
	i := testdata.Invoice{1, 1000, t20150122131101, 42, testdata.AddressInfo{"1 Main St.", "Palo Alto", "CA", "94303"}}
	customerEntries = append(customerEntries, kv{k, vdl.ValueOf(i)})
	if err := customerCollection.Put(ctx, k, i); err != nil {
		t.Fatalf("customerCollection.Put() failed: %v", err)
	}

	k = "001002"
	i = testdata.Invoice{1, 1003, t20150210161202, 7, testdata.AddressInfo{"2 Main St.", "Palo Alto", "CA", "94303"}}
	customerEntries = append(customerEntries, kv{k, vdl.ValueOf(i)})
	if err := customerCollection.Put(ctx, k, i); err != nil {
		t.Fatalf("customerCollection.Put() failed: %v", err)
	}

	k = "001003"
	i = testdata.Invoice{1, 1005, t20150311101303, 88, testdata.AddressInfo{"3 Main St.", "Palo Alto", "CA", "94303"}}
	customerEntries = append(customerEntries, kv{k, vdl.ValueOf(i)})
	if err := customerCollection.Put(ctx, k, i); err != nil {
		t.Fatalf("customerCollection.Put() failed: %v", err)
	}

	k = "002"
	c = testdata.Customer{"Bat Masterson", 2, true, testdata.AddressInfo{"777 Any St.", "Collins", "IA", "50055"}, testdata.CreditReport{Agency: testdata.CreditAgencyTransUnion, Report: testdata.AgencyReportTransUnionReport{testdata.TransUnionCreditReport{80}}}}
	customerEntries = append(customerEntries, kv{k, vdl.ValueOf(c)})
	if err := customerCollection.Put(ctx, k, c); err != nil {
		t.Fatalf("customerCollection.Put() failed: %v", err)
	}

	k = "002001"
	i = testdata.Invoice{2, 1001, t20150317111404, 166, testdata.AddressInfo{"777 Any St.", "collins", "IA", "50055"}}
	customerEntries = append(customerEntries, kv{k, vdl.ValueOf(i)})
	if err := customerCollection.Put(ctx, k, i); err != nil {
		t.Fatalf("customerCollection.Put() failed: %v", err)
	}

	k = "002002"
	i = testdata.Invoice{2, 1002, t20150317131505, 243, testdata.AddressInfo{"888 Any St.", "collins", "IA", "50055"}}
	customerEntries = append(customerEntries, kv{k, vdl.ValueOf(i)})
	if err := customerCollection.Put(ctx, k, i); err != nil {
		t.Fatalf("customerCollection.Put() failed: %v", err)
	}

	k = "002003"
	i = testdata.Invoice{2, 1004, t20150412221606, 787, testdata.AddressInfo{"999 Any St.", "collins", "IA", "50055"}}
	customerEntries = append(customerEntries, kv{k, vdl.ValueOf(i)})
	if err := customerCollection.Put(ctx, k, i); err != nil {
		t.Fatalf("customerCollection.Put() failed: %v", err)
	}

	k = "002004"
	i = testdata.Invoice{2, 1006, t20150413141707, 88, testdata.AddressInfo{"101010 Any St.", "collins", "IA", "50055"}}
	customerEntries = append(customerEntries, kv{k, vdl.ValueOf(i)})
	if err := customerCollection.Put(ctx, k, i); err != nil {
		t.Fatalf("customerCollection.Put() failed: %v", err)
	}

	k = "003"
	c = testdata.Customer{"John \"JOS\" O'Steed", 3, true, testdata.AddressInfo{"100 Queen St.", "New London", "CT", "06320"}, testdata.CreditReport{Agency: testdata.CreditAgencyExperian, Report: testdata.AgencyReportExperianReport{testdata.ExperianCreditReport{testdata.ExperianRatingGood}}}}
	customerEntries = append(customerEntries, kv{k, vdl.ValueOf(c)})
	if err := customerCollection.Put(ctx, k, c); err != nil {
		t.Fatalf("customerCollection.Put() failed: %v", err)
	}

	k = "001"
	n := testdata.Numbers{byte(12), uint16(1234), uint32(5678), uint64(999888777666), int16(9876), int32(876543), int64(128), float32(3.14159), float64(2.71828182846)}
	numbersEntries = append(numbersEntries, kv{k, vdl.ValueOf(n)})
	if err := numbersCollection.Put(ctx, k, n); err != nil {
		t.Fatalf("numbersCollection.Put() failed: %v", err)
	}

	k = "002"
	n = testdata.Numbers{byte(9), uint16(99), uint32(999), uint64(9999999), int16(9), int32(99), int64(88), float32(1.41421356237), float64(1.73205080757)}
	numbersEntries = append(numbersEntries, kv{k, vdl.ValueOf(n)})
	if err := numbersCollection.Put(ctx, k, n); err != nil {
		t.Fatalf("numbersCollection.Put() failed: %v", err)
	}

	k = "003"
	n = testdata.Numbers{byte(210), uint16(210), uint32(210), uint64(210), int16(210), int32(210), int64(210), float32(210.0), float64(210.0)}
	numbersEntries = append(numbersEntries, kv{k, vdl.ValueOf(n)})
	if err := numbersCollection.Put(ctx, k, n); err != nil {
		t.Fatalf("numbersCollection.Put() failed: %v", err)
	}

	k = "001"
	f := testdata.FooType{testdata.BarType{testdata.BazType{"FooBarBaz", testdata.TitleOrValueTypeTitle{"Vice President"}}}}
	fooEntries = append(fooEntries, kv{k, vdl.ValueOf(f)})
	if err := fooCollection.Put(ctx, k, f); err != nil {
		t.Fatalf("fooCollection.Put() failed: %v", err)
	}

	k = "002"
	f = testdata.FooType{testdata.BarType{testdata.BazType{"BazBarFoo", testdata.TitleOrValueTypeValue{42}}}}
	fooEntries = append(fooEntries, kv{k, vdl.ValueOf(f)})
	if err := fooCollection.Put(ctx, k, f); err != nil {
		t.Fatalf("fooCollection.Put() failed: %v", err)
	}

	k = "aaa"
	kid := testdata.KeyIndexData{
		[4]string{"Fee", "Fi", "Fo", "Fum"},
		[]string{"I", "smell", "the", "blood", "of", "an", "Englishman"},
		map[int64]string{6: "six", 7: "seven"},
		map[string]struct{}{"I’ll grind his bones to mix my bread": {}},
	}
	keyIndexDataEntries = append(keyIndexDataEntries, kv{k, vdl.ValueOf(kid)})
	if err := keyIndexDataCollection.Put(ctx, k, kid); err != nil {
		t.Fatalf("keyIndexDataCollection.Put() failed: %v", err)
	}

	for i := 100; i < 301; i++ {
		k = fmt.Sprintf("%d", i)
		b := testdata.BigData{k}
		if err := bigCollection.Put(ctx, k, b); err != nil {
			t.Fatalf("bigCollection.Put() failed: %v", err)
		}
	}
}

type execSelectTest struct {
	query   string
	headers []string
	r       [][]*vdl.Value
}

type execDeleteTest struct {
	delQuery   string
	delHeaders []string
	delResults [][]*vdl.Value
	selQuery   string
	selHeaders []string
	selResults [][]*vdl.Value
}

type preExecFunctionTest struct {
	query   string
	headers []string
}

type execSelectErrorTest struct {
	query string
	err   error
}

type execSelectParamTest struct {
	query   string
	params  []interface{}
	headers []string
	r       [][]*vdl.Value
}

type execDeleteParamTest struct {
	delQuery   string
	delParams  []interface{}
	delHeaders []string
	delResults [][]*vdl.Value
	selQuery   string
	selHeaders []string
	selResults [][]*vdl.Value
}

type execSelectParamErrorTest struct {
	query  string
	params []interface{}
	err    error
}

func TestExecSelect(t *testing.T) {
	setup(t)
	defer cleanup()
	initCollections(t)
	basic := []execSelectTest{
		{
			// Select values for all customer records.
			"select v from Customer where Type(v) like \"%.Customer\"",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[0].value},
				[]*vdl.Value{customerEntries[4].value},
				[]*vdl.Value{customerEntries[9].value},
			},
		},
		{
			// Select values where v.InvoiceNum is nil
			// Since InvoiceNum does not exists for Customers,
			// this will return just customers.
			"select v from Customer where v.InvoiceNum is nil",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[0].value},
				[]*vdl.Value{customerEntries[4].value},
				[]*vdl.Value{customerEntries[9].value},
			},
		},
		{
			// Select values where v.InvoiceNum is nil
			// or v.Name is nil This will select all customers
			// with the former and all invoices with the latter.
			// Hence, all records are returned.
			"select v from Customer where v.InvoiceNum is nil or v.Name is nil",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[0].value},
				[]*vdl.Value{customerEntries[1].value},
				[]*vdl.Value{customerEntries[2].value},
				[]*vdl.Value{customerEntries[3].value},
				[]*vdl.Value{customerEntries[4].value},
				[]*vdl.Value{customerEntries[5].value},
				[]*vdl.Value{customerEntries[6].value},
				[]*vdl.Value{customerEntries[7].value},
				[]*vdl.Value{customerEntries[8].value},
				[]*vdl.Value{customerEntries[9].value},
			},
		},
		{
			// Select values where v.InvoiceNum is nil
			// and v.Name is nil.  Expect nothing returned.
			"select v from Customer where v.InvoiceNum is nil and v.Name is nil",
			[]string{"v"},
			[][]*vdl.Value{},
		},
		{
			// Select values where v.InvoiceNum is not nil
			// This will return just invoices.
			"select v from Customer where v.InvoiceNum is not nil",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[1].value},
				[]*vdl.Value{customerEntries[2].value},
				[]*vdl.Value{customerEntries[3].value},
				[]*vdl.Value{customerEntries[5].value},
				[]*vdl.Value{customerEntries[6].value},
				[]*vdl.Value{customerEntries[7].value},
				[]*vdl.Value{customerEntries[8].value},
			},
		},
		{
			// Select values where v.InvoiceNum is not nil
			// or v.Name is not nil. All records are returned.
			"select v from Customer where v.InvoiceNum is not nil or v.Name is not nil",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[0].value},
				[]*vdl.Value{customerEntries[1].value},
				[]*vdl.Value{customerEntries[2].value},
				[]*vdl.Value{customerEntries[3].value},
				[]*vdl.Value{customerEntries[4].value},
				[]*vdl.Value{customerEntries[5].value},
				[]*vdl.Value{customerEntries[6].value},
				[]*vdl.Value{customerEntries[7].value},
				[]*vdl.Value{customerEntries[8].value},
				[]*vdl.Value{customerEntries[9].value},
			},
		},
		{
			// Select values where v.InvoiceNum is nil and v.Name is not nil.
			// All customers are returned.
			"select v from Customer where v.InvoiceNum is nil and v.Name is not nil",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[0].value},
				[]*vdl.Value{customerEntries[4].value},
				[]*vdl.Value{customerEntries[9].value},
			},
		},
		{
			// Select values where v.InvoiceNum is not nil
			// and v.Name is not nil.  Expect nothing returned.
			"select v from Customer where v.InvoiceNum is not nil and v.Name is not nil",
			[]string{"v"},
			[][]*vdl.Value{},
		},
		{
			// Select keys & values for all customer records.
			"select k, v from Customer where Type(v) like \"%.Customer\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[0].key), customerEntries[0].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[4].key), customerEntries[4].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[9].key), customerEntries[9].value},
			},
		},
		{
			// Select keys & names for all customer records.
			"select k, v.Name from Customer where Type(v) like \"%.Customer\"",
			[]string{"k", "v.Name"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[0].key), vdl.ValueOf("John Smith")},
				[]*vdl.Value{vdl.ValueOf(customerEntries[4].key), vdl.ValueOf("Bat Masterson")},
				[]*vdl.Value{vdl.ValueOf(customerEntries[9].key), vdl.ValueOf("John \"JOS\" O'Steed")},
			},
		},
		{
			// Select both customer and invoice records.
			// Customer records have Id.
			// Invoice records have CustId.
			"select v.Id, v.CustId from Customer",
			[]string{"v.Id", "v.CustId"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(int64(1)), vdl.ValueOf(nil)},
				[]*vdl.Value{vdl.ValueOf(nil), vdl.ValueOf(int64(1))},
				[]*vdl.Value{vdl.ValueOf(nil), vdl.ValueOf(int64(1))},
				[]*vdl.Value{vdl.ValueOf(nil), vdl.ValueOf(int64(1))},
				[]*vdl.Value{vdl.ValueOf(int64(2)), vdl.ValueOf(nil)},
				[]*vdl.Value{vdl.ValueOf(nil), vdl.ValueOf(int64(2))},
				[]*vdl.Value{vdl.ValueOf(nil), vdl.ValueOf(int64(2))},
				[]*vdl.Value{vdl.ValueOf(nil), vdl.ValueOf(int64(2))},
				[]*vdl.Value{vdl.ValueOf(nil), vdl.ValueOf(int64(2))},
				[]*vdl.Value{vdl.ValueOf(int64(3)), vdl.ValueOf(nil)},
			},
		},
		{
			// Select keys & values fo all invoice records.
			"select k, v from Customer where Type(v) like \"%.Invoice\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[1].key), customerEntries[1].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[2].key), customerEntries[2].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[3].key), customerEntries[3].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key), customerEntries[5].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key), customerEntries[6].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key), customerEntries[7].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key), customerEntries[8].value},
			},
		},
		{
			// Select key, cust id, invoice number and amount for $88 invoices.
			"select k, v.CustId as ID, v.InvoiceNum as InvoiceNumber, v.Amount as Amt from Customer where Type(v) like \"%.Invoice\" and v.Amount = 88",
			[]string{"k", "ID", "InvoiceNumber", "Amt"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[3].key), vdl.ValueOf(int64(1)), vdl.ValueOf(int64(1005)), vdl.ValueOf(int64(88))},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key), vdl.ValueOf(int64(2)), vdl.ValueOf(int64(1006)), vdl.ValueOf(int64(88))},
			},
		},
		{
			// Select keys & values for all records with a key prefix of "001".
			"select k, v from Customer where k like \"001%\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[0].key), customerEntries[0].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[1].key), customerEntries[1].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[2].key), customerEntries[2].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[3].key), customerEntries[3].value},
			},
		},
		{
			// Select keys & values for all records with a key prefix of "002".
			"select k, v from Customer where k like \"002%\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[4].key), customerEntries[4].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key), customerEntries[5].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key), customerEntries[6].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key), customerEntries[7].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key), customerEntries[8].value},
			},
		},
		{
			// Select keys & values for all records with NOT key prefix "002%".
			"select k, v from Customer where k not like \"002%\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[0].key), customerEntries[0].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[1].key), customerEntries[1].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[2].key), customerEntries[2].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[3].key), customerEntries[3].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[9].key), customerEntries[9].value},
			},
		},
		{
			// Select keys & values for all records with NOT key prefix "002".
			// Will be optimized to k <> "002"
			"select k, v from Customer where k not like \"002\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[0].key), customerEntries[0].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[1].key), customerEntries[1].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[2].key), customerEntries[2].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[3].key), customerEntries[3].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key), customerEntries[5].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key), customerEntries[6].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key), customerEntries[7].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key), customerEntries[8].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[9].key), customerEntries[9].value},
			},
		},
		{
			// Select keys & values for all records with a key prefix of "001".
			// or a key prefix of "002".
			"select k, v from Customer where k like \"001%\" or k like \"002%\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[0].key), customerEntries[0].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[1].key), customerEntries[1].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[2].key), customerEntries[2].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[3].key), customerEntries[3].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[4].key), customerEntries[4].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key), customerEntries[5].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key), customerEntries[6].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key), customerEntries[7].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key), customerEntries[8].value},
			},
		},
		{
			// Select keys & values for all records with a key prefix of "001".
			// or a key prefix of "002".
			"select k, v from Customer where k like \"002%\" or k like \"001%\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[0].key), customerEntries[0].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[1].key), customerEntries[1].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[2].key), customerEntries[2].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[3].key), customerEntries[3].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[4].key), customerEntries[4].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key), customerEntries[5].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key), customerEntries[6].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key), customerEntries[7].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key), customerEntries[8].value},
			},
		},
		{
			// Let's play with whitespace and mixed case.
			"   sElEcT  k,  v from \n  Customer WhErE k lIkE \"002%\" oR k LiKe \"001%\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[0].key), customerEntries[0].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[1].key), customerEntries[1].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[2].key), customerEntries[2].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[3].key), customerEntries[3].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[4].key), customerEntries[4].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key), customerEntries[5].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key), customerEntries[6].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key), customerEntries[7].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key), customerEntries[8].value},
			},
		},
		{
			// Add in a like clause that accepts all strings.
			"   sElEcT  k,  v from \n  Customer WhErE k lIkE \"002%\" oR k LiKe \"001%\" or k lIkE \"%\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[0].key), customerEntries[0].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[1].key), customerEntries[1].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[2].key), customerEntries[2].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[3].key), customerEntries[3].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[4].key), customerEntries[4].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key), customerEntries[5].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key), customerEntries[6].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key), customerEntries[7].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key), customerEntries[8].value},
				[]*vdl.Value{vdl.ValueOf(customerEntries[9].key), customerEntries[9].value},
			},
		},
		{
			// Select id, name for customers whose last name is Masterson.
			"select v.Id as ID, v.Name as Name from Customer where Type(v) like \"%.Customer\" and v.Name like \"%Masterson\"",
			[]string{"ID", "Name"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(int64(2)), vdl.ValueOf("Bat Masterson")},
			},
		},
		{
			// Select records where v.Address.City is "Collins" or type is Invoice.
			"select v from Customer where v.Address.City = \"Collins\" or Type(v) like \"%.Invoice\"",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[1].value},
				[]*vdl.Value{customerEntries[2].value},
				[]*vdl.Value{customerEntries[3].value},
				[]*vdl.Value{customerEntries[4].value},
				[]*vdl.Value{customerEntries[5].value},
				[]*vdl.Value{customerEntries[6].value},
				[]*vdl.Value{customerEntries[7].value},
				[]*vdl.Value{customerEntries[8].value},
			},
		},
		{
			// Select records where v.Address.City is "Collins" and v.InvoiceNum is not nil.
			"select v from Customer where v.Address.City = \"Collins\" and v.InvoiceNum is not nil",
			[]string{"v"},
			[][]*vdl.Value{},
		},
		{
			// Select records where v.Address.City is "Collins" and v.InvoiceNum is nil.
			"select v from Customer where v.Address.City = \"Collins\" and v.InvoiceNum is nil",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[4].value},
			},
		},
		{
			// Select customer name for customer Id (i.e., key) "001".
			"select v.Name as Name from Customer where Type(v) like \"%.Customer\" and k = \"001\"",
			[]string{"Name"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("John Smith")},
			},
		},
		{
			// Select v where v.Credit.Report.EquifaxReport.Rating = 'A'
			"select v from Customer where v.Credit.Report.EquifaxReport.Rating = 'A'",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[0].value},
			},
		},
		{
			// Select v where v.AgencyRating = "Bad"
			"select v from Customer where v.Credit.Report.EquifaxReport.Rating < 'A' or v.Credit.Report.ExperianReport.Rating = \"Bad\" or v.Credit.Report.TransUnionReport.Rating < 90",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[4].value},
			},
		},
		{
			// Select records where v.Bar.Baz.Name = "FooBarBaz"
			"select v from Foo where v.Bar.Baz.Name = \"FooBarBaz\"",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{fooEntries[0].value},
			},
		},
		{
			// Select records where v.Bar.Baz.TitleOrValue.Value = 42
			"select v from Foo where v.Bar.Baz.TitleOrValue.Value = 42",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{fooEntries[1].value},
			},
		},
		{
			// Select records where v.Bar.Baz.TitleOrValue.Title = "Vice President"
			"select v from Foo where v.Bar.Baz.TitleOrValue.Title = \"Vice President\"",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{fooEntries[0].value},
			},
		},
		{
			// Select records where v.Address.City = "Collins" or type is Invoice.
			// Limit 3
			"select v from Customer where v.Address.City = \"Collins\" or Type(v) like \"%.Invoice\" limit 3",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[1].value},
				[]*vdl.Value{customerEntries[2].value},
				[]*vdl.Value{customerEntries[3].value},
			},
		},
		{
			// Select records where v.Address.City = "Collins" or type is Invoice.
			// Offset 5
			"select v from Customer where v.Address.City = \"Collins\" or Type(v) like \"%.Invoice\" offset 5",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[6].value},
				[]*vdl.Value{customerEntries[7].value},
				[]*vdl.Value{customerEntries[8].value},
			},
		},
		{
			// Select records where v.Address.City = "Collins" is "Mountain View".
			"select v from Customer where v.Address.City = \"Mountain View\"",
			[]string{"v"},
			[][]*vdl.Value{},
		},
		{
			// Select records where v.Address.City = "Collins" or type is Invoice.
			// Offset 8
			"select v from Customer where v.Address.City = \"Collins\" or Type(v) like \"%.Invoice\" offset 8",
			[]string{"v"},
			[][]*vdl.Value{},
		},
		{
			// Select records where v.Address.City = "Collins" or type is Invoice.
			// Offset 23
			"select v from Customer where v.Address.City = \"Collins\" or Type(v) like \"%.Invoice\" offset 23",
			[]string{"v"},
			[][]*vdl.Value{},
		},
		{
			// Select records where v.Address.City = "Collins" is 84 or type is Invoice.
			// Limit 3 Offset 2
			"select v from Customer where v.Address.City = \"Collins\" or Type(v) like \"%.Invoice\" limit 3 offset 2",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[3].value},
				[]*vdl.Value{customerEntries[4].value},
				[]*vdl.Value{customerEntries[5].value},
			},
		},
		{
			// Select records where v.Address.City = "Collins" or (type is Invoice and v.InvoiceNum is not nil).
			// Limit 3 Offset 2
			"select v from Customer where v.Address.City = \"Collins\" or (Type(v) like \"%.Invoice\" and v.InvoiceNum is not nil) limit 3 offset 2",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[3].value},
				[]*vdl.Value{customerEntries[4].value},
				[]*vdl.Value{customerEntries[5].value},
			},
		},
		{
			// Select records where v.Address.City = "Collins" or (type is Invoice and v.InvoiceNum is nil).
			// Limit 3 Offset 2
			"select v from Customer where v.Address.City = \"Collins\" or (Type(v) like \"%.Invoice\" and v.InvoiceNum is nil) limit 3 offset 2",
			[]string{"v"},
			[][]*vdl.Value{},
		},
		// Test functions.
		{
			// Select invoice records where date is 2015-03-17
			"select v from Customer where Type(v) like \"%.Invoice\" and Year(v.InvoiceDate, \"America/Los_Angeles\") = 2015 and Month(v.InvoiceDate, \"America/Los_Angeles\") = 3 and Day(v.InvoiceDate, \"America/Los_Angeles\") = 17",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[5].value},
				[]*vdl.Value{customerEntries[6].value},
			},
		},
		{
			// Now will always be > 2012, so all customer records will be returned.
			"select v from Customer where Now() > Time(\"2006-01-02 MST\", \"2012-03-17 PDT\")",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[0].value},
				[]*vdl.Value{customerEntries[1].value},
				[]*vdl.Value{customerEntries[2].value},
				[]*vdl.Value{customerEntries[3].value},
				[]*vdl.Value{customerEntries[4].value},
				[]*vdl.Value{customerEntries[5].value},
				[]*vdl.Value{customerEntries[6].value},
				[]*vdl.Value{customerEntries[7].value},
				[]*vdl.Value{customerEntries[8].value},
				[]*vdl.Value{customerEntries[9].value},
			},
		},
		{
			// Select April 2015 PT invoices.
			// Note: this wouldn't work for March as daylight saving occurs March 8
			// and causes comparisons for those days to be off 1 hour.
			// It would work to use UTC -- see next test.
			"select k from Customer where Year(v.InvoiceDate, \"America/Los_Angeles\") = 2015 and Month(v.InvoiceDate, \"America/Los_Angeles\") = 4",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key)},
			},
		},
		{
			// Select March 2015 UTC invoices.
			"select k from Customer where Year(v.InvoiceDate, \"UTC\") = 2015 and Month(v.InvoiceDate, \"UTC\") = 3",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[3].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key)},
			},
		},
		{
			// Select 2015 UTC invoices.
			"select k from Customer where Year(v.InvoiceDate, \"UTC\") = 2015",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[1].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[2].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[3].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key)},
			},
		},
		{
			// Select the Mar 17 2015 11:14:04 America/Los_Angeles invoice.
			"select k from Customer where v.InvoiceDate = Time(\"2006-01-02 15:04:05 MST\", \"2015-03-17 11:14:04 PDT\")",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
			},
		},
		{
			// Select invoices in the minute Mar 17 2015 11:14 America/Los_Angeles invoice.
			"select k from Customer where Year(v.InvoiceDate, \"America/Los_Angeles\") = 2015 and Month(v.InvoiceDate, \"America/Los_Angeles\") = 3 and Day(v.InvoiceDate, \"America/Los_Angeles\") = 17 and Hour(v.InvoiceDate, \"America/Los_Angeles\") = 11 and Minute(v.InvoiceDate, \"America/Los_Angeles\") = 14",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
			},
		},
		{
			// Select invoices in the hour Mar 17 2015 11 hundred America/Los_Angeles invoice.
			"select k from Customer where Year(v.InvoiceDate, \"America/Los_Angeles\") = 2015 and Month(v.InvoiceDate, \"America/Los_Angeles\") = 3 and Day(v.InvoiceDate, \"America/Los_Angeles\") = 17 and Hour(v.InvoiceDate, \"America/Los_Angeles\") = 11",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
			},
		},
		{
			// Select invoices on the day Mar 17 2015 America/Los_Angeles invoice.
			"select k from Customer where Year(v.InvoiceDate, \"America/Los_Angeles\") = 2015 and Month(v.InvoiceDate, \"America/Los_Angeles\") = 3 and Day(v.InvoiceDate, \"America/Los_Angeles\") = 17",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key)},
			},
		},
		// Test string functions in where clause.
		{
			// Select invoices shipped to Any street -- using Lowercase.
			"select k from Customer where Type(v) like \"%.Invoice\" and Lowercase(v.ShipTo.Street) like \"%any%\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key)},
			},
		},
		{
			// Select invoices shipped to Any street -- using Uppercase.
			"select k from Customer where Type(v) like \"%.Invoice\" and Uppercase(v.ShipTo.Street) like \"%ANY%\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key)},
			},
		},
		// Select clause functions.
		// Time function
		{
			"select Time(\"2006-01-02 MST\", \"2015-07-01 PDT\") from Customer",
			[]string{"Time"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(t2015_07_01)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01)},
			},
		},
		// Time function
		{
			"select Time(\"2006-01-02 15:04:05 MST\", \"2015-07-01 01:23:45 PDT\") from Customer",
			[]string{"Time"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(t2015_07_01_01_23_45)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01_01_23_45)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01_01_23_45)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01_01_23_45)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01_01_23_45)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01_01_23_45)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01_01_23_45)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01_01_23_45)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01_01_23_45)},
				[]*vdl.Value{vdl.ValueOf(t2015_07_01_01_23_45)},
			},
		},
		// Lowercase function
		{
			"select Lowercase(v.Name) as name from Customer where Type(v) like \"%.Customer\"",
			[]string{"name"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("john smith")},
				[]*vdl.Value{vdl.ValueOf("bat masterson")},
				[]*vdl.Value{vdl.ValueOf("john \"jos\" o'steed")},
			},
		},
		// Uppercase function
		{
			"select Uppercase(v.Name) as NAME from Customer where Type(v) like \"%.Customer\"",
			[]string{"NAME"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("JOHN SMITH")},
				[]*vdl.Value{vdl.ValueOf("BAT MASTERSON")},
				[]*vdl.Value{vdl.ValueOf("JOHN \"JOS\" O'STEED")},
			},
		},
		// Second function
		{
			"select k, Second(v.InvoiceDate, \"America/Los_Angeles\") from Customer where Type(v) like \"%.Invoice\" and k = \"002003\"",
			[]string{
				"k",
				"Second",
			},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002003"), vdl.ValueOf(int64(6))},
			},
		},
		// Minute function
		{
			"select k, Minute(v.InvoiceDate, \"America/Los_Angeles\") from Customer where Type(v) like \"%.Invoice\" and k = \"002003\"",
			[]string{
				"k",
				"Minute",
			},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002003"), vdl.ValueOf(int64(16))},
			},
		},
		// Hour function
		{
			"select k, Hour(v.InvoiceDate, \"America/Los_Angeles\") from Customer where Type(v) like \"%.Invoice\" and k = \"002003\"",
			[]string{
				"k",
				"Hour",
			},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002003"), vdl.ValueOf(int64(22))},
			},
		},
		// Day function
		{
			"select k, Day(v.InvoiceDate, \"America/Los_Angeles\") from Customer where Type(v) like \"%.Invoice\" and k = \"002003\"",
			[]string{
				"k",
				"Day",
			},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002003"), vdl.ValueOf(int64(12))},
			},
		},
		// Month function
		{
			"select k, Month(v.InvoiceDate, \"America/Los_Angeles\") from Customer where Type(v) like \"%.Invoice\" and k = \"002003\"",
			[]string{
				"k",
				"Month",
			},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002003"), vdl.ValueOf(int64(4))},
			},
		},
		// Year function
		{
			"select k, Year(v.InvoiceDate, \"America/Los_Angeles\") from Customer where Type(v) like \"%.Invoice\" and k = \"001001\"",
			[]string{
				"k",
				"Year",
			},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001001"), vdl.ValueOf(int64(2015))},
			},
		},
		// Nested functions
		{
			"select Year(Time(\"2006-01-02 15:04:05 MST\", \"2015-07-01 01:23:45 PDT\"), \"America/Los_Angeles\")  from Customer where Type(v) like \"%.Invoice\" and k = \"001001\"",
			[]string{"Year"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(int64(2015))},
			},
		},
		// Bad arg to function.  Expression is false.
		{
			"select v from Customer where Type(v) like \"%.Invoice\" and Day(v.InvoiceDate, v.Foo) = v.InvoiceDate",
			[]string{"v"},
			[][]*vdl.Value{},
		},
		{
			// Test that all numeric types can compare to an uint64
			"select v from Numbers where v.Ui64 = v.B and v.Ui64 = v.Ui16 and v.Ui64 = v.Ui32 and v.Ui64 = v.F64 and v.Ui64 = v.I16 and v.Ui64 = v.I32 and v.Ui64 = v.I64 and v.Ui64 = v.F32",
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{numbersEntries[2].value},
			},
		},
		{
			// array, list, map, set
			"select v.A[2], v.L[6], v.M[7], v.S[\"I’ll grind his bones to mix my bread\"] from KeyIndexData",
			[]string{"v.A[2]", "v.L[6]", "v.M[7]", "v.S[I’ll grind his bones to mix my bread]"},
			[][]*vdl.Value{[]*vdl.Value{
				vdl.ValueOf("Fo"),
				vdl.ValueOf("Englishman"),
				vdl.ValueOf("seven"),
				vdl.ValueOf(true),
			}},
		},
		{
			"select k, v.Key from BigCollection where k < \"101\" or k = \"200\" or k like \"300%\"",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{svPair("100"), svPair("200"), svPair("300")},
		},
		{
			"select k, v.Key from BigCollection where k like \"10_\" or k like \"20_\"",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{
				svPair("100"),
				svPair("101"),
				svPair("102"),
				svPair("103"),
				svPair("104"),
				svPair("105"),
				svPair("106"),
				svPair("107"),
				svPair("108"),
				svPair("109"),
				svPair("200"),
				svPair("201"),
				svPair("202"),
				svPair("203"),
				svPair("204"),
				svPair("205"),
				svPair("206"),
				svPair("207"),
				svPair("208"),
				svPair("209"),
			},
		},
		{
			"select k, v.Key from BigCollection where k like \"_%9\"",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{
				svPair("109"),
				svPair("119"),
				svPair("129"),
				svPair("139"),
				svPair("149"),
				svPair("159"),
				svPair("169"),
				svPair("179"),
				svPair("189"),
				svPair("199"),
				svPair("209"),
				svPair("219"),
				svPair("229"),
				svPair("239"),
				svPair("249"),
				svPair("259"),
				svPair("269"),
				svPair("279"),
				svPair("289"),
				svPair("299"),
			},
		},
		{
			"select k, v.Key from BigCollection where k like \"__0\"",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{
				svPair("100"),
				svPair("110"),
				svPair("120"),
				svPair("130"),
				svPair("140"),
				svPair("150"),
				svPair("160"),
				svPair("170"),
				svPair("180"),
				svPair("190"),
				svPair("200"),
				svPair("210"),
				svPair("220"),
				svPair("230"),
				svPair("240"),
				svPair("250"),
				svPair("260"),
				svPair("270"),
				svPair("280"),
				svPair("290"),
				svPair("300"),
			},
		},
		{
			"select k, v.Key from BigCollection where k like \"10%\" or  k like \"20%\" or  k like \"30%\"",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{
				svPair("100"),
				svPair("101"),
				svPair("102"),
				svPair("103"),
				svPair("104"),
				svPair("105"),
				svPair("106"),
				svPair("107"),
				svPair("108"),
				svPair("109"),
				svPair("200"),
				svPair("201"),
				svPair("202"),
				svPair("203"),
				svPair("204"),
				svPair("205"),
				svPair("206"),
				svPair("207"),
				svPair("208"),
				svPair("209"),
				svPair("300"),
			},
		},
		{
			"select k, v.Key from BigCollection where k like \"1__\" and  k like \"_2_\" and  k like \"__3\"",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{svPair("123")},
		},
		{
			"select k, v.Key from BigCollection where (k >  \"100\" and k < \"103\") or (k > \"205\" and k < \"208\")",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{
				svPair("101"),
				svPair("102"),
				svPair("206"),
				svPair("207"),
			},
		},
		{
			"select k, v.Key from BigCollection where k <=  \"100\" or k = \"101\" or k >= \"300\" or (k <> \"299\" and k not like \"300\" and k >= \"298\")",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{
				svPair("100"),
				svPair("101"),
				svPair("298"),
				svPair("300"),
			},
		},
		{
			"select k, v.Key from BigCollection where k like  \"1%\" and k like \"%9\"",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{
				svPair("109"),
				svPair("119"),
				svPair("129"),
				svPair("139"),
				svPair("149"),
				svPair("159"),
				svPair("169"),
				svPair("179"),
				svPair("189"),
				svPair("199"),
			},
		},
		{
			"select k, v.Key from BigCollection where k like  \"3%\" and k like \"30%\" and k like \"300%\"",
			[]string{"k", "v.Key"},
			[][]*vdl.Value{svPair("300")},
		},
	}

	for _, test := range basic {
		headers, rs, err := db.Exec(ctx, test.query)
		if err != nil {
			t.Errorf("query: %s; got %v, want nil", test.query, err)
		} else {
			// Collect results.
			rbs := [][]interface{}{}
			for rs.Advance() {
				rbs = append(rbs, execResultsAsVector(t, rs, test.query))
			}
			if got, want := vdl.ValueOf(rbs), vdl.ValueOf(test.r); !vdl.EqualValue(got, want) {
				t.Errorf("query: %s; got %v, want %v", test.query, got, want)
			}
			if !reflect.DeepEqual(test.headers, headers) {
				t.Errorf("query: %s; got %v, want %v", test.query, headers, test.headers)
			}
		}
	}
}

// execResultsAsVector retuns the results of an element in a syncbase.ResultStream
// as a vector of interface{}.
func execResultsAsVector(t *testing.T, rs syncbase.ResultStream, query string) []interface{} {
	result := make([]interface{}, rs.ResultCount())
	for i := 0; i != len(result); i++ {
		if err := rs.Result(i, &result[i]); err != nil {
			t.Errorf("query: %s; can't get decoded result %d: %v", query, i, err)
		}
	}
	return result
}

func TestExecDelete(t *testing.T) {
	basic := []execDeleteTest{
		{
			// Delete all k/v pairs in the customer collection.
			"delete from Customer",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(10)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{},
		},
		{
			// Delete Customer type k/v pairs.
			"delete from Customer where Type(v) like \"%.Customer\"",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(3)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001001")},
				[]*vdl.Value{vdl.ValueOf("001002")},
				[]*vdl.Value{vdl.ValueOf("001003")},
				[]*vdl.Value{vdl.ValueOf("002001")},
				[]*vdl.Value{vdl.ValueOf("002002")},
				[]*vdl.Value{vdl.ValueOf("002003")},
				[]*vdl.Value{vdl.ValueOf("002004")},
			},
		},
		{
			// Delete non-existant k/v pair.
			"delete from Customer where k = \"foo\"",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(0)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("001001")},
				[]*vdl.Value{vdl.ValueOf("001002")},
				[]*vdl.Value{vdl.ValueOf("001003")},
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("002001")},
				[]*vdl.Value{vdl.ValueOf("002002")},
				[]*vdl.Value{vdl.ValueOf("002003")},
				[]*vdl.Value{vdl.ValueOf("002004")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			// Delete k/v pairs where v.InvoiceNum is nil
			// Since InvoiceNum does not exist for Customers,
			// this will delete all customer records.
			"delete from Customer where v.InvoiceNum is nil",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(3)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001001")},
				[]*vdl.Value{vdl.ValueOf("001002")},
				[]*vdl.Value{vdl.ValueOf("001003")},
				[]*vdl.Value{vdl.ValueOf("002001")},
				[]*vdl.Value{vdl.ValueOf("002002")},
				[]*vdl.Value{vdl.ValueOf("002003")},
				[]*vdl.Value{vdl.ValueOf("002004")},
			},
		},
		{
			// Delete values where v.InvoiceNum is nil
			// or v.Name is nil. This will deleteall customers
			// with the former and all invoices with the latter.
			// Hence, all k/v paris will be deleted.
			"delete from Customer where v.InvoiceNum is nil or v.Name is nil",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(10)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{},
		},
		{
			// Delete where v.InvoiceNum is nil AND v.Name is nil.
			// Nothing should be deleted.
			"delete from Customer where v.InvoiceNum is nil and v.Name is nil",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(0)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("001001")},
				[]*vdl.Value{vdl.ValueOf("001002")},
				[]*vdl.Value{vdl.ValueOf("001003")},
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("002001")},
				[]*vdl.Value{vdl.ValueOf("002002")},
				[]*vdl.Value{vdl.ValueOf("002003")},
				[]*vdl.Value{vdl.ValueOf("002004")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			// Delete where v.InvoiceNum is not nil
			// This will delete all invoices.
			"delete from Customer where v.InvoiceNum is not nil",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(7)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			// Delete where v.InvoiceNum is not nil
			// or v.Name is not nil. All records are deleted.
			"delete from Customer where v.InvoiceNum is not nil or v.Name is not nil",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(10)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{},
		},
		{
			// Delete where v.InvoiceNum is nil and v.Name is not nil.
			// All customers are deleted.
			"delete from Customer where v.InvoiceNum is nil and v.Name is not nil",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(3)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001001")},
				[]*vdl.Value{vdl.ValueOf("001002")},
				[]*vdl.Value{vdl.ValueOf("001003")},
				[]*vdl.Value{vdl.ValueOf("002001")},
				[]*vdl.Value{vdl.ValueOf("002002")},
				[]*vdl.Value{vdl.ValueOf("002003")},
				[]*vdl.Value{vdl.ValueOf("002004")},
			},
		},
		{
			// Delete where v.InvoiceNum is not nil
			// and v.Name is not nil.  Expect nothing deleted.
			"delete from Customer where v.InvoiceNum is not nil and v.Name is not nil",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(0)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("001001")},
				[]*vdl.Value{vdl.ValueOf("001002")},
				[]*vdl.Value{vdl.ValueOf("001003")},
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("002001")},
				[]*vdl.Value{vdl.ValueOf("002002")},
				[]*vdl.Value{vdl.ValueOf("002003")},
				[]*vdl.Value{vdl.ValueOf("002004")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			// Delete all $88 invoices.
			"delete from Customer where Type(v) like \"%.Invoice\" and v.Amount = 88",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(2)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("001001")},
				[]*vdl.Value{vdl.ValueOf("001002")},
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("002001")},
				[]*vdl.Value{vdl.ValueOf("002002")},
				[]*vdl.Value{vdl.ValueOf("002003")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			// Delete all with a key prefix of "001".
			"delete from Customer where k like \"001%\"",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(4)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("002001")},
				[]*vdl.Value{vdl.ValueOf("002002")},
				[]*vdl.Value{vdl.ValueOf("002003")},
				[]*vdl.Value{vdl.ValueOf("002004")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			// Delete all with a key prefix of "002".
			"delete from Customer where k like \"002%\"",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(5)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("001001")},
				[]*vdl.Value{vdl.ValueOf("001002")},
				[]*vdl.Value{vdl.ValueOf("001003")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			// Delete all with key prefix NOT like "002%".
			"delete from Customer where k not like \"002%\"",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(5)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("002001")},
				[]*vdl.Value{vdl.ValueOf("002002")},
				[]*vdl.Value{vdl.ValueOf("002003")},
				[]*vdl.Value{vdl.ValueOf("002004")},
			},
		},
		{
			// delete all with a key prefix of "001" or a key prefix of "002".
			"delete from Customer where k like \"001%\" or k like \"002%\"",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(9)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			// Delete 002% or 001%
			"delete from Customer where k like \"002%\" or k like \"001%\"",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(9)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			// Let's play with whitespace and mixed case.
			"   dElEtE  from \n  Customer WhErE k lIkE \"002%\" oR k LiKe \"001%\"",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(9)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			// Add in a like clause that accepts all strings.
			"   dElEtE  from \n  Customer WhErE k lIkE \"002%\" oR k LiKe \"001%\" or k lIkE \"%\"",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(10)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{},
		},
		{
			// delete customers whose last name is Masterson.
			"delete from Customer where Type(v) like \"%.Customer\" and v.Name like \"%Masterson\"",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(1)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("001001")},
				[]*vdl.Value{vdl.ValueOf("001002")},
				[]*vdl.Value{vdl.ValueOf("001003")},
				[]*vdl.Value{vdl.ValueOf("002001")},
				[]*vdl.Value{vdl.ValueOf("002002")},
				[]*vdl.Value{vdl.ValueOf("002003")},
				[]*vdl.Value{vdl.ValueOf("002004")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			// delete where v.Address.City is "Collins" or type is Invoice.
			"delete from Customer where v.Address.City = \"Collins\" or Type(v) like \"%.Invoice\"",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(8)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			// delete where v.Bar.Baz.TitleOrValue.Value = 42
			"delete from Foo where v.Bar.Baz.TitleOrValue.Value = 42",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(1)},
			},
			"select k from Foo",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
			},
		},
		{
			// delete where v.Address.City = "Collins" or (type is Invoice and v.InvoiceNum is not nil).
			// Limit 3
			"delete from Customer where v.Address.City = \"Collins\" or (Type(v) like \"%.Invoice\" and v.InvoiceNum is not nil) limit 3",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(3)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("002001")},
				[]*vdl.Value{vdl.ValueOf("002002")},
				[]*vdl.Value{vdl.ValueOf("002003")},
				[]*vdl.Value{vdl.ValueOf("002004")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			// delete invoices where date is 2015-03-17
			"delete from Customer where Type(v) like \"%.Invoice\" and Year(v.InvoiceDate, \"America/Los_Angeles\") = 2015 and Month(v.InvoiceDate, \"America/Los_Angeles\") = 3 and Day(v.InvoiceDate, \"America/Los_Angeles\") = 17",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(2)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("001001")},
				[]*vdl.Value{vdl.ValueOf("001002")},
				[]*vdl.Value{vdl.ValueOf("001003")},
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("002003")},
				[]*vdl.Value{vdl.ValueOf("002004")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			// Now will always be > 2012, so all customer records will be deleted.
			"delete from Customer where Now() > Time(\"2006-01-02 MST\", \"2012-03-17 PDT\")",
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(10)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{},
		},
	}

	setup(t)
	defer cleanup()
	for _, test := range basic {
		initCollections(t)
		headers, rs, err := db.Exec(ctx, test.delQuery)
		if err != nil {
			t.Errorf("delQuery: %s; got %v, want nil", test.delQuery, err)
		} else {
			// Collect results.
			rbs := [][]interface{}{}
			for rs.Advance() {
				rbs = append(rbs, execResultsAsVector(t, rs, test.delQuery))
			}
			if got, want := vdl.ValueOf(rbs), vdl.ValueOf(test.delResults); !vdl.EqualValue(got, want) {
				t.Errorf("delQuery: %s; got %v, want %v", test.delQuery, got, want)
			}
			if !reflect.DeepEqual(test.delHeaders, headers) {
				t.Errorf("delQuery: %s; got %v, want %v", test.delQuery, headers, test.delHeaders)
			}
		}
		headers, rs, err = db.Exec(ctx, test.selQuery)
		if err != nil {
			t.Errorf("delQuery: %s; got %v, want nil", test.delQuery, err)
		} else {
			// Collect results.
			rbs := [][]interface{}{}
			for rs.Advance() {
				rbs = append(rbs, execResultsAsVector(t, rs, test.delQuery))
			}
			if got, want := vdl.ValueOf(rbs), vdl.ValueOf(test.selResults); !vdl.EqualValue(got, want) {
				t.Errorf("delQuery: %s; got %v, want %v", test.delQuery, got, want)
			}
			if !reflect.DeepEqual(test.selHeaders, headers) {
				t.Errorf("delQuery: %s; got %v, want %v", test.delQuery, headers, test.selHeaders)
			}
		}
	}
}

func TestQuerySelectClause(t *testing.T) {
	setup(t)
	defer cleanup()
	initCollections(t)
	basic := []execSelectTest{
		{
			// Select numeric types
			"select v.B, v.Ui16, v.Ui32, v.Ui64, v.I16, v.I32, v.I64, v.F32, v.F64 from Numbers where k = \"001\"",
			[]string{"v.B", "v.Ui16", "v.Ui32", "v.Ui64", "v.I16", "v.I32", "v.I64", "v.F32", "v.F64"},
			[][]*vdl.Value{
				[]*vdl.Value{
					vdl.ValueOf(byte(12)),
					vdl.ValueOf(uint16(1234)),
					vdl.ValueOf(uint32(5678)),
					vdl.ValueOf(uint64(999888777666)),
					vdl.ValueOf(int16(9876)),
					vdl.ValueOf(int32(876543)),
					vdl.ValueOf(int64(128)),
					vdl.ValueOf(float32(3.14159)),
					vdl.ValueOf(float64(2.71828182846)),
				},
			},
		},
		{
			// Select struct, bool, string, enum, union
			"select v, v.Active, v.Address.State, v.Credit.Report.ExperianReport.Rating, v.Credit.Report from Customer where k = \"003\"",
			[]string{"v", "v.Active", "v.Address.State", "v.Credit.Report.ExperianReport.Rating", "v.Credit.Report"},
			[][]*vdl.Value{
				[]*vdl.Value{
					vdl.ValueOf(customerEntries[9].value),
					vdl.ValueOf(true),
					vdl.ValueOf("CT"),
					vdl.ValueOf(testdata.ExperianRatingGood),
					vdl.ValueOf(testdata.AgencyReportExperianReport{testdata.ExperianCreditReport{testdata.ExperianRatingGood}}),
				},
			},
		},
		{
			// Select array, list, set, map
			"select v.A[1], v.L[6], v.M[7], v.S[\"I’ll grind his bones to mix my bread\"] from KeyIndexData where k = \"aaa\"",
			[]string{"v.A[1]", "v.L[6]", "v.M[7]", "v.S[I’ll grind his bones to mix my bread]"},
			[][]*vdl.Value{
				[]*vdl.Value{
					vdl.ValueOf("Fi"),
					vdl.ValueOf("Englishman"),
					vdl.ValueOf("seven"),
					vdl.ValueOf(true),
				},
			},
		},
		{
			// Date functions.  Now is not included since its return value varies.
			// Also note, cannot parse a time with nanoseconds, so Nanosecond() returns
			// zero.
			"select Time(\"Jan 2 2006 15:04:05 MST\", \"Apr 12 2015 22:16:06 PDT\"), Year(Time(\"Jan 2 2006 15:04:05 MST\", \"Apr 12 2015 22:16:06 PDT\"), \"America/Los_Angeles\"), Month(Time(\"Jan 2 2006 15:04:05 MST\", \"Apr 12 2015 22:16:06 PDT\"), \"America/Los_Angeles\"), Day(Time(\"Jan 2 2006 15:04:05 MST\", \"Apr 12 2015 22:16:06 PDT\"), \"America/Los_Angeles\"), Hour(Time(\"Jan 2 2006 15:04:05 MST\", \"Apr 12 2015 22:16:06 PDT\"), \"America/Los_Angeles\"), Minute(Time(\"Jan 2 2006 15:04:05 MST\", \"Apr 12 2015 22:16:06 PDT\"), \"America/Los_Angeles\"), Second(Time(\"Jan 2 2006 15:04:05 MST\", \"Apr 12 2015 22:16:06 PDT\"), \"America/Los_Angeles\"), Nanosecond(Time(\"Jan 2 2006 15:04:05 MST\", \"Apr 12 2015 22:16:06 PDT\"), \"America/Los_Angeles\"), Weekday(Time(\"Jan 2 2006 15:04:05 MST\", \"Apr 12 2015 22:16:06 PDT\"), \"America/Los_Angeles\"), YearDay(Time(\"Jan 2 2006 15:04:05 MST\", \"Apr 12 2015 22:16:06 PDT\"), \"America/Los_Angeles\") from Customer where k = \"001\"",
			[]string{"Time", "Year", "Month", "Day", "Hour", "Minute", "Second", "Nanosecond", "Weekday", "YearDay"},
			[][]*vdl.Value{
				[]*vdl.Value{
					vdl.ValueOf(t2015_04_12_22_16_06), // Time
					vdl.ValueOf(int64(2015)),          // Year
					vdl.ValueOf(int64(4)),             // Month
					vdl.ValueOf(int64(12)),            // Day
					vdl.ValueOf(int64(22)),            // Hour
					vdl.ValueOf(int64(16)),            // Minute
					vdl.ValueOf(int64(6)),             // Second
					vdl.ValueOf(int64(0)),             // Nanosecond
					vdl.ValueOf(int64(0)),             // Weekday
					vdl.ValueOf(int64(102)),           // YearDay
				},
			},
		},
		{
			// String functions.
			"select Atoi(\"123\"), Atof(\"12.34\"), HtmlEscape(\"<a img='foo'>Foo Image</a>\"), HtmlUnescape(\"&lt;a img=&#39;foo&#39;&gt;Foo Image&lt;/a&gt;\"), Lowercase(\"AbCd\"), Split(\"ab,cd\", \",\"), Type(v), Uppercase(\"AbCd\"), RuneCount(\"Hello, 世界\"), Sprintf(\"abc: %s\", \"def\"), Str(123), StrCat(\"abc\", \"def\"), StrIndex(\"abcdef\", \"de\"), StrRepeat(\"abc\", 3), StrReplace(\"abczzzdef\", \"zzz\", \"ZZZ\"), StrLastIndex(\"abcabc\", \"abc\"), Trim(\"   abc   \"), TrimLeft(\"   abc   \"), TrimRight(\"   abc   \") from Customer where k = \"001\"",
			[]string{"Atoi", "Atof", "HtmlEscape", "HtmlUnescape", "Lowercase", "Split", "Type", "Uppercase", "RuneCount", "Sprintf", "Str", "StrCat", "StrIndex", "StrRepeat", "StrReplace", "StrLastIndex", "Trim", "TrimLeft", "TrimRight"},
			[][]*vdl.Value{
				[]*vdl.Value{
					vdl.ValueOf(int64(123)),                                       // Atoi
					vdl.ValueOf(float64(12.34)),                                   // Atof
					vdl.ValueOf("&lt;a img=&#39;foo&#39;&gt;Foo Image&lt;/a&gt;"), // HtmlEscape
					vdl.ValueOf("<a img='foo'>Foo Image</a>"),                     // HtmlUnescape
					vdl.ValueOf("abcd"),                                           // Lowercase
					vdl.ValueOf([]string{"ab", "cd"}),                             // Split
					vdl.ValueOf("v.io/v23/syncbase/testdata.Customer"),            // Type
					vdl.ValueOf("ABCD"),                                           // Uppercase
					vdl.ValueOf(9),                                                // RuneCount
					vdl.ValueOf("abc: def"),                                       // Sprintf
					vdl.ValueOf("123"),                                            // Str
					vdl.ValueOf("abcdef"),                                         // StrCat
					vdl.ValueOf(3),                                                // StrIndex
					vdl.ValueOf("abcabcabc"),                                      // StrRepeat
					vdl.ValueOf("abcZZZdef"),                                      // StrReplace
					vdl.ValueOf(3),                                                // StrLastIndex
					vdl.ValueOf("abc"),                                            // Trim
					vdl.ValueOf("abc   "),                                         // TrimLeft
					vdl.ValueOf("   abc"),                                         // TrimRight
				},
			},
		},
		{
			// Math Functions
			"select Ceiling(10.1), Ceiling(-10.1), Floor(10.1), Floor(-10.1), IsInf(100, 1), IsInf(-100, -1), IsInf(Inf(1), 1), IsInf(Inf(-1), -1), IsNaN(100.0), IsNaN(NaN()), Log(2.3), Log10(3.1), Pow(4.3, 10.1), Pow10(12), Mod(12.6, 4.4), Truncate(16.9), Truncate(-16.9), Remainder(16.9, 7.1) from Numbers where k = \"001\"",
			[]string{"Ceiling", "Ceiling", "Floor", "Floor", "IsInf", "IsInf", "IsInf", "IsInf", "IsNaN", "IsNaN", "Log", "Log10", "Pow", "Pow10", "Mod", "Truncate", "Truncate", "Remainder"},
			[][]*vdl.Value{
				[]*vdl.Value{
					vdl.ValueOf(float64(11)),                     // Ceiling(10.1)
					vdl.ValueOf(float64(-10)),                    // Ceiling(-10.1)
					vdl.ValueOf(float64(10)),                     // Floor(10.1)
					vdl.ValueOf(float64(-11)),                    // Floor(-10.1)
					vdl.ValueOf(false),                           // IsInf(100, 1)
					vdl.ValueOf(false),                           // IsInf(-100, -1)
					vdl.ValueOf(true),                            // IsInf(Inf(1), 1)
					vdl.ValueOf(true),                            // IsInf(Inf(-1), -1)
					vdl.ValueOf(false),                           // IsNaN(100.0)
					vdl.ValueOf(true),                            // IsNaN(NaN())
					vdl.ValueOf(float64(0.832909122935104)),      // Log(2.3)
					vdl.ValueOf(float64(0.4913616938342727)),     // Log10(3.1)
					vdl.ValueOf(float64(2.5005261539265467e+06)), // Pow(4.3, 10.1)
					vdl.ValueOf(float64(1e+12)),                  // Pow10(12)
					vdl.ValueOf(float64(3.799999999999999)),      // Mod(12.6, 4.4)
					vdl.ValueOf(float64(16)),                     // Truncate((16.9)
					vdl.ValueOf(float64(-16)),                    // Truncate((-16.9)
					vdl.ValueOf(float64(2.6999999999999993)),     // Remainder(16.9, 7.1)
				},
			},
		},
		{
			// Len function
			"select RuneCount(\"Hello, 世界\"), Len(\"Hello, 世界\"), Len(v.A), Len(v.L), Len(v.M), Len(v.S) from KeyIndexData where k = \"aaa\"",
			[]string{"RuneCount", "Len", "Len", "Len", "Len", "Len"},
			[][]*vdl.Value{
				[]*vdl.Value{
					vdl.ValueOf(int64(9)),  // RuneCount("Hello, 世界")
					vdl.ValueOf(int64(13)), // Len("Hello, 世界")
					vdl.ValueOf(int64(4)),  // Len(v.A)
					vdl.ValueOf(int64(7)),  // Len(v.L)
					vdl.ValueOf(int64(2)),  // Len(v.M)
					vdl.ValueOf(int64(1)),  // Len(v.S)
				},
			},
		},
		{
			// Nested Functions
			"select Year(Time(\"Jan 2 2006 15:04:05 MST\", \"Apr 12 2015 22:16:06 PDT\"), Sprintf(\"%s/%s\", \"America\", \"Los_Angeles\")) from Customer where k = \"001\"",
			[]string{"Year"},
			[][]*vdl.Value{
				[]*vdl.Value{
					vdl.ValueOf(int64(2015)),
				},
			},
		},
		{
			// Select with As Clause
			"select v.B as Bee from Numbers where k = \"001\"",
			[]string{"Bee"},
			[][]*vdl.Value{
				[]*vdl.Value{
					vdl.ValueOf(byte(12)),
				},
			},
		},
	}

	for _, test := range basic {
		headers, rs, err := db.Exec(ctx, test.query)
		if err != nil {
			t.Errorf("query: %s; got %v, want nil", test.query, err)
		} else {
			// Collect results.
			rbs := [][]interface{}{}
			for rs.Advance() {
				rbs = append(rbs, execResultsAsVector(t, rs, test.query))
			}
			if got, want := vdl.ValueOf(rbs), vdl.ValueOf(test.r); !vdl.EqualValue(got, want) {
				t.Errorf("query: %s; got %v, want %v", test.query, got, want)
			}
			if !reflect.DeepEqual(test.headers, headers) {
				t.Errorf("query: %s; got %v, want %v", test.query, headers, test.headers)
			}
		}
	}
}

func TestQueryWhereClause(t *testing.T) {
	setup(t)
	defer cleanup()
	initCollections(t)
	basic := []execSelectTest{
		{
			// Select on numeric comparisons with equals
			// (except, allow a range for F32 as the literal is interpreted as a float64.
			"select k, v from Numbers where v.B = 12 and v.Ui16 = 1234 and v.Ui32 = 5678 and v.Ui64 = 999888777666 and v.I16 = 9876 and v.I32 = 876543 and v.I64 = 128 and v.F32 > 3.14158 and v.F32 < 3.1416 and v.F64 = 2.71828182846 and k = \"001\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{
					vdl.ValueOf(numbersEntries[0].key),
					numbersEntries[0].value,
				},
			},
		},
		{
			// Select on numeric comparisons with >=
			"select k, v from Numbers where v.B >= 12 and v.Ui16 >= 1234 and v.Ui32 >= 5678 and v.Ui64 >= 999888777666 and v.I16 >= 9876 and v.I32 >= 876543 and v.I64 >= 128 and v.F32 >= 3.14159 and v.F64 >= 2.71828182846 and k = \"001\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{
					vdl.ValueOf(numbersEntries[0].key),
					numbersEntries[0].value,
				},
			},
		},
		{
			// Select on numeric comparisons with <=
			"select k, v from Numbers where v.B <= 12 and v.Ui16 <= 1234 and v.Ui32 <= 5678 and v.Ui64 <= 999888777666 and v.I16 <= 9876 and v.I32 <= 876543 and v.I64 <= 128 and v.F32 <= 3.14160 and v.F64 <= 2.71828182846 and k = \"001\"",
			[]string{"k", "v"},
			[][]*vdl.Value{
				[]*vdl.Value{
					vdl.ValueOf(numbersEntries[0].key),
					numbersEntries[0].value,
				},
			},
		},
		// Lots of selects on numeric comparisons with <>
		{
			"select k from Numbers where v.B <> 12",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			"select k from Numbers where v.Ui16 <> 1234",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			"select k from Numbers where v.Ui32 <> 5678",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			"select k from Numbers where v.Ui64 <> 999888777666",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			"select k from Numbers where v.I16 <> 9876",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			"select k from Numbers where v.I32 <> 876543",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			"select k from Numbers where v.I64 <> 128",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			"select k from Numbers where v.F32 <> 3.14159",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")}, // float32 <> float64
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			"select k from Numbers where v.F64 <> 2.71828182846",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		// bool =, <>
		{
			"select k from Customer where v.Active = true",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			"select k from Customer where v.Active <> false",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		// string =, <>, <, <=, >, >=, like, not like
		{
			"select k from Customer where v.Name = \"Bat Masterson\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002")},
			},
		},
		{
			"select k from Customer where v.Name <> \"Bat Masterson\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			"select k from Customer where v.Name < \"Bat Masterson\"",
			[]string{"k"},
			[][]*vdl.Value{},
		},
		{
			"select k from Customer where v.Name <= \"Bat Masterson\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002")},
			},
		},
		{
			"select k from Customer where v.Name > \"Bat Masterson\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			"select k from Customer where v.Name >= \"Bat Masterson\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			"select k from Customer where v.Name like \"John %\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			"select k from Customer where v.Name not like \"John %\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002")},
			},
		},
		// enum =, <>
		{
			"select k from Customer where v.Credit.Agency = \"Equifax\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
			},
		},
		{
			"select k from Customer where v.Credit.Agency <> \"Equifax\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		// inspect into array, list, set and map in where clause
		{
			"select k from KeyIndexData where v.A[1] = \"Fi\" and v.L[3] = \"blood\" and v.M[7]=\"seven\" and v.S[\"I’ll grind his bones to mix my bread\"] = true",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("aaa")},
			},
		},
		{
			// Use every date function in where clause.  Note: Now() requires that
			// clock not be set to year less than 2015.
			"select k from Customer where v.InvoiceDate = Time(\"Jan 2 2006 15:04:05 -0700 MST\", \"Jan 22 2015 13:11:01 -0800 PST\") and Year(v.InvoiceDate,  \"America/Los_Angeles\") = 2015 and Month(v.InvoiceDate, \"America/Los_Angeles\") = 1 and Day(v.InvoiceDate, \"America/Los_Angeles\") = 22 and Hour(v.InvoiceDate, \"America/Los_Angeles\") = 13 and Minute(v.InvoiceDate, \"America/Los_Angeles\") = 11 and Second(v.InvoiceDate, \"America/Los_Angeles\") = 1 and Nanosecond(v.InvoiceDate, \"America/Los_Angeles\") = 0 and Year(Now(), \"America/Los_Angeles\") >= 2015",

			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001001")},
			},
		},
		{
			// Use string functions in where clause.
			"select k from Customer where Atoi(\"3\") = 3 and Atof(\"3.1\") = 3.1 and HtmlEscape(\"<a img='foo'>Foo Image</a>\") = \"&lt;a img=&#39;foo&#39;&gt;Foo Image&lt;/a&gt;\" and HtmlUnescape(\"&lt;a img=&#39;foo&#39;&gt;Foo Image&lt;/a&gt;\") = \"<a img='foo'>Foo Image</a>\" and Lowercase(v.Name) = \"bat masterson\" and Type(v) like \"%.Customer\" and Uppercase(v.Name) = \"BAT MASTERSON\" and RuneCount(v.Name) = 13 and Sprintf(\"Name: %s\", v.Name) = \"Name: Bat Masterson\" and Str(v.Id) = \"2\" and StrCat(v.Name, Str(v.Id)) = \"Bat Masterson2\" and StrIndex(v.Name, \"M\") = 4 and StrRepeat(v.Name, 2) = \"Bat MastersonBat Masterson\" and StrReplace(v.Name, \"Master\", \"Amateur\") = \"Bat Amateurson\" and Trim(\"   xxx   \") = \"xxx\" and TrimLeft(\"   xxx   \") = \"xxx   \" and TrimRight(\"   xxx   \") = \"   xxx\"",

			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002")},
			},
		},
		{
			// Use math functions in where clause.
			"select k from Numbers where Ceiling(v.F64) = 2.0 and Floor(v.F64) = 1.0 and IsInf(v.F64, 1) = false and IsInf(Inf(1), 1) = true and IsInf(Inf(-1), -1) = true and IsNaN(v.F64) = false and IsNaN(NaN()) = true and Log(v.F64) = 0.5493061443347032 and Log10(v.F64) = 0.23856062736011277 and Pow(v.F64, 3.2) = 5.799546134807319 and Pow10(v.B) = 1e+09 and Pow10(v.Ui32) = Inf(1) and Mod(v.F64, v.F32) = 0.31783726940013923 and Truncate(v.F64) = 1.0 and Remainder(v.F64, v.F32) = 0.31783726940013923",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("002")},
			},
		},
		{
			// Use Len on string, array, list and set-- in where clause.
			"select k from KeyIndexData where Len(v.A[0]) = 3 and Len(v.A) = 4 and Len(v.L) = 7 and Len(v.M) = 2 and Len(v.S) = 1",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("aaa")},
			},
		},
		// The next six tests exercise and, or and parens for precedence.
		{
			"select k from Customer where v.Name like \"%Smith\" and v.Active = false or v.Address.City = \"Palo Alto\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
			},
		},
		{
			"select k from Customer where (v.Name like \"%Smith\" and v.Active = false) or v.Address.City = \"Palo Alto\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
			},
		},
		{
			"select k from Customer where v.Name like \"%Smith\" and (v.Active = false or v.Address.City = \"Palo Alto\")",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
			},
		},
		{
			"select k from Customer where v.Name like \"%Smith\" and v.Active = false or v.Address.City = \"Palo Alto\" or v.Address.City = \"Mountain View\"",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
			},
		},
		{
			"select k from Customer where v.Name like \"%Smith\" and (v.Active = false or v.Address.City = \"Palo Alto\" or v.Address.City = \"Mountain View\")",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
			},
		},
		{
			"select k from Customer where v.Name like \"%Smith\" and v.Active = false and (v.Address.City = \"Palo Alto\" or v.Address.City = \"Mountain View\")",
			[]string{"k"},
			[][]*vdl.Value{},
		},
	}

	for _, test := range basic {
		headers, rs, err := db.Exec(ctx, test.query)
		if err != nil {
			t.Errorf("query: %s; got %v, want nil", test.query, err)
		} else {
			// Collect results.
			rbs := [][]interface{}{}
			for rs.Advance() {
				rbs = append(rbs, execResultsAsVector(t, rs, test.query))
			}
			if got, want := vdl.ValueOf(rbs), vdl.ValueOf(test.r); !vdl.EqualValue(got, want) {
				t.Errorf("query: %s; got %v, want %v", test.query, got, want)
			}
			if !reflect.DeepEqual(test.headers, headers) {
				t.Errorf("query: %s; got %v, want %v", test.query, headers, test.headers)
			}
		}
	}
}

func TestQueryEscapeClause(t *testing.T) {
	setup(t)
	defer cleanup()
	initCollections(t)
	basic := []execSelectTest{
		{
			"select k from Customer where \"abc%\" like \"abc^%\" escape '^'",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("001001")},
				[]*vdl.Value{vdl.ValueOf("001002")},
				[]*vdl.Value{vdl.ValueOf("001003")},
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("002001")},
				[]*vdl.Value{vdl.ValueOf("002002")},
				[]*vdl.Value{vdl.ValueOf("002003")},
				[]*vdl.Value{vdl.ValueOf("002004")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			"select k from Customer where \"abc_\" like \"abc$_\" escape '$'",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("001001")},
				[]*vdl.Value{vdl.ValueOf("001002")},
				[]*vdl.Value{vdl.ValueOf("001003")},
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("002001")},
				[]*vdl.Value{vdl.ValueOf("002002")},
				[]*vdl.Value{vdl.ValueOf("002003")},
				[]*vdl.Value{vdl.ValueOf("002004")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			"select k from Customer where \"abc%defghi\" like \"abc^%%\" escape '^'",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("001001")},
				[]*vdl.Value{vdl.ValueOf("001002")},
				[]*vdl.Value{vdl.ValueOf("001003")},
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("002001")},
				[]*vdl.Value{vdl.ValueOf("002002")},
				[]*vdl.Value{vdl.ValueOf("002003")},
				[]*vdl.Value{vdl.ValueOf("002004")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			"select k from Customer where \"abc_defghi\" like \"abc^__efghi\" escape '^'",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("001001")},
				[]*vdl.Value{vdl.ValueOf("001002")},
				[]*vdl.Value{vdl.ValueOf("001003")},
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("002001")},
				[]*vdl.Value{vdl.ValueOf("002002")},
				[]*vdl.Value{vdl.ValueOf("002003")},
				[]*vdl.Value{vdl.ValueOf("002004")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
	}

	for _, test := range basic {
		headers, rs, err := db.Exec(ctx, test.query)
		if err != nil {
			t.Errorf("query: %s; got %v, want nil", test.query, err)
		} else {
			// Collect results.
			rbs := [][]interface{}{}
			for rs.Advance() {
				rbs = append(rbs, execResultsAsVector(t, rs, test.query))
			}
			if got, want := vdl.ValueOf(rbs), vdl.ValueOf(test.r); !vdl.EqualValue(got, want) {
				t.Errorf("query: %s; got %v, want %v", test.query, got, want)
			}
			if !reflect.DeepEqual(test.headers, headers) {
				t.Errorf("query: %s; got %v, want %v", test.query, headers, test.headers)
			}
		}
	}
}

func TestQueryLimitAndOffsetClauses(t *testing.T) {
	setup(t)
	defer cleanup()
	initCollections(t)
	basic := []execSelectTest{
		{
			"select k from Customer limit 2 offset 3",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001003")},
				[]*vdl.Value{vdl.ValueOf("002")},
			},
		},
		{
			"select k from Customer offset 9223372036854775807", // maxint64
			[]string{"k"},
			[][]*vdl.Value{},
		},
		{
			"select k from Customer limit 9223372036854775807", // maxint64
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("001001")},
				[]*vdl.Value{vdl.ValueOf("001002")},
				[]*vdl.Value{vdl.ValueOf("001003")},
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("002001")},
				[]*vdl.Value{vdl.ValueOf("002002")},
				[]*vdl.Value{vdl.ValueOf("002003")},
				[]*vdl.Value{vdl.ValueOf("002004")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
	}

	for _, test := range basic {
		headers, rs, err := db.Exec(ctx, test.query)
		if err != nil {
			t.Errorf("query: %s; got %v, want nil", test.query, err)
		} else {
			// Collect results.
			rbs := [][]interface{}{}
			for rs.Advance() {
				rbs = append(rbs, execResultsAsVector(t, rs, test.query))
			}
			if got, want := vdl.ValueOf(rbs), vdl.ValueOf(test.r); !vdl.EqualValue(got, want) {
				t.Errorf("query: %s; got %v, want %v", test.query, got, want)
			}
			if !reflect.DeepEqual(test.headers, headers) {
				t.Errorf("query: %s; got %v, want %v", test.query, headers, test.headers)
			}
		}
	}
}

func svPair(s string) []*vdl.Value {
	v := vdl.ValueOf(s)
	return []*vdl.Value{v, v}
}

// Use Now to verify it is "pre" executed such that all the rows
// have the same time.
func TestPreExecFunctions(t *testing.T) {
	setup(t)
	defer cleanup()
	initCollections(t)
	basic := []preExecFunctionTest{
		{
			"select Now() from Customer",
			[]string{
				"Now",
			},
		},
	}

	for _, test := range basic {
		headers, rs, err := db.Exec(ctx, test.query)
		if err != nil {
			t.Errorf("query: %s; got %v, want nil", test.query, err)
		} else {
			// Check that all results are identical.
			// Collect results.
			var last []interface{}
			for rs.Advance() {
				result := execResultsAsVector(t, rs, test.query)
				if last != nil && !reflect.DeepEqual(last, result) {
					t.Errorf("query: %s; got %v, want %v", test.query, result, last)
				}
				last = result
			}
			if !reflect.DeepEqual(test.headers, headers) {
				t.Errorf("query: %s; got %v, want %v", test.query, headers, test.headers)
			}
		}
	}
}

// TODO(jkline): More negative tests here (even though they are tested elsewhere)?
func TestExecErrors(t *testing.T) {
	setup(t)
	defer cleanup()
	initCollections(t)
	basic := []execSelectErrorTest{
		{
			"select a from Customer",
			syncql.NewErrInvalidSelectField(ctx, 7),
		},
		{
			"select v from Unknown",
			// The following error text is dependent on the implementation of the query.Database interface.
			// TODO(sadovsky): Error messages should never contain storage engine
			// prefixes ("c") and delimiters ("\xfe").
			syncql.NewErrTableCantAccess(ctx, 14, "Unknown", errors.New("syncbase.test:\"root:o:app,d\".Exec: Does not exist: c\xferoot:o:app:client,Unknown\xfe")),
		},
		{
			"select v from Customer offset -1",
			syncql.NewErrExpected(ctx, 30, "positive integer literal"),
		},
		{
			"select v from Customer limit -1",
			syncql.NewErrExpected(ctx, 29, "positive integer literal"),
		},
		{
			"select v from Customer offset 9223372036854775808", // maxint64 + 1
			syncql.NewErrCouldNotConvert(ctx, 30, "9223372036854775808", "int64"),
		},
		{
			"select v from Customer limit 9223372036854775808", // maxint64 + 1
			syncql.NewErrCouldNotConvert(ctx, 29, "9223372036854775808", "int64"),
		},
	}

	for _, test := range basic {
		_, rs, err := db.Exec(ctx, test.query)
		if err == nil {
			err = rs.Err()
		}
		// Test both that the IDs compare and the text compares (since the offset needs to match).
		// Note: This is a little tricky because the actual error message will contain the calling
		//       module.
		wantPrefix := test.err.Error()[:strings.Index(test.err.Error(), ":")]
		wantSuffix := test.err.Error()[len(wantPrefix)+1:]
		if verror.ErrorID(err) != verror.ErrorID(test.err) || !strings.HasPrefix(err.Error(), wantPrefix) || !strings.HasSuffix(err.Error(), wantSuffix) {
			t.Errorf("query: %s; got %v, want %v", test.query, err, test.err)
		}
	}
}

func TestQueryErrors(t *testing.T) {
	setup(t)
	defer cleanup()
	initCollections(t)
	basic := []execSelectErrorTest{
		// Produce every error in the book (make that, every one that is possible to produce).
		{
			"select k from Customer where Amt = 100",
			syncql.NewErrBadFieldInWhere(ctx, 29),
		},
		{
			"select k from Customer where v.Active > false",
			syncql.NewErrBoolInvalidExpression(ctx, 38),
		},
		// * CheckOfUnknownStatementType cannot be produced as the parser
		// does not produce unknown statement types.
		// * CouldNotConvert cannot be produced directly (it could be wrapped). The only
		// unwrapped errors are produced by the parser and would only occur if golang's
		// text/scanner liked to the parser.
		{
			"select k.a from Customer",
			syncql.NewErrDotNotationDisallowedForKey(ctx, 9),
		},
		// * ErrorCompilingRegularExpression cannot be produced unless there is a bug
		// in query_checker which should be escaping regex characters (by calling
		// a library function).
		// * ExecOfUnknownStatementType cannot be produced as the parser
		// does not produce unknown statement types.
		{
			"select k, v from Customer limit a",
			syncql.NewErrExpected(ctx, 32, "positive integer literal"),
		},
		{
			"select k, v limit 100",
			syncql.NewErrExpectedFrom(ctx, 12, "limit"),
		},
		{
			"select 100 from Customer",
			syncql.NewErrExpectedIdentifier(ctx, 7, "100"),
		},
		{
			"select k from Customer where v.A ^ v.B",
			syncql.NewErrExpectedOperator(ctx, 33, "^"),
		},
		{
			"select k from Customer where Mod(12.1) = 0.1",
			syncql.NewErrFunctionArgCount(ctx, 29, "Mod", 2, 1),
		},
		{
			"select k from Customer where Sprintf() = \"abc\"",
			syncql.NewErrFunctionAtLeastArgCount(ctx, 29, "Sprintf", 1, 0),
		},
		{
			"select k from Customer where Type(100) = \"abc\"",
			syncql.NewErrFunctionTypeInvalidArg(ctx, 34),
		},
		{
			"select k from Customer where Len(100) = \"abc\"",
			syncql.NewErrFunctionLenInvalidArg(ctx, 33),
		},
		// * FunctionArgBad error does not make it back to the client.
		{
			"select What(100) from Customer",
			syncql.NewErrFunctionNotFound(ctx, 7, "What"),
		},
		{
			"select Type(k) from Customer",
			syncql.NewErrArgMustBeField(ctx, 12),
		},
		// *BigIntConversionError isn't produced as vdl doesn't have big ints
		// *BigRatConversionError isn't produced as vdl doesn't have big rats
		// *BoolConversionError isn't currently produced as no functions take a bool arg.
		// *UintConversionError isn't currently produced as no functions take a uint arg.
		{
			"select Year(\"abc\", \"America/Los_Angeles\") from Customer",
			syncql.NewErrTimeConversionError(ctx, 12, errors.New("Cannot convert operand to time.")),
		},
		// * LocationConversionError - see TestQueryErrorsPlatformDependentText below.
		{
			"select Lowercase(100) from Customer",
			syncql.NewErrStringConversionError(ctx, 17, errors.New("Cannot convert operand to string.")),
		},
		{
			"select Ceiling(\"abc\") from Customer",
			syncql.NewErrFloatConversionError(ctx, 15, errors.New("Cannot convert operand to float64.")),
		},
		{
			"select Pow10(3.1) from Customer",
			syncql.NewErrIntConversionError(ctx, 13, errors.New("Cannot convert operand to int64.")),
		},
		{
			"select k from Customer where nil is v.Name",
			syncql.NewErrIsIsNotRequireLhsValue(ctx, 29),
		},
		{
			"select k from Customer where v.Name is \"John Smith\"",
			syncql.NewErrIsIsNotRequireRhsNil(ctx, 39),
		},
		{
			"select k from Customer where v.Name like \"a^b%\" escape '^'",
			syncql.NewErrInvalidLikePattern(ctx, 41, pattern.NewErrInvalidEscape(nil, "b")),
		},
		{
			"select k from Customer where v.Name like \"John^Smith\" escape '^'",
			syncql.NewErrInvalidLikePattern(ctx, 41, pattern.NewErrInvalidEscape(nil, "S")),
		},
		{
			"select a from Customer",
			syncql.NewErrInvalidSelectField(ctx, 7),
		},
		{
			"select k from Customer where k = 100",
			syncql.NewErrKeyExpressionLiteral(ctx, 33),
		},
		// *KeyValueStreamError Cannot produce a KeyValueStreamError.
		{
			"select k from Customer where v.Name like 100",
			syncql.NewErrLikeExpressionsRequireRhsString(ctx, 41),
		},
		{
			"select k from Customer limit 0",
			syncql.NewErrLimitMustBeGt0(ctx, 29),
		},
		// *MaxStatementLenExceeded See TestQueryStatementSizeExceeded.
		{
			"",
			syncql.NewErrNoStatementFound(ctx, 0),
		},
		// *OffsetMustBeGe0 cannot be produced because the parser won't produce
		// an offset < 0.
		// *ScanError Cannot produce a [collection].ScanError.
		{
			"select k from Blah",
			// TODO(sadovsky): Error messages should never contain storage engine
			// prefixes ("c") and delimiters ("\xfe").
			syncql.NewErrTableCantAccess(ctx, 14, "Blah", errors.New("syncbase.test:\"root:o:app,d\".Exec: Does not exist: c\xferoot:o:app:client,Blah\xfe")),
		},
		{
			"select k, v from Customer where a = b)",
			syncql.NewErrUnexpected(ctx, 37, ")"),
		},
		{
			"select k from Customer where",
			syncql.NewErrUnexpectedEndOfStatement(ctx, 28),
		},
		{
			"foo",
			syncql.NewErrUnknownIdentifier(ctx, 0, "foo"),
		},
		{
			"select k from Customer escape ' '",
			syncql.NewErrInvalidEscapeChar(ctx, 30, " "),
		},
		{
			"select K from Customer",
			syncql.NewErrDidYouMeanLowercaseK(ctx, 7),
		},
		{
			"select V from Customer",
			syncql.NewErrDidYouMeanLowercaseV(ctx, 7),
		},
		{
			"select now() from Customer",
			syncql.NewErrDidYouMeanFunction(ctx, 7, "Now"),
		},
		// *NotEnoughParamValuesSpecified Can't test from syncbase as PreparedStatement not
		// exposed to syncbase.
		// *TooManyParamValuesSpecified Can't test from syncbase as PreparedStatement not
		// exposed to syncbase.
		// *PreparedStatementNotFound Can't test from syncbase as PreparedStatement not
		// exposed to syncbase.
	}

	for _, test := range basic {
		_, rs, err := db.Exec(ctx, test.query)
		if err == nil {
			err = rs.Err()
		}
		// Test both that the IDs compare and the text compares (since the offset needs to match).
		// Note: This is a little tricky because the actual error message will contain the calling
		//       module.
		wantPrefix := test.err.Error()[:strings.Index(test.err.Error(), ":")]
		wantSuffix := test.err.Error()[len(wantPrefix)+1:]
		if verror.ErrorID(err) != verror.ErrorID(test.err) || !strings.HasPrefix(err.Error(), wantPrefix) || !strings.HasSuffix(err.Error(), wantSuffix) {
			t.Errorf("query: %s; got %v, want %v", test.query, err, test.err)
		}
	}
}

func TestQueryErrorsPlatformDependentText(t *testing.T) {
	setup(t)
	defer cleanup()
	initCollections(t)
	basic := []execSelectErrorTest{
		// These errors contain installation dependent parts to the error.  The test is
		// more relaxed (it doesn't check the suffix) in order to account for this.
		{
			"select Year(v.InvoiceDate, \"DummyTimeZone\") from Customer",
			syncql.NewErrLocationConversionError(ctx, 27, errors.New("unknown time zone DummyTimeZone")),
		},
	}

	for _, test := range basic {
		_, rs, err := db.Exec(ctx, test.query)
		if err == nil {
			err = rs.Err()
		}
		// Test both that the IDs compare and the text prefix compares (since the offset needs to match).
		// Note: This is a little tricky because the actual error message will contain the calling
		//       module.
		wantPrefix := test.err.Error()[:strings.Index(test.err.Error(), ":")]
		if verror.ErrorID(err) != verror.ErrorID(test.err) || !strings.HasPrefix(err.Error(), wantPrefix) {
			t.Errorf("query: %s; got %v, want %v", test.query, err, test.err)
		}
	}
}

func TestQueryStatementSizeExceeded(t *testing.T) {
	setup(t)
	defer cleanup()
	initCollections(t)
	q := fmt.Sprintf("select a from b where c = \"%s\"", strings.Repeat("x", 12000))

	_, rs, err := db.Exec(ctx, q)
	if err == nil {
		err = rs.Err()
	}

	expectedErr := syncql.NewErrMaxStatementLenExceeded(ctx, int64(0), int64(10000), int64(len(q)))

	wantPrefix := expectedErr.Error()[:strings.Index(expectedErr.Error(), ":")]
	wantSuffix := expectedErr.Error()[len(wantPrefix)+1:]
	if verror.ErrorID(err) != verror.ErrorID(expectedErr) || !strings.HasPrefix(err.Error(), wantPrefix) || !strings.HasSuffix(err.Error(), wantSuffix) {
		t.Errorf("query: %s; got %v, want %v", q, err, expectedErr)
	}
}

func TestExecParamSelect(t *testing.T) {
	setup(t)
	defer cleanup()
	initCollections(t)
	t20120317, _ := time.Parse("2006-01-02 MST", "2012-03-17 PDT")
	basic := []execSelectParamTest{
		{
			// Select records where v.Address.City is "Collins" or type is Invoice.
			"select v from Customer where v.Address.City = ? or Type(v) like ?",
			[]interface{}{
				"Collins",
				"%.Invoice",
			},
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[1].value},
				[]*vdl.Value{customerEntries[2].value},
				[]*vdl.Value{customerEntries[3].value},
				[]*vdl.Value{customerEntries[4].value},
				[]*vdl.Value{customerEntries[5].value},
				[]*vdl.Value{customerEntries[6].value},
				[]*vdl.Value{customerEntries[7].value},
				[]*vdl.Value{customerEntries[8].value},
			},
		},
		{
			// Select records where v.Address.City is "Collins" and v.InvoiceNum is nil.
			"select v from Customer where v.Address.City = \"Collins\" and v.InvoiceNum is nil",
			[]interface{}{},
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[4].value},
			},
		},
		{
			// Select customer name for customer Id (i.e., key) "001".
			"select v.Name as Name from Customer where Type(v) like \"%.Customer\" and k = ?",
			[]interface{}{
				"001",
			},
			[]string{"Name"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("John Smith")},
			},
		},
		{
			// Select v where v.AgencyRating = "Bad"
			"select v from Customer where v.Credit.Report.EquifaxReport.Rating < ? or v.Credit.Report.ExperianReport.Rating = ? or v.Credit.Report.TransUnionReport.Rating < ?",
			[]interface{}{
				'A',
				"Bad",
				90,
			},
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[4].value},
			},
		},
		{
			// Select records where v.Bar.Baz.TitleOrValue.Value = 42
			"select v from Foo where v.Bar.Baz.TitleOrValue.Value = ?",
			[]interface{}{
				42,
			},
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{fooEntries[1].value},
			},
		},
		{
			// Select records where v.Bar.Baz.TitleOrValue.Value = "42"
			"select v from Foo where v.Bar.Baz.TitleOrValue.Value = ?",
			[]interface{}{
				"42",
			},
			[]string{"v"},
			[][]*vdl.Value{},
		},
		{
			// Now will always be > 2012, so all customer records will be returned.
			"select v from Customer where Now() > ?",
			[]interface{}{
				t20120317,
			},
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[0].value},
				[]*vdl.Value{customerEntries[1].value},
				[]*vdl.Value{customerEntries[2].value},
				[]*vdl.Value{customerEntries[3].value},
				[]*vdl.Value{customerEntries[4].value},
				[]*vdl.Value{customerEntries[5].value},
				[]*vdl.Value{customerEntries[6].value},
				[]*vdl.Value{customerEntries[7].value},
				[]*vdl.Value{customerEntries[8].value},
				[]*vdl.Value{customerEntries[9].value},
			},
		},
		// Test functions.
		{
			// Now will always be > 2012, so all customer records will be returned.
			"select v from Customer where Now() > Time(\"2006-01-02 MST\", ?)",
			[]interface{}{
				"2012-03-17 PDT",
			},
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[0].value},
				[]*vdl.Value{customerEntries[1].value},
				[]*vdl.Value{customerEntries[2].value},
				[]*vdl.Value{customerEntries[3].value},
				[]*vdl.Value{customerEntries[4].value},
				[]*vdl.Value{customerEntries[5].value},
				[]*vdl.Value{customerEntries[6].value},
				[]*vdl.Value{customerEntries[7].value},
				[]*vdl.Value{customerEntries[8].value},
				[]*vdl.Value{customerEntries[9].value},
			},
		},
		{
			// Select invoices shipped to Any street -- using Uppercase.
			"select k from Customer where Type(v) like StrCat(\"%.\", ?) and Uppercase(v.ShipTo.Street) like ?",
			[]interface{}{
				"Invoice",
				"%ANY%",
			},
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(customerEntries[5].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[6].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[7].key)},
				[]*vdl.Value{vdl.ValueOf(customerEntries[8].key)},
			},
		},
		// TODO(ivanpi): Parameters are currently supported only in where clause.
		/*{
			"select v.A[?], v.L[6], v.M[?], v.S[?], ? from KeyIndexData",
			[]interface{}{
				2,
				1.1 + 2.2i,
				"I’ll grind his bones to mix my bread",
				42,
			},
			[]string{"v.A[2]", "v.L[6]", "v.S[I’ll grind his bones to mix my bread]"},
			[][]*vdl.Value{
				[]*vdl.Value{
					vdl.ValueOf("Fo"),
					vdl.ValueOf("Englishman"),
					vdl.ValueOf("Be he living, or be he dead"),
					vdl.ValueOf(true),
					vdl.ValueOf(42),
				},
			},
		},
		{
			"select v.?[6], v.M[?] as ? from KeyIndexData",
			[]interface{}{
				"L",
				1.1 + 2.2i,
				"Leet",
			},
			[]string{"v.L[6]", "Leet"},
			[][]*vdl.Value{
				[]*vdl.Value{
					vdl.ValueOf("Englishman"),
					vdl.ValueOf("Be he living, or be he dead"),
				},
			},
		},*/
		{
			"select k, v.Key from BigCollection where k like StrCat(?, \"_\") or k like StrCat(?, \"_\")",
			[]interface{}{
				"10",
				"20",
			},
			[]string{"k", "v.Key"},
			[][]*vdl.Value{
				svPair("100"),
				svPair("101"),
				svPair("102"),
				svPair("103"),
				svPair("104"),
				svPair("105"),
				svPair("106"),
				svPair("107"),
				svPair("108"),
				svPair("109"),
				svPair("200"),
				svPair("201"),
				svPair("202"),
				svPair("203"),
				svPair("204"),
				svPair("205"),
				svPair("206"),
				svPair("207"),
				svPair("208"),
				svPair("209"),
			},
		},
		{
			"select k, v.Key from BigCollection where k <= ? or k = ? or k >= ? or (k <> ? and k not like ? and k >= ?)",
			[]interface{}{
				"100",
				"101",
				"300",
				"299",
				"300",
				"298",
			},
			[]string{"k", "v.Key"},
			[][]*vdl.Value{
				svPair("100"),
				svPair("101"),
				svPair("298"),
				svPair("300"),
			},
		},
		// Successful syncQL injection. All values are returned.
		{
			"select v from Customer where k = \"" + "BobbyTables\" or \"1\" = \"1" + "\"",
			[]interface{}{},
			[]string{"v"},
			[][]*vdl.Value{
				[]*vdl.Value{customerEntries[0].value},
				[]*vdl.Value{customerEntries[1].value},
				[]*vdl.Value{customerEntries[2].value},
				[]*vdl.Value{customerEntries[3].value},
				[]*vdl.Value{customerEntries[4].value},
				[]*vdl.Value{customerEntries[5].value},
				[]*vdl.Value{customerEntries[6].value},
				[]*vdl.Value{customerEntries[7].value},
				[]*vdl.Value{customerEntries[8].value},
				[]*vdl.Value{customerEntries[9].value},
			},
		},
		// syncQL injection thwarted by parameterized query.
		{
			"select v from Customer where k = ?",
			[]interface{}{
				"BobbyTables\" or \"1\" = \"1",
			},
			[]string{"v"},
			[][]*vdl.Value{},
		},
	}

	for _, test := range basic {
		headers, rs, err := db.Exec(ctx, test.query, test.params...)
		if err != nil {
			t.Errorf("query: %s, params: %v; got %v, want nil", test.query, test.params, err)
		} else {
			// Collect results.
			rbs := [][]interface{}{}
			for rs.Advance() {
				rbs = append(rbs, execResultsAsVector(t, rs, test.query))
			}
			if got, want := vdl.ValueOf(rbs), vdl.ValueOf(test.r); !vdl.EqualValue(got, want) {
				t.Errorf("query: %s, params: %v; got %v, want %v", test.query, test.params, got, want)
			}
			if !reflect.DeepEqual(test.headers, headers) {
				t.Errorf("query: %s, params: %v; got %v, want %v", test.query, test.params, headers, test.headers)
			}
		}
	}
}

func TestExecParamDelete(t *testing.T) {
	basic := []execDeleteParamTest{
		{
			// Delete all $88 invoices.
			"delete from Customer where Type(v) like \"%.Invoice\" and v.Amount = ?",
			[]interface{}{
				88,
			},
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(2)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("001001")},
				[]*vdl.Value{vdl.ValueOf("001002")},
				[]*vdl.Value{vdl.ValueOf("002")},
				[]*vdl.Value{vdl.ValueOf("002001")},
				[]*vdl.Value{vdl.ValueOf("002002")},
				[]*vdl.Value{vdl.ValueOf("002003")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			// delete all with a key prefix of "001" or a key prefix of "002".
			"delete from Customer where k like ? or k like ?",
			[]interface{}{
				"001%",
				"002%",
			},
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(9)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			// delete where v.Address.City is "Collins" or type is Invoice.
			"delete from Customer where v.Address.City = ? or Type(v) like StrCat(\"%.\", ?)",
			[]interface{}{
				"Collins",
				"Invoice",
			},
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(8)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("003")},
			},
		},
		{
			// delete where v.Name is "John \"JOS\" O'Steed" or type is Invoice.
			"delete from Customer where v.Name = ? or Type(v) like ?",
			[]interface{}{
				"John \"JOS\" O'Steed",
				"%.Invoice",
			},
			[]string{"Count"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf(8)},
			},
			"select k from Customer",
			[]string{"k"},
			[][]*vdl.Value{
				[]*vdl.Value{vdl.ValueOf("001")},
				[]*vdl.Value{vdl.ValueOf("002")},
			},
		},
	}

	setup(t)
	defer cleanup()
	for _, test := range basic {
		initCollections(t)
		headers, rs, err := db.Exec(ctx, test.delQuery, test.delParams...)
		if err != nil {
			t.Errorf("delQuery: %s, delParams: %v; got %v, want nil", test.delQuery, test.delParams, err)
		} else {
			// Collect results.
			rbs := [][]interface{}{}
			for rs.Advance() {
				rbs = append(rbs, execResultsAsVector(t, rs, test.delQuery))
			}
			if got, want := vdl.ValueOf(rbs), vdl.ValueOf(test.delResults); !vdl.EqualValue(got, want) {
				t.Errorf("delQuery: %s, delParams: %v; got %v, want %v", test.delQuery, test.delParams, got, want)
			}
			if !reflect.DeepEqual(test.delHeaders, headers) {
				t.Errorf("delQuery: %s, delParams: %v; got %v, want %v", test.delQuery, test.delParams, headers, test.delHeaders)
			}
		}
		headers, rs, err = db.Exec(ctx, test.selQuery)
		if err != nil {
			t.Errorf("delQuery: %s, delParams: %v; got %v, want nil", test.delQuery, test.delParams, err)
		} else {
			// Collect results.
			rbs := [][]interface{}{}
			for rs.Advance() {
				rbs = append(rbs, execResultsAsVector(t, rs, test.delQuery))
			}
			if got, want := vdl.ValueOf(rbs), vdl.ValueOf(test.selResults); !vdl.EqualValue(got, want) {
				t.Errorf("delQuery: %s, delParams: %v; got %v, want %v", test.delQuery, test.delParams, got, want)
			}
			if !reflect.DeepEqual(test.selHeaders, headers) {
				t.Errorf("delQuery: %s, delParams: %v; got %v, want %v", test.delQuery, test.delParams, headers, test.selHeaders)
			}
		}
	}
}

func TestExecParamErrors(t *testing.T) {
	setup(t)
	defer cleanup()
	initCollections(t)
	basic := []execSelectParamErrorTest{
		// Produce parameter related errors.
		{
			"select k from Customer where v.Name = ? or v.Address.City = ?",
			[]interface{}{
				"John",
			},
			syncql.NewErrNotEnoughParamValuesSpecified(ctx, 60),
		},
		{
			"select k from Customer where v.Name = ? or v.Address.City = ?",
			[]interface{}{
				"John",
				"Collins",
				42,
			},
			syncql.NewErrTooManyParamValuesSpecified(ctx, 23),
		},
		{
			"delete from Customer where v.Name = ?",
			[]interface{}{},
			syncql.NewErrNotEnoughParamValuesSpecified(ctx, 36),
		},
	}

	for _, test := range basic {
		_, rs, err := db.Exec(ctx, test.query, test.params...)
		if err == nil {
			err = rs.Err()
		}
		// Test both that the IDs compare and the text compares (since the offset needs to match).
		// Note: This is a little tricky because the actual error message will contain the calling
		//       module.
		wantPrefix := test.err.Error()[:strings.Index(test.err.Error(), ":")]
		wantSuffix := test.err.Error()[len(wantPrefix)+1:]
		if verror.ErrorID(err) != verror.ErrorID(test.err) || !strings.HasPrefix(err.Error(), wantPrefix) || !strings.HasSuffix(err.Error(), wantSuffix) {
			t.Errorf("query: %s, params: %v; got %v, want %v", test.query, test.params, err, test.err)
		}
	}
}
