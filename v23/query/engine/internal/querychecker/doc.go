// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package querychecker performs a semantic check on an AST produced
// by the queryparser package.
//
// For the foreseeasble future, only SelectStatements are supported.
// The following clauses are checked sequentially,
// SelectClause
// FromClause
// WhereClause (optional)
// LimitClause (optional)
// ResultsOffsetClause (optional)
package querychecker
