// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package filter

import (
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
)

var (
	errNoPatternsSpecified = verror.Register(pkgPath+".errNoPatternsSpecified", verror.NoRetry, "{1:}{2:} at least one CollectionRowPattern must be specified")
)

// multiPatternFilter matches collections and rows matching at least one of the
// contained patternFilters.
// At least one pattern must be specified. Collection blessing and name patterns
// are not allowed to be empty, but the row key pattern is (for matching only
// collections and no rows).
type multiPatternFilter struct {
	patterns []*patternFilter
}

var _ CollectionRowFilter = (*patternFilter)(nil)

func NewMultiPatternFilter(wps []wire.CollectionRowPattern) (*multiPatternFilter, error) {
	if len(wps) == 0 {
		return nil, verror.New(errNoPatternsSpecified, nil)
	}
	var res multiPatternFilter
	for _, wp := range wps {
		pf, err := NewPatternFilter(wp)
		if err != nil {
			return nil, err
		}
		res.patterns = append(res.patterns, pf)
	}
	return &res, nil
}

func (m *multiPatternFilter) CollectionMatches(cxId wire.Id) bool {
	for _, p := range m.patterns {
		if p.CollectionMatches(cxId) {
			return true
		}
	}
	return false
}

func (m *multiPatternFilter) RowMatches(cxId wire.Id, key string) bool {
	// It is not sufficient to check that any collection matches; the key and
	// collection must be matched by the same member patternFilter.
	for _, p := range m.patterns {
		if p.RowMatches(cxId, key) {
			return true
		}
	}
	return false
}
