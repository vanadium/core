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
	err := verror.VErrorf(ctx, verror.RetryBackoff, "my error {_}", fmt.Errorf("oops"))
	if got, want := err.Error(), "component:op: my error oops"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	id := "v.io/v23/verror:verror_errorf_test.go"
	if got, want := string(verror.ErrorID(err)), id; !strings.Contains(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := verror.Action(err), verror.RetryBackoff; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := verror.DebugString(err), "runtime.goexit"; !strings.Contains(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := verror.Stack(err).String(), "verror_errorf_test.go:24"; !strings.Contains(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	err = verror.Errorf("my error {_}", fmt.Errorf("oops"))
	if got, want := err.Error(), "verror.test: my error oops"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	nerr := verror.AddSubErrs(err, nil, verror.SubErr{
		Name:    "a=1",
		Err:     aEN1,
		Options: verror.Print,
	})
	if got, want := strings.Count(verror.DebugString(nerr), "runtime.goexit"), 2; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

}

func TestCompatibility(t *testing.T) {
	err1 := verror.New(idActionA, nil)
	err2 := verror.New(idActionA, nil)

	if !errors.Is(err1, idActionA) {
		t.Errorf("errors.Is returned false, should be true")
	}
	if !errors.Is(idActionA, err1) {
		t.Errorf("errors.Is returned false, should be true")
	}
	if !errors.Is(err1, err2) {
		t.Errorf("errors.Is returned false, should be true")
	}

	err3 := verror.Errorf("oh my {_}", err1)
	err4 := verror.Errorf("oh my {_}", err1)
	if !errors.Is(err3, err3) {
		t.Errorf("errors.Is returned false, should be true")
	}
	if errors.Is(err1, err3) {
		t.Errorf("errors.Is returned true, should be false")
	}
	if !errors.Is(err3, err4) {
		t.Errorf("errors.Is returned true, should be false")
	}
}

func TestUnwrap(t *testing.T) {
	p := verror.ExplicitNew(idActionA, en, "server", "aEN0", 0)
	s1 := verror.SubErr{
		Name:    "a=1",
		Err:     aEN1,
		Options: verror.Print,
	}
	s2 := verror.SubErr{
		Name:    "a=2",
		Err:     aFR0,
		Options: verror.Print,
	}
	p1 := verror.AddSubErrs(p, nil, s1, s2)

	assertUnwrapDone := func(err error) {
		if errors.Unwrap(p) != nil {
			_, _, line, _ := runtime.Caller(1)
			t.Errorf("line: %v: expected a nil error", line)
		}
	}

	assertUnwrapDone(p)

	err := p1
	for i, tc := range []string{
		s1.Error(),
		s2.Error(),
	} {
		err = errors.Unwrap(err)
		if got, want := err.Error(), tc; got != want {
			t.Errorf("%v got %v, want %v", i, got, want)
		}
	}
	assertUnwrapDone(err)

	se1 := fmt.Errorf("an error")
	se2 := os.ErrClosed
	p2 := verror.ExplicitNew(idActionA, en, "server", "aEN0", se1, "something", se2)
	err = verror.AddSubErrs(p2, nil, s1, s2)
	for i, tc := range []string{
		se1.Error(),
		se2.Error(),
		s1.Error(),
		s2.Error(),
	} {
		err = errors.Unwrap(err)
		if got, want := err.Error(), tc; got != want {
			t.Errorf("%v got %v, want %v", i, got, want)
		}
	}
	assertUnwrapDone(err)

	err = verror.ExplicitNew(idActionA, en, "server", "aEN0", se1, s1, se2, s2)
	for i, tc := range []string{
		se1.Error(),
		s1.Error(),
		se2.Error(),
		s2.Error(),
	} {
		err = errors.Unwrap(err)
		if got, want := err.Error(), tc; got != want {
			t.Errorf("%v got %v, want %v", i, got, want)
		}
	}
	assertUnwrapDone(err)

	err = verror.ExplicitNew(idActionA, en, "server", "aEN0", s1, se1, se2, s2)
	for i, tc := range []string{
		s1.Error(),
		se1.Error(),
		se2.Error(),
		s2.Error(),
	} {
		err = errors.Unwrap(err)
		if got, want := err.Error(), tc; got != want {
			t.Errorf("%v got %v, want %v", i, got, want)
		}
	}

}
