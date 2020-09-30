// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package queryparser_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	v23 "v.io/v23"
	"v.io/v23/context"
	ds "v.io/v23/query/engine/datasource"
	"v.io/v23/query/engine/internal/queryparser"
	"v.io/v23/query/syncql"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
)

type parseSelectTest struct {
	query     string
	statement queryparser.SelectStatement
	err       error //nolint:structcheck,unused
}

type parseDeleteTest struct {
	query     string
	statement queryparser.DeleteStatement
	err       error //nolint:structcheck,unused
}

type parseErrorTest struct {
	query string
	err   error //nolint:structcheck,unused
}

type toStringTest struct {
	query string
	s     string //nolint:structcheck,unused
}

type copyAndSubstituteSelectTest struct {
	query     string
	subValues []*vdl.Value
	statement queryparser.SelectStatement
}

type copyAndSubstituteDeleteTest struct {
	query     string
	subValues []*vdl.Value
	statement queryparser.DeleteStatement
}

type copyAndSubstituteErrorTest struct {
	query     string
	subValues []*vdl.Value
	err       error //nolint:structcheck,unused
}

type mockDB struct {
	ctx *context.T
}

func (db *mockDB) GetContext() *context.T {
	return db.ctx
}

func (db *mockDB) GetTable(name string, writeAccessReq bool) (ds.Table, error) {
	return nil, nil
}

var db mockDB

func init() {
	var shutdown v23.Shutdown
	db.ctx, shutdown = test.V23Init()
	defer shutdown()
}

