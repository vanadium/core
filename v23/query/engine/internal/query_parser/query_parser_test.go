// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package query_parser_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	ds "v.io/v23/query/engine/datasource"
	"v.io/v23/query/engine/internal/query_parser"
	"v.io/v23/query/syncql"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
)

type parseSelectTest struct {
	query     string
	statement query_parser.SelectStatement
	err       error
}

type parseDeleteTest struct {
	query     string
	statement query_parser.DeleteStatement
	err       error
}

type parseErrorTest struct {
	query string
	err   error
}

type toStringTest struct {
	query string
	s     string
}

type copyAndSubstituteSelectTest struct {
	query     string
	subValues []*vdl.Value
	statement query_parser.SelectStatement
}

type copyAndSubstituteDeleteTest struct {
	query     string
	subValues []*vdl.Value
	statement query_parser.DeleteStatement
}

type copyAndSubstituteErrorTest struct {
	query     string
	subValues []*vdl.Value
	err       error
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
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select k as Key, v as Value from Customer",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "k",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							As: &query_parser.AsClause{
								AltName: query_parser.Name{
									Value: "Key",
									Node:  query_parser.Node{Off: 12},
								},
								Node: query_parser.Node{Off: 9},
							},
							Node: query_parser.Node{Off: 7},
						},
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 17},
									},
								},
								Node: query_parser.Node{Off: 17},
							},
							As: &query_parser.AsClause{
								AltName: query_parser.Name{
									Value: "Value",
									Node:  query_parser.Node{Off: 22},
								},
								Node: query_parser.Node{Off: 19},
							},
							Node: query_parser.Node{Off: 17},
						},
					},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 33},
					},
					Node: query_parser.Node{Off: 28},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"   select v from Customer",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 10},
									},
								},
								Node: query_parser.Node{Off: 10},
							},
							Node: query_parser.Node{Off: 10},
						},
					},
					Node: query_parser.Node{Off: 3},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 17},
					},
					Node: query_parser.Node{Off: 12},
				},
				Node: query_parser.Node{Off: 3},
			},
			nil,
		},
		{
			"select v from Customer limit 100 offset 200",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Limit: &query_parser.LimitClause{
					Limit: &query_parser.Int64Value{
						Value: 100,
						Node:  query_parser.Node{Off: 29},
					},
					Node: query_parser.Node{Off: 23},
				},
				ResultsOffset: &query_parser.ResultsOffsetClause{
					ResultsOffset: &query_parser.Int64Value{
						Value: 200,
						Node:  query_parser.Node{Off: 40},
					},
					Node: query_parser.Node{Off: 33},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer offset 400 limit 10",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Limit: &query_parser.LimitClause{
					Limit: &query_parser.Int64Value{
						Value: 10,
						Node:  query_parser.Node{Off: 40},
					},
					Node: query_parser.Node{Off: 34},
				},
				ResultsOffset: &query_parser.ResultsOffsetClause{
					ResultsOffset: &query_parser.Int64Value{
						Value: 400,
						Node:  query_parser.Node{Off: 30},
					},
					Node: query_parser.Node{Off: 23},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer limit 100 offset 200 limit 1 offset 2",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Limit: &query_parser.LimitClause{
					Limit: &query_parser.Int64Value{
						Value: 1,
						Node:  query_parser.Node{Off: 50},
					},
					Node: query_parser.Node{Off: 44},
				},
				ResultsOffset: &query_parser.ResultsOffsetClause{
					ResultsOffset: &query_parser.Int64Value{
						Value: 2,
						Node:  query_parser.Node{Off: 59},
					},
					Node: query_parser.Node{Off: 52},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo.x, bar.y from Customer",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "foo",
										Node:  query_parser.Node{Off: 7},
									},
									query_parser.Segment{
										Value: "x",
										Node:  query_parser.Node{Off: 11},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "bar",
										Node:  query_parser.Node{Off: 14},
									},
									query_parser.Segment{
										Value: "y",
										Node:  query_parser.Node{Off: 18},
									},
								},
								Node: query_parser.Node{Off: 14},
							},
							Node: query_parser.Node{Off: 14},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 25},
					},
					Node: query_parser.Node{Off: 20},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select select from from where where equal 42",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "select",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "from",
						Node: query_parser.Node{Off: 19},
					},
					Node: query_parser.Node{Off: 14},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "where",
										Node:  query_parser.Node{Off: 30},
									},
								},
								Node: query_parser.Node{Off: 30},
							},
							Node: query_parser.Node{Off: 30},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 36},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypInt,
							Int:  42,
							Node: query_parser.Node{Off: 42},
						},
						Node: query_parser.Node{Off: 30},
					},
					Node: query_parser.Node{Off: 24},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where v.Value equal true",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 29},
									},
									query_parser.Segment{
										Value: "Value",
										Node:  query_parser.Node{Off: 31},
									},
								},
								Node: query_parser.Node{Off: 29},
							},
							Node: query_parser.Node{Off: 29},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 37},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypBool,
							Bool: true,
							Node: query_parser.Node{Off: 43},
						},
						Node: query_parser.Node{Off: 29},
					},
					Node: query_parser.Node{Off: 23},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where v.Value = ?",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 29},
									},
									query_parser.Segment{
										Value: "Value",
										Node:  query_parser.Node{Off: 31},
									},
								},
								Node: query_parser.Node{Off: 29},
							},
							Node: query_parser.Node{Off: 29},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 37},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypParameter,
							Node: query_parser.Node{Off: 39},
						},
						Node: query_parser.Node{Off: 29},
					},
					Node: query_parser.Node{Off: 23},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where Now() < Time(?) and Foo(10,?,v.Bar) = true",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypFunction,
									Function: &query_parser.Function{
										Name: "Now",
										Args: nil,
										Node: query_parser.Node{Off: 29},
									},
									Node: query_parser.Node{Off: 29},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.LessThan,
									Node: query_parser.Node{Off: 35},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypFunction,
									Function: &query_parser.Function{
										Name: "Time",
										Args: []*query_parser.Operand{
											&query_parser.Operand{
												Type: query_parser.TypParameter,
												Node: query_parser.Node{Off: 42},
											},
										},
										Node: query_parser.Node{Off: 37},
									},
									Node: query_parser.Node{Off: 37},
								},
								Node: query_parser.Node{Off: 29},
							},
							Node: query_parser.Node{Off: 29},
						},
						Node: query_parser.Node{Off: 29},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.And,
							Node: query_parser.Node{Off: 45},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypFunction,
									Function: &query_parser.Function{
										Name: "Foo",
										Args: []*query_parser.Operand{
											&query_parser.Operand{
												Type: query_parser.TypInt,
												Int:  10,
												Node: query_parser.Node{Off: 53},
											},
											&query_parser.Operand{
												Type: query_parser.TypParameter,
												Node: query_parser.Node{Off: 56},
											},
											&query_parser.Operand{
												Type: query_parser.TypField,
												Column: &query_parser.Field{
													Segments: []query_parser.Segment{
														query_parser.Segment{
															Value: "v",
															Node:  query_parser.Node{Off: 58},
														},
														query_parser.Segment{
															Value: "Bar",
															Node:  query_parser.Node{Off: 60},
														},
													},
													Node: query_parser.Node{Off: 58},
												},
												Node: query_parser.Node{Off: 58},
											},
										},
										Node: query_parser.Node{Off: 49},
									},
									Node: query_parser.Node{Off: 49},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 65},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypBool,
									Bool: true,
									Node: query_parser.Node{Off: 67},
								},
								Node: query_parser.Node{Off: 49},
							},
							Node: query_parser.Node{Off: 49},
						},
					},
					Node: query_parser.Node{Off: 23},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where v.ZipCode is nil",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 29},
									},
									query_parser.Segment{
										Value: "ZipCode",
										Node:  query_parser.Node{Off: 31},
									},
								},
								Node: query_parser.Node{Off: 29},
							},
							Node: query_parser.Node{Off: 29},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Is,
							Node: query_parser.Node{Off: 39},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypNil,
							Node: query_parser.Node{Off: 42},
						},
						Node: query_parser.Node{Off: 29},
					},
					Node: query_parser.Node{Off: 23},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where v.ZipCode is not nil",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 29},
									},
									query_parser.Segment{
										Value: "ZipCode",
										Node:  query_parser.Node{Off: 31},
									},
								},
								Node: query_parser.Node{Off: 29},
							},
							Node: query_parser.Node{Off: 29},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.IsNot,
							Node: query_parser.Node{Off: 39},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypNil,
							Node: query_parser.Node{Off: 46},
						},
						Node: query_parser.Node{Off: 29},
					},
					Node: query_parser.Node{Off: 23},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where v.Value = false",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 29},
									},
									query_parser.Segment{
										Value: "Value",
										Node:  query_parser.Node{Off: 31},
									},
								},
								Node: query_parser.Node{Off: 29},
							},
							Node: query_parser.Node{Off: 29},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 37},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypBool,
							Bool: false,
							Node: query_parser.Node{Off: 39},
						},
						Node: query_parser.Node{Off: 29},
					},
					Node: query_parser.Node{Off: 23},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where v.Value equal -42",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 29},
									},
									query_parser.Segment{
										Value: "Value",
										Node:  query_parser.Node{Off: 31},
									},
								},
								Node: query_parser.Node{Off: 29},
							},
							Node: query_parser.Node{Off: 29},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 37},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypInt,
							Int:  -42,
							Node: query_parser.Node{Off: 43},
						},
						Node: query_parser.Node{Off: 29},
					},
					Node: query_parser.Node{Off: 23},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where v.Value equal -18.888",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 29},
									},
									query_parser.Segment{
										Value: "Value",
										Node:  query_parser.Node{Off: 31},
									},
								},
								Node: query_parser.Node{Off: 29},
							},
							Node: query_parser.Node{Off: 29},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 37},
						},
						Operand2: &query_parser.Operand{
							Type:  query_parser.TypFloat,
							Float: -18.888,
							Node:  query_parser.Node{Off: 43},
						},
						Node: query_parser.Node{Off: 29},
					},
					Node: query_parser.Node{Off: 23},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select x from y where b = 'c'",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "x",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "y",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "b",
										Node:  query_parser.Node{Off: 22},
									},
								},
								Node: query_parser.Node{Off: 22},
							},
							Node: query_parser.Node{Off: 22},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 24},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypInt,
							Int:  'c',
							Node: query_parser.Node{Off: 26},
						},
						Node: query_parser.Node{Off: 22},
					},
					Node: query_parser.Node{Off: 16},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select x from y where b = 'c' limit 10 offset 20",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "x",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "y",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "b",
										Node:  query_parser.Node{Off: 22},
									},
								},
								Node: query_parser.Node{Off: 22},
							},
							Node: query_parser.Node{Off: 22},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 24},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypInt,
							Int:  'c',
							Node: query_parser.Node{Off: 26},
						},
						Node: query_parser.Node{Off: 22},
					},
					Node: query_parser.Node{Off: 16},
				},
				Limit: &query_parser.LimitClause{
					Limit: &query_parser.Int64Value{
						Value: 10,
						Node:  query_parser.Node{Off: 36},
					},
					Node: query_parser.Node{Off: 30},
				},
				ResultsOffset: &query_parser.ResultsOffsetClause{
					ResultsOffset: &query_parser.Int64Value{
						Value: 20,
						Node:  query_parser.Node{Off: 46},
					},
					Node: query_parser.Node{Off: 39},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select x from y where b = 'c' limit 10",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "x",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "y",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "b",
										Node:  query_parser.Node{Off: 22},
									},
								},
								Node: query_parser.Node{Off: 22},
							},
							Node: query_parser.Node{Off: 22},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 24},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypInt,
							Int:  'c',
							Node: query_parser.Node{Off: 26},
						},
						Node: query_parser.Node{Off: 22},
					},
					Node: query_parser.Node{Off: 16},
				},
				Limit: &query_parser.LimitClause{
					Limit: &query_parser.Int64Value{
						Value: 10,
						Node:  query_parser.Node{Off: 36},
					},
					Node: query_parser.Node{Off: 30},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select x from y where b = 'c' offset 10",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "x",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "y",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "b",
										Node:  query_parser.Node{Off: 22},
									},
								},
								Node: query_parser.Node{Off: 22},
							},
							Node: query_parser.Node{Off: 22},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 24},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypInt,
							Int:  'c',
							Node: query_parser.Node{Off: 26},
						},
						Node: query_parser.Node{Off: 22},
					},
					Node: query_parser.Node{Off: 16},
				},
				ResultsOffset: &query_parser.ResultsOffsetClause{
					ResultsOffset: &query_parser.Int64Value{
						Value: 10,
						Node:  query_parser.Node{Off: 37},
					},
					Node: query_parser.Node{Off: 30},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where k like \"Foo^%Bar\" escape '^'",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "k",
										Node:  query_parser.Node{Off: 29},
									},
								},
								Node: query_parser.Node{Off: 29},
							},
							Node: query_parser.Node{Off: 29},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Like,
							Node: query_parser.Node{Off: 31},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypStr,
							Str:  "Foo^%Bar",
							Node: query_parser.Node{Off: 36},
						},
						Node: query_parser.Node{Off: 29},
					},
					Node: query_parser.Node{Off: 23},
				},
				Escape: &query_parser.EscapeClause{
					EscapeChar: &query_parser.CharValue{
						Value: '^',
						Node:  query_parser.Node{Off: 54},
					},
					Node: query_parser.Node{Off: 47},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo.bar, tom.dick.harry from Customer where a.b.c = \"baz\" and d.e.f like \"%foobarbaz\"",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "foo",
										Node:  query_parser.Node{Off: 7},
									},
									query_parser.Segment{
										Value: "bar",
										Node:  query_parser.Node{Off: 11},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "tom",
										Node:  query_parser.Node{Off: 16},
									},
									query_parser.Segment{
										Value: "dick",
										Node:  query_parser.Node{Off: 20},
									},
									query_parser.Segment{
										Value: "harry",
										Node:  query_parser.Node{Off: 25},
									},
								},
								Node: query_parser.Node{Off: 16},
							},
							Node: query_parser.Node{Off: 16},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 36},
					},
					Node: query_parser.Node{Off: 31},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "a",
												Node:  query_parser.Node{Off: 51},
											},
											query_parser.Segment{
												Value: "b",
												Node:  query_parser.Node{Off: 53},
											},
											query_parser.Segment{
												Value: "c",
												Node:  query_parser.Node{Off: 55},
											},
										},
										Node: query_parser.Node{Off: 51},
									},
									Node: query_parser.Node{Off: 51},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 57},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypStr,
									Str:  "baz",
									Node: query_parser.Node{Off: 59},
								},
								Node: query_parser.Node{Off: 51},
							},
							Node: query_parser.Node{Off: 51},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.And,
							Node: query_parser.Node{Off: 65},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "d",
												Node:  query_parser.Node{Off: 69},
											},
											query_parser.Segment{
												Value: "e",
												Node:  query_parser.Node{Off: 71},
											},
											query_parser.Segment{
												Value: "f",
												Node:  query_parser.Node{Off: 73},
											},
										},
										Node: query_parser.Node{Off: 69},
									},
									Node: query_parser.Node{Off: 69},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Like,
									Node: query_parser.Node{Off: 75},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypStr,
									Str:  "%foobarbaz",
									Node: query_parser.Node{Off: 80},
								},
								Node: query_parser.Node{Off: 69},
							},
							Node: query_parser.Node{Off: 69},
						},
						Node: query_parser.Node{Off: 51},
					},
					Node: query_parser.Node{Off: 45},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo, bar from Customer where CustRecord.CustID=123 or CustRecord.Name like \"f%\"",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "foo",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "bar",
										Node:  query_parser.Node{Off: 12},
									},
								},
								Node: query_parser.Node{Off: 12},
							},
							Node: query_parser.Node{Off: 12},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 21},
					},
					Node: query_parser.Node{Off: 16},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "CustRecord",
												Node:  query_parser.Node{Off: 36},
											},
											query_parser.Segment{
												Value: "CustID",
												Node:  query_parser.Node{Off: 47},
											},
										},
										Node: query_parser.Node{Off: 36},
									},
									Node: query_parser.Node{Off: 36},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 53},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypInt,
									Int:  123,
									Node: query_parser.Node{Off: 54},
								},
								Node: query_parser.Node{Off: 36},
							},
							Node: query_parser.Node{Off: 36},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Or,
							Node: query_parser.Node{Off: 58},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "CustRecord",
												Node:  query_parser.Node{Off: 61},
											},
											query_parser.Segment{
												Value: "Name",
												Node:  query_parser.Node{Off: 72},
											},
										},
										Node: query_parser.Node{Off: 61},
									},
									Node: query_parser.Node{Off: 61},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Like,
									Node: query_parser.Node{Off: 77},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypStr,
									Str:  "f%",
									Node: query_parser.Node{Off: 82},
								},
								Node: query_parser.Node{Off: 61},
							},
							Node: query_parser.Node{Off: 61},
						},
						Node: query_parser.Node{Off: 36},
					},
					Node: query_parser.Node{Off: 30},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo from Customer where A=123 or B=456 and C=789",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "foo",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 16},
					},
					Node: query_parser.Node{Off: 11},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "A",
														Node:  query_parser.Node{Off: 31},
													},
												},
												Node: query_parser.Node{Off: 31},
											},
											Node: query_parser.Node{Off: 31},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 32},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  123,
											Node: query_parser.Node{Off: 33},
										},
										Node: query_parser.Node{Off: 31},
									},
									Node: query_parser.Node{Off: 31},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Or,
									Node: query_parser.Node{Off: 37},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "B",
														Node:  query_parser.Node{Off: 40},
													},
												},
												Node: query_parser.Node{Off: 40},
											},
											Node: query_parser.Node{Off: 40},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 41},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  456,
											Node: query_parser.Node{Off: 42},
										},
										Node: query_parser.Node{Off: 40},
									},
									Node: query_parser.Node{Off: 40},
								},
								Node: query_parser.Node{Off: 31},
							},
							Node: query_parser.Node{Off: 31},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.And,
							Node: query_parser.Node{Off: 46},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "C",
												Node:  query_parser.Node{Off: 50},
											},
										},
										Node: query_parser.Node{Off: 50},
									},
									Node: query_parser.Node{Off: 50},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 51},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypInt,
									Int:  789,
									Node: query_parser.Node{Off: 52},
								},
								Node: query_parser.Node{Off: 50},
							},
							Node: query_parser.Node{Off: 50},
						},
						Node: query_parser.Node{Off: 31},
					},
					Node: query_parser.Node{Off: 25},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo from Customer where (A=123 or B=456) and C=789",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "foo",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 16},
					},
					Node: query_parser.Node{Off: 11},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "A",
														Node:  query_parser.Node{Off: 32},
													},
												},
												Node: query_parser.Node{Off: 32},
											},
											Node: query_parser.Node{Off: 32},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 33},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  123,
											Node: query_parser.Node{Off: 34},
										},
										Node: query_parser.Node{Off: 32},
									},
									Node: query_parser.Node{Off: 32},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Or,
									Node: query_parser.Node{Off: 38},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "B",
														Node:  query_parser.Node{Off: 41},
													},
												},
												Node: query_parser.Node{Off: 41},
											},
											Node: query_parser.Node{Off: 41},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 42},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  456,
											Node: query_parser.Node{Off: 43},
										},
										Node: query_parser.Node{Off: 41},
									},
									Node: query_parser.Node{Off: 41},
								},
								Node: query_parser.Node{Off: 32},
							},
							Node: query_parser.Node{Off: 32},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.And,
							Node: query_parser.Node{Off: 48},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "C",
												Node:  query_parser.Node{Off: 52},
											},
										},
										Node: query_parser.Node{Off: 52},
									},
									Node: query_parser.Node{Off: 52},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 53},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypInt,
									Int:  789,
									Node: query_parser.Node{Off: 54},
								},
								Node: query_parser.Node{Off: 52},
							},
							Node: query_parser.Node{Off: 52},
						},
						Node: query_parser.Node{Off: 32},
					},
					Node: query_parser.Node{Off: 25},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo from Customer where (A<=123 or B>456) and C>=789",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "foo",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 16},
					},
					Node: query_parser.Node{Off: 11},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "A",
														Node:  query_parser.Node{Off: 32},
													},
												},
												Node: query_parser.Node{Off: 32},
											},
											Node: query_parser.Node{Off: 32},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.LessThanOrEqual,
											Node: query_parser.Node{Off: 33},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  123,
											Node: query_parser.Node{Off: 35},
										},
										Node: query_parser.Node{Off: 32},
									},
									Node: query_parser.Node{Off: 32},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Or,
									Node: query_parser.Node{Off: 39},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "B",
														Node:  query_parser.Node{Off: 42},
													},
												},
												Node: query_parser.Node{Off: 42},
											},
											Node: query_parser.Node{Off: 42},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.GreaterThan,
											Node: query_parser.Node{Off: 43},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  456,
											Node: query_parser.Node{Off: 44},
										},
										Node: query_parser.Node{Off: 42},
									},
									Node: query_parser.Node{Off: 42},
								},
								Node: query_parser.Node{Off: 32},
							},
							Node: query_parser.Node{Off: 32},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.And,
							Node: query_parser.Node{Off: 49},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "C",
												Node:  query_parser.Node{Off: 53},
											},
										},
										Node: query_parser.Node{Off: 53},
									},
									Node: query_parser.Node{Off: 53},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.GreaterThanOrEqual,
									Node: query_parser.Node{Off: 54},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypInt,
									Int:  789,
									Node: query_parser.Node{Off: 56},
								},
								Node: query_parser.Node{Off: 53},
							},
							Node: query_parser.Node{Off: 53},
						},
						Node: query_parser.Node{Off: 32},
					},
					Node: query_parser.Node{Off: 25},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo from Customer where A=123 or (B=456 and C=789)",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "foo",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 16},
					},
					Node: query_parser.Node{Off: 11},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "A",
												Node:  query_parser.Node{Off: 31},
											},
										},
										Node: query_parser.Node{Off: 31},
									},
									Node: query_parser.Node{Off: 31},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 32},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypInt,
									Int:  123,
									Node: query_parser.Node{Off: 33},
								},
								Node: query_parser.Node{Off: 31},
							},
							Node: query_parser.Node{Off: 31},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Or,
							Node: query_parser.Node{Off: 37},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "B",
														Node:  query_parser.Node{Off: 41},
													},
												},
												Node: query_parser.Node{Off: 41},
											},
											Node: query_parser.Node{Off: 41},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 42},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  456,
											Node: query_parser.Node{Off: 43},
										},
										Node: query_parser.Node{Off: 41},
									},
									Node: query_parser.Node{Off: 41},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.And,
									Node: query_parser.Node{Off: 47},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "C",
														Node:  query_parser.Node{Off: 51},
													},
												},
												Node: query_parser.Node{Off: 51},
											},
											Node: query_parser.Node{Off: 51},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 52},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  789,
											Node: query_parser.Node{Off: 53},
										},
										Node: query_parser.Node{Off: 51},
									},
									Node: query_parser.Node{Off: 51},
								},
								Node: query_parser.Node{Off: 41},
							},
							Node: query_parser.Node{Off: 41},
						},
						Node: query_parser.Node{Off: 31},
					},
					Node: query_parser.Node{Off: 25},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo from Customer where (A=123) or ((B=456) and (C=789))",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "foo",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 16},
					},
					Node: query_parser.Node{Off: 11},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "A",
												Node:  query_parser.Node{Off: 32},
											},
										},
										Node: query_parser.Node{Off: 32},
									},
									Node: query_parser.Node{Off: 32},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 33},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypInt,
									Int:  123,
									Node: query_parser.Node{Off: 34},
								},
								Node: query_parser.Node{Off: 32},
							},
							Node: query_parser.Node{Off: 32},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Or,
							Node: query_parser.Node{Off: 39},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "B",
														Node:  query_parser.Node{Off: 44},
													},
												},
												Node: query_parser.Node{Off: 44},
											},
											Node: query_parser.Node{Off: 44},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 45},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  456,
											Node: query_parser.Node{Off: 46},
										},
										Node: query_parser.Node{Off: 44},
									},
									Node: query_parser.Node{Off: 44},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.And,
									Node: query_parser.Node{Off: 51},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "C",
														Node:  query_parser.Node{Off: 56},
													},
												},
												Node: query_parser.Node{Off: 56},
											},
											Node: query_parser.Node{Off: 56},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 57},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  789,
											Node: query_parser.Node{Off: 58},
										},
										Node: query_parser.Node{Off: 56},
									},
									Node: query_parser.Node{Off: 56},
								},
								Node: query_parser.Node{Off: 44},
							},
							Node: query_parser.Node{Off: 44},
						},
						Node: query_parser.Node{Off: 32},
					},
					Node: query_parser.Node{Off: 25},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select foo from Customer where A<>123 or B not equal 456 and C not like \"abc%\"",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "foo",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 16},
					},
					Node: query_parser.Node{Off: 11},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "A",
														Node:  query_parser.Node{Off: 31},
													},
												},
												Node: query_parser.Node{Off: 31},
											},
											Node: query_parser.Node{Off: 31},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.NotEqual,
											Node: query_parser.Node{Off: 32},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  123,
											Node: query_parser.Node{Off: 34},
										},
										Node: query_parser.Node{Off: 31},
									},
									Node: query_parser.Node{Off: 31},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Or,
									Node: query_parser.Node{Off: 38},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "B",
														Node:  query_parser.Node{Off: 41},
													},
												},
												Node: query_parser.Node{Off: 41},
											},
											Node: query_parser.Node{Off: 41},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.NotEqual,
											Node: query_parser.Node{Off: 43},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  456,
											Node: query_parser.Node{Off: 53},
										},
										Node: query_parser.Node{Off: 41},
									},
									Node: query_parser.Node{Off: 41},
								},
								Node: query_parser.Node{Off: 31},
							},
							Node: query_parser.Node{Off: 31},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.And,
							Node: query_parser.Node{Off: 57},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "C",
												Node:  query_parser.Node{Off: 61},
											},
										},
										Node: query_parser.Node{Off: 61},
									},
									Node: query_parser.Node{Off: 61},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.NotLike,
									Node: query_parser.Node{Off: 63},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypStr,
									Str:  "abc%",
									Node: query_parser.Node{Off: 72},
								},
								Node: query_parser.Node{Off: 61},
							},
							Node: query_parser.Node{Off: 61},
						},
						Node: query_parser.Node{Off: 31},
					},
					Node: query_parser.Node{Off: 25},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where Now() < Time(\"2015/07/22\") and Foo(10,20.1,v.Bar) = true",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypFunction,
									Function: &query_parser.Function{
										Name: "Now",
										Args: nil,
										Node: query_parser.Node{Off: 29},
									},
									Node: query_parser.Node{Off: 29},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.LessThan,
									Node: query_parser.Node{Off: 35},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypFunction,
									Function: &query_parser.Function{
										Name: "Time",
										Args: []*query_parser.Operand{
											&query_parser.Operand{
												Type: query_parser.TypStr,
												Str:  "2015/07/22",
												Node: query_parser.Node{Off: 42},
											},
										},
										Node: query_parser.Node{Off: 37},
									},
									Node: query_parser.Node{Off: 37},
								},
								Node: query_parser.Node{Off: 29},
							},
							Node: query_parser.Node{Off: 29},
						},
						Node: query_parser.Node{Off: 29},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.And,
							Node: query_parser.Node{Off: 56},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypFunction,
									Function: &query_parser.Function{
										Name: "Foo",
										Args: []*query_parser.Operand{
											&query_parser.Operand{
												Type: query_parser.TypInt,
												Int:  10,
												Node: query_parser.Node{Off: 64},
											},
											&query_parser.Operand{
												Type:  query_parser.TypFloat,
												Float: 20.1,
												Node:  query_parser.Node{Off: 67},
											},
											&query_parser.Operand{
												Type: query_parser.TypField,
												Column: &query_parser.Field{
													Segments: []query_parser.Segment{
														query_parser.Segment{
															Value: "v",
															Node:  query_parser.Node{Off: 72},
														},
														query_parser.Segment{
															Value: "Bar",
															Node:  query_parser.Node{Off: 74},
														},
													},
													Node: query_parser.Node{Off: 72},
												},
												Node: query_parser.Node{Off: 72},
											},
										},
										Node: query_parser.Node{Off: 60},
									},
									Node: query_parser.Node{Off: 60},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 79},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypBool,
									Bool: true,
									Node: query_parser.Node{Off: 81},
								},
								Node: query_parser.Node{Off: 60},
							},
							Node: query_parser.Node{Off: 60},
						},
					},
					Node: query_parser.Node{Off: 23},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select Now() from Customer",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelFunc,
							Function: &query_parser.Function{
								Name: "Now",
								Args: nil,
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 18},
					},
					Node: query_parser.Node{Off: 13},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select Now(), Date(\"2015-06-01 PST\") from Customer",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelFunc,
							Function: &query_parser.Function{
								Name: "Now",
								Args: nil,
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
						query_parser.Selector{
							Type: query_parser.TypSelFunc,
							Function: &query_parser.Function{
								Name: "Date",
								Args: []*query_parser.Operand{
									&query_parser.Operand{
										Type: query_parser.TypStr,
										Str:  "2015-06-01 PST",
										Node: query_parser.Node{Off: 19},
									},
								},
								Node: query_parser.Node{Off: 14},
							},
							Node: query_parser.Node{Off: 14},
						},
					},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 42},
					},
					Node: query_parser.Node{Off: 37},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v[\"foo\"] from Customer",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Keys: []*query_parser.Operand{
											&query_parser.Operand{
												Type: query_parser.TypStr,
												Str:  "foo",
												Node: query_parser.Node{Off: 9},
											},
										},
										Node: query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 21},
					},
					Node: query_parser.Node{Off: 16},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v[\"foo\"][\"bar\"] from Customer",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Keys: []*query_parser.Operand{
											&query_parser.Operand{
												Type: query_parser.TypStr,
												Str:  "foo",
												Node: query_parser.Node{Off: 9},
											},
											&query_parser.Operand{
												Type: query_parser.TypStr,
												Str:  "bar",
												Node: query_parser.Node{Off: 16},
											},
										},
										Node: query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 28},
					},
					Node: query_parser.Node{Off: 23},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"select v from Customer where v.Foo[v.Bar] = \"abc\"",
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 29},
									},
									query_parser.Segment{
										Value: "Foo",
										Keys: []*query_parser.Operand{
											&query_parser.Operand{
												Type: query_parser.TypField,
												Column: &query_parser.Field{
													Segments: []query_parser.Segment{
														query_parser.Segment{
															Value: "v",
															Node:  query_parser.Node{Off: 35},
														},
														query_parser.Segment{
															Value: "Bar",
															Node:  query_parser.Node{Off: 37},
														},
													},
													Node: query_parser.Node{Off: 35},
												},
												Node: query_parser.Node{Off: 35},
											},
										},
										Node: query_parser.Node{Off: 31},
									},
								},
								Node: query_parser.Node{Off: 29},
							},
							Node: query_parser.Node{Off: 29},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 42},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypStr,
							Str:  "abc",
							Node: query_parser.Node{Off: 44},
						},
						Node: query_parser.Node{Off: 29},
					},
					Node: query_parser.Node{Off: 23},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
	}

	for _, test := range basic {
		st, err := query_parser.Parse(&db, test.query)
		if err != nil {
			t.Errorf("query: %s; unexpected error: got %v, want nil", test.query, err)
		}
		switch (*st).(type) {
		case query_parser.SelectStatement:
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
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"   delete from Customer",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 15},
					},
					Node: query_parser.Node{Off: 10},
				},
				Node: query_parser.Node{Off: 3},
			},
			nil,
		},
		{
			"delete from Customer limit 100",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Limit: &query_parser.LimitClause{
					Limit: &query_parser.Int64Value{
						Value: 100,
						Node:  query_parser.Node{Off: 27},
					},
					Node: query_parser.Node{Off: 21},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer limit 100 limit 1",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Limit: &query_parser.LimitClause{
					Limit: &query_parser.Int64Value{
						Value: 1,
						Node:  query_parser.Node{Off: 37},
					},
					Node: query_parser.Node{Off: 31},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from from where where equal 42",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "from",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "where",
										Node:  query_parser.Node{Off: 23},
									},
								},
								Node: query_parser.Node{Off: 23},
							},
							Node: query_parser.Node{Off: 23},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 29},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypInt,
							Int:  42,
							Node: query_parser.Node{Off: 35},
						},
						Node: query_parser.Node{Off: 23},
					},
					Node: query_parser.Node{Off: 17},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where v.Value equal true",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 27},
									},
									query_parser.Segment{
										Value: "Value",
										Node:  query_parser.Node{Off: 29},
									},
								},
								Node: query_parser.Node{Off: 27},
							},
							Node: query_parser.Node{Off: 27},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 35},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypBool,
							Bool: true,
							Node: query_parser.Node{Off: 41},
						},
						Node: query_parser.Node{Off: 27},
					},
					Node: query_parser.Node{Off: 21},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where v.Value = ?",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 27},
									},
									query_parser.Segment{
										Value: "Value",
										Node:  query_parser.Node{Off: 29},
									},
								},
								Node: query_parser.Node{Off: 27},
							},
							Node: query_parser.Node{Off: 27},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 35},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypParameter,
							Node: query_parser.Node{Off: 37},
						},
						Node: query_parser.Node{Off: 27},
					},
					Node: query_parser.Node{Off: 21},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where Now() < Time(?) and Foo(10,?,v.Bar) = true",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypFunction,
									Function: &query_parser.Function{
										Name: "Now",
										Args: nil,
										Node: query_parser.Node{Off: 27},
									},
									Node: query_parser.Node{Off: 27},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.LessThan,
									Node: query_parser.Node{Off: 33},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypFunction,
									Function: &query_parser.Function{
										Name: "Time",
										Args: []*query_parser.Operand{
											&query_parser.Operand{
												Type: query_parser.TypParameter,
												Node: query_parser.Node{Off: 40},
											},
										},
										Node: query_parser.Node{Off: 35},
									},
									Node: query_parser.Node{Off: 35},
								},
								Node: query_parser.Node{Off: 27},
							},
							Node: query_parser.Node{Off: 27},
						},
						Node: query_parser.Node{Off: 27},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.And,
							Node: query_parser.Node{Off: 43},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypFunction,
									Function: &query_parser.Function{
										Name: "Foo",
										Args: []*query_parser.Operand{
											&query_parser.Operand{
												Type: query_parser.TypInt,
												Int:  10,
												Node: query_parser.Node{Off: 51},
											},
											&query_parser.Operand{
												Type: query_parser.TypParameter,
												Node: query_parser.Node{Off: 54},
											},
											&query_parser.Operand{
												Type: query_parser.TypField,
												Column: &query_parser.Field{
													Segments: []query_parser.Segment{
														query_parser.Segment{
															Value: "v",
															Node:  query_parser.Node{Off: 56},
														},
														query_parser.Segment{
															Value: "Bar",
															Node:  query_parser.Node{Off: 58},
														},
													},
													Node: query_parser.Node{Off: 56},
												},
												Node: query_parser.Node{Off: 56},
											},
										},
										Node: query_parser.Node{Off: 47},
									},
									Node: query_parser.Node{Off: 47},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 63},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypBool,
									Bool: true,
									Node: query_parser.Node{Off: 65},
								},
								Node: query_parser.Node{Off: 47},
							},
							Node: query_parser.Node{Off: 47},
						},
					},
					Node: query_parser.Node{Off: 21},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where v.ZipCode is nil",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 27},
									},
									query_parser.Segment{
										Value: "ZipCode",
										Node:  query_parser.Node{Off: 29},
									},
								},
								Node: query_parser.Node{Off: 27},
							},
							Node: query_parser.Node{Off: 27},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Is,
							Node: query_parser.Node{Off: 37},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypNil,
							Node: query_parser.Node{Off: 40},
						},
						Node: query_parser.Node{Off: 27},
					},
					Node: query_parser.Node{Off: 21},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where v.ZipCode is not nil",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 27},
									},
									query_parser.Segment{
										Value: "ZipCode",
										Node:  query_parser.Node{Off: 29},
									},
								},
								Node: query_parser.Node{Off: 27},
							},
							Node: query_parser.Node{Off: 27},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.IsNot,
							Node: query_parser.Node{Off: 37},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypNil,
							Node: query_parser.Node{Off: 44},
						},
						Node: query_parser.Node{Off: 27},
					},
					Node: query_parser.Node{Off: 21},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where v.Value = false",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 27},
									},
									query_parser.Segment{
										Value: "Value",
										Node:  query_parser.Node{Off: 29},
									},
								},
								Node: query_parser.Node{Off: 27},
							},
							Node: query_parser.Node{Off: 27},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 35},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypBool,
							Bool: false,
							Node: query_parser.Node{Off: 37},
						},
						Node: query_parser.Node{Off: 27},
					},
					Node: query_parser.Node{Off: 21},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where v.Value equal -42",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 27},
									},
									query_parser.Segment{
										Value: "Value",
										Node:  query_parser.Node{Off: 29},
									},
								},
								Node: query_parser.Node{Off: 27},
							},
							Node: query_parser.Node{Off: 27},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 35},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypInt,
							Int:  -42,
							Node: query_parser.Node{Off: 41},
						},
						Node: query_parser.Node{Off: 27},
					},
					Node: query_parser.Node{Off: 21},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where v.Value equal -18.888",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 27},
									},
									query_parser.Segment{
										Value: "Value",
										Node:  query_parser.Node{Off: 29},
									},
								},
								Node: query_parser.Node{Off: 27},
							},
							Node: query_parser.Node{Off: 27},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 35},
						},
						Operand2: &query_parser.Operand{
							Type:  query_parser.TypFloat,
							Float: -18.888,
							Node:  query_parser.Node{Off: 41},
						},
						Node: query_parser.Node{Off: 27},
					},
					Node: query_parser.Node{Off: 21},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from y where b = 'c'",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "y",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "b",
										Node:  query_parser.Node{Off: 20},
									},
								},
								Node: query_parser.Node{Off: 20},
							},
							Node: query_parser.Node{Off: 20},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 22},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypInt,
							Int:  'c',
							Node: query_parser.Node{Off: 24},
						},
						Node: query_parser.Node{Off: 20},
					},
					Node: query_parser.Node{Off: 14},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from y where b = 'c' limit 10",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "y",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "b",
										Node:  query_parser.Node{Off: 20},
									},
								},
								Node: query_parser.Node{Off: 20},
							},
							Node: query_parser.Node{Off: 20},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 22},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypInt,
							Int:  'c',
							Node: query_parser.Node{Off: 24},
						},
						Node: query_parser.Node{Off: 20},
					},
					Node: query_parser.Node{Off: 14},
				},
				Limit: &query_parser.LimitClause{
					Limit: &query_parser.Int64Value{
						Value: 10,
						Node:  query_parser.Node{Off: 34},
					},
					Node: query_parser.Node{Off: 28},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where k like \"Foo^%Bar\" escape '^'",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "k",
										Node:  query_parser.Node{Off: 27},
									},
								},
								Node: query_parser.Node{Off: 27},
							},
							Node: query_parser.Node{Off: 27},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Like,
							Node: query_parser.Node{Off: 29},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypStr,
							Str:  "Foo^%Bar",
							Node: query_parser.Node{Off: 34},
						},
						Node: query_parser.Node{Off: 27},
					},
					Node: query_parser.Node{Off: 21},
				},
				Escape: &query_parser.EscapeClause{
					EscapeChar: &query_parser.CharValue{
						Value: '^',
						Node:  query_parser.Node{Off: 52},
					},
					Node: query_parser.Node{Off: 45},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where a.b.c = \"baz\" and d.e.f like \"%foobarbaz\"",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "a",
												Node:  query_parser.Node{Off: 27},
											},
											query_parser.Segment{
												Value: "b",
												Node:  query_parser.Node{Off: 29},
											},
											query_parser.Segment{
												Value: "c",
												Node:  query_parser.Node{Off: 31},
											},
										},
										Node: query_parser.Node{Off: 27},
									},
									Node: query_parser.Node{Off: 27},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 33},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypStr,
									Str:  "baz",
									Node: query_parser.Node{Off: 35},
								},
								Node: query_parser.Node{Off: 27},
							},
							Node: query_parser.Node{Off: 27},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.And,
							Node: query_parser.Node{Off: 41},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "d",
												Node:  query_parser.Node{Off: 45},
											},
											query_parser.Segment{
												Value: "e",
												Node:  query_parser.Node{Off: 47},
											},
											query_parser.Segment{
												Value: "f",
												Node:  query_parser.Node{Off: 49},
											},
										},
										Node: query_parser.Node{Off: 45},
									},
									Node: query_parser.Node{Off: 45},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Like,
									Node: query_parser.Node{Off: 51},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypStr,
									Str:  "%foobarbaz",
									Node: query_parser.Node{Off: 56},
								},
								Node: query_parser.Node{Off: 45},
							},
							Node: query_parser.Node{Off: 45},
						},
						Node: query_parser.Node{Off: 27},
					},
					Node: query_parser.Node{Off: 21},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where CustRecord.CustID=123 or CustRecord.Name like \"f%\"",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "CustRecord",
												Node:  query_parser.Node{Off: 27},
											},
											query_parser.Segment{
												Value: "CustID",
												Node:  query_parser.Node{Off: 38},
											},
										},
										Node: query_parser.Node{Off: 27},
									},
									Node: query_parser.Node{Off: 27},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 44},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypInt,
									Int:  123,
									Node: query_parser.Node{Off: 45},
								},
								Node: query_parser.Node{Off: 27},
							},
							Node: query_parser.Node{Off: 27},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Or,
							Node: query_parser.Node{Off: 49},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "CustRecord",
												Node:  query_parser.Node{Off: 52},
											},
											query_parser.Segment{
												Value: "Name",
												Node:  query_parser.Node{Off: 63},
											},
										},
										Node: query_parser.Node{Off: 52},
									},
									Node: query_parser.Node{Off: 52},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Like,
									Node: query_parser.Node{Off: 68},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypStr,
									Str:  "f%",
									Node: query_parser.Node{Off: 73},
								},
								Node: query_parser.Node{Off: 52},
							},
							Node: query_parser.Node{Off: 52},
						},
						Node: query_parser.Node{Off: 27},
					},
					Node: query_parser.Node{Off: 21},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where A=123 or B=456 and C=789",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "A",
														Node:  query_parser.Node{Off: 27},
													},
												},
												Node: query_parser.Node{Off: 27},
											},
											Node: query_parser.Node{Off: 27},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 28},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  123,
											Node: query_parser.Node{Off: 29},
										},
										Node: query_parser.Node{Off: 27},
									},
									Node: query_parser.Node{Off: 27},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Or,
									Node: query_parser.Node{Off: 33},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "B",
														Node:  query_parser.Node{Off: 36},
													},
												},
												Node: query_parser.Node{Off: 36},
											},
											Node: query_parser.Node{Off: 36},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 37},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  456,
											Node: query_parser.Node{Off: 38},
										},
										Node: query_parser.Node{Off: 36},
									},
									Node: query_parser.Node{Off: 36},
								},
								Node: query_parser.Node{Off: 27},
							},
							Node: query_parser.Node{Off: 27},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.And,
							Node: query_parser.Node{Off: 42},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "C",
												Node:  query_parser.Node{Off: 46},
											},
										},
										Node: query_parser.Node{Off: 46},
									},
									Node: query_parser.Node{Off: 46},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 47},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypInt,
									Int:  789,
									Node: query_parser.Node{Off: 48},
								},
								Node: query_parser.Node{Off: 46},
							},
							Node: query_parser.Node{Off: 46},
						},
						Node: query_parser.Node{Off: 27},
					},
					Node: query_parser.Node{Off: 21},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where (A=123 or B=456) and C=789",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "A",
														Node:  query_parser.Node{Off: 28},
													},
												},
												Node: query_parser.Node{Off: 28},
											},
											Node: query_parser.Node{Off: 28},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 29},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  123,
											Node: query_parser.Node{Off: 30},
										},
										Node: query_parser.Node{Off: 28},
									},
									Node: query_parser.Node{Off: 28},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Or,
									Node: query_parser.Node{Off: 34},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "B",
														Node:  query_parser.Node{Off: 37},
													},
												},
												Node: query_parser.Node{Off: 37},
											},
											Node: query_parser.Node{Off: 37},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 38},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  456,
											Node: query_parser.Node{Off: 39},
										},
										Node: query_parser.Node{Off: 37},
									},
									Node: query_parser.Node{Off: 37},
								},
								Node: query_parser.Node{Off: 28},
							},
							Node: query_parser.Node{Off: 28},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.And,
							Node: query_parser.Node{Off: 44},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "C",
												Node:  query_parser.Node{Off: 48},
											},
										},
										Node: query_parser.Node{Off: 48},
									},
									Node: query_parser.Node{Off: 48},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 49},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypInt,
									Int:  789,
									Node: query_parser.Node{Off: 50},
								},
								Node: query_parser.Node{Off: 48},
							},
							Node: query_parser.Node{Off: 48},
						},
						Node: query_parser.Node{Off: 28},
					},
					Node: query_parser.Node{Off: 21},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where (A<=123 or B>456) and C>=789",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "A",
														Node:  query_parser.Node{Off: 28},
													},
												},
												Node: query_parser.Node{Off: 28},
											},
											Node: query_parser.Node{Off: 28},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.LessThanOrEqual,
											Node: query_parser.Node{Off: 29},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  123,
											Node: query_parser.Node{Off: 31},
										},
										Node: query_parser.Node{Off: 28},
									},
									Node: query_parser.Node{Off: 28},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Or,
									Node: query_parser.Node{Off: 35},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "B",
														Node:  query_parser.Node{Off: 38},
													},
												},
												Node: query_parser.Node{Off: 38},
											},
											Node: query_parser.Node{Off: 38},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.GreaterThan,
											Node: query_parser.Node{Off: 39},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  456,
											Node: query_parser.Node{Off: 40},
										},
										Node: query_parser.Node{Off: 38},
									},
									Node: query_parser.Node{Off: 38},
								},
								Node: query_parser.Node{Off: 28},
							},
							Node: query_parser.Node{Off: 28},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.And,
							Node: query_parser.Node{Off: 45},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "C",
												Node:  query_parser.Node{Off: 49},
											},
										},
										Node: query_parser.Node{Off: 49},
									},
									Node: query_parser.Node{Off: 49},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.GreaterThanOrEqual,
									Node: query_parser.Node{Off: 50},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypInt,
									Int:  789,
									Node: query_parser.Node{Off: 52},
								},
								Node: query_parser.Node{Off: 49},
							},
							Node: query_parser.Node{Off: 49},
						},
						Node: query_parser.Node{Off: 28},
					},
					Node: query_parser.Node{Off: 21},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where A=123 or (B=456 and C=789)",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "A",
												Node:  query_parser.Node{Off: 27},
											},
										},
										Node: query_parser.Node{Off: 27},
									},
									Node: query_parser.Node{Off: 27},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 28},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypInt,
									Int:  123,
									Node: query_parser.Node{Off: 29},
								},
								Node: query_parser.Node{Off: 27},
							},
							Node: query_parser.Node{Off: 27},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Or,
							Node: query_parser.Node{Off: 33},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "B",
														Node:  query_parser.Node{Off: 37},
													},
												},
												Node: query_parser.Node{Off: 37},
											},
											Node: query_parser.Node{Off: 37},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 38},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  456,
											Node: query_parser.Node{Off: 39},
										},
										Node: query_parser.Node{Off: 37},
									},
									Node: query_parser.Node{Off: 37},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.And,
									Node: query_parser.Node{Off: 43},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "C",
														Node:  query_parser.Node{Off: 47},
													},
												},
												Node: query_parser.Node{Off: 47},
											},
											Node: query_parser.Node{Off: 47},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 48},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  789,
											Node: query_parser.Node{Off: 49},
										},
										Node: query_parser.Node{Off: 47},
									},
									Node: query_parser.Node{Off: 47},
								},
								Node: query_parser.Node{Off: 37},
							},
							Node: query_parser.Node{Off: 37},
						},
						Node: query_parser.Node{Off: 27},
					},
					Node: query_parser.Node{Off: 21},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where (A=123) or ((B=456) and (C=789))",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "A",
												Node:  query_parser.Node{Off: 28},
											},
										},
										Node: query_parser.Node{Off: 28},
									},
									Node: query_parser.Node{Off: 28},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 29},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypInt,
									Int:  123,
									Node: query_parser.Node{Off: 30},
								},
								Node: query_parser.Node{Off: 28},
							},
							Node: query_parser.Node{Off: 28},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Or,
							Node: query_parser.Node{Off: 35},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "B",
														Node:  query_parser.Node{Off: 40},
													},
												},
												Node: query_parser.Node{Off: 40},
											},
											Node: query_parser.Node{Off: 40},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 41},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  456,
											Node: query_parser.Node{Off: 42},
										},
										Node: query_parser.Node{Off: 40},
									},
									Node: query_parser.Node{Off: 40},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.And,
									Node: query_parser.Node{Off: 47},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "C",
														Node:  query_parser.Node{Off: 52},
													},
												},
												Node: query_parser.Node{Off: 52},
											},
											Node: query_parser.Node{Off: 52},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.Equal,
											Node: query_parser.Node{Off: 53},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  789,
											Node: query_parser.Node{Off: 54},
										},
										Node: query_parser.Node{Off: 52},
									},
									Node: query_parser.Node{Off: 52},
								},
								Node: query_parser.Node{Off: 40},
							},
							Node: query_parser.Node{Off: 40},
						},
						Node: query_parser.Node{Off: 28},
					},
					Node: query_parser.Node{Off: 21},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where A<>123 or B not equal 456 and C not like \"abc%\"",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "A",
														Node:  query_parser.Node{Off: 27},
													},
												},
												Node: query_parser.Node{Off: 27},
											},
											Node: query_parser.Node{Off: 27},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.NotEqual,
											Node: query_parser.Node{Off: 28},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  123,
											Node: query_parser.Node{Off: 30},
										},
										Node: query_parser.Node{Off: 27},
									},
									Node: query_parser.Node{Off: 27},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Or,
									Node: query_parser.Node{Off: 34},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypExpr,
									Expr: &query_parser.Expression{
										Operand1: &query_parser.Operand{
											Type: query_parser.TypField,
											Column: &query_parser.Field{
												Segments: []query_parser.Segment{
													query_parser.Segment{
														Value: "B",
														Node:  query_parser.Node{Off: 37},
													},
												},
												Node: query_parser.Node{Off: 37},
											},
											Node: query_parser.Node{Off: 37},
										},
										Operator: &query_parser.BinaryOperator{
											Type: query_parser.NotEqual,
											Node: query_parser.Node{Off: 39},
										},
										Operand2: &query_parser.Operand{
											Type: query_parser.TypInt,
											Int:  456,
											Node: query_parser.Node{Off: 49},
										},
										Node: query_parser.Node{Off: 37},
									},
									Node: query_parser.Node{Off: 37},
								},
								Node: query_parser.Node{Off: 27},
							},
							Node: query_parser.Node{Off: 27},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.And,
							Node: query_parser.Node{Off: 53},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypField,
									Column: &query_parser.Field{
										Segments: []query_parser.Segment{
											query_parser.Segment{
												Value: "C",
												Node:  query_parser.Node{Off: 57},
											},
										},
										Node: query_parser.Node{Off: 57},
									},
									Node: query_parser.Node{Off: 57},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.NotLike,
									Node: query_parser.Node{Off: 59},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypStr,
									Str:  "abc%",
									Node: query_parser.Node{Off: 68},
								},
								Node: query_parser.Node{Off: 57},
							},
							Node: query_parser.Node{Off: 57},
						},
						Node: query_parser.Node{Off: 27},
					},
					Node: query_parser.Node{Off: 21},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where Now() < Time(\"2015/07/22\") and Foo(10,20.1,v.Bar) = true",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypFunction,
									Function: &query_parser.Function{
										Name: "Now",
										Args: nil,
										Node: query_parser.Node{Off: 27},
									},
									Node: query_parser.Node{Off: 27},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.LessThan,
									Node: query_parser.Node{Off: 33},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypFunction,
									Function: &query_parser.Function{
										Name: "Time",
										Args: []*query_parser.Operand{
											&query_parser.Operand{
												Type: query_parser.TypStr,
												Str:  "2015/07/22",
												Node: query_parser.Node{Off: 40},
											},
										},
										Node: query_parser.Node{Off: 35},
									},
									Node: query_parser.Node{Off: 35},
								},
								Node: query_parser.Node{Off: 27},
							},
							Node: query_parser.Node{Off: 27},
						},
						Node: query_parser.Node{Off: 27},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.And,
							Node: query_parser.Node{Off: 54},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypFunction,
									Function: &query_parser.Function{
										Name: "Foo",
										Args: []*query_parser.Operand{
											&query_parser.Operand{
												Type: query_parser.TypInt,
												Int:  10,
												Node: query_parser.Node{Off: 62},
											},
											&query_parser.Operand{
												Type:  query_parser.TypFloat,
												Float: 20.1,
												Node:  query_parser.Node{Off: 65},
											},
											&query_parser.Operand{
												Type: query_parser.TypField,
												Column: &query_parser.Field{
													Segments: []query_parser.Segment{
														query_parser.Segment{
															Value: "v",
															Node:  query_parser.Node{Off: 70},
														},
														query_parser.Segment{
															Value: "Bar",
															Node:  query_parser.Node{Off: 72},
														},
													},
													Node: query_parser.Node{Off: 70},
												},
												Node: query_parser.Node{Off: 70},
											},
										},
										Node: query_parser.Node{Off: 58},
									},
									Node: query_parser.Node{Off: 58},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 77},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypBool,
									Bool: true,
									Node: query_parser.Node{Off: 79},
								},
								Node: query_parser.Node{Off: 58},
							},
							Node: query_parser.Node{Off: 58},
						},
					},
					Node: query_parser.Node{Off: 21},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
		{
			"delete from Customer where v.Foo[v.Bar] = \"abc\"",
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 27},
									},
									query_parser.Segment{
										Value: "Foo",
										Keys: []*query_parser.Operand{
											&query_parser.Operand{
												Type: query_parser.TypField,
												Column: &query_parser.Field{
													Segments: []query_parser.Segment{
														query_parser.Segment{
															Value: "v",
															Node:  query_parser.Node{Off: 33},
														},
														query_parser.Segment{
															Value: "Bar",
															Node:  query_parser.Node{Off: 35},
														},
													},
													Node: query_parser.Node{Off: 33},
												},
												Node: query_parser.Node{Off: 33},
											},
										},
										Node: query_parser.Node{Off: 29},
									},
								},
								Node: query_parser.Node{Off: 27},
							},
							Node: query_parser.Node{Off: 27},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 40},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypStr,
							Str:  "abc",
							Node: query_parser.Node{Off: 42},
						},
						Node: query_parser.Node{Off: 27},
					},
					Node: query_parser.Node{Off: 21},
				},
				Node: query_parser.Node{Off: 0},
			},
			nil,
		},
	}

	for _, test := range basic {
		st, err := query_parser.Parse(&db, test.query)
		if err != nil {
			t.Errorf("query: %s; unexpected error: got %v, want nil", test.query, err)
		}
		switch (*st).(type) {
		case query_parser.DeleteStatement:
			if !reflect.DeepEqual(test.statement, *st) {
				t.Errorf("query: %s;\nGOT  %s\nWANT %s", test.query, *st, test.statement)
			}
		}
	}
}

