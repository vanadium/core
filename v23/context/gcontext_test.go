// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context_test

import (
	gocontext "context"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
	v23 "v.io/v23"
	vcontext "v.io/v23/context"
	_ "v.io/x/ref/runtime/factories/library"
	"v.io/x/ref/test"
)

func TestNoopConversion(t *testing.T) {
	c0, cancel0 := vcontext.RootContext()
	c1 := vcontext.FromGoContext(c0)
	if c0 != c1 {
		t.Error("convert")
	}
	cancel0()
}

func TestFromGoContext(t *testing.T) {
	goctx, cancel := gocontext.WithCancel(gocontext.Background())
	c := vcontext.FromGoContext(goctx)
	if !c.Initialized() {
		t.Error("!initialized")
	}
	select {
	case <-c.Done():
		t.Error("done")
	default:
	}
	cancel()
	<-c.Done()
}

func TestFromGoContextPrincipal(t *testing.T) {
	root, shutdown := test.V23Init()
	defer shutdown()
	principal := v23.GetPrincipal(root)

	assertValues := func(ctx gocontext.Context, values ...string) {
		for _, v := range values {
			if ctx.Value(ctxKey(v)) == nil {
				t.Errorf("missing key: %v", v)
				return
			}
			if got, want := ctx.Value(ctxKey(v)).(string), v; got != want {
				t.Errorf("got %v, want %v", got, want)
			}
		}
	}
	assertPrincipal := func(ctx *vcontext.T) {
		if got, want := v23.GetPrincipal(ctx), principal; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}

	ctx := vcontext.WithValue(root, ctxKey("a"), "a")
	assertValues(ctx, "a")
	gctx := gocontext.WithValue(ctx, ctxKey("b"), "b")
	assertValues(gctx, "a", "b")
	_, egctx := errgroup.WithContext(gctx)
	assertValues(egctx, "a", "b")

	vctx := vcontext.FromGoContext(egctx)
	assertValues(vctx, "a", "b")
	assertPrincipal(root)
	assertPrincipal(vctx)

	wrc, cancel := vcontext.WithRootCancel(vctx)
	defer cancel()
	assertValues(wrc, "a", "b")
	assertPrincipal(wrc)

	gctx = gocontext.Background()
	gctx = gocontext.WithValue(gctx, ctxKey("c"), "c")
	gctx = gocontext.WithValue(gctx, ctxKey("d"), "d")
	vctx = vcontext.FromGoContext(gctx)
	assertValues(gctx, "c", "d")
	assertValues(vctx, "c", "d")
	wrc, cancel = vcontext.WithRootCancel(vctx)
	defer cancel()
	assertValues(wrc, "c", "d")
}

func TestDeadline(t *testing.T) {
	deadline := time.Now().Add(time.Second)
	goctx, cancel := gocontext.WithDeadline(gocontext.Background(), deadline)
	defer cancel()
	c := vcontext.FromGoContext(goctx)
	<-c.Done()
}

func TestValue(t *testing.T) {
	c0, cancel0 := vcontext.RootContext()
	type ts string // define a new type to avoid collisions with string
	c1 := vcontext.WithValue(c0, ts("foo1"), ts("bar1"))
	c2 := gocontext.WithValue(c1, ts("foo2"), ts("bar2"))
	c3 := vcontext.FromGoContext(c2)

	if v := c3.Value(ts("foo1")); v.(ts) != "bar1" {
		t.Error(v)
	}
	if v := c3.Value(ts("foo2")); v.(ts) != "bar2" {
		t.Error(v)
	}
	select {
	case <-c3.Done():
		t.Error("done")
	default:
	}
	cancel0()
	<-c1.Done()
	<-c2.Done()
	<-c3.Done()
}

func TestValueCopy(t *testing.T) {
	root, rootcancel := vcontext.RootContext()
	type ts string // define a new type to avoid collisions with string
	root = vcontext.WithValue(root, ts("foo"), ts("bar"))
	defer rootcancel()
	gctx := gocontext.Background()
	gctx = gocontext.WithValue(gctx, ts("foo1"), ts("bar1"))
	ctx := vcontext.FromGoContextWithValues(gctx, root)
	if got, want := ctx.Value(ts("foo")).(ts), ts("bar"); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := ctx.Value(ts("foo1")).(ts), ts("bar1"); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
