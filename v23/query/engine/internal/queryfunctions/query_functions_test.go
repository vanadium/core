// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package queryfunctions_test

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	ds "v.io/v23/query/engine/datasource"
	"v.io/v23/query/engine/internal/queryfunctions"
	"v.io/v23/query/engine/internal/queryparser"
	"v.io/v23/query/syncql"
	"v.io/v23/vdl"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
)

type mockDB struct {
	ctx *context.T
}

func (db *mockDB) GetContext() *context.T {
	return db.ctx
}

func init() {
	var shutdown v23.Shutdown
	db.ctx, shutdown = test.V23Init()
	defer shutdown()
}

func (db *mockDB) GetTable(table string, writeAccessReq bool) (ds.Table, error) {
	return nil, errors.New("unimplemented")
}

var db mockDB

// type queryFunc func(int64, []*queryparser.Operand) (*queryparser.Operand, error)
// type checkArgsFunc func(int64, []*queryparser.Operand) (*queryparser.Operand, error)

type functionsTest struct {
	f      *queryparser.Function
	args   []*queryparser.Operand
	result *queryparser.Operand
}

type functionsErrorTest struct {
	f    *queryparser.Function
	args []*queryparser.Operand
	err  error
}

var ty2015m06d21 time.Time
var ty2015m06d21h01m23s45 time.Time
var ty2015m06d09h01m23s45_8327 time.Time

func init() {
	ty2015m06d21, _ = time.Parse("2006/01/02 MST", "2015/06/21 PDT")

	ty2015m06d21h01m23s45, _ = time.Parse("2006/01/02 15:04:05 MST", "2015/06/21 01:23:45 PDT")

	loc, _ := time.LoadLocation("America/Los_Angeles")
	ty2015m06d09h01m23s45_8327 = time.Date(2015, 6, 9, 1, 23, 45, 8327, loc)
}

