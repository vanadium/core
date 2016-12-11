// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The query packages implement Vanadium's query capabilities.
//
// The syncql package is used by clients of a Vanadium
// component that supports queries.  It includes: ResultStream
// (which is the product of executing a query), the error messages
// that could be returned from performing a query, and a function
// to parse an error to get the offset into a query that
// caused the error.  syncql does not include the Exec function
// as that must be provided by the component that
// supports queries.
//
// The engine package implements the query engine. A component
// that supports queries calls the Exec function in this package.
//
// The datasource package contains the interfaces that a
// Vanadium component must implement in order to
// use the query engine.
//
// The internal package provides a reference implementation of the
// queries package.
// Its sole client is the engine package.
package query