func TestSelectParser(t *testing.T) {
	basic := []parseSelectTest{
		{
			"select v from Customer",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 14},
					},
					Node: queryparser.Node{Off: 9},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select k as Key, v as Value from Customer",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "k",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							As: &queryparser.AsClause{
								AltName: queryparser.Name{
									Value: "Key",
									Node:  queryparser.Node{Off: 12},
								},
								Node: queryparser.Node{Off: 9},
							},
							Node: queryparser.Node{Off: 7},
						},
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 17},
									},
								},
								Node: queryparser.Node{Off: 17},
							},
							As: &queryparser.AsClause{
								AltName: queryparser.Name{
									Value: "Value",
									Node:  queryparser.Node{Off: 22},
								},
								Node: queryparser.Node{Off: 19},
							},
							Node: queryparser.Node{Off: 17},
						},
					},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 33},
					},
					Node: queryparser.Node{Off: 28},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"   select v from Customer",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 10},
									},
								},
								Node: queryparser.Node{Off: 10},
							},
							Node: queryparser.Node{Off: 10},
						},
					},
					Node: queryparser.Node{Off: 3},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 17},
					},
					Node: queryparser.Node{Off: 12},
				},
				Node: queryparser.Node{Off: 3},
			},
			nil,
		},
		{
			"select v from Customer limit 100 offset 200",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 14},
					},
					Node: queryparser.Node{Off: 9},
				},
				Limit: &queryparser.LimitClause{
					Limit: &queryparser.Int64Value{
						Value: 100,
						Node:  queryparser.Node{Off: 29},
					},
					Node: queryparser.Node{Off: 23},
				},
				ResultsOffset: &queryparser.ResultsOffsetClause{
					ResultsOffset: &queryparser.Int64Value{
						Value: 200,
						Node:  queryparser.Node{Off: 40},
					},
					Node: queryparser.Node{Off: 33},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer offset 400 limit 10",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 14},
					},
					Node: queryparser.Node{Off: 9},
				},
				Limit: &queryparser.LimitClause{
					Limit: &queryparser.Int64Value{
						Value: 10,
						Node:  queryparser.Node{Off: 40},
					},
					Node: queryparser.Node{Off: 34},
				},
				ResultsOffset: &queryparser.ResultsOffsetClause{
					ResultsOffset: &queryparser.Int64Value{
						Value: 400,
						Node:  queryparser.Node{Off: 30},
					},
					Node: queryparser.Node{Off: 23},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer limit 100 offset 200 limit 1 offset 2",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 14},
					},
					Node: queryparser.Node{Off: 9},
				},
				Limit: &queryparser.LimitClause{
					Limit: &queryparser.Int64Value{
						Value: 1,
						Node:  queryparser.Node{Off: 50},
					},
					Node: queryparser.Node{Off: 44},
				},
				ResultsOffset: &queryparser.ResultsOffsetClause{
					ResultsOffset: &queryparser.Int64Value{
						Value: 2,
						Node:  queryparser.Node{Off: 59},
					},
					Node: queryparser.Node{Off: 52},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo.x, bar.y from Customer",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "foo",
										Node:  queryparser.Node{Off: 7},
									},
									{
										Value: "x",
										Node:  queryparser.Node{Off: 11},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "bar",
										Node:  queryparser.Node{Off: 14},
									},
									{
										Value: "y",
										Node:  queryparser.Node{Off: 18},
									},
								},
								Node: queryparser.Node{Off: 14},
							},
							Node: queryparser.Node{Off: 14},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 25},
					},
					Node: queryparser.Node{Off: 20},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select select from from where where equal 42",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "select",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "from",
						Node: queryparser.Node{Off: 19},
					},
					Node: queryparser.Node{Off: 14},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "where",
										Node:  queryparser.Node{Off: 30},
									},
								},
								Node: queryparser.Node{Off: 30},
							},
							Node: queryparser.Node{Off: 30},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 36},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypInt,
							Int:  42,
							Node: queryparser.Node{Off: 42},
						},
						Node: queryparser.Node{Off: 30},
					},
					Node: queryparser.Node{Off: 24},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where v.Value equal true",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 14},
					},
					Node: queryparser.Node{Off: 9},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 29},
									},
									{
										Value: "Value",
										Node:  queryparser.Node{Off: 31},
									},
								},
								Node: queryparser.Node{Off: 29},
							},
							Node: queryparser.Node{Off: 29},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 37},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypBool,
							Bool: true,
							Node: queryparser.Node{Off: 43},
						},
						Node: queryparser.Node{Off: 29},
					},
					Node: queryparser.Node{Off: 23},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where v.Value = ?",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 14},
					},
					Node: queryparser.Node{Off: 9},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 29},
									},
									{
										Value: "Value",
										Node:  queryparser.Node{Off: 31},
									},
								},
								Node: queryparser.Node{Off: 29},
							},
							Node: queryparser.Node{Off: 29},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 37},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypParameter,
							Node: queryparser.Node{Off: 39},
						},
						Node: queryparser.Node{Off: 29},
					},
					Node: queryparser.Node{Off: 23},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where Now() < Time(?) and Foo(10,?,v.Bar) = true",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 14},
					},
					Node: queryparser.Node{Off: 9},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypFunction,
									Function: &queryparser.Function{
										Name: "Now",
										Args: nil,
										Node: queryparser.Node{Off: 29},
									},
									Node: queryparser.Node{Off: 29},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.LessThan,
									Node: queryparser.Node{Off: 35},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypFunction,
									Function: &queryparser.Function{
										Name: "Time",
										Args: []*queryparser.Operand{
											{
												Type: queryparser.TypParameter,
												Node: queryparser.Node{Off: 42},
											},
										},
										Node: queryparser.Node{Off: 37},
									},
									Node: queryparser.Node{Off: 37},
								},
								Node: queryparser.Node{Off: 29},
							},
							Node: queryparser.Node{Off: 29},
						},
						Node: queryparser.Node{Off: 29},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.And,
							Node: queryparser.Node{Off: 45},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypFunction,
									Function: &queryparser.Function{
										Name: "Foo",
										Args: []*queryparser.Operand{
											{
												Type: queryparser.TypInt,
												Int:  10,
												Node: queryparser.Node{Off: 53},
											},
											{
												Type: queryparser.TypParameter,
												Node: queryparser.Node{Off: 56},
											},
											{
												Type: queryparser.TypField,
												Column: &queryparser.Field{
													Segments: []queryparser.Segment{
														{
															Value: "v",
															Node:  queryparser.Node{Off: 58},
														},
														{
															Value: "Bar",
															Node:  queryparser.Node{Off: 60},
														},
													},
													Node: queryparser.Node{Off: 58},
												},
												Node: queryparser.Node{Off: 58},
											},
										},
										Node: queryparser.Node{Off: 49},
									},
									Node: queryparser.Node{Off: 49},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Equal,
									Node: queryparser.Node{Off: 65},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypBool,
									Bool: true,
									Node: queryparser.Node{Off: 67},
								},
								Node: queryparser.Node{Off: 49},
							},
							Node: queryparser.Node{Off: 49},
						},
					},
					Node: queryparser.Node{Off: 23},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where v.ZipCode is nil",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 14},
					},
					Node: queryparser.Node{Off: 9},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 29},
									},
									{
										Value: "ZipCode",
										Node:  queryparser.Node{Off: 31},
									},
								},
								Node: queryparser.Node{Off: 29},
							},
							Node: queryparser.Node{Off: 29},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Is,
							Node: queryparser.Node{Off: 39},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypNil,
							Node: queryparser.Node{Off: 42},
						},
						Node: queryparser.Node{Off: 29},
					},
					Node: queryparser.Node{Off: 23},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where v.ZipCode is not nil",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 14},
					},
					Node: queryparser.Node{Off: 9},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 29},
									},
									{
										Value: "ZipCode",
										Node:  queryparser.Node{Off: 31},
									},
								},
								Node: queryparser.Node{Off: 29},
							},
							Node: queryparser.Node{Off: 29},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.IsNot,
							Node: queryparser.Node{Off: 39},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypNil,
							Node: queryparser.Node{Off: 46},
						},
						Node: queryparser.Node{Off: 29},
					},
					Node: queryparser.Node{Off: 23},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where v.Value = false",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 14},
					},
					Node: queryparser.Node{Off: 9},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 29},
									},
									{
										Value: "Value",
										Node:  queryparser.Node{Off: 31},
									},
								},
								Node: queryparser.Node{Off: 29},
							},
							Node: queryparser.Node{Off: 29},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 37},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypBool,
							Bool: false,
							Node: queryparser.Node{Off: 39},
						},
						Node: queryparser.Node{Off: 29},
					},
					Node: queryparser.Node{Off: 23},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where v.Value equal -42",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 14},
					},
					Node: queryparser.Node{Off: 9},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 29},
									},
									{
										Value: "Value",
										Node:  queryparser.Node{Off: 31},
									},
								},
								Node: queryparser.Node{Off: 29},
							},
							Node: queryparser.Node{Off: 29},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 37},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypInt,
							Int:  -42,
							Node: queryparser.Node{Off: 43},
						},
						Node: queryparser.Node{Off: 29},
					},
					Node: queryparser.Node{Off: 23},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where v.Value equal -18.888",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 14},
					},
					Node: queryparser.Node{Off: 9},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 29},
									},
									{
										Value: "Value",
										Node:  queryparser.Node{Off: 31},
									},
								},
								Node: queryparser.Node{Off: 29},
							},
							Node: queryparser.Node{Off: 29},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 37},
						},
						Operand2: &queryparser.Operand{
							Type:  queryparser.TypFloat,
							Float: -18.888,
							Node:  queryparser.Node{Off: 43},
						},
						Node: queryparser.Node{Off: 29},
					},
					Node: queryparser.Node{Off: 23},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select x from y where b = 'c'",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "x",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "y",
						Node: queryparser.Node{Off: 14},
					},
					Node: queryparser.Node{Off: 9},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "b",
										Node:  queryparser.Node{Off: 22},
									},
								},
								Node: queryparser.Node{Off: 22},
							},
							Node: queryparser.Node{Off: 22},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 24},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypInt,
							Int:  'c',
							Node: queryparser.Node{Off: 26},
						},
						Node: queryparser.Node{Off: 22},
					},
					Node: queryparser.Node{Off: 16},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select x from y where b = 'c' limit 10 offset 20",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "x",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "y",
						Node: queryparser.Node{Off: 14},
					},
					Node: queryparser.Node{Off: 9},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "b",
										Node:  queryparser.Node{Off: 22},
									},
								},
								Node: queryparser.Node{Off: 22},
							},
							Node: queryparser.Node{Off: 22},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 24},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypInt,
							Int:  'c',
							Node: queryparser.Node{Off: 26},
						},
						Node: queryparser.Node{Off: 22},
					},
					Node: queryparser.Node{Off: 16},
				},
				Limit: &queryparser.LimitClause{
					Limit: &queryparser.Int64Value{
						Value: 10,
						Node:  queryparser.Node{Off: 36},
					},
					Node: queryparser.Node{Off: 30},
				},
				ResultsOffset: &queryparser.ResultsOffsetClause{
					ResultsOffset: &queryparser.Int64Value{
						Value: 20,
						Node:  queryparser.Node{Off: 46},
					},
					Node: queryparser.Node{Off: 39},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select x from y where b = 'c' limit 10",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "x",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "y",
						Node: queryparser.Node{Off: 14},
					},
					Node: queryparser.Node{Off: 9},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "b",
										Node:  queryparser.Node{Off: 22},
									},
								},
								Node: queryparser.Node{Off: 22},
							},
							Node: queryparser.Node{Off: 22},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 24},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypInt,
							Int:  'c',
							Node: queryparser.Node{Off: 26},
						},
						Node: queryparser.Node{Off: 22},
					},
					Node: queryparser.Node{Off: 16},
				},
				Limit: &queryparser.LimitClause{
					Limit: &queryparser.Int64Value{
						Value: 10,
						Node:  queryparser.Node{Off: 36},
					},
					Node: queryparser.Node{Off: 30},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select x from y where b = 'c' offset 10",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "x",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "y",
						Node: queryparser.Node{Off: 14},
					},
					Node: queryparser.Node{Off: 9},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "b",
										Node:  queryparser.Node{Off: 22},
									},
								},
								Node: queryparser.Node{Off: 22},
							},
							Node: queryparser.Node{Off: 22},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 24},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypInt,
							Int:  'c',
							Node: queryparser.Node{Off: 26},
						},
						Node: queryparser.Node{Off: 22},
					},
					Node: queryparser.Node{Off: 16},
				},
				ResultsOffset: &queryparser.ResultsOffsetClause{
					ResultsOffset: &queryparser.Int64Value{
						Value: 10,
						Node:  queryparser.Node{Off: 37},
					},
					Node: queryparser.Node{Off: 30},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where k like \"Foo^%Bar\" escape '^'",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 14},
					},
					Node: queryparser.Node{Off: 9},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "k",
										Node:  queryparser.Node{Off: 29},
									},
								},
								Node: queryparser.Node{Off: 29},
							},
							Node: queryparser.Node{Off: 29},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Like,
							Node: queryparser.Node{Off: 31},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypStr,
							Str:  "Foo^%Bar",
							Node: queryparser.Node{Off: 36},
						},
						Node: queryparser.Node{Off: 29},
					},
					Node: queryparser.Node{Off: 23},
				},
				Escape: &queryparser.EscapeClause{
					EscapeChar: &queryparser.CharValue{
						Value: '^',
						Node:  queryparser.Node{Off: 54},
					},
					Node: queryparser.Node{Off: 47},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo.bar, tom.dick.harry from Customer where a.b.c = \"baz\" and d.e.f like \"%foobarbaz\"",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "foo",
										Node:  queryparser.Node{Off: 7},
									},
									{
										Value: "bar",
										Node:  queryparser.Node{Off: 11},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "tom",
										Node:  queryparser.Node{Off: 16},
									},
									{
										Value: "dick",
										Node:  queryparser.Node{Off: 20},
									},
									{
										Value: "harry",
										Node:  queryparser.Node{Off: 25},
									},
								},
								Node: queryparser.Node{Off: 16},
							},
							Node: queryparser.Node{Off: 16},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 36},
					},
					Node: queryparser.Node{Off: 31},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypField,
									Column: &queryparser.Field{
										Segments: []queryparser.Segment{
											{
												Value: "a",
												Node:  queryparser.Node{Off: 51},
											},
											{
												Value: "b",
												Node:  queryparser.Node{Off: 53},
											},
											{
												Value: "c",
												Node:  queryparser.Node{Off: 55},
											},
										},
										Node: queryparser.Node{Off: 51},
									},
									Node: queryparser.Node{Off: 51},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Equal,
									Node: queryparser.Node{Off: 57},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypStr,
									Str:  "baz",
									Node: queryparser.Node{Off: 59},
								},
								Node: queryparser.Node{Off: 51},
							},
							Node: queryparser.Node{Off: 51},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.And,
							Node: queryparser.Node{Off: 65},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypField,
									Column: &queryparser.Field{
										Segments: []queryparser.Segment{
											{
												Value: "d",
												Node:  queryparser.Node{Off: 69},
											},
											{
												Value: "e",
												Node:  queryparser.Node{Off: 71},
											},
											{
												Value: "f",
												Node:  queryparser.Node{Off: 73},
											},
										},
										Node: queryparser.Node{Off: 69},
									},
									Node: queryparser.Node{Off: 69},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Like,
									Node: queryparser.Node{Off: 75},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypStr,
									Str:  "%foobarbaz",
									Node: queryparser.Node{Off: 80},
								},
								Node: queryparser.Node{Off: 69},
							},
							Node: queryparser.Node{Off: 69},
						},
						Node: queryparser.Node{Off: 51},
					},
					Node: queryparser.Node{Off: 45},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo, bar from Customer where CustRecord.CustID=123 or CustRecord.Name like \"f%\"",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "foo",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "bar",
										Node:  queryparser.Node{Off: 12},
									},
								},
								Node: queryparser.Node{Off: 12},
							},
							Node: queryparser.Node{Off: 12},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 21},
					},
					Node: queryparser.Node{Off: 16},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypField,
									Column: &queryparser.Field{
										Segments: []queryparser.Segment{
											{
												Value: "CustRecord",
												Node:  queryparser.Node{Off: 36},
											},
											{
												Value: "CustID",
												Node:  queryparser.Node{Off: 47},
											},
										},
										Node: queryparser.Node{Off: 36},
									},
									Node: queryparser.Node{Off: 36},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Equal,
									Node: queryparser.Node{Off: 53},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypInt,
									Int:  123,
									Node: queryparser.Node{Off: 54},
								},
								Node: queryparser.Node{Off: 36},
							},
							Node: queryparser.Node{Off: 36},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Or,
							Node: queryparser.Node{Off: 58},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypField,
									Column: &queryparser.Field{
										Segments: []queryparser.Segment{
											{
												Value: "CustRecord",
												Node:  queryparser.Node{Off: 61},
											},
											{
												Value: "Name",
												Node:  queryparser.Node{Off: 72},
											},
										},
										Node: queryparser.Node{Off: 61},
									},
									Node: queryparser.Node{Off: 61},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Like,
									Node: queryparser.Node{Off: 77},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypStr,
									Str:  "f%",
									Node: queryparser.Node{Off: 82},
								},
								Node: queryparser.Node{Off: 61},
							},
							Node: queryparser.Node{Off: 61},
						},
						Node: queryparser.Node{Off: 36},
					},
					Node: queryparser.Node{Off: 30},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo from Customer where A=123 or B=456 and C=789",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "foo",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 16},
					},
					Node: queryparser.Node{Off: 11},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "A",
														Node:  queryparser.Node{Off: 31},
													},
												},
												Node: queryparser.Node{Off: 31},
											},
											Node: queryparser.Node{Off: 31},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.Equal,
											Node: queryparser.Node{Off: 32},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  123,
											Node: queryparser.Node{Off: 33},
										},
										Node: queryparser.Node{Off: 31},
									},
									Node: queryparser.Node{Off: 31},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Or,
									Node: queryparser.Node{Off: 37},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "B",
														Node:  queryparser.Node{Off: 40},
													},
												},
												Node: queryparser.Node{Off: 40},
											},
											Node: queryparser.Node{Off: 40},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.Equal,
											Node: queryparser.Node{Off: 41},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  456,
											Node: queryparser.Node{Off: 42},
										},
										Node: queryparser.Node{Off: 40},
									},
									Node: queryparser.Node{Off: 40},
								},
								Node: queryparser.Node{Off: 31},
							},
							Node: queryparser.Node{Off: 31},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.And,
							Node: queryparser.Node{Off: 46},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypField,
									Column: &queryparser.Field{
										Segments: []queryparser.Segment{
											{
												Value: "C",
												Node:  queryparser.Node{Off: 50},
											},
										},
										Node: queryparser.Node{Off: 50},
									},
									Node: queryparser.Node{Off: 50},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Equal,
									Node: queryparser.Node{Off: 51},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypInt,
									Int:  789,
									Node: queryparser.Node{Off: 52},
								},
								Node: queryparser.Node{Off: 50},
							},
							Node: queryparser.Node{Off: 50},
						},
						Node: queryparser.Node{Off: 31},
					},
					Node: queryparser.Node{Off: 25},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo from Customer where (A=123 or B=456) and C=789",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "foo",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 16},
					},
					Node: queryparser.Node{Off: 11},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "A",
														Node:  queryparser.Node{Off: 32},
													},
												},
												Node: queryparser.Node{Off: 32},
											},
											Node: queryparser.Node{Off: 32},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.Equal,
											Node: queryparser.Node{Off: 33},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  123,
											Node: queryparser.Node{Off: 34},
										},
										Node: queryparser.Node{Off: 32},
									},
									Node: queryparser.Node{Off: 32},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Or,
									Node: queryparser.Node{Off: 38},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "B",
														Node:  queryparser.Node{Off: 41},
													},
												},
												Node: queryparser.Node{Off: 41},
											},
											Node: queryparser.Node{Off: 41},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.Equal,
											Node: queryparser.Node{Off: 42},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  456,
											Node: queryparser.Node{Off: 43},
										},
										Node: queryparser.Node{Off: 41},
									},
									Node: queryparser.Node{Off: 41},
								},
								Node: queryparser.Node{Off: 32},
							},
							Node: queryparser.Node{Off: 32},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.And,
							Node: queryparser.Node{Off: 48},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypField,
									Column: &queryparser.Field{
										Segments: []queryparser.Segment{
											{
												Value: "C",
												Node:  queryparser.Node{Off: 52},
											},
										},
										Node: queryparser.Node{Off: 52},
									},
									Node: queryparser.Node{Off: 52},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Equal,
									Node: queryparser.Node{Off: 53},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypInt,
									Int:  789,
									Node: queryparser.Node{Off: 54},
								},
								Node: queryparser.Node{Off: 52},
							},
							Node: queryparser.Node{Off: 52},
						},
						Node: queryparser.Node{Off: 32},
					},
					Node: queryparser.Node{Off: 25},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo from Customer where (A<=123 or B>456) and C>=789",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "foo",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 16},
					},
					Node: queryparser.Node{Off: 11},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "A",
														Node:  queryparser.Node{Off: 32},
													},
												},
												Node: queryparser.Node{Off: 32},
											},
											Node: queryparser.Node{Off: 32},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.LessThanOrEqual,
											Node: queryparser.Node{Off: 33},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  123,
											Node: queryparser.Node{Off: 35},
										},
										Node: queryparser.Node{Off: 32},
									},
									Node: queryparser.Node{Off: 32},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Or,
									Node: queryparser.Node{Off: 39},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "B",
														Node:  queryparser.Node{Off: 42},
													},
												},
												Node: queryparser.Node{Off: 42},
											},
											Node: queryparser.Node{Off: 42},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.GreaterThan,
											Node: queryparser.Node{Off: 43},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  456,
											Node: queryparser.Node{Off: 44},
										},
										Node: queryparser.Node{Off: 42},
									},
									Node: queryparser.Node{Off: 42},
								},
								Node: queryparser.Node{Off: 32},
							},
							Node: queryparser.Node{Off: 32},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.And,
							Node: queryparser.Node{Off: 49},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypField,
									Column: &queryparser.Field{
										Segments: []queryparser.Segment{
											{
												Value: "C",
												Node:  queryparser.Node{Off: 53},
											},
										},
										Node: queryparser.Node{Off: 53},
									},
									Node: queryparser.Node{Off: 53},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.GreaterThanOrEqual,
									Node: queryparser.Node{Off: 54},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypInt,
									Int:  789,
									Node: queryparser.Node{Off: 56},
								},
								Node: queryparser.Node{Off: 53},
							},
							Node: queryparser.Node{Off: 53},
						},
						Node: queryparser.Node{Off: 32},
					},
					Node: queryparser.Node{Off: 25},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo from Customer where A=123 or (B=456 and C=789)",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "foo",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 16},
					},
					Node: queryparser.Node{Off: 11},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypField,
									Column: &queryparser.Field{
										Segments: []queryparser.Segment{
											{
												Value: "A",
												Node:  queryparser.Node{Off: 31},
											},
										},
										Node: queryparser.Node{Off: 31},
									},
									Node: queryparser.Node{Off: 31},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Equal,
									Node: queryparser.Node{Off: 32},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypInt,
									Int:  123,
									Node: queryparser.Node{Off: 33},
								},
								Node: queryparser.Node{Off: 31},
							},
							Node: queryparser.Node{Off: 31},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Or,
							Node: queryparser.Node{Off: 37},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "B",
														Node:  queryparser.Node{Off: 41},
													},
												},
												Node: queryparser.Node{Off: 41},
											},
											Node: queryparser.Node{Off: 41},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.Equal,
											Node: queryparser.Node{Off: 42},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  456,
											Node: queryparser.Node{Off: 43},
										},
										Node: queryparser.Node{Off: 41},
									},
									Node: queryparser.Node{Off: 41},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.And,
									Node: queryparser.Node{Off: 47},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "C",
														Node:  queryparser.Node{Off: 51},
													},
												},
												Node: queryparser.Node{Off: 51},
											},
											Node: queryparser.Node{Off: 51},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.Equal,
											Node: queryparser.Node{Off: 52},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  789,
											Node: queryparser.Node{Off: 53},
										},
										Node: queryparser.Node{Off: 51},
									},
									Node: queryparser.Node{Off: 51},
								},
								Node: queryparser.Node{Off: 41},
							},
							Node: queryparser.Node{Off: 41},
						},
						Node: queryparser.Node{Off: 31},
					},
					Node: queryparser.Node{Off: 25},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo from Customer where (A=123) or ((B=456) and (C=789))",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "foo",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 16},
					},
					Node: queryparser.Node{Off: 11},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypField,
									Column: &queryparser.Field{
										Segments: []queryparser.Segment{
											{
												Value: "A",
												Node:  queryparser.Node{Off: 32},
											},
										},
										Node: queryparser.Node{Off: 32},
									},
									Node: queryparser.Node{Off: 32},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Equal,
									Node: queryparser.Node{Off: 33},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypInt,
									Int:  123,
									Node: queryparser.Node{Off: 34},
								},
								Node: queryparser.Node{Off: 32},
							},
							Node: queryparser.Node{Off: 32},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Or,
							Node: queryparser.Node{Off: 39},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "B",
														Node:  queryparser.Node{Off: 44},
													},
												},
												Node: queryparser.Node{Off: 44},
											},
											Node: queryparser.Node{Off: 44},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.Equal,
											Node: queryparser.Node{Off: 45},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  456,
											Node: queryparser.Node{Off: 46},
										},
										Node: queryparser.Node{Off: 44},
									},
									Node: queryparser.Node{Off: 44},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.And,
									Node: queryparser.Node{Off: 51},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "C",
														Node:  queryparser.Node{Off: 56},
													},
												},
												Node: queryparser.Node{Off: 56},
											},
											Node: queryparser.Node{Off: 56},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.Equal,
											Node: queryparser.Node{Off: 57},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  789,
											Node: queryparser.Node{Off: 58},
										},
										Node: queryparser.Node{Off: 56},
									},
									Node: queryparser.Node{Off: 56},
								},
								Node: queryparser.Node{Off: 44},
							},
							Node: queryparser.Node{Off: 44},
						},
						Node: queryparser.Node{Off: 32},
					},
					Node: queryparser.Node{Off: 25},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo from Customer where A<>123 or B not equal 456 and C not like \"abc%\"",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "foo",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 16},
					},
					Node: queryparser.Node{Off: 11},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "A",
														Node:  queryparser.Node{Off: 31},
													},
												},
												Node: queryparser.Node{Off: 31},
											},
											Node: queryparser.Node{Off: 31},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.NotEqual,
											Node: queryparser.Node{Off: 32},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  123,
											Node: queryparser.Node{Off: 34},
										},
										Node: queryparser.Node{Off: 31},
									},
									Node: queryparser.Node{Off: 31},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Or,
									Node: queryparser.Node{Off: 38},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "B",
														Node:  queryparser.Node{Off: 41},
													},
												},
												Node: queryparser.Node{Off: 41},
											},
											Node: queryparser.Node{Off: 41},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.NotEqual,
											Node: queryparser.Node{Off: 43},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  456,
											Node: queryparser.Node{Off: 53},
										},
										Node: queryparser.Node{Off: 41},
									},
									Node: queryparser.Node{Off: 41},
								},
								Node: queryparser.Node{Off: 31},
							},
							Node: queryparser.Node{Off: 31},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.And,
							Node: queryparser.Node{Off: 57},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypField,
									Column: &queryparser.Field{
										Segments: []queryparser.Segment{
											{
												Value: "C",
												Node:  queryparser.Node{Off: 61},
											},
										},
										Node: queryparser.Node{Off: 61},
									},
									Node: queryparser.Node{Off: 61},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.NotLike,
									Node: queryparser.Node{Off: 63},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypStr,
									Str:  "abc%",
									Node: queryparser.Node{Off: 72},
								},
								Node: queryparser.Node{Off: 61},
							},
							Node: queryparser.Node{Off: 61},
						},
						Node: queryparser.Node{Off: 31},
					},
					Node: queryparser.Node{Off: 25},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where Now() < Time(\"2015/07/22\") and Foo(10,20.1,v.Bar) = true",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 14},
					},
					Node: queryparser.Node{Off: 9},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypFunction,
									Function: &queryparser.Function{
										Name: "Now",
										Args: nil,
										Node: queryparser.Node{Off: 29},
									},
									Node: queryparser.Node{Off: 29},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.LessThan,
									Node: queryparser.Node{Off: 35},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypFunction,
									Function: &queryparser.Function{
										Name: "Time",
										Args: []*queryparser.Operand{
											{
												Type: queryparser.TypStr,
												Str:  "2015/07/22",
												Node: queryparser.Node{Off: 42},
											},
										},
										Node: queryparser.Node{Off: 37},
									},
									Node: queryparser.Node{Off: 37},
								},
								Node: queryparser.Node{Off: 29},
							},
							Node: queryparser.Node{Off: 29},
						},
						Node: queryparser.Node{Off: 29},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.And,
							Node: queryparser.Node{Off: 56},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypFunction,
									Function: &queryparser.Function{
										Name: "Foo",
										Args: []*queryparser.Operand{
											{
												Type: queryparser.TypInt,
												Int:  10,
												Node: queryparser.Node{Off: 64},
											},
											{
												Type:  queryparser.TypFloat,
												Float: 20.1,
												Node:  queryparser.Node{Off: 67},
											},
											{
												Type: queryparser.TypField,
												Column: &queryparser.Field{
													Segments: []queryparser.Segment{
														{
															Value: "v",
															Node:  queryparser.Node{Off: 72},
														},
														{
															Value: "Bar",
															Node:  queryparser.Node{Off: 74},
														},
													},
													Node: queryparser.Node{Off: 72},
												},
												Node: queryparser.Node{Off: 72},
											},
										},
										Node: queryparser.Node{Off: 60},
									},
									Node: queryparser.Node{Off: 60},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Equal,
									Node: queryparser.Node{Off: 79},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypBool,
									Bool: true,
									Node: queryparser.Node{Off: 81},
								},
								Node: queryparser.Node{Off: 60},
							},
							Node: queryparser.Node{Off: 60},
						},
					},
					Node: queryparser.Node{Off: 23},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select Now() from Customer",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelFunc,
							Function: &queryparser.Function{
								Name: "Now",
								Args: nil,
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 18},
					},
					Node: queryparser.Node{Off: 13},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select Now(), Date(\"2015-06-01 PST\") from Customer",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelFunc,
							Function: &queryparser.Function{
								Name: "Now",
								Args: nil,
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
						{
							Type: queryparser.TypSelFunc,
							Function: &queryparser.Function{
								Name: "Date",
								Args: []*queryparser.Operand{
									{
										Type: queryparser.TypStr,
										Str:  "2015-06-01 PST",
										Node: queryparser.Node{Off: 19},
									},
								},
								Node: queryparser.Node{Off: 14},
							},
							Node: queryparser.Node{Off: 14},
						},
					},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 42},
					},
					Node: queryparser.Node{Off: 37},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v[\"foo\"] from Customer",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Keys: []*queryparser.Operand{
											{
												Type: queryparser.TypStr,
												Str:  "foo",
												Node: queryparser.Node{Off: 9},
											},
										},
										Node: queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 21},
					},
					Node: queryparser.Node{Off: 16},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v[\"foo\"][\"bar\"] from Customer",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Keys: []*queryparser.Operand{
											{
												Type: queryparser.TypStr,
												Str:  "foo",
												Node: queryparser.Node{Off: 9},
											},
											{
												Type: queryparser.TypStr,
												Str:  "bar",
												Node: queryparser.Node{Off: 16},
											},
										},
										Node: queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 28},
					},
					Node: queryparser.Node{Off: 23},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where v.Foo[v.Bar] = \"abc\"",
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 14},
					},
					Node: queryparser.Node{Off: 9},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 29},
									},
									{
										Value: "Foo",
										Keys: []*queryparser.Operand{
											{
												Type: queryparser.TypField,
												Column: &queryparser.Field{
													Segments: []queryparser.Segment{
														{
															Value: "v",
															Node:  queryparser.Node{Off: 35},
														},
														{
															Value: "Bar",
															Node:  queryparser.Node{Off: 37},
														},
													},
													Node: queryparser.Node{Off: 35},
												},
												Node: queryparser.Node{Off: 35},
											},
										},
										Node: queryparser.Node{Off: 31},
									},
								},
								Node: queryparser.Node{Off: 29},
							},
							Node: queryparser.Node{Off: 29},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 42},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypStr,
							Str:  "abc",
							Node: queryparser.Node{Off: 44},
						},
						Node: queryparser.Node{Off: 29},
					},
					Node: queryparser.Node{Off: 23},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
	}

	for _, test := range basic {
		st, err := queryparser.Parse(&db, test.query)
		if err != nil {
			t.Errorf("query: %s; unexpected error: got %v, want nil", test.query, err)
		}
		if _, ok := (*st).(queryparser.SelectStatement); ok {
			if !reflect.DeepEqual(test.statement, *st) {
				t.Errorf("query: %s;\nGOT  %s\nWANT %s", test.query, *st, test.statement)
			}
		}
	}
}

