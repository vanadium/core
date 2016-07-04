// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package filter contains filter implementations for matching collections and
// rows within them.
package filter

import (
	wire "v.io/v23/services/syncbase"
)

const pkgPath = "v.io/x/ref/services/syncbase/server/filter"

// CollectionRowFilter specifies a filter on collections and rows within them.
type CollectionRowFilter interface {
	// CollectionMatches should return true iff the specified collection matches
	// the filter.
	CollectionMatches(cxId wire.Id) bool

	// RowMatches should return true iff the specified row in the specified
	// collection matches the filter. It must return false if CollectionMatches
	// returns false for the specified collection.
	// TODO(ivanpi): Match might require passing in value, e.g. for query filters.
	RowMatches(cxId wire.Id, key string) bool
}
