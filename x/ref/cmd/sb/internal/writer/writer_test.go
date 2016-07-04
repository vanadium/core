// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package writer_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"v.io/v23/syncbase"
	"v.io/v23/vdl"
	"v.io/v23/vom"
	db "v.io/x/ref/cmd/sb/internal/demodb"
	"v.io/x/ref/cmd/sb/internal/writer"
)

type fakeResultStream struct {
	rows [][]vdl.Value
	curr int
}

var (
	customer = db.Customer{
		Name:   "John Smith",
		Id:     1,
		Active: true,
		Address: db.AddressInfo{
			Street: "1 Main St.",
			City:   "Palo Alto",
			State:  "CA",
			Zip:    "94303",
		},
		Credit: db.CreditReport{
			Agency: db.CreditAgencyEquifax,
			Report: db.AgencyReportEquifaxReport{Value: db.EquifaxCreditReport{Rating: 'A'}},
		},
	}
	invoice = db.Invoice{
		CustId:     1,
		InvoiceNum: 1000,
		Amount:     42,
		ShipTo: db.AddressInfo{
			Street: "1 Main St.",
			City:   "Palo Alto",
			State:  "CA",
			Zip:    "94303",
		},
	}
)

func array2String(s1, s2 string) db.Array2String {
	a := [2]string{s1, s2}
	return db.Array2String(a)
}

func newResultStream(iRows [][]interface{}) syncbase.ResultStream {
	vRows := make([][]vdl.Value, len(iRows))
	for i, iRow := range iRows {
		vRow := make([]vdl.Value, len(iRow))
		for j, iCol := range iRow {
			vRow[j] = *vdl.ValueOf(iCol)
		}
		vRows[i] = vRow
	}
	return &fakeResultStream{
		rows: vRows,
		curr: -1,
	}
}

func (f *fakeResultStream) Advance() bool {
	f.curr++
	return f.curr < len(f.rows)
}

func (f *fakeResultStream) ResultCount() int {
	if f.curr == -1 {
		panic("call advance first")
	}
	return len(f.rows[f.curr])
}

func (f *fakeResultStream) Result(i int, value interface{}) (err error) {
	if f.curr == -1 {
		panic("call advance first")
	}
	switch intf := value.(type) {
	case **vdl.Value:
		*intf = &f.rows[f.curr][i]
	default:
		err = fmt.Errorf("unsupported value type: %t", value)
	}
	return err
}

func (f *fakeResultStream) Err() error {
	return nil
}

func (f *fakeResultStream) Cancel() {
	// Nothing to do.
}