func TestDeleteParser(t *testing.T) {
	basic := []parseDeleteTest{
		{
			"delete from Customer",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"   delete from Customer",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 15},
					},
					Node: queryparser.Node{Off: 10},
				},
				Node: queryparser.Node{Off: 3},
			},
			nil,
		},
		{
			"delete from Customer limit 100",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Limit: &queryparser.LimitClause{
					Limit: &queryparser.Int64Value{
						Value: 100,
						Node:  queryparser.Node{Off: 27},
					},
					Node: queryparser.Node{Off: 21},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer limit 100 limit 1",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Limit: &queryparser.LimitClause{
					Limit: &queryparser.Int64Value{
						Value: 1,
						Node:  queryparser.Node{Off: 37},
					},
					Node: queryparser.Node{Off: 31},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from from where where equal 42",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "from",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "where",
										Node:  queryparser.Node{Off: 23},
									},
								},
								Node: queryparser.Node{Off: 23},
							},
							Node: queryparser.Node{Off: 23},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 29},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypInt,
							Int:  42,
							Node: queryparser.Node{Off: 35},
						},
						Node: queryparser.Node{Off: 23},
					},
					Node: queryparser.Node{Off: 17},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where v.Value equal true",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 27},
									},
									{
										Value: "Value",
										Node:  queryparser.Node{Off: 29},
									},
								},
								Node: queryparser.Node{Off: 27},
							},
							Node: queryparser.Node{Off: 27},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 35},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypBool,
							Bool: true,
							Node: queryparser.Node{Off: 41},
						},
						Node: queryparser.Node{Off: 27},
					},
					Node: queryparser.Node{Off: 21},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where v.Value = ?",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 27},
									},
									{
										Value: "Value",
										Node:  queryparser.Node{Off: 29},
									},
								},
								Node: queryparser.Node{Off: 27},
							},
							Node: queryparser.Node{Off: 27},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 35},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypParameter,
							Node: queryparser.Node{Off: 37},
						},
						Node: queryparser.Node{Off: 27},
					},
					Node: queryparser.Node{Off: 21},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where Now() < Time(?) and Foo(10,?,v.Bar) = true",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypFunction,
									Function: &queryparser.Function{
										Name: "Now",
										Args: nil,
										Node: queryparser.Node{Off: 27},
									},
									Node: queryparser.Node{Off: 27},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.LessThan,
									Node: queryparser.Node{Off: 33},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypFunction,
									Function: &queryparser.Function{
										Name: "Time",
										Args: []*queryparser.Operand{
											{
												Type: queryparser.TypParameter,
												Node: queryparser.Node{Off: 40},
											},
										},
										Node: queryparser.Node{Off: 35},
									},
									Node: queryparser.Node{Off: 35},
								},
								Node: queryparser.Node{Off: 27},
							},
							Node: queryparser.Node{Off: 27},
						},
						Node: queryparser.Node{Off: 27},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.And,
							Node: queryparser.Node{Off: 43},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypFunction,
									Function: &queryparser.Function{
										Name: "Foo",
										Args: []*queryparser.Operand{
											{
												Type: queryparser.TypInt,
												Int:  10,
												Node: queryparser.Node{Off: 51},
											},
											{
												Type: queryparser.TypParameter,
												Node: queryparser.Node{Off: 54},
											},
											{
												Type: queryparser.TypField,
												Column: &queryparser.Field{
													Segments: []queryparser.Segment{
														{
															Value: "v",
															Node:  queryparser.Node{Off: 56},
														},
														{
															Value: "Bar",
															Node:  queryparser.Node{Off: 58},
														},
													},
													Node: queryparser.Node{Off: 56},
												},
												Node: queryparser.Node{Off: 56},
											},
										},
										Node: queryparser.Node{Off: 47},
									},
									Node: queryparser.Node{Off: 47},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Equal,
									Node: queryparser.Node{Off: 63},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypBool,
									Bool: true,
									Node: queryparser.Node{Off: 65},
								},
								Node: queryparser.Node{Off: 47},
							},
							Node: queryparser.Node{Off: 47},
						},
					},
					Node: queryparser.Node{Off: 21},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where v.ZipCode is nil",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 27},
									},
									{
										Value: "ZipCode",
										Node:  queryparser.Node{Off: 29},
									},
								},
								Node: queryparser.Node{Off: 27},
							},
							Node: queryparser.Node{Off: 27},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Is,
							Node: queryparser.Node{Off: 37},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypNil,
							Node: queryparser.Node{Off: 40},
						},
						Node: queryparser.Node{Off: 27},
					},
					Node: queryparser.Node{Off: 21},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where v.ZipCode is not nil",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 27},
									},
									{
										Value: "ZipCode",
										Node:  queryparser.Node{Off: 29},
									},
								},
								Node: queryparser.Node{Off: 27},
							},
							Node: queryparser.Node{Off: 27},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.IsNot,
							Node: queryparser.Node{Off: 37},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypNil,
							Node: queryparser.Node{Off: 44},
						},
						Node: queryparser.Node{Off: 27},
					},
					Node: queryparser.Node{Off: 21},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where v.Value = false",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 27},
									},
									{
										Value: "Value",
										Node:  queryparser.Node{Off: 29},
									},
								},
								Node: queryparser.Node{Off: 27},
							},
							Node: queryparser.Node{Off: 27},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 35},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypBool,
							Bool: false,
							Node: queryparser.Node{Off: 37},
						},
						Node: queryparser.Node{Off: 27},
					},
					Node: queryparser.Node{Off: 21},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where v.Value equal -42",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 27},
									},
									{
										Value: "Value",
										Node:  queryparser.Node{Off: 29},
									},
								},
								Node: queryparser.Node{Off: 27},
							},
							Node: queryparser.Node{Off: 27},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 35},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypInt,
							Int:  -42,
							Node: queryparser.Node{Off: 41},
						},
						Node: queryparser.Node{Off: 27},
					},
					Node: queryparser.Node{Off: 21},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where v.Value equal -18.888",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 27},
									},
									{
										Value: "Value",
										Node:  queryparser.Node{Off: 29},
									},
								},
								Node: queryparser.Node{Off: 27},
							},
							Node: queryparser.Node{Off: 27},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 35},
						},
						Operand2: &queryparser.Operand{
							Type:  queryparser.TypFloat,
							Float: -18.888,
							Node:  queryparser.Node{Off: 41},
						},
						Node: queryparser.Node{Off: 27},
					},
					Node: queryparser.Node{Off: 21},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from y where b = 'c'",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "y",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "b",
										Node:  queryparser.Node{Off: 20},
									},
								},
								Node: queryparser.Node{Off: 20},
							},
							Node: queryparser.Node{Off: 20},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 22},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypInt,
							Int:  'c',
							Node: queryparser.Node{Off: 24},
						},
						Node: queryparser.Node{Off: 20},
					},
					Node: queryparser.Node{Off: 14},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from y where b = 'c' limit 10",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "y",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "b",
										Node:  queryparser.Node{Off: 20},
									},
								},
								Node: queryparser.Node{Off: 20},
							},
							Node: queryparser.Node{Off: 20},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 22},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypInt,
							Int:  'c',
							Node: queryparser.Node{Off: 24},
						},
						Node: queryparser.Node{Off: 20},
					},
					Node: queryparser.Node{Off: 14},
				},
				Limit: &queryparser.LimitClause{
					Limit: &queryparser.Int64Value{
						Value: 10,
						Node:  queryparser.Node{Off: 34},
					},
					Node: queryparser.Node{Off: 28},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where k like \"Foo^%Bar\" escape '^'",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "k",
										Node:  queryparser.Node{Off: 27},
									},
								},
								Node: queryparser.Node{Off: 27},
							},
							Node: queryparser.Node{Off: 27},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Like,
							Node: queryparser.Node{Off: 29},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypStr,
							Str:  "Foo^%Bar",
							Node: queryparser.Node{Off: 34},
						},
						Node: queryparser.Node{Off: 27},
					},
					Node: queryparser.Node{Off: 21},
				},
				Escape: &queryparser.EscapeClause{
					EscapeChar: &queryparser.CharValue{
						Value: '^',
						Node:  queryparser.Node{Off: 52},
					},
					Node: queryparser.Node{Off: 45},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where a.b.c = \"baz\" and d.e.f like \"%foobarbaz\"",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypField,
									Column: &queryparser.Field{
										Segments: []queryparser.Segment{
											{
												Value: "a",
												Node:  queryparser.Node{Off: 27},
											},
											{
												Value: "b",
												Node:  queryparser.Node{Off: 29},
											},
											{
												Value: "c",
												Node:  queryparser.Node{Off: 31},
											},
										},
										Node: queryparser.Node{Off: 27},
									},
									Node: queryparser.Node{Off: 27},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Equal,
									Node: queryparser.Node{Off: 33},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypStr,
									Str:  "baz",
									Node: queryparser.Node{Off: 35},
								},
								Node: queryparser.Node{Off: 27},
							},
							Node: queryparser.Node{Off: 27},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.And,
							Node: queryparser.Node{Off: 41},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypField,
									Column: &queryparser.Field{
										Segments: []queryparser.Segment{
											{
												Value: "d",
												Node:  queryparser.Node{Off: 45},
											},
											{
												Value: "e",
												Node:  queryparser.Node{Off: 47},
											},
											{
												Value: "f",
												Node:  queryparser.Node{Off: 49},
											},
										},
										Node: queryparser.Node{Off: 45},
									},
									Node: queryparser.Node{Off: 45},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Like,
									Node: queryparser.Node{Off: 51},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypStr,
									Str:  "%foobarbaz",
									Node: queryparser.Node{Off: 56},
								},
								Node: queryparser.Node{Off: 45},
							},
							Node: queryparser.Node{Off: 45},
						},
						Node: queryparser.Node{Off: 27},
					},
					Node: queryparser.Node{Off: 21},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where CustRecord.CustID=123 or CustRecord.Name like \"f%\"",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypField,
									Column: &queryparser.Field{
										Segments: []queryparser.Segment{
											{
												Value: "CustRecord",
												Node:  queryparser.Node{Off: 27},
											},
											{
												Value: "CustID",
												Node:  queryparser.Node{Off: 38},
											},
										},
										Node: queryparser.Node{Off: 27},
									},
									Node: queryparser.Node{Off: 27},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Equal,
									Node: queryparser.Node{Off: 44},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypInt,
									Int:  123,
									Node: queryparser.Node{Off: 45},
								},
								Node: queryparser.Node{Off: 27},
							},
							Node: queryparser.Node{Off: 27},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Or,
							Node: queryparser.Node{Off: 49},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypField,
									Column: &queryparser.Field{
										Segments: []queryparser.Segment{
											{
												Value: "CustRecord",
												Node:  queryparser.Node{Off: 52},
											},
											{
												Value: "Name",
												Node:  queryparser.Node{Off: 63},
											},
										},
										Node: queryparser.Node{Off: 52},
									},
									Node: queryparser.Node{Off: 52},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Like,
									Node: queryparser.Node{Off: 68},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypStr,
									Str:  "f%",
									Node: queryparser.Node{Off: 73},
								},
								Node: queryparser.Node{Off: 52},
							},
							Node: queryparser.Node{Off: 52},
						},
						Node: queryparser.Node{Off: 27},
					},
					Node: queryparser.Node{Off: 21},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where A=123 or B=456 and C=789",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "A",
														Node:  queryparser.Node{Off: 27},
													},
												},
												Node: queryparser.Node{Off: 27},
											},
											Node: queryparser.Node{Off: 27},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.Equal,
											Node: queryparser.Node{Off: 28},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  123,
											Node: queryparser.Node{Off: 29},
										},
										Node: queryparser.Node{Off: 27},
									},
									Node: queryparser.Node{Off: 27},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Or,
									Node: queryparser.Node{Off: 33},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "B",
														Node:  queryparser.Node{Off: 36},
													},
												},
												Node: queryparser.Node{Off: 36},
											},
											Node: queryparser.Node{Off: 36},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.Equal,
											Node: queryparser.Node{Off: 37},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  456,
											Node: queryparser.Node{Off: 38},
										},
										Node: queryparser.Node{Off: 36},
									},
									Node: queryparser.Node{Off: 36},
								},
								Node: queryparser.Node{Off: 27},
							},
							Node: queryparser.Node{Off: 27},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.And,
							Node: queryparser.Node{Off: 42},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypField,
									Column: &queryparser.Field{
										Segments: []queryparser.Segment{
											{
												Value: "C",
												Node:  queryparser.Node{Off: 46},
											},
										},
										Node: queryparser.Node{Off: 46},
									},
									Node: queryparser.Node{Off: 46},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Equal,
									Node: queryparser.Node{Off: 47},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypInt,
									Int:  789,
									Node: queryparser.Node{Off: 48},
								},
								Node: queryparser.Node{Off: 46},
							},
							Node: queryparser.Node{Off: 46},
						},
						Node: queryparser.Node{Off: 27},
					},
					Node: queryparser.Node{Off: 21},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where (A=123 or B=456) and C=789",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "A",
														Node:  queryparser.Node{Off: 28},
													},
												},
												Node: queryparser.Node{Off: 28},
											},
											Node: queryparser.Node{Off: 28},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.Equal,
											Node: queryparser.Node{Off: 29},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  123,
											Node: queryparser.Node{Off: 30},
										},
										Node: queryparser.Node{Off: 28},
									},
									Node: queryparser.Node{Off: 28},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Or,
									Node: queryparser.Node{Off: 34},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "B",
														Node:  queryparser.Node{Off: 37},
													},
												},
												Node: queryparser.Node{Off: 37},
											},
											Node: queryparser.Node{Off: 37},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.Equal,
											Node: queryparser.Node{Off: 38},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  456,
											Node: queryparser.Node{Off: 39},
										},
										Node: queryparser.Node{Off: 37},
									},
									Node: queryparser.Node{Off: 37},
								},
								Node: queryparser.Node{Off: 28},
							},
							Node: queryparser.Node{Off: 28},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.And,
							Node: queryparser.Node{Off: 44},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypField,
									Column: &queryparser.Field{
										Segments: []queryparser.Segment{
											{
												Value: "C",
												Node:  queryparser.Node{Off: 48},
											},
										},
										Node: queryparser.Node{Off: 48},
									},
									Node: queryparser.Node{Off: 48},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Equal,
									Node: queryparser.Node{Off: 49},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypInt,
									Int:  789,
									Node: queryparser.Node{Off: 50},
								},
								Node: queryparser.Node{Off: 48},
							},
							Node: queryparser.Node{Off: 48},
						},
						Node: queryparser.Node{Off: 28},
					},
					Node: queryparser.Node{Off: 21},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where (A<=123 or B>456) and C>=789",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "A",
														Node:  queryparser.Node{Off: 28},
													},
												},
												Node: queryparser.Node{Off: 28},
											},
											Node: queryparser.Node{Off: 28},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.LessThanOrEqual,
											Node: queryparser.Node{Off: 29},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  123,
											Node: queryparser.Node{Off: 31},
										},
										Node: queryparser.Node{Off: 28},
									},
									Node: queryparser.Node{Off: 28},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Or,
									Node: queryparser.Node{Off: 35},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "B",
														Node:  queryparser.Node{Off: 38},
													},
												},
												Node: queryparser.Node{Off: 38},
											},
											Node: queryparser.Node{Off: 38},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.GreaterThan,
											Node: queryparser.Node{Off: 39},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  456,
											Node: queryparser.Node{Off: 40},
										},
										Node: queryparser.Node{Off: 38},
									},
									Node: queryparser.Node{Off: 38},
								},
								Node: queryparser.Node{Off: 28},
							},
							Node: queryparser.Node{Off: 28},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.And,
							Node: queryparser.Node{Off: 45},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypField,
									Column: &queryparser.Field{
										Segments: []queryparser.Segment{
											{
												Value: "C",
												Node:  queryparser.Node{Off: 49},
											},
										},
										Node: queryparser.Node{Off: 49},
									},
									Node: queryparser.Node{Off: 49},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.GreaterThanOrEqual,
									Node: queryparser.Node{Off: 50},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypInt,
									Int:  789,
									Node: queryparser.Node{Off: 52},
								},
								Node: queryparser.Node{Off: 49},
							},
							Node: queryparser.Node{Off: 49},
						},
						Node: queryparser.Node{Off: 28},
					},
					Node: queryparser.Node{Off: 21},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where A=123 or (B=456 and C=789)",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypField,
									Column: &queryparser.Field{
										Segments: []queryparser.Segment{
											{
												Value: "A",
												Node:  queryparser.Node{Off: 27},
											},
										},
										Node: queryparser.Node{Off: 27},
									},
									Node: queryparser.Node{Off: 27},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Equal,
									Node: queryparser.Node{Off: 28},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypInt,
									Int:  123,
									Node: queryparser.Node{Off: 29},
								},
								Node: queryparser.Node{Off: 27},
							},
							Node: queryparser.Node{Off: 27},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Or,
							Node: queryparser.Node{Off: 33},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "B",
														Node:  queryparser.Node{Off: 37},
													},
												},
												Node: queryparser.Node{Off: 37},
											},
											Node: queryparser.Node{Off: 37},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.Equal,
											Node: queryparser.Node{Off: 38},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  456,
											Node: queryparser.Node{Off: 39},
										},
										Node: queryparser.Node{Off: 37},
									},
									Node: queryparser.Node{Off: 37},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.And,
									Node: queryparser.Node{Off: 43},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "C",
														Node:  queryparser.Node{Off: 47},
													},
												},
												Node: queryparser.Node{Off: 47},
											},
											Node: queryparser.Node{Off: 47},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.Equal,
											Node: queryparser.Node{Off: 48},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  789,
											Node: queryparser.Node{Off: 49},
										},
										Node: queryparser.Node{Off: 47},
									},
									Node: queryparser.Node{Off: 47},
								},
								Node: queryparser.Node{Off: 37},
							},
							Node: queryparser.Node{Off: 37},
						},
						Node: queryparser.Node{Off: 27},
					},
					Node: queryparser.Node{Off: 21},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where (A=123) or ((B=456) and (C=789))",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypField,
									Column: &queryparser.Field{
										Segments: []queryparser.Segment{
											{
												Value: "A",
												Node:  queryparser.Node{Off: 28},
											},
										},
										Node: queryparser.Node{Off: 28},
									},
									Node: queryparser.Node{Off: 28},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Equal,
									Node: queryparser.Node{Off: 29},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypInt,
									Int:  123,
									Node: queryparser.Node{Off: 30},
								},
								Node: queryparser.Node{Off: 28},
							},
							Node: queryparser.Node{Off: 28},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Or,
							Node: queryparser.Node{Off: 35},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "B",
														Node:  queryparser.Node{Off: 40},
													},
												},
												Node: queryparser.Node{Off: 40},
											},
											Node: queryparser.Node{Off: 40},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.Equal,
											Node: queryparser.Node{Off: 41},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  456,
											Node: queryparser.Node{Off: 42},
										},
										Node: queryparser.Node{Off: 40},
									},
									Node: queryparser.Node{Off: 40},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.And,
									Node: queryparser.Node{Off: 47},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "C",
														Node:  queryparser.Node{Off: 52},
													},
												},
												Node: queryparser.Node{Off: 52},
											},
											Node: queryparser.Node{Off: 52},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.Equal,
											Node: queryparser.Node{Off: 53},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  789,
											Node: queryparser.Node{Off: 54},
										},
										Node: queryparser.Node{Off: 52},
									},
									Node: queryparser.Node{Off: 52},
								},
								Node: queryparser.Node{Off: 40},
							},
							Node: queryparser.Node{Off: 40},
						},
						Node: queryparser.Node{Off: 28},
					},
					Node: queryparser.Node{Off: 21},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where A<>123 or B not equal 456 and C not like \"abc%\"",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "A",
														Node:  queryparser.Node{Off: 27},
													},
												},
												Node: queryparser.Node{Off: 27},
											},
											Node: queryparser.Node{Off: 27},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.NotEqual,
											Node: queryparser.Node{Off: 28},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  123,
											Node: queryparser.Node{Off: 30},
										},
										Node: queryparser.Node{Off: 27},
									},
									Node: queryparser.Node{Off: 27},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Or,
									Node: queryparser.Node{Off: 34},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypExpr,
									Expr: &queryparser.Expression{
										Operand1: &queryparser.Operand{
											Type: queryparser.TypField,
											Column: &queryparser.Field{
												Segments: []queryparser.Segment{
													{
														Value: "B",
														Node:  queryparser.Node{Off: 37},
													},
												},
												Node: queryparser.Node{Off: 37},
											},
											Node: queryparser.Node{Off: 37},
										},
										Operator: &queryparser.BinaryOperator{
											Type: queryparser.NotEqual,
											Node: queryparser.Node{Off: 39},
										},
										Operand2: &queryparser.Operand{
											Type: queryparser.TypInt,
											Int:  456,
											Node: queryparser.Node{Off: 49},
										},
										Node: queryparser.Node{Off: 37},
									},
									Node: queryparser.Node{Off: 37},
								},
								Node: queryparser.Node{Off: 27},
							},
							Node: queryparser.Node{Off: 27},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.And,
							Node: queryparser.Node{Off: 53},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypField,
									Column: &queryparser.Field{
										Segments: []queryparser.Segment{
											{
												Value: "C",
												Node:  queryparser.Node{Off: 57},
											},
										},
										Node: queryparser.Node{Off: 57},
									},
									Node: queryparser.Node{Off: 57},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.NotLike,
									Node: queryparser.Node{Off: 59},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypStr,
									Str:  "abc%",
									Node: queryparser.Node{Off: 68},
								},
								Node: queryparser.Node{Off: 57},
							},
							Node: queryparser.Node{Off: 57},
						},
						Node: queryparser.Node{Off: 27},
					},
					Node: queryparser.Node{Off: 21},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where Now() < Time(\"2015/07/22\") and Foo(10,20.1,v.Bar) = true",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypFunction,
									Function: &queryparser.Function{
										Name: "Now",
										Args: nil,
										Node: queryparser.Node{Off: 27},
									},
									Node: queryparser.Node{Off: 27},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.LessThan,
									Node: queryparser.Node{Off: 33},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypFunction,
									Function: &queryparser.Function{
										Name: "Time",
										Args: []*queryparser.Operand{
											{
												Type: queryparser.TypStr,
												Str:  "2015/07/22",
												Node: queryparser.Node{Off: 40},
											},
										},
										Node: queryparser.Node{Off: 35},
									},
									Node: queryparser.Node{Off: 35},
								},
								Node: queryparser.Node{Off: 27},
							},
							Node: queryparser.Node{Off: 27},
						},
						Node: queryparser.Node{Off: 27},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.And,
							Node: queryparser.Node{Off: 54},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypFunction,
									Function: &queryparser.Function{
										Name: "Foo",
										Args: []*queryparser.Operand{
											{
												Type: queryparser.TypInt,
												Int:  10,
												Node: queryparser.Node{Off: 62},
											},
											{
												Type:  queryparser.TypFloat,
												Float: 20.1,
												Node:  queryparser.Node{Off: 65},
											},
											{
												Type: queryparser.TypField,
												Column: &queryparser.Field{
													Segments: []queryparser.Segment{
														{
															Value: "v",
															Node:  queryparser.Node{Off: 70},
														},
														{
															Value: "Bar",
															Node:  queryparser.Node{Off: 72},
														},
													},
													Node: queryparser.Node{Off: 70},
												},
												Node: queryparser.Node{Off: 70},
											},
										},
										Node: queryparser.Node{Off: 58},
									},
									Node: queryparser.Node{Off: 58},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Equal,
									Node: queryparser.Node{Off: 77},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypBool,
									Bool: true,
									Node: queryparser.Node{Off: 79},
								},
								Node: queryparser.Node{Off: 58},
							},
							Node: queryparser.Node{Off: 58},
						},
					},
					Node: queryparser.Node{Off: 21},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where v.Foo[v.Bar] = \"abc\"",
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 27},
									},
									{
										Value: "Foo",
										Keys: []*queryparser.Operand{
											{
												Type: queryparser.TypField,
												Column: &queryparser.Field{
													Segments: []queryparser.Segment{
														{
															Value: "v",
															Node:  queryparser.Node{Off: 33},
														},
														{
															Value: "Bar",
															Node:  queryparser.Node{Off: 35},
														},
													},
													Node: queryparser.Node{Off: 33},
												},
												Node: queryparser.Node{Off: 33},
											},
										},
										Node: queryparser.Node{Off: 29},
									},
								},
								Node: queryparser.Node{Off: 27},
							},
							Node: queryparser.Node{Off: 27},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 40},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypStr,
							Str:  "abc",
							Node: queryparser.Node{Off: 42},
						},
						Node: queryparser.Node{Off: 27},
					},
					Node: queryparser.Node{Off: 21},
				},
				Node: queryparser.Node{Off: 0},
			},
			nil,
		},
	}

	for _, test := range basic {
		st, err := queryparser.Parse(&db, test.query)
		if err != nil {
			t.Errorf("query: %s; unexpected error: got %v, want nil", test.query, err)
		}
		if _, ok := (*st).(queryparser.DeleteStatement); ok {
			if !reflect.DeepEqual(test.statement, *st) {
				t.Errorf("query: %s;\nGOT  %s\nWANT %s", test.query, *st, test.statement)
			}
		}
	}
}

