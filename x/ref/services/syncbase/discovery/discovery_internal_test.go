// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package discovery

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"testing"

	"v.io/v23/context"
	"v.io/v23/logging"
	"v.io/v23/security"
	wire "v.io/v23/services/syncbase"
)

func TestJoinSplitPatterns(t *testing.T) {
	cases := []struct {
		patterns []security.BlessingPattern
		joined   string
	}{
		{nil, ""},
		{[]security.BlessingPattern{"a", "b"}, "a,b"},
		{[]security.BlessingPattern{"a:b:c", "d:e:f"}, "a:b:c,d:e:f"},
		{[]security.BlessingPattern{"alpha:one", "alpha:two", "alpha:three"}, "alpha:one,alpha:two,alpha:three"},
	}
	for _, c := range cases {
		if got := joinPatterns(c.patterns); got != c.joined {
			t.Errorf("%#v, got %q, wanted %q", c.patterns, got, c.joined)
		}
		if got := splitPatterns(c.joined); !reflect.DeepEqual(got, c.patterns) {
			t.Errorf("%q, got %#v, wanted %#v", c.joined, got, c.patterns)
		}
	}
	// Special case, Joining an empty non-nil list results in empty string.
	if got := joinPatterns([]security.BlessingPattern{}); got != "" {
		t.Errorf("Joining empty list: got %q, want %q", got, "")
	}
}

type logger struct {
	logging.Logger
}

func (l logger) InfoDepth(depth int, args ...interface{}) {
	fmt.Fprintln(os.Stdout, args...)
}

func TestInviteQueue(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()

	ctx = context.WithLogger(ctx, logger{})

	q := newCopyableQueue()
	if q.size() != 0 {
		t.Errorf("got %d, want 0", q.size())
	}

	want := []string{"0", "1", "2", "3", "4"}

	// Test inserts during a long scan.
	ch := make(chan struct{})
	go func() {
		scanForFoundOnly(ctx, q, want)
		close(ch)
	}()

	// Add a bunch of entries.
	for i, w := range want {
		q.add(Invite{Syncgroup: wire.Id{Name: w}, key: w})
		if q.size() != i+1 {
			t.Errorf("got %d, want %d", q.size(), i+1)
		}
		if err := scanForFoundOnly(ctx, q, want[:i+1]); err != nil {
			t.Error(err)
		}
	}

	// Make sure long term scan finished.
	<-ch

	// Start another long scan that will proceed across deletes
	// Start a similar scan that will see 0, 1, 3, 4 and deletes for 0, 1, 3, 4.
	// Start a final scan that will see 0, 3 and deletes for 0, 3.
	steps := make(chan struct{})
	go func() {
		ctx1, cancel := context.WithCancel(ctx)
		c := q.scan()
		ctx2, cancel2 := context.WithCancel(ctx)
		c2 := q.scan()
		ctx3, cancel3 := context.WithCancel(ctx)
		c3 := q.scan()

		advance(t, ctx1, c, q, want[0], false)
		advance(t, ctx1, c, q, want[1], false)
		advance(t, ctx2, c2, q, want[0], false)
		advance(t, ctx2, c2, q, want[1], false)
		advance(t, ctx3, c3, q, want[0], false)

		steps <- struct{}{}
		<-steps

		// Scanner c should just see 3 and 4.
		advance(t, ctx1, c, q, want[3], false)
		advance(t, ctx1, c, q, want[4], false)
		if err := trueCancel(ctx1, cancel, q, c); err != nil {
			t.Error(err)
		}

		// Scanner c2 should see 3, 4, lost 0, lost 1.
		advance(t, ctx2, c2, q, want[3], false)
		advance(t, ctx2, c2, q, want[4], false)
		advance(t, ctx2, c2, q, want[0], true)
		advance(t, ctx2, c2, q, want[1], true)

		// Scanner c3 will just take a look at 3.
		advance(t, ctx3, c3, q, want[3], false)

		steps <- struct{}{}
		<-steps

		// After this point, 3 and 4 were removed, so c2 should see those too.
		advance(t, ctx2, c2, q, want[3], true)
		advance(t, ctx2, c2, q, want[4], true)
		if err := trueCancel(ctx2, cancel2, q, c2); err != nil {
			t.Error(err)
		}

		// Since c3 only looked at 0 and 3, it'll only see losses for those two.
		advance(t, ctx3, c3, q, want[0], true)
		advance(t, ctx3, c3, q, want[3], true)
		if err := trueCancel(ctx3, cancel3, q, c3); err != nil {
			t.Error(err)
		}

		steps <- struct{}{} // Done. Every cursor should be canceled now.
	}()

	// Wait for the scan to read the first two values.
	<-steps

	// Remove a bunch of entries.
	for i, w := range want {
		q.remove(Invite{Syncgroup: wire.Id{Name: w}, key: w})
		if i == 2 {
			// Tell the scan to read the next two values
			steps <- struct{}{}
			// Wait for it to finish.
			<-steps
		}
		// A lost element replaces the removed found element.
		if i < 2 && q.size() != 5 {
			t.Errorf("got %d, want %d", q.size(), 5)
		}
		// #1 was garbage collected (c1, c2 canceled. c3 only saw #0, but not #1).
		// #2 was garbage collected (nobody saw it)
		// Thus, we need to see 3 items (3, 4, and lost 0)
		if i >= 2 && q.size() != 3 {
			t.Errorf("got %d, want %d", q.size(), 3)
		}
		if err := scanForFoundOnly(ctx, q, want[i+1:]); err != nil {
			t.Errorf("on iteration %d, scan for found only failed: %v", i, err)
		}
	}

	steps <- struct{}{} // return control to verify the scan for c2 and c3.
	<-steps

	// Verify that the queue is now empty.
	// Garbage collection of all lost elements should have occurred.
	if q.size() != 0 {
		t.Errorf("got %d, want 0", q.size())
	}
}