func TestWriteTable(t *testing.T) {
	type testCase struct {
		columns []string
		rows    [][]interface{}
		// To make the test cases easier to read, output should have a leading
		// newline.
		output string
	}
	tests := []testCase{
		{
			[]string{"c1", "c2"},
			[][]interface{}{
				{5, "foo"},
				{6, "bar"},
			},
			`
+----+-----+
| c1 |  c2 |
+----+-----+
|  5 | foo |
|  6 | bar |
+----+-----+
`,
		},
		{
			[]string{"c1", "c2"},
			[][]interface{}{
				{500, "foo"},
				{6, "barbaz"},
			},
			`
+-----+--------+
|  c1 |     c2 |
+-----+--------+
| 500 | foo    |
|   6 | barbaz |
+-----+--------+
`,
		},
		{
			[]string{"c1", "reallylongcolumnheader"},
			[][]interface{}{
				{5, "foo"},
				{6, "bar"},
			},
			`
+----+------------------------+
| c1 | reallylongcolumnheader |
+----+------------------------+
|  5 | foo                    |
|  6 | bar                    |
+----+------------------------+
`,
		},
		{ // Numbers.
			[]string{"byte", "uint16", "uint32", "uint64", "int16", "int32", "int64",
				"float32", "float64"},
			[][]interface{}{
				{
					byte(12), uint16(1234), uint32(5678), uint64(999888777666), int16(9876), int32(876543), int64(128),
					float32(3.14159), float64(2.71828182846),
				},
				{
					byte(9), uint16(99), uint32(999), uint64(9999999), int16(9), int32(99), int64(88),
					float32(1.41421356237), float64(1.73205080757),
				},
			},
			`
+------+--------+--------+--------------+-------+--------+-------+--------------------+---------------+
| byte | uint16 | uint32 |       uint64 | int16 |  int32 | int64 |            float32 |       float64 |
+------+--------+--------+--------------+-------+--------+-------+--------------------+---------------+
|   12 |   1234 |   5678 | 999888777666 |  9876 | 876543 |   128 |  3.141590118408203 | 2.71828182846 |
|    9 |     99 |    999 |      9999999 |     9 |     99 |    88 | 1.4142135381698608 | 1.73205080757 |
+------+--------+--------+--------------+-------+--------+-------+--------------------+---------------+
`,
		},
		{ // Strings with whitespace should be printed literally.
			[]string{"c1", "c2"},
			[][]interface{}{
				{"foo\tbar", "foo\nbar"},
			},
			`
+---------+---------+
|      c1 |      c2 |
+---------+---------+
| foo	bar | foo
bar |
+---------+---------+
`,
		},
		{ // nil is shown as blank.
			[]string{"c1"},
			[][]interface{}{
				{nil},
			},
			`
+----+
| c1 |
+----+
|    |
+----+
`,
		},
		{
			[]string{"c1"},
			[][]interface{}{{customer}, {invoice}},
			`
+------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
|                                                                                                                                                                                       c1 |
+------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| {Name: "John Smith", Id: 1, Active: true, Address: {Street: "1 Main St.", City: "Palo Alto", State: "CA", Zip: "94303"}, Credit: {Agency: Equifax, Report: EquifaxReport: {Rating: 65}}} |
| {CustId: 1, InvoiceNum: 1000, Amount: 42, ShipTo: {Street: "1 Main St.", City: "Palo Alto", State: "CA", Zip: "94303"}}                                                                  |
+------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
`,
		},
		{
			[]string{"c1"},
			[][]interface{}{
				{db.Composite{array2String("foo", "棎鶊鵱"), []int32{1, 2}, map[int32]struct{}{1: struct{}{}, 2: struct{}{}}, map[string]int32{"foo": 1, "bar": 2}}},
			},
			`
+----------------------------------------------------------------------------------+
|                                                                               c1 |
+----------------------------------------------------------------------------------+
| {Arr: ["foo", "棎鶊鵱"], ListInt: [1, 2], MySet: {1, 2}, Map: {"bar": 2, "foo": 1}} |
+----------------------------------------------------------------------------------+
`,
		},
		{ // Types not built in to Go.
			[]string{"time", "type", "union", "enum", "set"},
			[][]interface{}{
				{time.Unix(13377331, 0), vdl.TypeOf(map[float32]struct{ B bool }{}), db.TitleOrValueTypeTitle{"dahar master"}, db.ExperianRatingBad, map[int32]struct{}{47: struct{}{}}},
			},
			`
+-------------------------------+----------------------------------------+-----------------------+------+------+
|                          time |                                   type |                 union | enum |  set |
+-------------------------------+----------------------------------------+-----------------------+------+------+
| 1970-06-04 19:55:31 +0000 UTC | typeobject(map[float32]struct{B bool}) | Title: "dahar master" | Bad  | {47} |
+-------------------------------+----------------------------------------+-----------------------+------+------+
`,
		},
		{
			[]string{"c1"},
			[][]interface{}{
				{
					db.Recursive{
						Any: nil,
						Maybe: &db.Times{
							Stamp:    time.Unix(123456789, 42244224),
							Interval: time.Duration(13377331),
						},
						Rec: map[db.Array2String]db.Recursive{
							array2String("a", "b"): db.Recursive{},
							array2String("x\nx", "y\"y"): db.Recursive{
								Any:   vom.RawBytesOf(db.AgencyReportExperianReport{Value: db.ExperianCreditReport{Rating: db.ExperianRatingGood}}),
								Maybe: nil,
								Rec:   nil,
							},
						},
					},
				},
			},
			`
+----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
|                                                                                                                                                                                                                               c1 |
+----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| {Any: nil, Maybe: {Stamp: "1973-11-29 21:33:09.042244224 +0000 UTC", Interval: "13.377331ms"}, Rec: {["a", "b"]: {Any: nil, Maybe: nil, Rec: {}}, ["x\nx", "y\"y"]: {Any: ExperianReport: {Rating: Good}, Maybe: nil, Rec: {}}}} |
+----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
`,
		},
	}
	for _, test := range tests {
		var b bytes.Buffer
		if err := writer.NewTableWriter(&b).Write(test.columns, newResultStream(test.rows)); err != nil {
			t.Errorf("Unexpected error: %v", err)
			continue
		}
		// Add a leading newline to the output to match the leading newline
		// in our test cases.
		if got, want := "\n"+b.String(), test.output; got != want {
			t.Errorf("Wrong output:\nGOT:%s\nWANT:%s", got, want)
		}
	}
}

