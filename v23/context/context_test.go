// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context_test

import (
	"bytes"
	gocontext "context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/logging"
)

func testCancel(t *testing.T, ctx *context.T, cancel context.CancelFunc) {
	select {
	case <-ctx.Done():
		t.Errorf("Done closed when deadline not yet passed")
	default:
	}
	ch := make(chan bool)
	go func() {
		cancel()
		close(ch)
	}()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for cancel.")
	}

	select {
	case <-ctx.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for cancellation.")
	}
	if err := ctx.Err(); err != context.Canceled {
		t.Errorf("Unexpected error want %v, got %v", context.Canceled, err)
	}
}

func TestInitialization(t *testing.T) {
	var ctx *context.T
	if ctx.Initialized() {
		t.Error("Nil context should be uninitialized")
	}
	if context.WithLogger(ctx, nil) != nil {
		t.Error("Nil context should be uninitialized")
	}
	if context.WithContextLogger(ctx, nil) != nil {
		t.Error("Nil context should be uninitialized")
	}
	ctx = &context.T{}
	if ctx.Initialized() {
		t.Error("Zero context should be uninitialized")
	}
}

func TestCancelContext(t *testing.T) {
	root, _ := context.RootContext()
	ctx, cancel := context.WithCancel(root)
	testCancel(t, ctx, cancel)

	// Test cancelling a cancel context which is the child
	// of a cancellable context.
	parent, _ := context.WithCancel(root)
	child, cancel := context.WithCancel(parent)
	cancel()
	<-child.Done()

	// Test adding a cancellable child context after the parent is
	// already cancelled.
	parent, cancel = context.WithCancel(root)
	cancel()
	child, _ = context.WithCancel(parent)
	<-child.Done() // The child should have been cancelled right away.
}

func TestMultiLevelCancelContext(t *testing.T) {
	root, _ := context.RootContext()
	c0, c0Cancel := context.WithCancel(root)
	c1, _ := context.WithCancel(c0)
	c2, _ := context.WithCancel(c1)
	c3, _ := context.WithCancel(c2)
	testCancel(t, c3, c0Cancel)
}

func testDeadline(t *testing.T, ctx *context.T, start time.Time, desiredTimeout time.Duration) {
	<-ctx.Done()
	if delta := time.Since(start); delta < desiredTimeout {
		t.Errorf("Deadline too short want %s got %s", desiredTimeout, delta)
	}
	if err := ctx.Err(); err != context.DeadlineExceeded {
		t.Errorf("Unexpected error want %s, got %s", context.DeadlineExceeded, err)
	}
}

func TestDeadlineContext(t *testing.T) {
	cases := []time.Duration{
		3 * time.Millisecond,
		0,
	}
	rootCtx, _ := context.RootContext()
	cancelCtx, _ := context.WithCancel(rootCtx)
	deadlineCtx, _ := context.WithDeadline(rootCtx, time.Now().Add(time.Hour))

	for _, desiredTimeout := range cases {
		// Test all the various ways of getting deadline contexts.
		start := time.Now()
		ctx, _ := context.WithDeadline(rootCtx, start.Add(desiredTimeout))
		testDeadline(t, ctx, start, desiredTimeout)

		start = time.Now()
		ctx, _ = context.WithDeadline(cancelCtx, start.Add(desiredTimeout))
		testDeadline(t, ctx, start, desiredTimeout)

		start = time.Now()
		ctx, _ = context.WithDeadline(deadlineCtx, start.Add(desiredTimeout))
		testDeadline(t, ctx, start, desiredTimeout)

		start = time.Now()
		ctx, _ = context.WithTimeout(rootCtx, desiredTimeout)
		testDeadline(t, ctx, start, desiredTimeout)

		start = time.Now()
		ctx, _ = context.WithTimeout(cancelCtx, desiredTimeout)
		testDeadline(t, ctx, start, desiredTimeout)

		start = time.Now()
		ctx, _ = context.WithTimeout(deadlineCtx, desiredTimeout)
		testDeadline(t, ctx, start, desiredTimeout)
	}

	ctx, cancel := context.WithDeadline(rootCtx, time.Now().Add(100*time.Hour))
	testCancel(t, ctx, cancel)
}

func TestDeadlineContextWithRace(t *testing.T) {
	root, _ := context.RootContext()
	ctx, cancel := context.WithDeadline(root, time.Now().Add(100*time.Hour))
	var wg sync.WaitGroup
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			cancel()
			wg.Done()
		}()
	}
	wg.Wait()
	<-ctx.Done()
	if err := ctx.Err(); err != context.Canceled {
		t.Errorf("Unexpected error want %v, got %v", context.Canceled, err)
	}
}