func TestQueryParserErrors(t *testing.T) {
	basic := []parseErrorTest{
		{"", syncql.ErrorfNoStatementFound(db.GetContext(), "[%v]no statement found", 0)},
		{";", syncql.ErrorfExpectedIdentifier(db.GetContext(), "[%v]expected identifier, found %v", 0, ";")},
		{"foo", syncql.ErrorfUnknownIdentifier(db.GetContext(), "[%v]unknown identifier: %v", 0, "foo")},
		{"(foo)", syncql.ErrorfExpectedIdentifier(db.GetContext(), "[%v]expected identifier, found %v", 0, "(")},
		{"create table Customer (CustRecord cust_pkg.Cust, primary key(CustRecord.CustID))", syncql.ErrorfUnknownIdentifier(db.GetContext(), "[%v]unknown identifier: %v", 0, "create")},

		// Select
		{"select foo.", syncql.ErrorfExpectedIdentifier(db.GetContext(), "[%v]expected identifier, found %v", 11, "")},
		{"select foo. from a", syncql.ErrorfExpectedFrom(db.GetContext(), "[%v]expected 'from', found %v", 17, "a")},
		{"select (foo)", syncql.ErrorfExpectedIdentifier(db.GetContext(), "[%v]expected identifier, found %v", 7, "(")},
		{"select from where", syncql.ErrorfExpectedFrom(db.GetContext(), "[%v]expected 'from', found %v", 12, "where")},
		{"select foo from Customer where (A=123 or B=456) and C=789)", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 57, ")")},
		{"select foo from Customer where ((A=123 or B=456) and C=789", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 58)},
		{"select foo from Customer where (((((A=123 or B=456 and C=789))))", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 64)},
		{"select foo from Customer where (A=123 or B=456) and C=789)))))", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 57, ")")},
		{"select foo from Customer where", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 30)},
		{"select foo from Customer where ", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 31)},
		{"select foo from Customer where )", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 31, ")")},
		{"select foo from Customer where )A=123 or B=456) and C=789", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 31, ")")},
		{"select foo from Customer where ()A=123 or B=456) and C=789", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 32, ")")},
		{"select foo from Customer where (A=123 or B=456) and C=789)", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 57, ")")},
		{"select foo bar from Customer", syncql.ErrorfExpectedFrom(db.GetContext(), "[%v]expected 'from', found %v", 11, "bar")},
		{"select foo from Customer Invoice", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 25, "Invoice")},
		{"select (foo) from (Customer)", syncql.ErrorfExpectedIdentifier(db.GetContext(), "[%v]expected identifier, found %v", 7, "(")},
		{"select foo, bar from Customer where a = (b)", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 40, "(")},
		{"select foo, bar from Customer where a = b and (c) = d", syncql.ErrorfExpectedOperator(db.GetContext(), "[%v]expected operator, found %v", 48, ")")},
		{"select foo, bar from Customer where a = b and c =", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 49)},
		{"select foo, bar from Customer where a = ", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 40)},
		{"select foo, bar from Customer where a", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 37)},
		{"select", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 6)},
		{"select a from", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 13)},
		{"select a from b where c = d and e =", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 35)},
		{"select a from b where c = d and f", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 33)},
		{"select a from b where c = d and f *", syncql.ErrorfExpectedOperator(db.GetContext(), "[%v]expected operator, found %v", 34, "*")},
		{"select a from b where c <", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 25)},
		{"select a from b where c not", syncql.ErrorfExpected(db.GetContext(), "[%v]expected '%v'", 27, "'equal' or 'like'")},
		{"select a from b where c not 8", syncql.ErrorfExpected(db.GetContext(), "[%v]expected '%v'", 28, "'equal' or 'like'")},
		{"select x from y where a and b = c", syncql.ErrorfExpectedOperator(db.GetContext(), "[%v]expected operator, found %v", 24, "and")},
		{"select v from Customer limit 100 offset a", syncql.ErrorfExpected(db.GetContext(), "[%v]expected '%v'", 40, "positive integer literal")},
		{"select v from Customer limit -100 offset 5", syncql.ErrorfExpected(db.GetContext(), "[%v]expected '%v'", 29, "positive integer literal")},
		{"select v from Customer limit 100 offset -5", syncql.ErrorfExpected(db.GetContext(), "[%v]expected '%v'", 40, "positive integer literal")},
		{"select v from Customer limit a offset 200", syncql.ErrorfExpected(db.GetContext(), "[%v]expected '%v'", 29, "positive integer literal")},
		{"select v from Customer limit", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 28)},
		{"select v from Customer, Invoice", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 22, ",")},
		{"select v from Customer As Cust where foo = bar", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 23, "As")},
		{"select 1abc from Customer where foo = bar", syncql.ErrorfExpectedIdentifier(db.GetContext(), "[%v]expected identifier, found %v", 7, "1")},
		{"select v from Customer where Foo(1,) = true", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 35, ")")},
		{"select v from Customer where Foo(,1) = true", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 33, ",")},
		{"select v from Customer where Foo(1, 2.0 = true", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 40, "=")},
		{"select v from Customer where Foo(1, 2.0 limit 100", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 40, "limit")},
		{"select v from Customer where v is", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 33)},
		{"select v from Customer where v = 1.0 is k = \"abc\"", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 37, "is")},
		{"select v as from Customer", syncql.ErrorfExpectedFrom(db.GetContext(), "[%v]expected 'from', found %v", 17, "Customer")},
		{"select v from Customer where v.Foo = \"", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 37, "\"")},
		{"select v from Customer where v.Foo = \"Bar", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 37, "\"Bar")},
		{"select v from Customer where v.Foo = '", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 37, "'")},
		{"select v from Customer where v.Foo = 'a", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 37, "'a")},
		{"select v from Customer where v.Foo = 'abc", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 37, "'abc")},
		{"select v from Customer where v.Foo = 'abc'", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 37, "'abc'")},
		{"select v from Customer where v.Foo = 10.10.10", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 42, ".10")},
		// Parameters can only appear as operands in the where clause
		{"select ? from Customer", syncql.ErrorfExpectedIdentifier(db.GetContext(), "[%v]expected identifier, found %v", 7, "?")},
		// Parameters can only appear as operands in the where clause
		{"select v from Customer where k ? v", syncql.ErrorfExpectedOperator(db.GetContext(), "[%v]expected operator, found %v", 31, "?")},
		{"select a,", syncql.ErrorfExpectedIdentifier(db.GetContext(), "[%v]expected identifier, found %v", 9, "")},
		{"select a from b escape", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 22)},
		{"select a from b escape 123", syncql.ErrorfExpected(db.GetContext(), "[%v]expected '%v'", 23, "char literal")},
		{"select v[1](foo) from b", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 11, "(")},
		{"select v[1] as 123 from b", syncql.ErrorfExpectedIdentifier(db.GetContext(), "[%v]expected identifier, found %v", 15, "123")},
		{"select v.", syncql.ErrorfExpectedIdentifier(db.GetContext(), "[%v]expected identifier, found %v", 9, "")},
		{"select Foo(abc def) from Customers", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 15, "def")},
		{"select v.abc.\"8\" from Customers", syncql.ErrorfExpectedIdentifier(db.GetContext(), "[%v]expected identifier, found %v", 13, "8")},
		{"select v[abc from Customers", syncql.ErrorfExpected(db.GetContext(), "[%v]expected '%v'", 13, "]")},
		{"select v[* from Customers", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 9, "*")},
		{"select v from 123", syncql.ErrorfExpectedIdentifier(db.GetContext(), "[%v]expected identifier, found %v", 14, "123")},
		{"select v from Customers where (a = b *", syncql.ErrorfExpected(db.GetContext(), "[%v]expected '%v'", 37, ")")},
		// Delete
		{"delete.", syncql.ErrorfExpectedFrom(db.GetContext(), "[%v]expected 'from', found %v", 6, ".")},
		{"delete. from a", syncql.ErrorfExpectedFrom(db.GetContext(), "[%v]expected 'from', found %v", 6, ".")},
		{"delete ", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 7)},
		{"delete from Customer where (A=123 or B=456) and C=789)", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 53, ")")},
		{"delete from Customer where ((A=123 or B=456) and C=789", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 54)},
		{"delete from Customer where (((((A=123 or B=456 and C=789))))", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 60)},
		{"delete from Customer where (A=123 or B=456) and C=789)))))", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 53, ")")},
		{"delete from Customer where", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 26)},
		{"delete from Customer where ", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 27)},
		{"delete from Customer where )", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 27, ")")},
		{"delete from Customer where )A=123 or B=456) and C=789", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 27, ")")},
		{"delete from Customer where ()A=123 or B=456) and C=789", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 28, ")")},
		{"delete from Customer where (A=123 or B=456) and C=789)", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 53, ")")},
		{"delete bar from Customer", syncql.ErrorfExpectedFrom(db.GetContext(), "[%v]expected 'from', found %v", 7, "bar")},
		{"delete from Customer Invoice", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 21, "Invoice")},
		{"delete from (Customer)", syncql.ErrorfExpectedIdentifier(db.GetContext(), "[%v]expected identifier, found %v", 12, "(")},
		{"delete from Customer where a = (b)", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 31, "(")},
		{"delete from Customer where a = b and (c) = d", syncql.ErrorfExpectedOperator(db.GetContext(), "[%v]expected operator, found %v", 39, ")")},
		{"delete from Customer where a = b and c =", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 40)},
		{"delete from Customer where a = ", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 31)},
		{"delete from Customer where a", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 28)},
		{"delete", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 6)},
		{"delete from", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 11)},
		{"delete from b where c = d and e =", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 33)},
		{"delete from b where c = d and f", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 31)},
		{"delete from b where c = d and f *", syncql.ErrorfExpectedOperator(db.GetContext(), "[%v]expected operator, found %v", 32, "*")},
		{"delete from b where c <", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 23)},
		{"delete from b where c not", syncql.ErrorfExpected(db.GetContext(), "[%v]expected '%v'", 25, "'equal' or 'like'")},
		{"delete from b where c not 8", syncql.ErrorfExpected(db.GetContext(), "[%v]expected '%v'", 26, "'equal' or 'like'")},
		{"delete from y where a and b = c", syncql.ErrorfExpectedOperator(db.GetContext(), "[%v]expected operator, found %v", 22, "and")},
		{"delete from Customer limit a", syncql.ErrorfExpected(db.GetContext(), "[%v]expected '%v'", 27, "positive integer literal")},
		{"delete from Customer limit -100", syncql.ErrorfExpected(db.GetContext(), "[%v]expected '%v'", 27, "positive integer literal")},
		{"delete from Customer limit", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 26)},
		{"delete from Customer, Invoice", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 20, ",")},
		{"delete from Customer As Cust where foo = bar", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 21, "As")},
		{"delete from Customer where Foo(1,) = true", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 33, ")")},
		{"delete from Customer where Foo(,1) = true", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 31, ",")},
		{"delete from Customer where Foo(1, 2.0 = true", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 38, "=")},
		{"delete from Customer where Foo(1, 2.0 limit 100", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 38, "limit")},
		{"delete from Customer where v is", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 31)},
		{"delete from Customer where v = 1.0 is k = \"abc\"", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 35, "is")},
		{"delete as from Customer", syncql.ErrorfExpectedFrom(db.GetContext(), "[%v]expected 'from', found %v", 7, "as")},
		{"delete from Customer where v.Foo = \"", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 35, "\"")},
		{"delete from Customer where v.Foo = \"Bar", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 35, "\"Bar")},
		{"delete from Customer where v.Foo = '", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 35, "'")},
		{"delete from Customer where v.Foo = 'a", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 35, "'a")},
		{"delete from Customer where v.Foo = 'abc", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 35, "'abc")},
		{"delete from Customer where v.Foo = 'abc'", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 35, "'abc'")},
		{"delete from Customer where v.Foo = 10.10.10", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 40, ".10")},
		// Parameters can only appear as operands in the where clause
		{"delete ? from Customer", syncql.ErrorfExpectedFrom(db.GetContext(), "[%v]expected 'from', found %v", 7, "?")},
		// Parameters can only appear as operands in the where clause
		{"delete from Customer where k ? v", syncql.ErrorfExpectedOperator(db.GetContext(), "[%v]expected operator, found %v", 29, "?")},
		{"delete from b escape", syncql.ErrorfUnexpectedEndOfStatement(db.GetContext(), "[%v]unexpected end of statement", 20)},
		{"delete from b escape 123", syncql.ErrorfExpected(db.GetContext(), "[%v]expected '%v'", 21, "char literal")},
		{"delete from b where v[1](foo) = 1", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 24, "(")},
		{"delete from Customers where Foo(abc def) = 1", syncql.ErrorfUnexpected(db.GetContext(), "[%v]unexpected: %v", 36, "def")},
		{"delete from Customers where v.abc.\"8\" = 1", syncql.ErrorfExpectedIdentifier(db.GetContext(), "[%v]expected identifier, found %v", 34, "8")},
		{"delete from Customers where v[abc = 1", syncql.ErrorfExpected(db.GetContext(), "[%v]expected '%v'", 34, "]")},
		{"delete from Customers where v[*", syncql.ErrorfExpectedOperand(db.GetContext(), "[%v]expected operand, found %v", 30, "*")},
		{"delete from 123", syncql.ErrorfExpectedIdentifier(db.GetContext(), "[%v]expected identifier, found %v", 12, "123")},
		{"delete from Customers where (a = b *", syncql.ErrorfExpected(db.GetContext(), "[%v]expected '%v'", 35, ")")},
	}

	for i, test := range basic {
		_, err := queryparser.Parse(&db, test.query)
		// Test both that the IDs compare and the text compares (since the offset needs to match).
		if !errors.Is(err, test.err) || err.Error() != test.err.Error() {
			t.Log(verror.DebugString(err))
			t.Errorf("query: %v: %s; got %v, want %v", i, test.query, err, test.err)
		}
	}
}

