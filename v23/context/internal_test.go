// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import (
	"context"
	"runtime"
	"testing"
)

func hasKeys(ctx *T, t *testing.T, keys ...interface{}) {
	for _, key := range keys {
		if ctx.Value(key) == nil {
			_, _, line, _ := runtime.Caller(1)
			t.Errorf("line: %v: key %T %v missing", line, key, key)
		}
	}
}

func TestCopy(t *testing.T) {
	root, cancel := RootContext()
	defer cancel()
	hasKeys(root, t, rootKey, loggerKey)

	gctx := FromGoContextWithValues(context.Background(), root)
	hasKeys(gctx, t, rootKey, loggerKey)

	type ts int
	root = WithValue(root, ts(0), ts(-1))
	hasKeys(root, t, rootKey, loggerKey, ts(0))
	gctx = FromGoContextWithValues(context.Background(), root)
	hasKeys(gctx, t, rootKey, loggerKey, ts(0))

	rc, cancel := WithRootCancel(root)
	defer cancel()
	hasKeys(rc, t, rootKey, loggerKey, ts(0))
	if got, want := rc.Value(ts(0)).(ts), ts(-1); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

}