func advance(t *testing.T, ctx *context.T, c int, q *copyableQueue, want string, isLost bool) {
	if inv, ok := q.next(ctx, c); !ok {
		t.Error("next should have suceeded.")
	} else if inv.(Invite).Syncgroup.Name != want {
		t.Errorf("next should be %s, but got: %s", want, inv.(Invite).Syncgroup.Name)
	} else if inv.(Invite).Lost && !isLost {
		t.Error("next should have been found")
	} else if !inv.(Invite).Lost && isLost {
		t.Error("next should have been lost")
	}
}

func logList(ctx *context.T, q *copyableQueue) {
	buf := &bytes.Buffer{}
	for e := q.sentinel.next; e != &q.sentinel; e = e.next {
		fmt.Fprintf(buf, "%p ", e)
	}
	ctx.Info("list", buf.String())
}

func scanForFoundOnly(ctx *context.T, q *copyableQueue, want []string) error {
	ctx, cancel := context.WithCancel(ctx)
	c := q.scan()
	for i, w := range want {
		inv, ok := q.next(ctx, c)

		// Skip the lost for this test.
		for inv.isLost() {
			inv, ok = q.next(ctx, c)
		}

		if !ok {
			return fmt.Errorf("scan ended after %d entries, wanted %d", i, len(want))
		}
		if got := inv.(Invite).Syncgroup.Name; got != w {
			return fmt.Errorf("got %s, want %s", got, w)
		}
		if inv.(Invite).Lost {
			return fmt.Errorf("invite %v was lost", inv)
		}
	}
	if err := trueCancel(ctx, cancel, q, c); err != nil {
		return err
	}

	return nil
}

func trueCancel(ctx *context.T, cancel context.CancelFunc, q *copyableQueue, c int) error {
	cancel()
	q.stopScan(c)
	if unwanted, ok := q.next(ctx, c); ok {
		return fmt.Errorf("next returned %v after scan canceled", unwanted)
	}
	return nil
}