func TestToString(t *testing.T) {
	basic := []toStringTest{
		{
			"select Upper(v.Name) from Customers where Upper(v.Name) = \"SMITH\"",
			"Off(0): Off(0):SELECT Columns( Off(7):Off(7):Upper(Off(13):(field) Off(13): Off(13):v. Off(15):Name)) Off(21):FROM Off(26):Customers  Off(36):WHERE (Off(42):Off(42):(function)Off(42):Upper(Off(48):(field) Off(48): Off(48):v. Off(50):Name) Off(56):= Off(58):(string)SMITH)",
		},
		{
			"select v.Address[0] from Customers",
			"Off(0): Off(0):SELECT Columns( Off(7): Off(7): Off(7):v. Off(9):Address[Off(17):(int)0]) Off(20):FROM Off(25):Customers",
		},
		{
			"select k from Customers where v like \"Foo^%%\" escape '^'",
			"Off(0): Off(0):SELECT Columns( Off(7): Off(7): Off(7):k) Off(9):FROM Off(14):Customers  Off(24):WHERE (Off(30):Off(30):(field) Off(30): Off(30):v Off(32):LIKE Off(37):(string)Foo^%%)  Off(46):ESCAPE  Off(53): ^",
		},
		{
			"select k from Customers limit 100 offset 200",
			"Off(0): Off(0):SELECT Columns( Off(7): Off(7): Off(7):k) Off(9):FROM Off(14):Customers  Off(24):LIMIT  Off(30): 100  Off(34):OFFSET  Off(41): 200",
		},
		{
			"select k from Customers where v.A = 10 and v.B <> 20",
			"Off(0): Off(0):SELECT Columns( Off(7): Off(7): Off(7):k) Off(9):FROM Off(14):Customers  Off(24):WHERE (Off(30):Off(30):(expr)(Off(30):Off(30):(field) Off(30): Off(30):v. Off(32):A Off(34):= Off(36):(int)10) Off(39):AND Off(43):(expr)(Off(43):Off(43):(field) Off(43): Off(43):v. Off(45):B Off(47):<> Off(50):(int)20))",
		},
		{
			"select k from Customers where Lower(v.A) < \"abc\" and v.B <= 'a'",
			"Off(0): Off(0):SELECT Columns( Off(7): Off(7): Off(7):k) Off(9):FROM Off(14):Customers  Off(24):WHERE (Off(30):Off(30):(expr)(Off(30):Off(30):(function)Off(30):Lower(Off(36):(field) Off(36): Off(36):v. Off(38):A) Off(41):< Off(43):(string)abc) Off(49):AND Off(53):(expr)(Off(53):Off(53):(field) Off(53): Off(53):v. Off(55):B Off(57):<= Off(60):(int)97))",
		},
		{
			"select k from Customers where v.A > 2.0 and v.B >= 10",
			"Off(0): Off(0):SELECT Columns( Off(7): Off(7): Off(7):k) Off(9):FROM Off(14):Customers  Off(24):WHERE (Off(30):Off(30):(expr)(Off(30):Off(30):(field) Off(30): Off(30):v. Off(32):A Off(34):> Off(36):(float)2) Off(40):AND Off(44):(expr)(Off(44):Off(44):(field) Off(44): Off(44):v. Off(46):B Off(48):>= Off(51):(int)10))",
		},
		{
			"select k from Customers where v.A = false and v.B is nil and v.C is not nil",
			"Off(0): Off(0):SELECT Columns( Off(7): Off(7): Off(7):k) Off(9):FROM Off(14):Customers  Off(24):WHERE (Off(30):Off(30):(expr)(Off(30):Off(30):(expr)(Off(30):Off(30):(field) Off(30): Off(30):v. Off(32):A Off(34):= Off(36):(bool)false) Off(42):AND Off(46):(expr)(Off(46):Off(46):(field) Off(46): Off(46):v. Off(48):B Off(50):IS Off(53):<nil>)) Off(57):AND Off(61):(expr)(Off(61):Off(61):(field) Off(61): Off(61):v. Off(63):C Off(65):IS NOT Off(72):<nil>))",
		},
		{
			"select v.Foo as Foo from Customers where v.A not like \"foo%\" or v.B = ?",
			"Off(0): Off(0):SELECT Columns( Off(7): Off(7): Off(7):v. Off(9):Foo Off(13): Off(16):Foo) Off(20):FROM Off(25):Customers  Off(35):WHERE (Off(41):Off(41):(expr)(Off(41):Off(41):(field) Off(41): Off(41):v. Off(43):A Off(45):NOT LIKE Off(54):(string)foo%) Off(61):OR Off(64):(expr)(Off(64):Off(64):(field) Off(64): Off(64):v. Off(66):B Off(68):= Off(70):?))",
		},
		{
			"select v.Foo as Foo from Customers where v.A not like \"foo%\" or v.B = ?",
			"Off(0): Off(0):SELECT Columns( Off(7): Off(7): Off(7):v. Off(9):Foo Off(13): Off(16):Foo) Off(20):FROM Off(25):Customers  Off(35):WHERE (Off(41):Off(41):(expr)(Off(41):Off(41):(field) Off(41): Off(41):v. Off(43):A Off(45):NOT LIKE Off(54):(string)foo%) Off(61):OR Off(64):(expr)(Off(64):Off(64):(field) Off(64): Off(64):v. Off(66):B Off(68):= Off(70):?))",
		},
	}
	for _, test := range basic {
		st, err := queryparser.Parse(&db, test.query)
		if err != nil {
			t.Errorf("query: %s; unexpected error: got %v, want nil", test.query, err)
		}
		s := (*st).String()
		if s != test.s {
			t.Errorf("query: %s; got %s, want %s", test.query, s, test.s)
		}
	}
}

