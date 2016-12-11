// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context_test

import (
	"bytes"
	"fmt"
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
	ch := make(chan bool, 0)
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

func TestRootContext(t *testing.T) {
	var ctx *context.T

	if ctx.Initialized() {
		t.Error("Nil context should be uninitialized")
	}
	if got := ctx.Err(); got != nil {
		t.Errorf("Expected nil error, got: %v", got)
	}
	ctx = &context.T{}
	if ctx.Initialized() {
		t.Error("Zero context should be uninitialized")
	}
	if got := ctx.Err(); got != nil {
		t.Errorf("Expected nil error, got: %v", got)
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
	if delta := time.Now().Sub(start); delta < desiredTimeout {
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

func TestRootCancel(t *testing.T) {
	root, rootcancel := context.RootContext()
	a, acancel := context.WithCancel(root)
	b := context.WithValue(a, "key", "value")

	c, ccancel := context.WithRootCancel(b)
	d, _ := context.WithCancel(c)

	e, _ := context.WithRootCancel(b)

	if s, ok := d.Value("key").(string); !ok || s != "value" {
		t.Error("Lost a value but shouldn't have.")
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
	<-root.Done()
	<-e.Done()
}

type stringLogger struct {
	bytes.Buffer
}

func (*stringLogger) Info(args ...interface{})                    {}
func (sl *stringLogger) InfoDepth(depth int, args ...interface{}) { sl.WriteString(fmt.Sprint(args...)) }
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

func (d *stringLogger) VI(level int) interface {
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	InfoDepth(depth int, args ...interface{})
	InfoStack(all bool)
} {
	return d
}
func (d *stringLogger) VIDepth(depth int, level int) interface {
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	InfoDepth(depth int, args ...interface{})
	InfoStack(all bool)
} {
	return d
}

func (*stringLogger) FlushLog()      {}
func (*stringLogger) LogDir() string { return "" }
func (*stringLogger) Stats() (Info, Error struct{ Lines, Bytes int64 }) {
	return struct{ Lines, Bytes int64 }{0, 0}, struct{ Lines, Bytes int64 }{0, 0}
}
func (*stringLogger) ConfigureFromFlags() error             { return nil }
func (*stringLogger) ExplicitlySetFlags() map[string]string { return nil }

func TestLogging(t *testing.T) {
	root, rootcancel := context.RootContext()
	var _ logging.Logger = root
	root.Infof("this message should be silently discarded")

	logger := &stringLogger{}
	ctx := context.WithLogger(root, logger)
	ctx.Infof("Oh, %s", "hello there")

	if got, want := logger.String(), "Oh, hello there"; got != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	rootcancel()
}