func TestQueryParserErrors(t *testing.T) {
	basic := []parseErrorTest{
		{"", syncql.NewErrNoStatementFound(db.GetContext(), 0)},
		{";", syncql.NewErrExpectedIdentifier(db.GetContext(), 0, ";")},
		{"foo", syncql.NewErrUnknownIdentifier(db.GetContext(), 0, "foo")},
		{"(foo)", syncql.NewErrExpectedIdentifier(db.GetContext(), 0, "(")},
		{"create table Customer (CustRecord cust_pkg.Cust, primary key(CustRecord.CustID))", syncql.NewErrUnknownIdentifier(db.GetContext(), 0, "create")},

		// Select
		{"select foo.", syncql.NewErrExpectedIdentifier(db.GetContext(), 11, "")},
		{"select foo. from a", syncql.NewErrExpectedFrom(db.GetContext(), 17, "a")},
		{"select (foo)", syncql.NewErrExpectedIdentifier(db.GetContext(), 7, "(")},
		{"select from where", syncql.NewErrExpectedFrom(db.GetContext(), 12, "where")},
		{"select foo from Customer where (A=123 or B=456) and C=789)", syncql.NewErrUnexpected(db.GetContext(), 57, ")")},
		{"select foo from Customer where ((A=123 or B=456) and C=789", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 58)},
		{"select foo from Customer where (((((A=123 or B=456 and C=789))))", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 64)},
		{"select foo from Customer where (A=123 or B=456) and C=789)))))", syncql.NewErrUnexpected(db.GetContext(), 57, ")")},
		{"select foo from Customer where", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 30)},
		{"select foo from Customer where ", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 31)},
		{"select foo from Customer where )", syncql.NewErrExpectedOperand(db.GetContext(), 31, ")")},
		{"select foo from Customer where )A=123 or B=456) and C=789", syncql.NewErrExpectedOperand(db.GetContext(), 31, ")")},
		{"select foo from Customer where ()A=123 or B=456) and C=789", syncql.NewErrExpectedOperand(db.GetContext(), 32, ")")},
		{"select foo from Customer where (A=123 or B=456) and C=789)", syncql.NewErrUnexpected(db.GetContext(), 57, ")")},
		{"select foo bar from Customer", syncql.NewErrExpectedFrom(db.GetContext(), 11, "bar")},
		{"select foo from Customer Invoice", syncql.NewErrUnexpected(db.GetContext(), 25, "Invoice")},
		{"select (foo) from (Customer)", syncql.NewErrExpectedIdentifier(db.GetContext(), 7, "(")},
		{"select foo, bar from Customer where a = (b)", syncql.NewErrExpectedOperand(db.GetContext(), 40, "(")},
		{"select foo, bar from Customer where a = b and (c) = d", syncql.NewErrExpectedOperator(db.GetContext(), 48, ")")},
		{"select foo, bar from Customer where a = b and c =", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 49)},
		{"select foo, bar from Customer where a = ", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 40)},
		{"select foo, bar from Customer where a", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 37)},
		{"select", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 6)},
		{"select a from", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 13)},
		{"select a from b where c = d and e =", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 35)},
		{"select a from b where c = d and f", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 33)},
		{"select a from b where c = d and f *", syncql.NewErrExpectedOperator(db.GetContext(), 34, "*")},
		{"select a from b where c <", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 25)},
		{"select a from b where c not", syncql.NewErrExpected(db.GetContext(), 27, "'equal' or 'like'")},
		{"select a from b where c not 8", syncql.NewErrExpected(db.GetContext(), 28, "'equal' or 'like'")},
		{"select x from y where a and b = c", syncql.NewErrExpectedOperator(db.GetContext(), 24, "and")},
		{"select v from Customer limit 100 offset a", syncql.NewErrExpected(db.GetContext(), 40, "positive integer literal")},
		{"select v from Customer limit -100 offset 5", syncql.NewErrExpected(db.GetContext(), 29, "positive integer literal")},
		{"select v from Customer limit 100 offset -5", syncql.NewErrExpected(db.GetContext(), 40, "positive integer literal")},
		{"select v from Customer limit a offset 200", syncql.NewErrExpected(db.GetContext(), 29, "positive integer literal")},
		{"select v from Customer limit", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 28)},
		{"select v from Customer, Invoice", syncql.NewErrUnexpected(db.GetContext(), 22, ",")},
		{"select v from Customer As Cust where foo = bar", syncql.NewErrUnexpected(db.GetContext(), 23, "As")},
		{"select 1abc from Customer where foo = bar", syncql.NewErrExpectedIdentifier(db.GetContext(), 7, "1")},
		{"select v from Customer where Foo(1,) = true", syncql.NewErrExpectedOperand(db.GetContext(), 35, ")")},
		{"select v from Customer where Foo(,1) = true", syncql.NewErrExpectedOperand(db.GetContext(), 33, ",")},
		{"select v from Customer where Foo(1, 2.0 = true", syncql.NewErrUnexpected(db.GetContext(), 40, "=")},
		{"select v from Customer where Foo(1, 2.0 limit 100", syncql.NewErrUnexpected(db.GetContext(), 40, "limit")},
		{"select v from Customer where v is", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 33)},
		{"select v from Customer where v = 1.0 is k = \"abc\"", syncql.NewErrUnexpected(db.GetContext(), 37, "is")},
		{"select v as from Customer", syncql.NewErrExpectedFrom(db.GetContext(), 17, "Customer")},
		{"select v from Customer where v.Foo = \"", syncql.NewErrExpectedOperand(db.GetContext(), 37, "\"")},
		{"select v from Customer where v.Foo = \"Bar", syncql.NewErrExpectedOperand(db.GetContext(), 37, "\"Bar")},
		{"select v from Customer where v.Foo = '", syncql.NewErrExpectedOperand(db.GetContext(), 37, "'")},
		{"select v from Customer where v.Foo = 'a", syncql.NewErrExpectedOperand(db.GetContext(), 37, "'a")},
		{"select v from Customer where v.Foo = 'abc", syncql.NewErrExpectedOperand(db.GetContext(), 37, "'abc")},
		{"select v from Customer where v.Foo = 'abc'", syncql.NewErrExpectedOperand(db.GetContext(), 37, "'abc'")},
		{"select v from Customer where v.Foo = 10.10.10", syncql.NewErrUnexpected(db.GetContext(), 42, ".10")},
		// Parameters can only appear as operands in the where clause
		{"select ? from Customer", syncql.NewErrExpectedIdentifier(db.GetContext(), 7, "?")},
		// Parameters can only appear as operands in the where clause
		{"select v from Customer where k ? v", syncql.NewErrExpectedOperator(db.GetContext(), 31, "?")},
		{"select a,", syncql.NewErrExpectedIdentifier(db.GetContext(), 9, "")},
		{"select a from b escape", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 22)},
		{"select a from b escape 123", syncql.NewErrExpected(db.GetContext(), 23, "char literal")},
		{"select v[1](foo) from b", syncql.NewErrUnexpected(db.GetContext(), 11, "(")},
		{"select v[1] as 123 from b", syncql.NewErrExpectedIdentifier(db.GetContext(), 15, "123")},
		{"select v.", syncql.NewErrExpectedIdentifier(db.GetContext(), 9, "")},
		{"select Foo(abc def) from Customers", syncql.NewErrUnexpected(db.GetContext(), 15, "def")},
		{"select v.abc.\"8\" from Customers", syncql.NewErrExpectedIdentifier(db.GetContext(), 13, "8")},
		{"select v[abc from Customers", syncql.NewErrExpected(db.GetContext(), 13, "]")},
		{"select v[* from Customers", syncql.NewErrExpectedOperand(db.GetContext(), 9, "*")},
		{"select v from 123", syncql.NewErrExpectedIdentifier(db.GetContext(), 14, "123")},
		{"select v from Customers where (a = b *", syncql.NewErrExpected(db.GetContext(), 37, ")")},
		// Delete
		{"delete.", syncql.NewErrExpectedFrom(db.GetContext(), 6, ".")},
		{"delete. from a", syncql.NewErrExpectedFrom(db.GetContext(), 6, ".")},
		{"delete ", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 7)},
		{"delete from Customer where (A=123 or B=456) and C=789)", syncql.NewErrUnexpected(db.GetContext(), 53, ")")},
		{"delete from Customer where ((A=123 or B=456) and C=789", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 54)},
		{"delete from Customer where (((((A=123 or B=456 and C=789))))", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 60)},
		{"delete from Customer where (A=123 or B=456) and C=789)))))", syncql.NewErrUnexpected(db.GetContext(), 53, ")")},
		{"delete from Customer where", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 26)},
		{"delete from Customer where ", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 27)},
		{"delete from Customer where )", syncql.NewErrExpectedOperand(db.GetContext(), 27, ")")},
		{"delete from Customer where )A=123 or B=456) and C=789", syncql.NewErrExpectedOperand(db.GetContext(), 27, ")")},
		{"delete from Customer where ()A=123 or B=456) and C=789", syncql.NewErrExpectedOperand(db.GetContext(), 28, ")")},
		{"delete from Customer where (A=123 or B=456) and C=789)", syncql.NewErrUnexpected(db.GetContext(), 53, ")")},
		{"delete bar from Customer", syncql.NewErrExpectedFrom(db.GetContext(), 7, "bar")},
		{"delete from Customer Invoice", syncql.NewErrUnexpected(db.GetContext(), 21, "Invoice")},
		{"delete from (Customer)", syncql.NewErrExpectedIdentifier(db.GetContext(), 12, "(")},
		{"delete from Customer where a = (b)", syncql.NewErrExpectedOperand(db.GetContext(), 31, "(")},
		{"delete from Customer where a = b and (c) = d", syncql.NewErrExpectedOperator(db.GetContext(), 39, ")")},
		{"delete from Customer where a = b and c =", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 40)},
		{"delete from Customer where a = ", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 31)},
		{"delete from Customer where a", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 28)},
		{"delete", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 6)},
		{"delete from", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 11)},
		{"delete from b where c = d and e =", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 33)},
		{"delete from b where c = d and f", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 31)},
		{"delete from b where c = d and f *", syncql.NewErrExpectedOperator(db.GetContext(), 32, "*")},
		{"delete from b where c <", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 23)},
		{"delete from b where c not", syncql.NewErrExpected(db.GetContext(), 25, "'equal' or 'like'")},
		{"delete from b where c not 8", syncql.NewErrExpected(db.GetContext(), 26, "'equal' or 'like'")},
		{"delete from y where a and b = c", syncql.NewErrExpectedOperator(db.GetContext(), 22, "and")},
		{"delete from Customer limit a", syncql.NewErrExpected(db.GetContext(), 27, "positive integer literal")},
		{"delete from Customer limit -100", syncql.NewErrExpected(db.GetContext(), 27, "positive integer literal")},
		{"delete from Customer limit", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 26)},
		{"delete from Customer, Invoice", syncql.NewErrUnexpected(db.GetContext(), 20, ",")},
		{"delete from Customer As Cust where foo = bar", syncql.NewErrUnexpected(db.GetContext(), 21, "As")},
		{"delete from Customer where Foo(1,) = true", syncql.NewErrExpectedOperand(db.GetContext(), 33, ")")},
		{"delete from Customer where Foo(,1) = true", syncql.NewErrExpectedOperand(db.GetContext(), 31, ",")},
		{"delete from Customer where Foo(1, 2.0 = true", syncql.NewErrUnexpected(db.GetContext(), 38, "=")},
		{"delete from Customer where Foo(1, 2.0 limit 100", syncql.NewErrUnexpected(db.GetContext(), 38, "limit")},
		{"delete from Customer where v is", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 31)},
		{"delete from Customer where v = 1.0 is k = \"abc\"", syncql.NewErrUnexpected(db.GetContext(), 35, "is")},
		{"delete as from Customer", syncql.NewErrExpectedFrom(db.GetContext(), 7, "as")},
		{"delete from Customer where v.Foo = \"", syncql.NewErrExpectedOperand(db.GetContext(), 35, "\"")},
		{"delete from Customer where v.Foo = \"Bar", syncql.NewErrExpectedOperand(db.GetContext(), 35, "\"Bar")},
		{"delete from Customer where v.Foo = '", syncql.NewErrExpectedOperand(db.GetContext(), 35, "'")},
		{"delete from Customer where v.Foo = 'a", syncql.NewErrExpectedOperand(db.GetContext(), 35, "'a")},
		{"delete from Customer where v.Foo = 'abc", syncql.NewErrExpectedOperand(db.GetContext(), 35, "'abc")},
		{"delete from Customer where v.Foo = 'abc'", syncql.NewErrExpectedOperand(db.GetContext(), 35, "'abc'")},
		{"delete from Customer where v.Foo = 10.10.10", syncql.NewErrUnexpected(db.GetContext(), 40, ".10")},
		// Parameters can only appear as operands in the where clause
		{"delete ? from Customer", syncql.NewErrExpectedFrom(db.GetContext(), 7, "?")},
		// Parameters can only appear as operands in the where clause
		{"delete from Customer where k ? v", syncql.NewErrExpectedOperator(db.GetContext(), 29, "?")},
		{"delete from b escape", syncql.NewErrUnexpectedEndOfStatement(db.GetContext(), 20)},
		{"delete from b escape 123", syncql.NewErrExpected(db.GetContext(), 21, "char literal")},
		{"delete from b where v[1](foo) = 1", syncql.NewErrUnexpected(db.GetContext(), 24, "(")},
		{"delete from Customers where Foo(abc def) = 1", syncql.NewErrUnexpected(db.GetContext(), 36, "def")},
		{"delete from Customers where v.abc.\"8\" = 1", syncql.NewErrExpectedIdentifier(db.GetContext(), 34, "8")},
		{"delete from Customers where v[abc = 1", syncql.NewErrExpected(db.GetContext(), 34, "]")},
		{"delete from Customers where v[*", syncql.NewErrExpectedOperand(db.GetContext(), 30, "*")},
		{"delete from 123", syncql.NewErrExpectedIdentifier(db.GetContext(), 12, "123")},
		{"delete from Customers where (a = b *", syncql.NewErrExpected(db.GetContext(), 35, ")")},
	}

	for _, test := range basic {
		_, err := query_parser.Parse(&db, test.query)
		// Test both that the IDs compare and the text compares (since the offset needs to match).
		if verror.ErrorID(err) != verror.ErrorID(test.err) || err.Error() != test.err.Error() {
			t.Errorf("query: %s; got %v, want %v", test.query, err, test.err)
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
		st, err := query_parser.Parse(&db, test.query)
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
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 29},
									},
									query_parser.Segment{
										Value: "Value",
										Node:  query_parser.Node{Off: 31},
									},
								},
								Node: query_parser.Node{Off: 29},
							},
							Node: query_parser.Node{Off: 29},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 37},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypInt,
							Int:  10,
							Node: query_parser.Node{Off: 39},
						},
						Node: query_parser.Node{Off: 29},
					},
					Node: query_parser.Node{Off: 23},
				},
				Node: query_parser.Node{Off: 0},
			},
		},
		{
			"select v from Customer where v.Value = ?",
			[]*vdl.Value{vdl.ValueOf(true)},
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 29},
									},
									query_parser.Segment{
										Value: "Value",
										Node:  query_parser.Node{Off: 31},
									},
								},
								Node: query_parser.Node{Off: 29},
							},
							Node: query_parser.Node{Off: 29},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 37},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypBool,
							Bool: true,
							Node: query_parser.Node{Off: 39},
						},
						Node: query_parser.Node{Off: 29},
					},
					Node: query_parser.Node{Off: 23},
				},
				Node: query_parser.Node{Off: 0},
			},
		},
		{
			"select v from Customer where Now() < Time(?) and Foo(10,?,v.Bar) = true",
			[]*vdl.Value{vdl.ValueOf("abc"), vdl.ValueOf(42.0)},
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypFunction,
									Function: &query_parser.Function{
										Name: "Now",
										Args: nil,
										Node: query_parser.Node{Off: 29},
									},
									Node: query_parser.Node{Off: 29},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.LessThan,
									Node: query_parser.Node{Off: 35},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypFunction,
									Function: &query_parser.Function{
										Name: "Time",
										Args: []*query_parser.Operand{
											&query_parser.Operand{
												Type: query_parser.TypStr,
												Str:  "abc",
												Node: query_parser.Node{Off: 42},
											},
										},
										Node: query_parser.Node{Off: 37},
									},
									Node: query_parser.Node{Off: 37},
								},
								Node: query_parser.Node{Off: 29},
							},
							Node: query_parser.Node{Off: 29},
						},
						Node: query_parser.Node{Off: 29},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.And,
							Node: query_parser.Node{Off: 45},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypFunction,
									Function: &query_parser.Function{
										Name: "Foo",
										Args: []*query_parser.Operand{
											&query_parser.Operand{
												Type: query_parser.TypInt,
												Int:  10,
												Node: query_parser.Node{Off: 53},
											},
											&query_parser.Operand{
												Type:  query_parser.TypFloat,
												Float: 42.0,
												Node:  query_parser.Node{Off: 56},
											},
											&query_parser.Operand{
												Type: query_parser.TypField,
												Column: &query_parser.Field{
													Segments: []query_parser.Segment{
														query_parser.Segment{
															Value: "v",
															Node:  query_parser.Node{Off: 58},
														},
														query_parser.Segment{
															Value: "Bar",
															Node:  query_parser.Node{Off: 60},
														},
													},
													Node: query_parser.Node{Off: 58},
												},
												Node: query_parser.Node{Off: 58},
											},
										},
										Node: query_parser.Node{Off: 49},
									},
									Node: query_parser.Node{Off: 49},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 65},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypBool,
									Bool: true,
									Node: query_parser.Node{Off: 67},
								},
								Node: query_parser.Node{Off: 49},
							},
							Node: query_parser.Node{Off: 49},
						},
					},
					Node: query_parser.Node{Off: 23},
				},
				Node: query_parser.Node{Off: 0},
			},
		},
		{
			"select v from Customer where k like ? limit 10 offset 20 escape '^'",
			[]*vdl.Value{vdl.ValueOf("abc^%%")},
			query_parser.SelectStatement{
				Select: &query_parser.SelectClause{
					Selectors: []query_parser.Selector{
						query_parser.Selector{
							Type: query_parser.TypSelField,
							Field: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 7},
									},
								},
								Node: query_parser.Node{Off: 7},
							},
							Node: query_parser.Node{Off: 7},
						},
					},
					Node: query_parser.Node{Off: 0},
				},
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 14},
					},
					Node: query_parser.Node{Off: 9},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "k",
										Node:  query_parser.Node{Off: 29},
									},
								},
								Node: query_parser.Node{Off: 29},
							},
							Node: query_parser.Node{Off: 29},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Like,
							Node: query_parser.Node{Off: 31},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypStr,
							Str:  "abc^%%",
							Node: query_parser.Node{Off: 36},
						},
						Node: query_parser.Node{Off: 29},
					},
					Node: query_parser.Node{Off: 23},
				},
				Limit: &query_parser.LimitClause{
					Limit: &query_parser.Int64Value{
						Value: 10,
						Node:  query_parser.Node{Off: 44},
					},
					Node: query_parser.Node{Off: 38},
				},
				ResultsOffset: &query_parser.ResultsOffsetClause{
					ResultsOffset: &query_parser.Int64Value{
						Value: 20,
						Node:  query_parser.Node{Off: 54},
					},
					Node: query_parser.Node{Off: 47},
				},
				Escape: &query_parser.EscapeClause{
					EscapeChar: &query_parser.CharValue{
						Value: '^',
						Node:  query_parser.Node{Off: 64},
					},
					Node: query_parser.Node{Off: 57},
				},
				Node: query_parser.Node{Off: 0},
			},
		},
	}
	for _, test := range basic {
		st, err := query_parser.Parse(&db, test.query)
		if err != nil {
			t.Errorf("query: %s; unexpected parse error: got %v, want nil", test.query, err)
		}
		st2, err := (*st).CopyAndSubstitute(&db, test.subValues)
		if err != nil {
			t.Errorf("query: %s; unexpected error on st.CopyAndSubstitute: got %v, want nil", test.query, err)
		}
		switch (st2).(type) {
		case query_parser.SelectStatement:
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
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 27},
									},
									query_parser.Segment{
										Value: "Value",
										Node:  query_parser.Node{Off: 29},
									},
								},
								Node: query_parser.Node{Off: 27},
							},
							Node: query_parser.Node{Off: 27},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 35},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypInt,
							Int:  10,
							Node: query_parser.Node{Off: 37},
						},
						Node: query_parser.Node{Off: 27},
					},
					Node: query_parser.Node{Off: 21},
				},
				Node: query_parser.Node{Off: 0},
			},
		},
		{
			"delete from Customer where v.Value = ?",
			[]*vdl.Value{vdl.ValueOf(true)},
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "v",
										Node:  query_parser.Node{Off: 27},
									},
									query_parser.Segment{
										Value: "Value",
										Node:  query_parser.Node{Off: 29},
									},
								},
								Node: query_parser.Node{Off: 27},
							},
							Node: query_parser.Node{Off: 27},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Equal,
							Node: query_parser.Node{Off: 35},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypBool,
							Bool: true,
							Node: query_parser.Node{Off: 37},
						},
						Node: query_parser.Node{Off: 27},
					},
					Node: query_parser.Node{Off: 21},
				},
				Node: query_parser.Node{Off: 0},
			},
		},
		{
			"delete from Customer where Now() < Time(?) and Foo(10,?,v.Bar) = true",
			[]*vdl.Value{vdl.ValueOf("abc"), vdl.ValueOf(42.0)},
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypFunction,
									Function: &query_parser.Function{
										Name: "Now",
										Args: nil,
										Node: query_parser.Node{Off: 27},
									},
									Node: query_parser.Node{Off: 27},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.LessThan,
									Node: query_parser.Node{Off: 33},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypFunction,
									Function: &query_parser.Function{
										Name: "Time",
										Args: []*query_parser.Operand{
											&query_parser.Operand{
												Type: query_parser.TypStr,
												Str:  "abc",
												Node: query_parser.Node{Off: 40},
											},
										},
										Node: query_parser.Node{Off: 35},
									},
									Node: query_parser.Node{Off: 35},
								},
								Node: query_parser.Node{Off: 27},
							},
							Node: query_parser.Node{Off: 27},
						},
						Node: query_parser.Node{Off: 27},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.And,
							Node: query_parser.Node{Off: 43},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypExpr,
							Expr: &query_parser.Expression{
								Operand1: &query_parser.Operand{
									Type: query_parser.TypFunction,
									Function: &query_parser.Function{
										Name: "Foo",
										Args: []*query_parser.Operand{
											&query_parser.Operand{
												Type: query_parser.TypInt,
												Int:  10,
												Node: query_parser.Node{Off: 51},
											},
											&query_parser.Operand{
												Type:  query_parser.TypFloat,
												Float: 42.0,
												Node:  query_parser.Node{Off: 54},
											},
											&query_parser.Operand{
												Type: query_parser.TypField,
												Column: &query_parser.Field{
													Segments: []query_parser.Segment{
														query_parser.Segment{
															Value: "v",
															Node:  query_parser.Node{Off: 56},
														},
														query_parser.Segment{
															Value: "Bar",
															Node:  query_parser.Node{Off: 58},
														},
													},
													Node: query_parser.Node{Off: 56},
												},
												Node: query_parser.Node{Off: 56},
											},
										},
										Node: query_parser.Node{Off: 47},
									},
									Node: query_parser.Node{Off: 47},
								},
								Operator: &query_parser.BinaryOperator{
									Type: query_parser.Equal,
									Node: query_parser.Node{Off: 63},
								},
								Operand2: &query_parser.Operand{
									Type: query_parser.TypBool,
									Bool: true,
									Node: query_parser.Node{Off: 65},
								},
								Node: query_parser.Node{Off: 47},
							},
							Node: query_parser.Node{Off: 47},
						},
					},
					Node: query_parser.Node{Off: 21},
				},
				Node: query_parser.Node{Off: 0},
			},
		},
		{
			"delete from Customer where k like ? limit 10 escape '^'",
			[]*vdl.Value{vdl.ValueOf("abc^%%")},
			query_parser.DeleteStatement{
				From: &query_parser.FromClause{
					Table: query_parser.TableEntry{
						Name: "Customer",
						Node: query_parser.Node{Off: 12},
					},
					Node: query_parser.Node{Off: 7},
				},
				Where: &query_parser.WhereClause{
					Expr: &query_parser.Expression{
						Operand1: &query_parser.Operand{
							Type: query_parser.TypField,
							Column: &query_parser.Field{
								Segments: []query_parser.Segment{
									query_parser.Segment{
										Value: "k",
										Node:  query_parser.Node{Off: 29},
									},
								},
								Node: query_parser.Node{Off: 29},
							},
							Node: query_parser.Node{Off: 29},
						},
						Operator: &query_parser.BinaryOperator{
							Type: query_parser.Like,
							Node: query_parser.Node{Off: 31},
						},
						Operand2: &query_parser.Operand{
							Type: query_parser.TypStr,
							Str:  "abc^%%",
							Node: query_parser.Node{Off: 36},
						},
						Node: query_parser.Node{Off: 29},
					},
					Node: query_parser.Node{Off: 23},
				},
				Limit: &query_parser.LimitClause{
					Limit: &query_parser.Int64Value{
						Value: 10,
						Node:  query_parser.Node{Off: 44},
					},
					Node: query_parser.Node{Off: 38},
				},
				Escape: &query_parser.EscapeClause{
					EscapeChar: &query_parser.CharValue{
						Value: '^',
						Node:  query_parser.Node{Off: 54},
					},
					Node: query_parser.Node{Off: 47},
				},
				Node: query_parser.Node{Off: 0},
			},
		},
	}
	for _, test := range basic {
		st, err := query_parser.Parse(&db, test.query)
		if err != nil {
			t.Errorf("query: %s; unexpected parse error: got %v, want nil", test.query, err)
		}
		st2, err := (*st).CopyAndSubstitute(&db, test.subValues)
		if err != nil {
			t.Errorf("query: %s; unexpected error on st.CopyAndSubstitute: got %v, want nil", test.query, err)
		}
		switch (st2).(type) {
		case query_parser.SelectStatement:
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
			syncql.NewErrTooManyParamValuesSpecified(db.GetContext(), 0),
		},
		{
			"select v from Customers where v.A = 10",
			[]*vdl.Value{vdl.ValueOf(10)},
			syncql.NewErrTooManyParamValuesSpecified(db.GetContext(), 24),
		},
		{
			"select v from Customers where v.A = ? and v.B = ?",
			[]*vdl.Value{vdl.ValueOf(10)},
			syncql.NewErrNotEnoughParamValuesSpecified(db.GetContext(), 48),
		},
		{
			"delete from Customers",
			[]*vdl.Value{vdl.ValueOf(10)},
			syncql.NewErrTooManyParamValuesSpecified(db.GetContext(), 0),
		},
		{
			"delete from Customers where v.A = 10",
			[]*vdl.Value{vdl.ValueOf(10)},
			syncql.NewErrTooManyParamValuesSpecified(db.GetContext(), 22),
		},
		{
			"delete from Customers where v.A = ? and v.B = ?",
			[]*vdl.Value{vdl.ValueOf(10)},
			syncql.NewErrNotEnoughParamValuesSpecified(db.GetContext(), 46),
		},
	}

	for _, test := range basic {
		st, err := query_parser.Parse(&db, test.query)
		if err != nil {
			t.Errorf("query: %s; unexpected parse error: got %v, want nil", test.query, err)
		}
		_, err = (*st).CopyAndSubstitute(&db, test.subValues)
		// Test both that the IDs compare and the text compares (since the offset needs to match).
		if verror.ErrorID(err) != verror.ErrorID(test.err) || err.Error() != test.err.Error() {
			t.Errorf("query: %s; got %v, want %v", test.query, err, test.err)
		}
	}
}

func TestStatementSizeExceeded(t *testing.T) {
	q := fmt.Sprintf("select a from b where c = \"%s\"", strings.Repeat("x", 12000))
	_, err := query_parser.Parse(&db, q)
	expectedErr := syncql.NewErrMaxStatementLenExceeded(db.GetContext(), int64(0), query_parser.MaxStatementLen, int64(len(q)))
	if verror.ErrorID(err) != verror.ErrorID(expectedErr) || err.Error() != expectedErr.Error() {
		t.Errorf("query: %s; got %v, want %v", q, err, expectedErr)
	}
}