func TestCopyAndSubstituteSelect(t *testing.T) {
	basic := []copyAndSubstituteSelectTest{
		{
			"select v from Customer where v.Value = ?",
			[]*vdl.Value{vdl.ValueOf(10)},
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 14},
					},
					Node: queryparser.Node{Off: 9},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 29},
									},
									{
										Value: "Value",
										Node:  queryparser.Node{Off: 31},
									},
								},
								Node: queryparser.Node{Off: 29},
							},
							Node: queryparser.Node{Off: 29},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 37},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypInt,
							Int:  10,
							Node: queryparser.Node{Off: 39},
						},
						Node: queryparser.Node{Off: 29},
					},
					Node: queryparser.Node{Off: 23},
				},
				Node: queryparser.Node{Off: 0},
			},
		},
		{
			"select v from Customer where v.Value = ?",
			[]*vdl.Value{vdl.ValueOf(true)},
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 14},
					},
					Node: queryparser.Node{Off: 9},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 29},
									},
									{
										Value: "Value",
										Node:  queryparser.Node{Off: 31},
									},
								},
								Node: queryparser.Node{Off: 29},
							},
							Node: queryparser.Node{Off: 29},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 37},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypBool,
							Bool: true,
							Node: queryparser.Node{Off: 39},
						},
						Node: queryparser.Node{Off: 29},
					},
					Node: queryparser.Node{Off: 23},
				},
				Node: queryparser.Node{Off: 0},
			},
		},
		{
			"select v from Customer where Now() < Time(?) and Foo(10,?,v.Bar) = true",
			[]*vdl.Value{vdl.ValueOf("abc"), vdl.ValueOf(42.0)},
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 14},
					},
					Node: queryparser.Node{Off: 9},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypFunction,
									Function: &queryparser.Function{
										Name: "Now",
										Args: nil,
										Node: queryparser.Node{Off: 29},
									},
									Node: queryparser.Node{Off: 29},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.LessThan,
									Node: queryparser.Node{Off: 35},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypFunction,
									Function: &queryparser.Function{
										Name: "Time",
										Args: []*queryparser.Operand{
											{
												Type: queryparser.TypStr,
												Str:  "abc",
												Node: queryparser.Node{Off: 42},
											},
										},
										Node: queryparser.Node{Off: 37},
									},
									Node: queryparser.Node{Off: 37},
								},
								Node: queryparser.Node{Off: 29},
							},
							Node: queryparser.Node{Off: 29},
						},
						Node: queryparser.Node{Off: 29},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.And,
							Node: queryparser.Node{Off: 45},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypFunction,
									Function: &queryparser.Function{
										Name: "Foo",
										Args: []*queryparser.Operand{
											{
												Type: queryparser.TypInt,
												Int:  10,
												Node: queryparser.Node{Off: 53},
											},
											{
												Type:  queryparser.TypFloat,
												Float: 42.0,
												Node:  queryparser.Node{Off: 56},
											},
											{
												Type: queryparser.TypField,
												Column: &queryparser.Field{
													Segments: []queryparser.Segment{
														{
															Value: "v",
															Node:  queryparser.Node{Off: 58},
														},
														{
															Value: "Bar",
															Node:  queryparser.Node{Off: 60},
														},
													},
													Node: queryparser.Node{Off: 58},
												},
												Node: queryparser.Node{Off: 58},
											},
										},
										Node: queryparser.Node{Off: 49},
									},
									Node: queryparser.Node{Off: 49},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Equal,
									Node: queryparser.Node{Off: 65},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypBool,
									Bool: true,
									Node: queryparser.Node{Off: 67},
								},
								Node: queryparser.Node{Off: 49},
							},
							Node: queryparser.Node{Off: 49},
						},
					},
					Node: queryparser.Node{Off: 23},
				},
				Node: queryparser.Node{Off: 0},
			},
		},
		{
			"select v from Customer where k like ? limit 10 offset 20 escape '^'",
			[]*vdl.Value{vdl.ValueOf("abc^%%")},
			queryparser.SelectStatement{
				Select: &queryparser.SelectClause{
					Selectors: []queryparser.Selector{
						{
							Type: queryparser.TypSelField,
							Field: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 7},
									},
								},
								Node: queryparser.Node{Off: 7},
							},
							Node: queryparser.Node{Off: 7},
						},
					},
					Node: queryparser.Node{Off: 0},
				},
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 14},
					},
					Node: queryparser.Node{Off: 9},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "k",
										Node:  queryparser.Node{Off: 29},
									},
								},
								Node: queryparser.Node{Off: 29},
							},
							Node: queryparser.Node{Off: 29},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Like,
							Node: queryparser.Node{Off: 31},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypStr,
							Str:  "abc^%%",
							Node: queryparser.Node{Off: 36},
						},
						Node: queryparser.Node{Off: 29},
					},
					Node: queryparser.Node{Off: 23},
				},
				Limit: &queryparser.LimitClause{
					Limit: &queryparser.Int64Value{
						Value: 10,
						Node:  queryparser.Node{Off: 44},
					},
					Node: queryparser.Node{Off: 38},
				},
				ResultsOffset: &queryparser.ResultsOffsetClause{
					ResultsOffset: &queryparser.Int64Value{
						Value: 20,
						Node:  queryparser.Node{Off: 54},
					},
					Node: queryparser.Node{Off: 47},
				},
				Escape: &queryparser.EscapeClause{
					EscapeChar: &queryparser.CharValue{
						Value: '^',
						Node:  queryparser.Node{Off: 64},
					},
					Node: queryparser.Node{Off: 57},
				},
				Node: queryparser.Node{Off: 0},
			},
		},
	}
	for _, test := range basic {
		st, err := queryparser.Parse(&db, test.query)
		if err != nil {
			t.Errorf("query: %s; unexpected parse error: got %v, want nil", test.query, err)
		}
		st2, err := (*st).CopyAndSubstitute(&db, test.subValues)
		if err != nil {
			t.Errorf("query: %s; unexpected error on st.CopyAndSubstitute: got %v, want nil", test.query, err)
		}
		if _, ok := (st2).(queryparser.SelectStatement); ok {
			if !reflect.DeepEqual(test.statement, st2) {
				t.Errorf("query: %s;\nGOT  %s\nWANT %s", test.query, st2, test.statement)
			}
		}
	}
}