func TestValueContext(t *testing.T) {
	type testContextKey int
	const (
		key1 = testContextKey(iota)
		key2
		key3
		key4
	)
	const (
		val1 = iota
		val2
		val3
	)
	root, _ := context.RootContext()
	ctx1 := context.WithValue(root, key1, val1)
	ctx2 := context.WithValue(ctx1, key2, val2)
	ctx3 := context.WithValue(ctx2, key3, val3)

	expected := map[interface{}]interface{}{
		key1: val1,
		key2: val2,
		key3: val3,
		key4: nil,
	}
	for k, v := range expected {
		if got := ctx3.Value(k); got != v {
			t.Errorf("Got wrong value for %v: want %v got %v", k, v, got)
		}
	}

}

type ctxKey string

func TestRootCancel(t *testing.T) {
	root, rootcancel := context.RootContext()
	root = context.WithValue(root, ctxKey("tlKey"), "tlValue")
	a, acancel := context.WithCancel(root)
	// Most recent WithValue calls override previous ones,
	// make sure that WithRootCancel preserves that ordering.
	b := context.WithValue(a, ctxKey("key"), "valueLast")
	b = context.WithValue(b, ctxKey("key"), "valueMiddle")
	b = context.WithValue(b, ctxKey("key"), "value")

	c, ccancel := context.WithRootCancel(b)
	d, _ := context.WithCancel(c)
	f, _ := context.WithCancel(d)

	e, _ := context.WithRootCancel(b)

	for i, vctx := range []*context.T{d, e, f} {
		if s, ok := vctx.Value(ctxKey("tlKey")).(string); !ok || s != "tlValue" {
			t.Errorf("%v: lost or wrong value....", i)
		}
		if s, ok := vctx.Value(ctxKey("key")).(string); !ok || s != "value" {
			t.Errorf("%v: lost or wrong value....", i)
		}
	}

	// If we cancel a, b will get canceled but c will not.
	acancel()
	<-a.Done()
	<-b.Done()
	select {
	case <-c.Done():
		t.Error("C should not yet be canceled.")
	case <-e.Done():
		t.Error("E should not yet be canceled.")
	case <-time.After(100 * time.Millisecond):
	}

	// Cancelling c should still cancel d.
	ccancel()
	<-c.Done()
	<-d.Done()

	// Cancelling the root should cancel e.
	rootcancel()
	select {
	case <-root.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("timedout waiting for root")

	}
	select {
	case <-e.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("timedout waiting for e")
	}
}

func TestRootCancelChain(t *testing.T) {
	root, rootcancel := context.RootContext()
	root = context.WithValue(root, ctxKey("tlKey"), "tlValue")
	a, acancel := context.WithCancel(root)
	b := context.WithValue(a, ctxKey("key"), "value")

	c, ccancel := context.WithRootCancel(b)
	d, _ := context.WithCancel(c)
	e, _ := context.WithRootCancel(d)
	f, _ := context.WithCancel(e)

	for _, vctx := range []*context.T{c, d, e, f} {
		if s, ok := vctx.Value(ctxKey("tlKey")).(string); !ok || s != "tlValue" {
			t.Error("Lost a value but shouldn't have.")
		}

		if s, ok := vctx.Value(ctxKey("key")).(string); !ok || s != "value" {
			t.Error("Lost a value but shouldn't have.")
		}
	}

	// If we cancel a, b will get canceled but c, d, e, f will not.
	acancel()
	<-a.Done()
	<-b.Done()
	select {
	case <-c.Done():
		t.Error("C should not yet be canceled.")
	case <-d.Done():
		t.Error("D should not yet be canceled.")
	case <-e.Done():
		t.Error("E should not yet be canceled.")
	case <-f.Done():
		t.Error("F should not yet be canceled.")
	case <-time.After(100 * time.Millisecond):
	}

	// If we cancel c, d will get canceled but not e or f.
	ccancel()
	<-c.Done()
	<-d.Done()
	select {
	case <-e.Done():
		t.Error("E should not yet be canceled.")
	case <-f.Done():
		t.Error("F should not yet be canceled.")
	case <-time.After(100 * time.Millisecond):
	}

	// If we cancel the original root, everything should now be canceled.
	rootcancel()
	<-root.Done()
	<-e.Done()
	<-f.Done()

	// Make sure rootcancel cancels everything.
	root, rootcancel = context.RootContext()
	a, _ = context.WithCancel(root)
	b = context.WithValue(a, ctxKey("key"), "value")

	c, _ = context.WithRootCancel(b)
	d, _ = context.WithCancel(c)
	e, _ = context.WithRootCancel(d)
	f, _ = context.WithCancel(e)

	rootcancel()
	for _, ctx := range []*context.T{a, b, c, d, e, f} {
		<-ctx.Done()
	}
}

func TestRootCancel_GoContext(t *testing.T) {
	root, rootcancel := context.RootContext()

	a, acancel := gocontext.WithCancel(root)
	b := context.FromGoContext(a)
	c, _ := context.WithRootCancel(b)

	// Cancelling a should cancel b, but not c.
	acancel()
	<-b.Done()
	select {
	case <-c.Done():
		t.Error("C should not yet be cancelled")
	case <-time.After(100 * time.Millisecond):
	}
	// Cancelling the root should cancel c.
	rootcancel()
	<-c.Done()
	<-root.Done()
}

type stringLogger struct {
	bytes.Buffer
}

