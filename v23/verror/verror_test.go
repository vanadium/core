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

	"v.io/v23/i18n"
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

	// Some languages
	en = i18n.LangID("en")
	fr = i18n.LangID("fr")
	de = i18n.LangID("de")
)

var (
	aEN0 error
	aEN1 error
	aFR0 error
	aFR1 error
	aDE0 error
	aDE1 error

	bEN0 error
	bEN1 error
	bFR0 error
	bFR1 error
	bDE0 error
	bDE1 error

	uEN0 error
	uEN1 error
	uFR0 error
	uFR1 error
	uDE0 error
	uDE1 error

	nEN0 error
	nEN1 error
	nFR0 error
	nFR1 error
	nDE0 error
	nDE1 error

	gEN error
	gFR error
	gDE error

	v2EN  error
	v2FR0 error
	v2FR1 error
	v2DE  error
)

// This function comes first because it has line numbers embedded in it, and putting it first
// reduces the chances that its line numbers will change.
func TestSubordinateErrors(t *testing.T) {
	p := verror.ExplicitNew(idActionA, en, "server", "aEN0", 0)
	p1 := verror.AddSubErrs(p, nil, verror.SubErr{
		Name:    "a=1",
		Err:     aEN1,
		Options: verror.Print,
	}, verror.SubErr{
		Name:    "a=2",
		Err:     aFR0,
		Options: verror.Print,
	})
	r1 := "server aEN0 error A 0 [a=1: server aEN1 error A 1 2], [a=2: server aFR0 erreur A 0]"
	if got, want := p1.Error(), r1; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	p2 := verror.AddSubErrs(p, nil, verror.SubErr{
		Name:    "go_err=1",
		Err:     fmt.Errorf("Oh"),
		Options: verror.Print,
	})
	r2 := "server aEN0 error A 0 [go_err=1: verror.test  unknown error Oh]"
	if got, want := p2.Error(), r2; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	p2str := verror.DebugString(p2)
	if !strings.Contains(p2str, r2) {
		t.Errorf("debug string missing error message: %q, %q", p2str, r2)
	}
	if !strings.Contains(p2str, "verror_test.go:85") {
		t.Errorf("debug string missing correct line #: %s", p2str)
	}
	// When Go1.4 generates stack traces, it reports the last line of the
	// expression. Go1.5 reports the first line. We check for both so the
	// test passes on either version.
	if !strings.Contains(p2str, "verror_test.go:99") && !strings.Contains(p2str, "verror_test.go:103") {
		t.Errorf("debug string missing correct line #: %s", p2str)
	}
	p3 := verror.AddSubErrs(p, nil, verror.SubErr{
		Name:    "go_err=2",
		Err:     fmt.Errorf("Oh"),
		Options: 0,
	})
	r3 := "server aEN0 error A 0 "
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
	cat := i18n.Cat()
	cat.Set(en, i18n.MsgID(first.ID), "first {_}")
	cat.Set(en, i18n.MsgID(second.ID), "second {3}")
	cat.Set(en, i18n.MsgID(third.ID), "{3}")

	l1 := verror.ExplicitNew(third, en, "", "", "third")
	if got, want := len(verror.Stack(l1)), 3; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	l2 := verror.ExplicitNew(second, en, "", "", l1)
	if got, want := len(verror.Stack(l2)), 7; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	l3 := verror.ExplicitNew(first, en, "", "", "not first", l2, l1)
	if got, want := len(verror.Stack(l3)), 11; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	lines := strings.Split(verror.Stack(l3).String(), "\n")
	if got, want := lines[0], "verror_test.go:160"; !strings.Contains(got, want) {
		t.Fatalf("%q, doesn't contain %q", got, want)
	}
	if got, want := lines[4], "verror_test.go:156"; !strings.Contains(got, want) {
		t.Fatalf("%q, doesn't contain %q", got, want)
	}
	if got, want := lines[8], "verror_test.go:152"; !strings.Contains(got, want) {
		t.Fatalf("%q, doesn't contain %q", got, want)
	}
	for _, i := range []int{3, 7} {
		if got, want := lines[i], "----- chained verror -----"; got != want {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	l1s := l1.Error()
	l2s := l2.Error()
	if got, want := l3.Error(), "first   not first "+l2s+" "+l1s; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func init() {
	rootCtx, shutdown := test.V23Init()
	defer shutdown()

	def := i18n.WithLangID(rootCtx, en)
	def = verror.WithComponentName(def, "verror.test")
	verror.SetDefaultContext(def)

	cat := i18n.Cat()
	// Set messages for English and French.  Do not set messages for
	// German, to test the case where the messages are not present.
	cat.Set(en, i18n.MsgID(idActionA.ID), "{1} {2} error A {_}")
	cat.Set(fr, i18n.MsgID(idActionA.ID), "{1} {2} erreur A {_}")

	cat.Set(en, i18n.MsgID(idActionB.ID), "{1} {2} problem B {_}")
	cat.Set(fr, i18n.MsgID(idActionB.ID), "{1} {2} problème B {_}")

	// Set English and French messages for Unknown and NoExist
	// to ones the test can predict.
	// Delete any German messages that may be present.
	cat.Set(en, i18n.MsgID(verror.ErrUnknown.ID), "{1} {2} unknown error {_}")
	cat.Set(fr, i18n.MsgID(verror.ErrUnknown.ID), "{1} {2} erreur inconnu {_}")
	cat.Set(de, i18n.MsgID(verror.ErrUnknown.ID), "")

	cat.Set(en, i18n.MsgID(verror.ErrNoExist.ID), "{1} {2} not found {_}")
	cat.Set(fr, i18n.MsgID(verror.ErrNoExist.ID), "{1} {2} pas trouvé {_}")
	cat.Set(de, i18n.MsgID(verror.ErrNoExist.ID), "")

	// Set up a context that advertises French, on a server called FooServer,
	// running an operation called aFR0.
	ctx := i18n.WithLangID(rootCtx, fr)
	ctx = verror.WithComponentName(ctx, "FooServer")
	ctx, _ = vtrace.WithNewSpan(ctx, "aFR1")

	// A first IDAction in various languages.
	aEN0 = verror.ExplicitNew(idActionA, en, "server", "aEN0", 0)
	aEN1 = verror.ExplicitNew(idActionA, en, "server", "aEN1", 1, 2)
	aFR0 = verror.ExplicitNew(idActionA, fr, "server", "aFR0", 0)
	aFR1 = verror.New(idActionA, ctx, 1, 2)
	aDE0 = verror.ExplicitNew(idActionA, de, "server", "aDE0", 0)
	aDE1 = verror.ExplicitNew(idActionA, de, "server", "aDE1", 1, 2)

	// A second IDAction in various languages.
	bEN0 = verror.ExplicitNew(idActionB, en, "server", "bEN0", 0)
	bEN1 = verror.ExplicitNew(idActionB, en, "server", "bEN1", 1, 2)
	bFR0 = verror.ExplicitNew(idActionB, fr, "server", "bFR0", 0)
	bFR1 = verror.ExplicitNew(idActionB, fr, "server", "bFR1", 1, 2)
	bDE0 = verror.ExplicitNew(idActionB, de, "server", "bDE0", 0)
	bDE1 = verror.ExplicitNew(idActionB, de, "server", "bDE1", 1, 2)

	// The Unknown error in various languages.
	uEN0 = verror.ExplicitNew(verror.ErrUnknown, en, "server", "uEN0", 0)
	uEN1 = verror.ExplicitNew(verror.ErrUnknown, en, "server", "uEN1", 1, 2)
	uFR0 = verror.ExplicitNew(verror.ErrUnknown, fr, "server", "uFR0", 0)
	uFR1 = verror.ExplicitNew(verror.ErrUnknown, fr, "server", "uFR1", 1, 2)
	uDE0 = verror.ExplicitNew(verror.ErrUnknown, de, "server", "uDE0", 0)
	uDE1 = verror.ExplicitNew(verror.ErrUnknown, de, "server", "uDE1", 1, 2)

	// The NoExist error in various languages.
	nEN0 = verror.ExplicitNew(verror.ErrNoExist, en, "server", "nEN0", 0)
	nEN1 = verror.ExplicitNew(verror.ErrNoExist, en, "server", "nEN1", 1, 2)
	nFR0 = verror.ExplicitNew(verror.ErrNoExist, fr, "server", "nFR0", 0)
	nFR1 = verror.ExplicitNew(verror.ErrNoExist, fr, "server", "nFR1", 1, 2)
	nDE0 = verror.ExplicitNew(verror.ErrNoExist, de, "server", "nDE0", 0)
	nDE1 = verror.ExplicitNew(verror.ErrNoExist, de, "server", "nDE1", 1, 2)

	// Errors derived from Go errors.
	gerr := errors.New("Go error")
	gEN = verror.ExplicitConvert(verror.ErrUnknown, en, "server", "op", gerr)
	gFR = verror.ExplicitConvert(verror.ErrUnknown, fr, "server", "op", gerr)
	gDE = verror.ExplicitConvert(verror.ErrUnknown, de, "server", "op", gerr)

	// Errors derived from other verror errors.
	// eEN1 has an English message.
	v2EN = verror.ExplicitConvert(verror.ErrUnknown, en, "", "", aEN1)        // still in English.
	v2FR0 = verror.ExplicitConvert(verror.ErrUnknown, fr, "", "", aEN1)       // converted to French, with original server and op.
	v2FR1 = verror.Convert(verror.ErrUnknown, ctx, aEN1)                      // converted to French, but still with param[1]==aEN1.
	v2DE = verror.ExplicitConvert(verror.ErrUnknown, de, "other", "op", aEN1) // left as English, since we lack German.
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

	unknown := verror.ExplicitNew(verror.ErrUnknown, i18n.NoLangID, "", "")
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
		{aEN0, idActionA, "server aEN0 error A 0"},
		{aEN1, idActionA, "server aEN1 error A 1 2"},
		{aFR0, idActionA, "server aFR0 erreur A 0"},
		{aFR1, idActionA, "FooServer aFR1 erreur A 1 2"},
		{aDE0, idActionA, "A: server aDE0 0"},
		{aDE1, idActionA, "A: server aDE1 1 2"},

		{bEN0, idActionB, "server bEN0 problem B 0"},
		{bEN1, idActionB, "server bEN1 problem B 1 2"},
		{bFR0, idActionB, "server bFR0 problème B 0"},
		{bFR1, idActionB, "server bFR1 problème B 1 2"},
		{bDE0, idActionB, "B: server bDE0 0"},
		{bDE1, idActionB, "B: server bDE1 1 2"},

		{nEN0, verror.ErrNoExist, "server nEN0 not found 0"},
		{nEN1, verror.ErrNoExist, "server nEN1 not found 1 2"},
		{nFR0, verror.ErrNoExist, "server nFR0 pas trouvé 0"},
		{nFR1, verror.ErrNoExist, "server nFR1 pas trouvé 1 2"},
		{nDE0, verror.ErrNoExist, "v.io/v23/verror.NoExist: server nDE0 0"},
		{nDE1, verror.ErrNoExist, "v.io/v23/verror.NoExist: server nDE1 1 2"},

		{gEN, verror.ErrUnknown, "server op unknown error Go error"},
		{gFR, verror.ErrUnknown, "server op erreur inconnu Go error"},
		{gDE, verror.ErrUnknown, "v.io/v23/verror.Unknown: server op Go error"},

		{v2EN, idActionA, "server aEN1 error A 1 2"},
		{v2FR0, idActionA, "server aEN1 erreur A 1 2"},
		{v2FR1, idActionA, "server aEN1 erreur A 1 2"},
		{v2DE, idActionA, "server aEN1 error A 1 2"},
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
	l1 := verror.ExplicitNew(idActionA, en, "server", "aEN0", 0)
	return l1, verror.ExplicitNew(idActionA, en, "server", "aEN0", 1)
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
