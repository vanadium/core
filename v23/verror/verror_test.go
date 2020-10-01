// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package verror_test

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"v.io/v23/verror"
	"v.io/v23/vtrace"

	_ "v.io/x/ref/runtime/factories/library"
	"v.io/x/ref/test"
)

var (
	// Some error IDActions.
	idActionA = verror.IDAction{
		ID:     "A",
		Action: verror.NoRetry,
	}
	idActionB = verror.IDAction{
		ID:     "B",
		Action: verror.RetryBackoff,
	}
	idActionC = verror.IDAction{
		ID:     "C",
		Action: verror.NoRetry,
	}
)

var (
	aEN0 error
	aEN1 error

	bEN0 error
	bEN1 error

	uEN0 error

	nEN0 error

	gEN error
)

// This function comes first because it has line numbers embedded in it, and putting it first
// reduces the chances that its line numbers will change.
func TestSubordinateErrors(t *testing.T) {
	p := idActionA.Errorf(nil, "error A %v", 0)
	p1 := verror.WithSubErrors(p, nil, verror.SubErr{
		Name:    "a=1",
		Err:     aEN1,
		Options: verror.Print,
	}, verror.SubErr{
		Name:    "a=2",
		Err:     bEN1,
		Options: verror.Print,
	})
	r1 := "verror.test: error A 0 [a=1: server:aEN1: error A 1 2] [a=2: server:bEN1: problem B 1 2]"
	if got, want := p1.Error(), r1; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	p2 := verror.WithSubErrors(p, nil, verror.SubErr{
		Name:    "go_err=1",
		Err:     fmt.Errorf("Oh"),
		Options: verror.Print,
	})
	r2 := "verror.test: error A 0 [go_err=1: Oh]"
	if got, want := p2.Error(), r2; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	p2str := verror.DebugString(p2)
	if !strings.Contains(p2str, r2) {
		t.Errorf("debug string missing error message: %q, %q", p2str, r2)
	}
	if !strings.Contains(p2str, "verror_test.go:54") {
		t.Errorf("debug string missing correct line #: %s", p2str)
	}
	p3 := verror.WithSubErrors(p, nil, verror.SubErr{
		Name:    "go_err=2",
		Err:     fmt.Errorf("Oh"),
		Options: 0,
	})
	r3 := "verror.test: error A 0"
	if got, want := p3.Error(), r3; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// This function comes first because it has line numbers embedded in it, and putting it first
// reduces the chances that its line numbers will change.
func TestChained(t *testing.T) {
	first := verror.IDAction{
		ID:     "first",
		Action: verror.NoRetry,
	}
	second := verror.IDAction{
		ID:     "second",
		Action: verror.NoRetry,
	}
	third := verror.IDAction{
		ID:     "third",
		Action: verror.NoRetry,
	}

	l1 := third.Errorf(nil, "%v", "third")
	if got, want := len(verror.Stack(l1)), 3; got != want {
		t.Log(verror.Stack(l1))
		t.Fatalf("got %v, want %v", got, want)
	}
	l2 := second.Errorf(nil, "second %v", l1)
	if got, want := len(verror.Stack(l2)), 7; got != want {
		t.Log(verror.Stack(l2))
		t.Fatalf("got %v, want %v", got, want)
	}
	l3 := first.Errorf(nil, "first %v %v %v", "not first", l1, l2)
	if got, want := len(verror.Stack(l3)), 11; got != want {
		t.Log(verror.Stack(l3))
		t.Fatalf("got %v, want %v", got, want)
	}
	lines := strings.Split(verror.Stack(l3).String(), "\n")
	if got, want := lines[0], "verror_test.go:121"; !strings.Contains(got, want) {
		t.Fatalf("%q, doesn't contain %q", got, want)
	}
	if got, want := lines[4], "verror_test.go:116"; !strings.Contains(got, want) {
		t.Fatalf("%q, doesn't contain %q", got, want)
	}
	if got, want := lines[8], "verror_test.go:111"; !strings.Contains(got, want) {
		t.Fatalf("%q, doesn't contain %q", got, want)
	}
	for _, i := range []int{3, 7} {
		if got, want := lines[i], "----- chained verror -----"; got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	l1s := l1.Error()
	l2s := l2.Error()
	if got, want := l3.Error(), "verror.test: first not first "+l1s+" "+l2s; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func init() {
	rootCtx, shutdown := test.V23Init()
	defer shutdown()

	verror.SetDefaultContext(verror.WithComponentName(rootCtx, "verror.test"))

	// Set up a context that advertises French, on a server called FooServer,
	// running an operation called aFR0.
	ctx := verror.WithComponentName(rootCtx, "server")

	// A first IDAction.
	ctx, _ = vtrace.WithNewSpan(ctx, "aEN0")
	aEN0 = idActionA.Errorf(ctx, "error A %v", 0)
	ctx, _ = vtrace.WithNewSpan(ctx, "aEN1")
	aEN1 = idActionA.Errorf(ctx, "error A %v %v", 1, 2)

	// A second IDAction.
	ctx, _ = vtrace.WithNewSpan(ctx, "bEN0")
	bEN0 = idActionB.Errorf(ctx, "problem B %v", 0)
	ctx, _ = vtrace.WithNewSpan(ctx, "bEN1")
	bEN1 = idActionB.Errorf(ctx, "problem B %v %v", 1, 2)

	// The Unknown error in various languages.
	ctx, _ = vtrace.WithNewSpan(ctx, "uEN0")
	uEN0 = verror.ErrUnknown.Errorf(ctx, "unknown error: %v", 0)

	// The NoExist error in various languages.
	ctx, _ = vtrace.WithNewSpan(ctx, "nEN0")
	nEN0 = verror.ErrNoExist.Errorf(ctx, "not found %v", 0)

	// Errors derived from Go errors.
	gerr := errors.New("Go error")
	ctx, _ = vtrace.WithNewSpan(ctx, "op")
	gEN = verror.ErrUnknown.Errorf(ctx, "unknown error: %v", gerr)

}

func TestDefaultValues(t *testing.T) {
	if got, want := verror.ErrorID(nil), verror.ID(""); got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
	if got, want := verror.Action(nil), verror.NoRetry; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}

	if got, want := verror.ErrorID(verror.E{}), verror.ID(""); got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
	if got, want := verror.Action(verror.E{}), verror.NoRetry; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}

	unknown := verror.ErrUnknown.Errorf(nil, "")
	if got, want := verror.ErrorID(unknown), verror.ErrUnknown.ID; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
	if got, want := verror.Action(unknown), verror.NoRetry; got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestBasic(t *testing.T) {
	var tests = []struct {
		err      error
		idAction verror.IDAction
		msg      string
	}{
		{aEN0, idActionA, "server:aEN0: error A 0"},
		{aEN1, idActionA, "server:aEN1: error A 1 2"},

		{bEN0, idActionB, "server:bEN0: problem B 0"},
		{bEN1, idActionB, "server:bEN1: problem B 1 2"},

		{nEN0, verror.ErrNoExist, "server:nEN0: not found 0"},
	}

	for i, test := range tests {
		if got, want := verror.ErrorID(test.err), test.idAction.ID; got != want {
			t.Errorf("%d: ErrorID(%#v); got %v, want %v", i, test.err, got, want)
		}
		if got, nowant := verror.ErrorID(test.err), idActionC.ID; got == nowant {
			t.Errorf("%d: ErrorID(%#v); got %v, nowant %v", i, test.err, got, nowant)
		}
		if got, want := verror.Action(test.err), test.idAction.Action; got != want {
			t.Errorf("%d: Action(%#v); got %v, want %v", i, test.err, got, want)
		}
		if got, want := test.err.Error(), test.msg; got != want {
			t.Errorf("%d: %#v.Error(); got %q, want %q", i, test.err, got, want)
		}

		stack := verror.Stack(test.err)
		switch {
		case stack == nil:
			t.Errorf("Stack(%q) got nil, want non-nil", verror.ErrorID(test.err))
		case len(stack) < 1 || 10 < len(stack):
			t.Errorf("len(Stack(%q)) got %d, want between 1 and 10", verror.ErrorID(test.err), len(stack))
		default:
			fnc := runtime.FuncForPC(stack[0])
			if !strings.Contains(fnc.Name(), "verror_test.init") {
				t.Errorf("Func.Name(Stack(%q)[0]) got %q, want \"verror.init\"",
					verror.ErrorID(test.err), fnc.Name())
			}
		}
	}
}

func tester() (error, error) {
	rootCtx, shutdown := test.V23Init()
	defer shutdown()
	ctx := verror.WithComponentName(rootCtx, "server")
	ctx, _ = vtrace.WithNewSpan(ctx, "aEN0")
	l1 := idActionA.Errorf(ctx, "%v", 0)
	return l1, idActionA.Errorf(ctx, "%v", 1)
}

func TestStack(t *testing.T) {
	l1, l2 := tester()
	stack1 := verror.Stack(l1).String()
	stack2 := verror.Stack(l2).String()
	if stack1 == stack2 {
		t.Errorf("expected %q and %q to differ", stack1, stack2)
	}
	for _, stack := range []string{stack1, stack2} {
		if got := strings.Count(stack, "\n"); got < 1 || 10 < got {
			t.Errorf("got %d, want between 1 and 10", got)
		}
		if !strings.Contains(stack, "verror_test.tester") {
			t.Errorf("got %q, doesn't contain 'verror_test.tester", stack)
		}
	}
}