func TestWriteCSV(t *testing.T) {
	type testCase struct {
		columns   []string
		rows      [][]interface{}
		delimiter string
		// To make the test cases easier to read, output should have a leading
		// newline.
		output string
	}
	tests := []testCase{
		{ // Basic.
			[]string{"c1", "c2"},
			[][]interface{}{
				{5, "foo"},
				{6, "bar"},
			},
			",",
			`
c1,c2
5,foo
6,bar
`,
		},
		{ // Numbers.
			[]string{"byte", "uint16", "uint32", "uint64", "int16", "int32", "int64",
				"float32", "float64"},
			[][]interface{}{
				{
					byte(12), uint16(1234), uint32(5678), uint64(999888777666), int16(9876), int32(876543), int64(128),
					float32(3.14159), float64(2.71828182846),
				},
				{
					byte(9), uint16(99), uint32(999), uint64(9999999), int16(9), int32(99), int64(88),
					float32(1.41421356237), float64(1.73205080757),
				},
			},
			",",
			`
byte,uint16,uint32,uint64,int16,int32,int64,float32,float64
12,1234,5678,999888777666,9876,876543,128,3.141590118408203,2.71828182846
9,99,999,9999999,9,99,88,1.4142135381698608,1.73205080757
`,
		},
		{
			// Values containing newlines, double quotes, and the delimiter must be
			// enclosed in double quotes.
			[]string{"c1", "c2"},
			[][]interface{}{
				{"foo\tbar", "foo\nbar"},
				{"foo\"bar\"", "foo,bar"},
			},
			",",
			`
c1,c2
foo	bar,"foo
bar"
"foo""bar""","foo,bar"
`,
		},
		{ // Delimiters other than comma should be supported.
			[]string{"c1", "c2"},
			[][]interface{}{
				{"foo\tbar", "foo\nbar"},
				{"foo\"bar\"", "foo,bar"},
			},
			"\t",
			`
c1	c2
"foo	bar"	"foo
bar"
"foo""bar"""	foo,bar
`,
		},
		{ // Column names should be escaped properly.
			[]string{"foo\tbar", "foo,bar"},
			[][]interface{}{},
			",",
			`
foo	bar,"foo,bar"
`,
		},
		{ // Same as above but use a non-default delimiter.
			[]string{"foo\tbar", "foo,棎鶊鵱"},
			[][]interface{}{},
			"\t",
			`
"foo	bar"	foo,棎鶊鵱
`,
		},
		{
			[]string{"c1"},
			[][]interface{}{{customer}, {invoice}},
			",",
			`
c1
"{Name: ""John Smith"", Id: 1, Active: true, Address: {Street: ""1 Main St."", City: ""Palo Alto"", State: ""CA"", Zip: ""94303""}, Credit: {Agency: Equifax, Report: EquifaxReport: {Rating: 65}}}"
"{CustId: 1, InvoiceNum: 1000, Amount: 42, ShipTo: {Street: ""1 Main St."", City: ""Palo Alto"", State: ""CA"", Zip: ""94303""}}"
`,
		},
	}
	for _, test := range tests {
		var b bytes.Buffer
		if err := writer.NewCSVWriter(&b, test.delimiter).Write(test.columns, newResultStream(test.rows)); err != nil {
			t.Errorf("Unexpected error: %v", err)
			continue
		}
		// Add a leading newline to the output to match the leading newline
		// in our test cases.
		if got, want := "\n"+b.String(), test.output; got != want {
			t.Errorf("Wrong output:\nGOT: %q\nWANT:%q", got, want)
		}
	}
}

