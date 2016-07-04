// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"bytes"
	"encoding/binary"

	"google.golang.org/cloud/bigtable"

	"v.io/v23/context"
	"v.io/v23/verror"
)

var (
	errNodeLimitExceeded   = verror.Register(pkgPath+".errNodeLimitExceeded", verror.NoRetry, "{1:}{2:} node limit per user ({3}) exceeded{:_}")
	errServerLimitExceeded = verror.Register(pkgPath+".errServerLimitExceeded", verror.NoRetry, "{1:}{2:} mounted server limit per user ({3}) exceeded{:_}")
)

func incrementCounter(ctx *context.T, bt *BigTable, name string, delta int64) (int64, error) {
	bctx, cancel := btctx(ctx)
	defer cancel()

	m := bigtable.NewReadModifyWrite()
	m.Increment(metadataFamily, "c", delta)
	row, err := bt.counterTbl.ApplyReadModifyWrite(bctx, name, m)
	if err != nil {
		return 0, err
	}
	return decodeCounterValue(ctx, row)
}

func decodeCounterValue(ctx *context.T, row bigtable.Row) (c int64, err error) {
	if len(row[metadataFamily]) != 1 {
		return 0, verror.NewErrInternal(ctx)
	}
	b := row[metadataFamily][0].Value
	err = binary.Read(bytes.NewReader(b), binary.BigEndian, &c)
	return
}

func incrementWithLimit(ctx *context.T, bt *BigTable, name string, delta, limit int64, overLimitError verror.IDAction) error {
	if delta == 0 {
		return nil
	}
	c, err := incrementCounter(ctx, bt, name, delta)
	if err != nil {
		return err
	}
	if delta > 0 && limit > 0 && c > limit {
		if _, err := incrementCounter(ctx, bt, name, -delta); err != nil {
			ctx.Errorf("incrementCounter(%q, %v) failed: %v", name, -delta, err)
		}
		return verror.New(overLimitError, ctx, limit)
	}
	return nil
}

func incrementCreatorNodeCount(ctx *context.T, bt *BigTable, creator string, delta, limit int64) error {
	return incrementWithLimit(ctx, bt, "num-nodes-per-user:"+creator, delta, limit, errNodeLimitExceeded)
}

func incrementCreatorServerCount(ctx *context.T, bt *BigTable, creator string, delta, limit int64) error {
	return incrementWithLimit(ctx, bt, "num-servers-per-user:"+creator, delta, limit, errServerLimitExceeded)
}

func recalculateCounters(ctx *context.T, bt *BigTable) error {
	bctx, cancel := btctx(ctx)
	defer cancel()

	// Delete all the counters.
	if err := bt.counterTbl.ReadRows(bctx, bigtable.InfiniteRange(""),
		func(row bigtable.Row) bool {
			mut := bigtable.NewMutation()
			mut.DeleteRow()
			if err := bt.counterTbl.Apply(ctx, row.Key(), mut); err != nil {
				ctx.Errorf("apply delete row (%q) failed: %v", row.Key(), err)
				return false
			}
			return true
		},
	); err != nil {
		return err
	}

	// Re-create all the counters.
	return bt.nodeTbl.ReadRows(bctx, bigtable.InfiniteRange(""),
		func(row bigtable.Row) bool {
			n := nodeFromRow(ctx, bt, row, clock)
			if err := incrementCreatorNodeCount(ctx, bt, n.creator, 1, 0); err != nil {
				ctx.Errorf("incrementCreatorNodeCount(%q) failed: %v", n.name, err)
				return false
			}
			if err := incrementCreatorServerCount(ctx, bt, n.creator, int64(len(n.servers)+len(n.expiredServers)), 0); err != nil {
				ctx.Errorf("incrementCreatorServerCount(%q) failed: %v", n.name, err)
				return false
			}
			return true
		},
		bigtable.RowFilter(bigtable.LatestNFilter(1)),
	)
}
