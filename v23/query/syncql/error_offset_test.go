// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syncql_test

import (
	"errors"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/query/syncql"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
)

var ctx *context.T

func init() {
	var shutdown v23.Shutdown
	ctx, shutdown = test.V23Init()
	defer shutdown()
}

type splitErrorTest struct {
	err    error
	offset int64
	errStr string
}

func TestSplitError(t *testing.T) {
	basic := []splitErrorTest{
		{
			syncql.NewErrInvalidSelectField(ctx, 7),
			7,
			"Select field must be 'k' or 'v[{.<ident>}...]'.",
		},
		{
			syncql.NewErrTableCantAccess(ctx, 14, "Bob", errors.New("No such table: Bob")),
			14,
			"Table Bob does not exist (or cannot be accessed): No such table: Bob.",
		},
	}

	for _, test := range basic {
		offset, errStr := syncql.SplitError(test.err)
		if offset != test.offset || errStr != test.errStr {
			t.Errorf("err: %v; got %d:%s, want %d:%s", test.err, offset, errStr, test.offset, test.errStr)
		}
	}
}