func TestFunctions(t *testing.T) {
	tests := []functionsTest{
		// Time
		{
			&queryparser.Function{
				Name: "Time",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "2006-01-02 MST",
					},
					{
						Type: queryparser.TypStr,
						Str:  "2015-06-21 PDT",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
				},
				RetType:  queryparser.TypTime,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "2006-01-02 MST",
				},
				{
					Type: queryparser.TypStr,
					Str:  "2015-06-21 PDT",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypTime,
				Time: ty2015m06d21,
			},
		},
		// Time
		{
			&queryparser.Function{
				Name: "Time",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "2006-01-02 15:04:05 MST",
					},
					{
						Type: queryparser.TypStr,
						Str:  "2015-06-21 01:23:45 PDT",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
				},
				RetType:  queryparser.TypTime,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "2006-01-02 15:04:05 MST",
				},
				{
					Type: queryparser.TypStr,
					Str:  "2015-06-21 01:23:45 PDT",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypTime,
				Time: ty2015m06d21h01m23s45,
			},
		},
		// Year
		{
			&queryparser.Function{
				Name: "Year",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypTime,
						Time: ty2015m06d09h01m23s45_8327,
					},
					{
						Type: queryparser.TypStr,
						Str:  "America/Los_Angeles",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypTime,
					queryparser.TypStr,
				},
				RetType:  queryparser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypTime,
					Time: ty2015m06d09h01m23s45_8327,
				},
				{
					Type: queryparser.TypStr,
					Str:  "America/Los_Angeles",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypInt,
				Int:  2015,
			},
		},
		// Month
		{
			&queryparser.Function{
				Name: "Month",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypTime,
						Time: ty2015m06d09h01m23s45_8327,
					},
					{
						Type: queryparser.TypStr,
						Str:  "America/Los_Angeles",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypTime,
					queryparser.TypStr,
				},
				RetType:  queryparser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypTime,
					Time: ty2015m06d09h01m23s45_8327,
				},
				{
					Type: queryparser.TypStr,
					Str:  "America/Los_Angeles",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypInt,
				Int:  6,
			},
		},
		// Day
		{
			&queryparser.Function{
				Name: "Day",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypTime,
						Time: ty2015m06d09h01m23s45_8327,
					},
					{
						Type: queryparser.TypStr,
						Str:  "America/Los_Angeles",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypTime,
					queryparser.TypStr,
				},
				RetType:  queryparser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypTime,
					Time: ty2015m06d09h01m23s45_8327,
				},
				{
					Type: queryparser.TypStr,
					Str:  "America/Los_Angeles",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypInt,
				Int:  9,
			},
		},
		// Hour
		{
			&queryparser.Function{
				Name: "Hour",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypTime,
						Time: ty2015m06d09h01m23s45_8327,
					},
					{
						Type: queryparser.TypStr,
						Str:  "America/Los_Angeles",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypTime,
					queryparser.TypStr,
				},
				RetType:  queryparser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypTime,
					Time: ty2015m06d09h01m23s45_8327,
				},
				{
					Type: queryparser.TypStr,
					Str:  "America/Los_Angeles",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypInt,
				Int:  1,
			},
		},
		// Minute
		{
			&queryparser.Function{
				Name: "Minute",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypTime,
						Time: ty2015m06d09h01m23s45_8327,
					},
					{
						Type: queryparser.TypStr,
						Str:  "America/Los_Angeles",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypTime,
					queryparser.TypStr,
				},
				RetType:  queryparser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypTime,
					Time: ty2015m06d09h01m23s45_8327,
				},
				{
					Type: queryparser.TypStr,
					Str:  "America/Los_Angeles",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypInt,
				Int:  23,
			},
		},
		// Second
		{
			&queryparser.Function{
				Name: "Second",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypTime,
						Time: ty2015m06d09h01m23s45_8327,
					},
					{
						Type: queryparser.TypStr,
						Str:  "America/Los_Angeles",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypTime,
					queryparser.TypStr,
				},
				RetType:  queryparser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypTime,
					Time: ty2015m06d09h01m23s45_8327,
				},
				{
					Type: queryparser.TypStr,
					Str:  "America/Los_Angeles",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypInt,
				Int:  45,
			},
		},
		// Nanosecond
		{
			&queryparser.Function{
				Name: "Nanosecond",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypTime,
						Time: ty2015m06d09h01m23s45_8327,
					},
					{
						Type: queryparser.TypStr,
						Str:  "America/Los_Angeles",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypTime,
					queryparser.TypStr,
				},
				RetType:  queryparser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypTime,
					Time: ty2015m06d09h01m23s45_8327,
				},
				{
					Type: queryparser.TypStr,
					Str:  "America/Los_Angeles",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypInt,
				Int:  8327,
			},
		},
		// Weekday
		{
			&queryparser.Function{
				Name: "Weekday",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypTime,
						Time: ty2015m06d09h01m23s45_8327,
					},
					{
						Type: queryparser.TypStr,
						Str:  "America/Los_Angeles",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypTime,
					queryparser.TypStr,
				},
				RetType:  queryparser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypTime,
					Time: ty2015m06d09h01m23s45_8327,
				},
				{
					Type: queryparser.TypStr,
					Str:  "America/Los_Angeles",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypInt,
				Int:  2,
			},
		},
		// YearDay
		{
			&queryparser.Function{
				Name: "YearDay",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypTime,
						Time: ty2015m06d09h01m23s45_8327,
					},
					{
						Type: queryparser.TypStr,
						Str:  "America/Los_Angeles",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypTime,
					queryparser.TypStr,
				},
				RetType:  queryparser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypTime,
					Time: ty2015m06d09h01m23s45_8327,
				},
				{
					Type: queryparser.TypStr,
					Str:  "America/Los_Angeles",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypInt,
				Int:  160,
			},
		},
		// Atoi
		{
			&queryparser.Function{
				Name: "Atoi",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "12345",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
				},
				RetType:  queryparser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "12345",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypInt,
				Int:  12345,
			},
		},
		// Atof
		{
			&queryparser.Function{
				Name: "Atof",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "1234.5",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
				},
				RetType:  queryparser.TypFloat,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "1234.5",
				},
			},
			&queryparser.Operand{
				Type:  queryparser.TypFloat,
				Float: 1234.5,
			},
		},
		// Lowercase
		{
			&queryparser.Function{
				Name: "Lowercase",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "FooBar",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
				},
				RetType:  queryparser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "FooBar",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypStr,
				Str:  "foobar",
			},
		},
		// HtmlEscape
		{
			&queryparser.Function{
				Name: "HtmlEscape",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "<a>FooBar</a>",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
				},
				RetType:  queryparser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "<a>FooBar</a>",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypStr,
				Str:  "&lt;a&gt;FooBar&lt;/a&gt;",
			},
		},
		// HtmlUnescape
		{
			&queryparser.Function{
				Name: "HtmlUnescape",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "&lt;a&gt;FooBar&lt;/a&gt;",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
				},
				RetType:  queryparser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "&lt;a&gt;FooBar&lt;/a&gt;",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypStr,
				Str:  "<a>FooBar</a>",
			},
		},
		// Uppercase
		{
			&queryparser.Function{
				Name: "Uppercase",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "FooBar",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
				},
				RetType:  queryparser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "FooBar",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypStr,
				Str:  "FOOBAR",
			},
		},
		// Split
		{
			&queryparser.Function{
				Name: "Split",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "alpha.bravo.charlie.delta.echo",
					},
					{
						Type: queryparser.TypStr,
						Str:  ".",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
					queryparser.TypStr,
				},
				RetType:  queryparser.TypObject,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "alpha.bravo.charlie.delta.echo",
				},
				{
					Type: queryparser.TypStr,
					Str:  ".",
				},
			},
			&queryparser.Operand{
				Type:   queryparser.TypObject,
				Object: vdl.ValueOf([]string{"alpha", "bravo", "charlie", "delta", "echo"}),
			},
		},
		// Len (of list)
		{
			&queryparser.Function{
				Name: "Len",
				Args: []*queryparser.Operand{
					{
						Type:   queryparser.TypObject,
						Object: vdl.ValueOf([]string{"alpha", "bravo"}),
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypObject,
				},
				RetType:  queryparser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type:   queryparser.TypObject,
					Object: vdl.ValueOf([]string{"alpha", "bravo"}),
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypInt,
				Int:  2,
			},
		},
		// Len (of nil)
		{
			&queryparser.Function{
				Name: "Len",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypNil,
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypObject,
				},
				RetType:  queryparser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypNil,
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypInt,
				Int:  0,
			},
		},
		// Len (of string)
		{
			&queryparser.Function{
				Name: "Len",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "foo",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypObject,
				},
				RetType:  queryparser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "foo",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypInt,
				Int:  3,
			},
		},
		// Len (of map)
		{
			&queryparser.Function{
				Name: "Len",
				Args: []*queryparser.Operand{
					{
						Type:   queryparser.TypObject,
						Object: vdl.ValueOf(map[string]string{"alpha": "ALPHA", "bravo": "BRAVO"}),
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypObject,
				},
				RetType:  queryparser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type:   queryparser.TypObject,
					Object: vdl.ValueOf(map[string]string{"alpha": "ALPHA", "bravo": "BRAVO"}),
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypInt,
				Int:  2,
			},
		},
		// Sprintf
		{
			&queryparser.Function{
				Name: "Sprintf",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "abc%sghi%s",
					},
					{
						Type: queryparser.TypStr,
						Str:  "def",
					},
					{
						Type: queryparser.TypStr,
						Str:  "jkl",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
					queryparser.TypStr,
					queryparser.TypStr,
				},
				RetType:  queryparser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "abc%sghi%s",
				},
				{
					Type: queryparser.TypStr,
					Str:  "def",
				},
				{
					Type: queryparser.TypStr,
					Str:  "jkl",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypStr,
				Str:  "abcdefghijkl",
			},
		},
		// Sprintf
		{
			&queryparser.Function{
				Name: "Sprintf",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "The meaning of life is %d.",
					},
					{
						Type: queryparser.TypInt,
						Int:  42,
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
					queryparser.TypStr,
				},
				RetType:  queryparser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "The meaning of life is %d.",
				},
				{
					Type: queryparser.TypInt,
					Int:  42,
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypStr,
				Str:  "The meaning of life is 42.",
			},
		},
		// Sprintf
		{
			&queryparser.Function{
				Name: "Sprintf",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "foo",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
				},
				RetType:  queryparser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "foo",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypStr,
				Str:  "foo",
			},
		},
		// StrCat (2 args)
		{
			&queryparser.Function{
				Name: "StrCat",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "Foo",
					},
					{
						Type: queryparser.TypStr,
						Str:  "Bar",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
					queryparser.TypStr,
				},
				RetType:  queryparser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "Foo",
				},
				{
					Type: queryparser.TypStr,
					Str:  "Bar",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypStr,
				Str:  "FooBar",
			},
		},
		// StrCat (3 args)
		{
			&queryparser.Function{
				Name: "StrCat",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "Foo",
					},
					{
						Type: queryparser.TypStr,
						Str:  ",",
					},
					{
						Type: queryparser.TypStr,
						Str:  "Bar",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
					queryparser.TypStr,
					queryparser.TypStr,
				},
				RetType:  queryparser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "Foo",
				},
				{
					Type: queryparser.TypStr,
					Str:  ",",
				},
				{
					Type: queryparser.TypStr,
					Str:  "Bar",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypStr,
				Str:  "Foo,Bar",
			},
		},
		// StrCat (5 args)
		{
			&queryparser.Function{
				Name: "StrCat",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "[",
					},
					{
						Type: queryparser.TypStr,
						Str:  "Foo",
					},
					{
						Type: queryparser.TypStr,
						Str:  "]",
					},
					{
						Type: queryparser.TypStr,
						Str:  "[",
					},
					{
						Type: queryparser.TypStr,
						Str:  "Bar",
					},
					{
						Type: queryparser.TypStr,
						Str:  "]",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
					queryparser.TypStr,
				},
				RetType:  queryparser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "[",
				},
				{
					Type: queryparser.TypStr,
					Str:  "Foo",
				},
				{
					Type: queryparser.TypStr,
					Str:  "]",
				},
				{
					Type: queryparser.TypStr,
					Str:  "[",
				},
				{
					Type: queryparser.TypStr,
					Str:  "Bar",
				},
				{
					Type: queryparser.TypStr,
					Str:  "]",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypStr,
				Str:  "[Foo][Bar]",
			},
		},
		// StrIndex
		{
			&queryparser.Function{
				Name: "StrIndex",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "FooBar",
					},
					{
						Type: queryparser.TypStr,
						Str:  "Bar",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
					queryparser.TypStr,
				},
				RetType:  queryparser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "FooBar",
				},
				{
					Type: queryparser.TypStr,
					Str:  "Bar",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypInt,
				Int:  3,
			},
		},
		// StrIndex
		{
			&queryparser.Function{
				Name: "StrIndex",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "FooBar",
					},
					{
						Type: queryparser.TypStr,
						Str:  "Baz",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
					queryparser.TypStr,
				},
				RetType:  queryparser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "FooBar",
				},
				{
					Type: queryparser.TypStr,
					Str:  "Baz",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypInt,
				Int:  -1,
			},
		},
		// StrLastIndex
		{
			&queryparser.Function{
				Name: "StrLastIndex",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "FooBarBar",
					},
					{
						Type: queryparser.TypStr,
						Str:  "Bar",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
					queryparser.TypStr,
				},
				RetType:  queryparser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "FooBarBar",
				},
				{
					Type: queryparser.TypStr,
					Str:  "Bar",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypInt,
				Int:  6,
			},
		},
		// StrLastIndex
		{
			&queryparser.Function{
				Name: "StrLastIndex",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "FooBar",
					},
					{
						Type: queryparser.TypStr,
						Str:  "Baz",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
					queryparser.TypStr,
				},
				RetType:  queryparser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "FooBar",
				},
				{
					Type: queryparser.TypStr,
					Str:  "Baz",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypInt,
				Int:  -1,
			},
		},
		// StrRepeat
		{
			&queryparser.Function{
				Name: "StrRepeat",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "FooBar",
					},
					{
						Type: queryparser.TypInt,
						Int:  2,
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
					queryparser.TypInt,
				},
				RetType:  queryparser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "FooBar",
				},
				{
					Type: queryparser.TypInt,
					Int:  2,
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypStr,
				Str:  "FooBarFooBar",
			},
		},
		// StrRepeat
		{
			&queryparser.Function{
				Name: "StrRepeat",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "FooBar",
					},
					{
						Type: queryparser.TypInt,
						Int:  0,
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
					queryparser.TypInt,
				},
				RetType:  queryparser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "FooBar",
				},
				{
					Type: queryparser.TypInt,
					Int:  0,
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypStr,
				Str:  "",
			},
		},
		// StrRepeat
		{
			&queryparser.Function{
				Name: "StrRepeat",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "FooBar",
					},
					{
						Type: queryparser.TypInt,
						Int:  -1,
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
					queryparser.TypInt,
				},
				RetType:  queryparser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "FooBar",
				},
				{
					Type: queryparser.TypInt,
					Int:  -1,
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypStr,
				Str:  "",
			},
		},
		// StrReplace
		{
			&queryparser.Function{
				Name: "StrReplace",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "FooBar",
					},
					{
						Type: queryparser.TypStr,
						Str:  "B",
					},
					{
						Type: queryparser.TypStr,
						Str:  "ZZZ",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
					queryparser.TypStr,
					queryparser.TypStr,
				},
				RetType:  queryparser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "FooBar",
				},
				{
					Type: queryparser.TypStr,
					Str:  "B",
				},
				{
					Type: queryparser.TypStr,
					Str:  "ZZZ",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypStr,
				Str:  "FooZZZar",
			},
		},
		// StrReplace
		{
			&queryparser.Function{
				Name: "StrReplace",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "FooBar",
					},
					{
						Type: queryparser.TypStr,
						Str:  "X",
					},
					{
						Type: queryparser.TypStr,
						Str:  "ZZZ",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
					queryparser.TypStr,
					queryparser.TypStr,
				},
				RetType:  queryparser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "FooBar",
				},
				{
					Type: queryparser.TypStr,
					Str:  "X",
				},
				{
					Type: queryparser.TypStr,
					Str:  "ZZZ",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypStr,
				Str:  "FooBar",
			},
		},
		// Trim
		{
			&queryparser.Function{
				Name: "Trim",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "     Foo  ",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
				},
				RetType:  queryparser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "     Foo  ",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypStr,
				Str:  "Foo",
			},
		},
		// Trim
		{
			&queryparser.Function{
				Name: "Trim",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "Foo",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
				},
				RetType:  queryparser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "Foo",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypStr,
				Str:  "Foo",
			},
		},
		// TrimLeft
		{
			&queryparser.Function{
				Name: "TrimLeft",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "     Foo  ",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
				},
				RetType:  queryparser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "     Foo  ",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypStr,
				Str:  "Foo  ",
			},
		},
		// TrimLeft
		{
			&queryparser.Function{
				Name: "TrimLeft",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "Foo",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
				},
				RetType:  queryparser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "Foo",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypStr,
				Str:  "Foo",
			},
		},
		// TrimRight
		{
			&queryparser.Function{
				Name: "TrimRight",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "     Foo  ",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
				},
				RetType:  queryparser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "     Foo  ",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypStr,
				Str:  "     Foo",
			},
		},
		// TrimLeft
		{
			&queryparser.Function{
				Name: "TrimLeft",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "Foo",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
				},
				RetType:  queryparser.TypStr,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "Foo",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypStr,
				Str:  "Foo",
			},
		},
		// Ceiling
		{
			&queryparser.Function{
				Name: "Ceiling",
				Args: []*queryparser.Operand{
					{
						Type:  queryparser.TypFloat,
						Float: 123.567,
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypFloat,
				},
				RetType:  queryparser.TypFloat,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type:  queryparser.TypFloat,
					Float: 123.567,
				},
			},
			&queryparser.Operand{
				Type:  queryparser.TypFloat,
				Float: 124.0,
			},
		},
		// Ceiling
		{
			&queryparser.Function{
				Name: "Ceiling",
				Args: []*queryparser.Operand{
					{
						Type:  queryparser.TypFloat,
						Float: -123.567,
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypFloat,
				},
				RetType:  queryparser.TypFloat,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type:  queryparser.TypFloat,
					Float: -123.567,
				},
			},
			&queryparser.Operand{
				Type:  queryparser.TypFloat,
				Float: -123.0,
			},
		},
		// Floor
		{
			&queryparser.Function{
				Name: "Floor",
				Args: []*queryparser.Operand{
					{
						Type:  queryparser.TypFloat,
						Float: 123.567,
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypFloat,
				},
				RetType:  queryparser.TypFloat,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type:  queryparser.TypFloat,
					Float: 123.567,
				},
			},
			&queryparser.Operand{
				Type:  queryparser.TypFloat,
				Float: 123.0,
			},
		},
		// Floor
		{
			&queryparser.Function{
				Name: "Floor",
				Args: []*queryparser.Operand{
					{
						Type:  queryparser.TypFloat,
						Float: -123.567,
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypFloat,
				},
				RetType:  queryparser.TypFloat,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type:  queryparser.TypFloat,
					Float: -123.567,
				},
			},
			&queryparser.Operand{
				Type:  queryparser.TypFloat,
				Float: -124.0,
			},
		},
		// Truncate
		{
			&queryparser.Function{
				Name: "Truncate",
				Args: []*queryparser.Operand{
					{
						Type:  queryparser.TypFloat,
						Float: 123.567,
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypFloat,
				},
				RetType:  queryparser.TypFloat,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type:  queryparser.TypFloat,
					Float: 123.567,
				},
			},
			&queryparser.Operand{
				Type:  queryparser.TypFloat,
				Float: 123.0,
			},
		},
		// Truncate
		{
			&queryparser.Function{
				Name: "Truncate",
				Args: []*queryparser.Operand{
					{
						Type:  queryparser.TypFloat,
						Float: -123.567,
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypFloat,
				},
				RetType:  queryparser.TypFloat,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type:  queryparser.TypFloat,
					Float: -123.567,
				},
			},
			&queryparser.Operand{
				Type:  queryparser.TypFloat,
				Float: -123.0,
			},
		},
		// IsNaN
		{
			&queryparser.Function{
				Name: "IsNaN",
				Args: []*queryparser.Operand{
					{
						Type:  queryparser.TypFloat,
						Float: math.NaN(),
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypFloat,
				},
				RetType:  queryparser.TypBool,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type:  queryparser.TypFloat,
					Float: math.NaN(),
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypBool,
				Bool: true,
			},
		},
		// IsNaN
		{
			&queryparser.Function{
				Name: "IsNaN",
				Args: []*queryparser.Operand{
					{
						Type:  queryparser.TypFloat,
						Float: 123.456,
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypFloat,
				},
				RetType:  queryparser.TypBool,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type:  queryparser.TypFloat,
					Float: 123.456,
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypBool,
				Bool: false,
			},
		},
		// IsInf
		{
			&queryparser.Function{
				Name: "IsInf",
				Args: []*queryparser.Operand{
					{
						Type:  queryparser.TypFloat,
						Float: math.Inf(1),
					},
					{
						Type: queryparser.TypInt,
						Int:  1,
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypFloat,
					queryparser.TypInt,
				},
				RetType:  queryparser.TypBool,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type:  queryparser.TypFloat,
					Float: math.Inf(1),
				},
				{
					Type: queryparser.TypInt,
					Int:  1,
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypBool,
				Bool: true,
			},
		},
		// IsInf
		{
			&queryparser.Function{
				Name: "IsInf",
				Args: []*queryparser.Operand{
					{
						Type:  queryparser.TypFloat,
						Float: math.Inf(-1),
					},
					{
						Type: queryparser.TypInt,
						Int:  -1,
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypFloat,
					queryparser.TypInt,
				},
				RetType:  queryparser.TypBool,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type:  queryparser.TypFloat,
					Float: math.Inf(-1),
				},
				{
					Type: queryparser.TypInt,
					Int:  -1,
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypBool,
				Bool: true,
			},
		},
		// IsInf
		{
			&queryparser.Function{
				Name: "IsInf",
				Args: []*queryparser.Operand{
					{
						Type:  queryparser.TypFloat,
						Float: 123.456,
					},
					{
						Type: queryparser.TypInt,
						Int:  0,
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypFloat,
					queryparser.TypInt,
				},
				RetType:  queryparser.TypBool,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type:  queryparser.TypFloat,
					Float: 123.456,
				},
				{
					Type: queryparser.TypInt,
					Int:  0,
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypBool,
				Bool: false,
			},
		},
		// Log
		{
			&queryparser.Function{
				Name: "Log",
				Args: []*queryparser.Operand{
					{
						Type:  queryparser.TypFloat,
						Float: 10.5,
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypFloat,
				},
				RetType:  queryparser.TypFloat,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type:  queryparser.TypFloat,
					Float: 10.5,
				},
			},
			&queryparser.Operand{
				Type:  queryparser.TypFloat,
				Float: 2.3513752571634776,
			},
		},
		// Log10
		{
			&queryparser.Function{
				Name: "Log10",
				Args: []*queryparser.Operand{
					{
						Type:  queryparser.TypFloat,
						Float: 10.5,
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypFloat,
				},
				RetType:  queryparser.TypFloat,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type:  queryparser.TypFloat,
					Float: 10.5,
				},
			},
			&queryparser.Operand{
				Type:  queryparser.TypFloat,
				Float: 1.021189299069938,
			},
		},
		// Pow
		{
			&queryparser.Function{
				Name: "Pow",
				Args: []*queryparser.Operand{
					{
						Type:  queryparser.TypFloat,
						Float: 10.0,
					},
					{
						Type:  queryparser.TypFloat,
						Float: 2.0,
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypFloat,
					queryparser.TypFloat,
				},
				RetType:  queryparser.TypFloat,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type:  queryparser.TypFloat,
					Float: 10.0,
				},
				{
					Type:  queryparser.TypFloat,
					Float: 2.0,
				},
			},
			&queryparser.Operand{
				Type:  queryparser.TypFloat,
				Float: float64(100.0),
			},
		},
		// Pow10
		{
			&queryparser.Function{
				Name: "Pow10",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypInt,
						Int:  3,
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypInt,
				},
				RetType:  queryparser.TypFloat,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypInt,
					Int:  3,
				},
			},
			&queryparser.Operand{
				Type:  queryparser.TypFloat,
				Float: float64(1000.0),
			},
		},
		// Mod
		{
			&queryparser.Function{
				Name: "Mod",
				Args: []*queryparser.Operand{
					{
						Type:  queryparser.TypFloat,
						Float: 10.5,
					},
					{
						Type:  queryparser.TypFloat,
						Float: 3.2,
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypFloat,
					queryparser.TypFloat,
				},
				RetType:  queryparser.TypFloat,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type:  queryparser.TypFloat,
					Float: 10.5,
				},
				{
					Type:  queryparser.TypFloat,
					Float: 3.2,
				},
			},
			&queryparser.Operand{
				Type:  queryparser.TypFloat,
				Float: 0.8999999999999995,
			},
		},
		// Mod
		{
			&queryparser.Function{
				Name: "Mod",
				Args: []*queryparser.Operand{
					{
						Type:  queryparser.TypFloat,
						Float: -10.5,
					},
					{
						Type:  queryparser.TypFloat,
						Float: 3.2,
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypFloat,
					queryparser.TypFloat,
				},
				RetType:  queryparser.TypFloat,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type:  queryparser.TypFloat,
					Float: -10.5,
				},
				{
					Type:  queryparser.TypFloat,
					Float: 3.2,
				},
			},
			&queryparser.Operand{
				Type:  queryparser.TypFloat,
				Float: -0.8999999999999995,
			},
		},
		// Remainder
		{
			&queryparser.Function{
				Name: "Remainder",
				Args: []*queryparser.Operand{
					{
						Type:  queryparser.TypFloat,
						Float: 10.5,
					},
					{
						Type:  queryparser.TypFloat,
						Float: 3.2,
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypFloat,
					queryparser.TypFloat,
				},
				RetType:  queryparser.TypFloat,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type:  queryparser.TypFloat,
					Float: 10.5,
				},
				{
					Type:  queryparser.TypFloat,
					Float: 3.2,
				},
			},
			&queryparser.Operand{
				Type:  queryparser.TypFloat,
				Float: 0.8999999999999995,
			},
		},
		// Remainder
		{
			&queryparser.Function{
				Name: "Remainder",
				Args: []*queryparser.Operand{
					{
						Type:  queryparser.TypFloat,
						Float: -10.5,
					},
					{
						Type:  queryparser.TypFloat,
						Float: 3.2,
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypFloat,
					queryparser.TypFloat,
				},
				RetType:  queryparser.TypFloat,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type:  queryparser.TypFloat,
					Float: -10.5,
				},
				{
					Type:  queryparser.TypFloat,
					Float: 3.2,
				},
			},
			&queryparser.Operand{
				Type:  queryparser.TypFloat,
				Float: -0.8999999999999995,
			},
		},
		// RuneCount
		{
			&queryparser.Function{
				Name: "RuneCount",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "Hello, 世界",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
				},
				RetType:  queryparser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "Hello, 世界",
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypInt,
				Int:  9,
			},
		},
		// Len
		{
			&queryparser.Function{
				Name: "Len",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "Hello, 世界",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
				},
				RetType:  queryparser.TypInt,
				Computed: false,
				RetValue: nil,
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "Hello, 世界",
				},
				{
					Type:  queryparser.TypInt,
					Float: 13,
				},
			},
			&queryparser.Operand{
				Type: queryparser.TypInt,
				Int:  13,
			},
		},
	}

	for _, test := range tests {
		r, err := queryfunctions.ExecFunction(&db, test.f, test.args)
		if err != nil {
			t.Errorf("function: %v; unexpected error: got %v, want nil", test.f, err)
		}
		if !reflect.DeepEqual(test.result, r) {
			t.Errorf("function: %v; got %v, want %v", test.f, r, test.result)
		}
	}
}

