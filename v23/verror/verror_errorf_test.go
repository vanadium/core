// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package verror_test

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"v.io/v23/verror"
	"v.io/v23/vtrace"
	"v.io/x/ref/runtime/factories/library"
	"v.io/x/ref/test"
)

func init() {
	library.AllowMultipleInitializations = true
}

func TestErrorf(t *testing.T) {
	rootCtx, shutdown := test.V23Init()
	defer shutdown()
	ctx := verror.WithComponentName(rootCtx, "component")
	ctx, _ = vtrace.WithNewSpan(ctx, "op")
	myerr := verror.NewIDAction("myerr", verror.RetryBackoff)
	err := myerr.Errorf(ctx, "my error %v", fmt.Errorf("oops"))
	if got, want := err.Error(), "component:op: my error oops"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := verror.ErrorID(err), verror.ID("v.io/v23/verror.myerr"); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := verror.Action(err), verror.RetryBackoff; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := verror.DebugString(err), "runtime.goexit"; !strings.Contains(got, want) {
		t.Errorf("%v does not contain %v", got, want)
	}
	if got, want := verror.Stack(err).String(), "verror_errorf_test.go:31"; !strings.Contains(got, want) {
		fmt.Println(verror.Stack(err))
		t.Errorf("%v does not contain %v", got, want)
	}

	err = verror.Errorf(nil, "my error %v", fmt.Errorf("oops"))
	if got, want := err.Error(), "verror.test: my error oops"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	nerr := verror.WithSubErrors(err, nil, verror.SubErr{
		Name:    "a=1",
		Err:     aEN1,
		Options: verror.Print,
	})

	if got, want := strings.Count(verror.DebugString(nerr), "runtime.goexit"), 2; got != want {
		t.Log(verror.DebugString(nerr))
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestMessage(t *testing.T) {
	rootCtx, shutdown := test.V23Init()
	defer shutdown()
	ctx := verror.WithComponentName(rootCtx, "component")
	ctx, _ = vtrace.WithNewSpan(ctx, "op")
	myerr := verror.NewIDAction("myerr", verror.RetryBackoff)
	nerr := fmt.Errorf("oops")
	err := myerr.Message(ctx, fmt.Sprintf("my error %v in some language", nerr), nerr)
	if got, want := err.Error(), "component:op: my error oops in some language"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := verror.ErrorID(err), verror.ID("v.io/v23/verror.myerr"); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := verror.Action(err), verror.RetryBackoff; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	err = verror.Message(nil, fmt.Sprintf("my error %v in some language", nerr), nerr)
	if got, want := err.Error(), "verror.test: my error oops in some language"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := verror.ErrorID(err), verror.ErrUnknown.ID; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := verror.Action(err), verror.NoRetry; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	nerr = verror.WithSubErrors(err, nil, verror.SubErr{
		Name:    "a=1",
		Err:     aEN1,
		Options: verror.Print,
	})
	if got, want := strings.Count(verror.DebugString(nerr), "runtime.goexit"), 2; got != want {
		t.Log(verror.DebugString(nerr))
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCompatibility(t *testing.T) {
	err1 := idActionA.Errorf(nil, "oh my")
	err2 := idActionA.Errorf(nil, "an error")

	if !errors.Is(err1, idActionA) {
		t.Errorf("errors.Is returned false, should be true")
	}
	if !errors.Is(idActionA, err1) {
		t.Errorf("errors.Is returned false, should be true")
	}
	if !errors.Is(err1, err2) {
		t.Errorf("errors.Is returned false, should be true")
	}

	err3 := verror.Errorf(nil, "oh my %v", err1)
	err4 := verror.Errorf(nil, "oh my %v", err1)
	if !errors.Is(err3, err3) {
		t.Errorf("errors.Is returned false, should be true")
	}
	if errors.Is(err1, err3) {
		t.Errorf("errors.Is returned true, should be false")
	}
	if !errors.Is(err3, err4) {
		t.Errorf("errors.Is returned false, should be true")
	}

	// Verify chained error matching.
	if !errors.Is(err3, err1) {
		t.Errorf("errors.Is returned false, should be true")
	}
	if !errors.Is(err3, idActionA) {
		t.Errorf("errors.Is returned false, should be true")
	}
	if errors.Is(err3, idActionB) {
		t.Errorf("errors.Is returned true, should be false")
	}
}

func TestUnwrap(t *testing.T) {
	p := idActionA.Errorf(nil, "error A: %v", 0)
	s1 := verror.SubErr{
		Name:    "a=1",
		Err:     aEN1,
		Options: verror.Print,
	}
	s2 := verror.SubErr{
		Name:    "a=2",
		Err:     bEN1,
		Options: verror.Print,
	}

	assertUnwrapDone := func(err error) {
		if errors.Unwrap(p) != nil {
			_, _, line, _ := runtime.Caller(1)
			t.Errorf("line: %v: expected a nil error", line)
		}
	}

	testUnwrap := func(err error, expected ...error) error {
		for i, tc := range expected {
			err = errors.Unwrap(err)
			_, _, line, _ := runtime.Caller(1)
			if err == nil {
				t.Errorf("line %v: %v: too few unwrapped errors", line, i)
				return err
			}
			if got, want := err.Error(), tc.Error(); got != want {
				t.Errorf("line %v: %v got %v, want %v", line, i, got, want)
			}
		}
		return err
	}

	assertUnwrapDone(p)

	p1 := verror.WithSubErrors(p, nil, s1, s2)
	err := testUnwrap(p1, s1, s2)
	assertUnwrapDone(err)

	se1 := fmt.Errorf("an error")
	se2 := os.ErrClosed
	p2 := idActionA.Errorf(nil, "%v %v %v", se1, "something", se2)
	p2 = verror.WithSubErrors(p2, nil, s1, s2)

	err = testUnwrap(p2, s1, s2, se2)
	assertUnwrapDone(err)

	err = idActionA.Errorf(nil, "%v %v %v %v", se1, s1, se2, s2)
	err = testUnwrap(err, s1, s2)
	assertUnwrapDone(err)

	err = idActionA.Errorf(nil, "%v %v %v %v", s2, se1, se2, s1)
	err = testUnwrap(err, s2, s1)
	assertUnwrapDone(err)

	err = idActionA.Errorf(nil, "my errors: %w %v", os.ErrNotExist, os.ErrExist)
	err = testUnwrap(err, os.ErrNotExist)
	assertUnwrapDone(err)
	err = idActionA.Errorf(nil, "my errors: %v %v", os.ErrNotExist, os.ErrExist)
	err = testUnwrap(err, os.ErrExist)
	assertUnwrapDone(err)

	err = idActionA.Errorf(nil, "my errors: %w %v", os.ErrNotExist, os.ErrExist)
	err = verror.WithSubErrors(err, s2, s1)
	err = testUnwrap(err, s2, s1, os.ErrNotExist)
	assertUnwrapDone(err)

	p3 := idActionA.Errorf(nil, "error: %v", p)

	if got, want := strings.Count(verror.DebugString(p3), "Unwrapped error"), 1; got != want {
		t.Log(verror.DebugString(p3))
		t.Errorf("got %v, want %v", got, want)
	}

	err = errors.Unwrap(p3)
	if got, want := verror.DebugString(err), verror.DebugString(p); got != want {
		t.Log(verror.DebugString(err))
		t.Log(verror.DebugString(p))
		t.Errorf("got %v, want %v", got, want)
	}

	p4 := verror.WithSubErrors(p3, s1, s2)
	if got, want := strings.Count(verror.DebugString(p4), "Unwrapped error"), 3; got != want {
		t.Log(verror.DebugString(p4))
		t.Errorf("got %v, want %v", got, want)
	}

}

func TestRegister(t *testing.T) {
	e1 := verror.NewIDAction(".err1", verror.NoRetry)
	e2 := verror.NewID("err1")
	e3 := verror.NewIDAction("v.io/v23/verror.err1", verror.NoRetry)
	if got, want := e1.ID, e2.ID; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := e1.ID, e3.ID; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWithSubErrorsSimple(t *testing.T) {
	tl := verror.NewIDAction(".errTL", verror.RetryBackoff)
	s1 := fmt.Errorf("oops")
	s2 := os.ErrExist
	s3 := verror.SubErr{
		Name:    "suberr",
		Err:     fmt.Errorf("a network errror"),
		Options: verror.Print,
	}
	err := verror.WithSubErrors(
		tl.Errorf(nil, "on my an error: %v", os.ErrNotExist),
		s1,
		s2,
		s3,
	)
	if got, want := err.Error(), "verror.test: on my an error: file does not exist oops file already exists [suberr: a network errror]"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

}
