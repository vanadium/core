// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package filter

import (
	"v.io/v23/query/pattern"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/verror"
)

const escapeChar = '\\'

var (
	errCollectionPatternEmpty = verror.Register(pkgPath+".errCollectionPatternEmpty", verror.NoRetry, "{1:}{2:} collection blessing and name patterns in CollectionRowPattern must be non-empty")
)

// patternFilter matches collections and rows as specified by
// wire.CollectionRowPattern (using separate LIKE-style globs for collection
// name, blessing, and row key).
// Collection blessing and name patterns are not allowed to be empty, but the
// row key pattern is (for matching only collections and no rows).
type patternFilter struct {
	cxBlessing *pattern.Pattern
	cxName     *pattern.Pattern
	rowKey     *pattern.Pattern
}

var _ CollectionRowFilter = (*patternFilter)(nil)

func NewPatternFilter(wp wire.CollectionRowPattern) (*patternFilter, error) {
	if wp.CollectionBlessing == "" || wp.CollectionName == "" {
		return nil, verror.New(errCollectionPatternEmpty, nil)
	}
	var res patternFilter
	var err error
	if res.cxBlessing, err = pattern.Parse(wp.CollectionBlessing); err != nil {
		return nil, err
	}
	if res.cxName, err = pattern.Parse(wp.CollectionName); err != nil {
		return nil, err
	}
	if res.rowKey, err = pattern.Parse(wp.RowKey); err != nil {
		return nil, err
	}
	return &res, nil
}

func (p *patternFilter) CollectionMatches(cxId wire.Id) bool {
	return p.cxName.MatchString(cxId.Name) && p.cxBlessing.MatchString(cxId.Blessing)
}

func (p *patternFilter) RowMatches(cxId wire.Id, key string) bool {
	return p.CollectionMatches(cxId) && p.rowKey.MatchString(key)
}