func (*stringLogger) Info(args ...interface{}) {}
func (sl *stringLogger) InfoDepth(depth int, args ...interface{}) {
	sl.WriteString(fmt.Sprint(args...) + "\n")
}
func (sl *stringLogger) Infof(format string, args ...interface{}) {}
func (*stringLogger) InfoStack(all bool)                          {}

func (*stringLogger) Error(args ...interface{})                 {}
func (*stringLogger) ErrorDepth(depth int, args ...interface{}) {}
func (*stringLogger) Errorf(format string, args ...interface{}) {}

func (*stringLogger) Fatal(args ...interface{})                 {}
func (*stringLogger) FatalDepth(depth int, args ...interface{}) {}
func (*stringLogger) Fatalf(format string, args ...interface{}) {}

func (*stringLogger) Panic(args ...interface{})                 {}
func (*stringLogger) PanicDepth(depth int, args ...interface{}) {}
func (*stringLogger) Panicf(format string, args ...interface{}) {}

func (*stringLogger) V(level int) bool                 { return false }
func (*stringLogger) VDepth(depth int, level int) bool { return false }

func (sl *stringLogger) VI(level int) interface {
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	InfoDepth(depth int, args ...interface{})
	InfoStack(all bool)
} {
	return sl
}
func (sl *stringLogger) VIDepth(depth int, level int) interface {
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	InfoDepth(depth int, args ...interface{})
	InfoStack(all bool)
} {
	return sl
}

func (*stringLogger) FlushLog()      {}
func (*stringLogger) LogDir() string { return "" }
func (*stringLogger) Stats() (infoStats, errorStats struct{ Lines, Bytes int64 }) {
	return struct{ Lines, Bytes int64 }{0, 0}, struct{ Lines, Bytes int64 }{0, 0}
}
func (*stringLogger) ConfigureFromFlags() error             { return nil }
func (*stringLogger) ExplicitlySetFlags() map[string]string { return nil }

type ctxLogger stringLogger

func (cl *ctxLogger) InfoDepth(ctx *context.T, depth int, args ...interface{}) {
	(*stringLogger)(cl).InfoDepth(depth, args...)
}

func (cl *ctxLogger) InfoStack(ctx *context.T, all bool) {
	(*stringLogger)(cl).InfoStack(all)
}

func (cl *ctxLogger) VDepth(ctx *context.T, depth int, level int) bool {
	return (*stringLogger)(cl).VDepth(depth, level)
}

func (cl *ctxLogger) VIDepth(ctx *context.T, depth int, level int) context.Logger {
	return cl
}

func (cl *ctxLogger) FlushLog() {}

func TestLogging(t *testing.T) {
	root, rootcancel := context.RootContext()
	defer rootcancel()

	var _ logging.Logger = root
	root.Infof("this message should be silently discarded")

	logger := &stringLogger{}
	ctx := context.WithLogger(root, logger)
	ctx.Infof("Oh, %s", "hello there")

	if got, want := context.LoggerFromContext(ctx), logger; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	if got, want := logger.String(), "Oh, hello there\n"; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	clogger := &stringLogger{}
	ctx = context.WithContextLogger(root, (*ctxLogger)(clogger))
	ctx.Infof("Oh, %s", "hello there")

	if got, want := clogger.String(), "Oh, hello there\n"; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	ctx = context.WithLogger(root, logger)
	ctx = context.WithContextLogger(ctx, (*ctxLogger)(clogger))
	cctx := gocontext.WithValue(ctx, ctxKey("a"), "a")
	gctx := context.FromGoContext(cctx)
	if got, want := context.LoggerFromContext(gctx).(*stringLogger), logger; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	rctx, _ := context.WithRootCancel(ctx)
	if got, want := context.LoggerFromContext(rctx), logger; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	logger.Reset()
	clogger.Reset()
	ctx = context.WithLoggingPrefix(ctx, "my-prefix")
	ctx.Info("1st", 1)
	ctx.Infof("2nd: %v", 2)
	if got, want := logger.String(), "my-prefix1st1\nmy-prefix: 2nd: 2\n"; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got, want := clogger.String(), "my-prefix1st1\nmy-prefix: 2nd: 2\n"; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	logger.Reset()
	clogger.Reset()
	ctx = context.WithLoggingPrefix(ctx, os.ErrNotExist)
	ctx.Info("1st", 1)
	if got, want := logger.String(), "file does not exist1st1\n"; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
	logger.Reset()
	clogger.Reset()
	prefix := uuid("1234")
	ctx = context.WithLoggingPrefix(ctx, uuid("1234"))
	ctx.Infof("1st: %v", 1)
	if got, want := logger.String(), "uuid: 1234: 1st: 1\n"; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctx.Infof("2nd: %v", 2)
	if got, want := logger.String(), "uuid: 1234: 1st: 1\nuuid: 1234: 2nd: 2\n"; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	if got, want := context.LoggingPrefix(ctx).(uuid), prefix; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

type uuid string

func (id uuid) String() string {
	return "uuid: " + string(id)
}
