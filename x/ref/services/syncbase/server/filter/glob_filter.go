// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package filter

import (
	"strings"

	"v.io/v23/glob"
	wire "v.io/v23/services/syncbase"
	pubutil "v.io/v23/syncbase/util"
	"v.io/v23/verror"
)

var (
	errGlobParseFailed = verror.Register(pkgPath+".errGlobParseFailed", verror.NoRetry, "{1:}{2:} glob parse failed{:_}")
)

// globFilter matches using a glob of the format <encodedCxId>/<decodedKey>
// with v23.Glob semantics. The key is decoded to allow matching name
// components within the key separated by slashes using '*'.
type globFilter struct {
	cxId   *glob.Element
	rowKey *glob.Glob
}

var _ CollectionRowFilter = (*globFilter)(nil)

func NewGlobFilter(pattern string) (*globFilter, error) {
	fullGlob, err := glob.Parse(pattern)
	if err != nil {
		return nil, verror.New(errGlobParseFailed, nil, err)
	}
	// TODO(ivanpi): Check that collection pattern can match a valid collection
	// name, e.g. contains a comma or wildcard that could be a comma.
	return &globFilter{
		cxId:   fullGlob.Head(),
		rowKey: fullGlob.Tail(),
	}, nil
}

func (g *globFilter) CollectionMatches(cxId wire.Id) bool {
	return g.cxId.Match(pubutil.EncodeId(cxId))
}

func (g *globFilter) RowMatches(cxId wire.Id, key string) bool {
	if !g.CollectionMatches(cxId) {
		return false
	}
	gIt := g.rowKey
	// Globs are matched by iterating over slash-separated key segments and
	// matching each segment to the appropriate glob component. Globs are
	// iterated over by removing the first component after it has been matched.
	// Recursive globs allow indefinite iteration after glob components have
	// been exhausted to match subsequent key components, while non-recursive
	// globs fail.
	for _, kPart := range strings.Split(key, "/") {
		if !gIt.Head().Match(kPart) {
			return false
		}
		gIt = gIt.Tail()
	}
	// All glob components must be matched, otherwise the key is just a prefix of
	// a valid match.
	return gIt.Len() == 0
}