func TestCopyAndSubstituteDelete(t *testing.T) {
	basic := []copyAndSubstituteDeleteTest{
		{
			"delete from Customer where v.Value = ?",
			[]*vdl.Value{vdl.ValueOf(10)},
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 27},
									},
									{
										Value: "Value",
										Node:  queryparser.Node{Off: 29},
									},
								},
								Node: queryparser.Node{Off: 27},
							},
							Node: queryparser.Node{Off: 27},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 35},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypInt,
							Int:  10,
							Node: queryparser.Node{Off: 37},
						},
						Node: queryparser.Node{Off: 27},
					},
					Node: queryparser.Node{Off: 21},
				},
				Node: queryparser.Node{Off: 0},
			},
		},
		{
			"delete from Customer where v.Value = ?",
			[]*vdl.Value{vdl.ValueOf(true)},
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "v",
										Node:  queryparser.Node{Off: 27},
									},
									{
										Value: "Value",
										Node:  queryparser.Node{Off: 29},
									},
								},
								Node: queryparser.Node{Off: 27},
							},
							Node: queryparser.Node{Off: 27},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Equal,
							Node: queryparser.Node{Off: 35},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypBool,
							Bool: true,
							Node: queryparser.Node{Off: 37},
						},
						Node: queryparser.Node{Off: 27},
					},
					Node: queryparser.Node{Off: 21},
				},
				Node: queryparser.Node{Off: 0},
			},
		},
		{
			"delete from Customer where Now() < Time(?) and Foo(10,?,v.Bar) = true",
			[]*vdl.Value{vdl.ValueOf("abc"), vdl.ValueOf(42.0)},
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypFunction,
									Function: &queryparser.Function{
										Name: "Now",
										Args: nil,
										Node: queryparser.Node{Off: 27},
									},
									Node: queryparser.Node{Off: 27},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.LessThan,
									Node: queryparser.Node{Off: 33},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypFunction,
									Function: &queryparser.Function{
										Name: "Time",
										Args: []*queryparser.Operand{
											{
												Type: queryparser.TypStr,
												Str:  "abc",
												Node: queryparser.Node{Off: 40},
											},
										},
										Node: queryparser.Node{Off: 35},
									},
									Node: queryparser.Node{Off: 35},
								},
								Node: queryparser.Node{Off: 27},
							},
							Node: queryparser.Node{Off: 27},
						},
						Node: queryparser.Node{Off: 27},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.And,
							Node: queryparser.Node{Off: 43},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypExpr,
							Expr: &queryparser.Expression{
								Operand1: &queryparser.Operand{
									Type: queryparser.TypFunction,
									Function: &queryparser.Function{
										Name: "Foo",
										Args: []*queryparser.Operand{
											{
												Type: queryparser.TypInt,
												Int:  10,
												Node: queryparser.Node{Off: 51},
											},
											{
												Type:  queryparser.TypFloat,
												Float: 42.0,
												Node:  queryparser.Node{Off: 54},
											},
											{
												Type: queryparser.TypField,
												Column: &queryparser.Field{
													Segments: []queryparser.Segment{
														{
															Value: "v",
															Node:  queryparser.Node{Off: 56},
														},
														{
															Value: "Bar",
															Node:  queryparser.Node{Off: 58},
														},
													},
													Node: queryparser.Node{Off: 56},
												},
												Node: queryparser.Node{Off: 56},
											},
										},
										Node: queryparser.Node{Off: 47},
									},
									Node: queryparser.Node{Off: 47},
								},
								Operator: &queryparser.BinaryOperator{
									Type: queryparser.Equal,
									Node: queryparser.Node{Off: 63},
								},
								Operand2: &queryparser.Operand{
									Type: queryparser.TypBool,
									Bool: true,
									Node: queryparser.Node{Off: 65},
								},
								Node: queryparser.Node{Off: 47},
							},
							Node: queryparser.Node{Off: 47},
						},
					},
					Node: queryparser.Node{Off: 21},
				},
				Node: queryparser.Node{Off: 0},
			},
		},
		{
			"delete from Customer where k like ? limit 10 escape '^'",
			[]*vdl.Value{vdl.ValueOf("abc^%%")},
			queryparser.DeleteStatement{
				From: &queryparser.FromClause{
					Table: queryparser.TableEntry{
						Name: "Customer",
						Node: queryparser.Node{Off: 12},
					},
					Node: queryparser.Node{Off: 7},
				},
				Where: &queryparser.WhereClause{
					Expr: &queryparser.Expression{
						Operand1: &queryparser.Operand{
							Type: queryparser.TypField,
							Column: &queryparser.Field{
								Segments: []queryparser.Segment{
									{
										Value: "k",
										Node:  queryparser.Node{Off: 29},
									},
								},
								Node: queryparser.Node{Off: 29},
							},
							Node: queryparser.Node{Off: 29},
						},
						Operator: &queryparser.BinaryOperator{
							Type: queryparser.Like,
							Node: queryparser.Node{Off: 31},
						},
						Operand2: &queryparser.Operand{
							Type: queryparser.TypStr,
							Str:  "abc^%%",
							Node: queryparser.Node{Off: 36},
						},
						Node: queryparser.Node{Off: 29},
					},
					Node: queryparser.Node{Off: 23},
				},
				Limit: &queryparser.LimitClause{
					Limit: &queryparser.Int64Value{
						Value: 10,
						Node:  queryparser.Node{Off: 44},
					},
					Node: queryparser.Node{Off: 38},
				},
				Escape: &queryparser.EscapeClause{
					EscapeChar: &queryparser.CharValue{
						Value: '^',
						Node:  queryparser.Node{Off: 54},
					},
					Node: queryparser.Node{Off: 47},
				},
				Node: queryparser.Node{Off: 0},
			},
		},
	}
	for _, test := range basic {
		st, err := queryparser.Parse(&db, test.query)
		if err != nil {
			t.Errorf("query: %s; unexpected parse error: got %v, want nil", test.query, err)
		}
		st2, err := (*st).CopyAndSubstitute(&db, test.subValues)
		if err != nil {
			t.Errorf("query: %s; unexpected error on st.CopyAndSubstitute: got %v, want nil", test.query, err)
		}
		if _, ok := (st2).(queryparser.SelectStatement); ok {
			if !reflect.DeepEqual(test.statement, st2) {
				t.Errorf("query: %s;\nGOT  %s\nWANT %s", test.query, st2, test.statement)
			}
		}
	}
}