func TestWriteJson(t *testing.T) {
	type testCase struct {
		columns []string
		rows    [][]interface{}
		// To make the test cases easier to read, output should have a leading
		// newline.
		output string
	}
	tests := []testCase{
		{ // Basic.
			[]string{"c\n1", "c鶊2"},
			[][]interface{}{
				{5, "foo\nbar"},
				{6, "bar\tfoo"},
			},
			`
[{
  "c\n1": 5,
  "c鶊2": "foo\nbar"
}, {
  "c\n1": 6,
  "c鶊2": "bar\tfoo"
}]
`,
		},
		{ // Numbers.
			[]string{"byte", "uint16", "uint32", "uint64", "int16", "int32", "int64",
				"float32", "float64"},
			[][]interface{}{
				{
					byte(12), uint16(1234), uint32(5678), uint64(999888777666), int16(9876), int32(876543), int64(128),
					float32(3.14159), float64(2.71828182846),
				},
				{
					byte(9), uint16(99), uint32(999), uint64(9999999), int16(9), int32(99), int64(88),
					float32(1.41421356237), float64(1.73205080757),
				},
			},
			`
[{
  "byte": 12,
  "uint16": 1234,
  "uint32": 5678,
  "uint64": 999888777666,
  "int16": 9876,
  "int32": 876543,
  "int64": 128,
  "float32": 3.141590118408203,
  "float64": 2.71828182846
}, {
  "byte": 9,
  "uint16": 99,
  "uint32": 999,
  "uint64": 9999999,
  "int16": 9,
  "int32": 99,
  "int64": 88,
  "float32": 1.4142135381698608,
  "float64": 1.73205080757
}]
`,
		},
		{ // Empty result.
			[]string{"nothing", "nada", "zilch"},
			[][]interface{}{},
			`
[]
`,
		},
		{ // Empty column set.
			[]string{},
			[][]interface{}{
				{},
				{},
			},
			`
[{
}, {
}]
`,
		},
		{ // Empty values.
			[]string{"blank", "empty", "nil"},
			[][]interface{}{
				{struct{}{}, []string{}, nil},
			},
			`
[{
  "blank": {},
  "empty": [],
  "nil": null
}]
`,
		},
		{
			[]string{"c1"},
			[][]interface{}{{customer}, {invoice}},
			`
[{
  "c1": {"Name":"John Smith","Id":1,"Active":true,"Address":{"Street":"1 Main St.","City":"Palo Alto","State":"CA","Zip":"94303"},"Credit":{"Agency":"Equifax","Report":{"EquifaxReport":{"Rating":65}}}}
}, {
  "c1": {"CustId":1,"InvoiceNum":1000,"Amount":42,"ShipTo":{"Street":"1 Main St.","City":"Palo Alto","State":"CA","Zip":"94303"}}
}]
`,
		},
		{
			[]string{"nil", "composite", "typeobj"},
			[][]interface{}{
				{
					nil,
					db.Composite{array2String("foo", "bar"), []int32{1, 2}, map[int32]struct{}{1: struct{}{}, 2: struct{}{}}, map[string]int32{"foo": 1, "bar": 2}},
					vdl.TypeOf(map[string]struct{}{}),
				},
			},
			`
[{
  "nil": null,
  "composite": {"Arr":["foo","bar"],"ListInt":[1,2],"MySet":{"1":true,"2":true},"Map":{"bar":2,"foo":1}},
  "typeobj": "typeobject(set[string])"
}]
`,
		},
		{
			[]string{"c1"},
			[][]interface{}{
				{
					db.Recursive{
						Any: nil,
						Maybe: &db.Times{
							Stamp:    time.Unix(123456789, 42244224),
							Interval: time.Duration(1337),
						},
						Rec: map[db.Array2String]db.Recursive{
							array2String("a", "棎鶊鵱"): db.Recursive{},
							array2String("x", "y"): db.Recursive{
								Any: vom.RawBytesOf(db.CreditReport{
									Agency: db.CreditAgencyExperian,
									Report: db.AgencyReportExperianReport{Value: db.ExperianCreditReport{Rating: db.ExperianRatingGood}},
								}),
								Maybe: nil,
								Rec:   nil,
							},
						}},
				},
			},
			`
[{
  "c1": {"Any":null,"Maybe":{"Stamp":"1973-11-29 21:33:09.042244224 +0000 UTC","Interval":"1.337µs"},"Rec":{"[\"a\", \"棎鶊鵱\"]":{"Any":null,"Maybe":null,"Rec":{}},"[\"x\", \"y\"]":{"Any":{"Agency":"Experian","Report":{"ExperianReport":{"Rating":"Good"}}},"Maybe":null,"Rec":{}}}}
}]
`,
		},
	}
	for _, test := range tests {
		var b bytes.Buffer
		if err := writer.NewJSONWriter(&b).Write(test.columns, newResultStream(test.rows)); err != nil {
			t.Errorf("Unexpected error: %v", err)
			continue
		}
		var decoded interface{}
		if err := json.Unmarshal(b.Bytes(), &decoded); err != nil {
			t.Errorf("Got invalid JSON: %v", err)
		}
		// Add a leading newline to the output to match the leading newline
		// in our test cases.
		if got, want := "\n"+b.String(), test.output; got != want {
			t.Errorf("Wrong output:\nGOT: %q\nWANT:%q", got, want)
		}
	}
}