func TestErrorFunctions(t *testing.T) {
	tests := []functionsErrorTest{
		// time
		{
			&queryparser.Function{
				Name: "time",
				Args: []*queryparser.Operand{
					{
						Type: queryparser.TypStr,
						Str:  "2006-01-02 MST",
					},
					{
						Type: queryparser.TypStr,
						Str:  "2015-06-21 PDT",
					},
				},
				ArgTypes: []queryparser.OperandType{
					queryparser.TypStr,
				},
				RetType:  queryparser.TypTime,
				Computed: false,
				RetValue: nil,
				Node:     queryparser.Node{Off: 42},
			},
			[]*queryparser.Operand{
				{
					Type: queryparser.TypStr,
					Str:  "2006-01-02 MST",
				},
				{
					Type: queryparser.TypStr,
					Str:  "2015-06-21 PDT",
				},
			},
			syncql.ErrorfDidYouMeanFunction(db.GetContext(), "[%v]did you mean: '%v'?", int64(42), "Time"),
		},
	}

	for _, test := range tests {
		_, err := queryfunctions.ExecFunction(&db, test.f, test.args)
		if !errors.Is(err, test.err) || err.Error() != test.err.Error() {
			t.Errorf("function: %v; got %v, want %v", test.f, err, test.err)
		}
	}
}