func TestCopyAndSubstituteError(t *testing.T) {
	basic := []copyAndSubstituteErrorTest{
		{
			"select v from Customers",
			[]*vdl.Value{vdl.ValueOf(10)},
			syncql.ErrorfTooManyParamValuesSpecified(db.GetContext(), "[%v]too many parameter values specified", 0),
		},
		{
			"select v from Customers where v.A = 10",
			[]*vdl.Value{vdl.ValueOf(10)},
			syncql.ErrorfTooManyParamValuesSpecified(db.GetContext(), "[%v]too many parameter values specified", 24),
		},
		{
			"select v from Customers where v.A = ? and v.B = ?",
			[]*vdl.Value{vdl.ValueOf(10)},
			syncql.ErrorfNotEnoughParamValuesSpecified(db.GetContext(), "[%v]not enough parameter values specified", 48),
		},
		{
			"delete from Customers",
			[]*vdl.Value{vdl.ValueOf(10)},
			syncql.ErrorfTooManyParamValuesSpecified(db.GetContext(), "[%v]too many parameter values specified", 0),
		},
		{
			"delete from Customers where v.A = 10",
			[]*vdl.Value{vdl.ValueOf(10)},
			syncql.ErrorfTooManyParamValuesSpecified(db.GetContext(), "[%v]too many parameter values specified", 22),
		},
		{
			"delete from Customers where v.A = ? and v.B = ?",
			[]*vdl.Value{vdl.ValueOf(10)},
			syncql.ErrorfNotEnoughParamValuesSpecified(db.GetContext(), "[%v]not enough parameter values specified", 46),
		},
	}

	for _, test := range basic {
		st, err := queryparser.Parse(&db, test.query)
		if err != nil {
			t.Errorf("query: %s; unexpected parse error: got %v, want nil", test.query, err)
		}
		_, err = (*st).CopyAndSubstitute(&db, test.subValues)
		// Test both that the IDs compare and the text compares (since the offset needs to match).
		if !errors.Is(err, test.err) || err.Error() != test.err.Error() {
			t.Errorf("query: %s; got %v, want %v", test.query, err, test.err)
		}
	}
}

func TestStatementSizeExceeded(t *testing.T) {
	q := fmt.Sprintf("select a from b where c = \"%s\"", strings.Repeat("x", 12000))
	_, err := queryparser.Parse(&db, q)
	expectedErr := syncql.ErrorfMaxStatementLenExceeded(db.GetContext(), "[%v]maximum length of statements is %v; found %v", int64(0), queryparser.MaxStatementLen, int64(len(q)))
	if !errors.Is(err, expectedErr) || err.Error() != expectedErr.Error() {
		t.Errorf("query: %s; got %v, want %v", q, err, expectedErr)
	}
}
