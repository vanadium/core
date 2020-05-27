// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goroutines

import (
	"bytes"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

const leakWaitTime = 250 * time.Millisecond

func wrappedWaitForIt(wg *sync.WaitGroup, wait chan struct{}, n int64) {
	if n == 0 {
		waitForIt(wg, wait)
	} else {
		wrappedWaitForIt(wg, wait, n-1)
	}
}

func waitForIt(wg *sync.WaitGroup, wait chan struct{}) {
	wg.Done()
	<-wait
}

func runGoA(wg *sync.WaitGroup, wait chan struct{}) {
	go waitForIt(wg, wait)
}

func runGoB(wg *sync.WaitGroup, wait chan struct{}) {
	go wrappedWaitForIt(wg, wait, 3)
}

func runGoC(wg *sync.WaitGroup, wait chan struct{}) {
	go func() {
		wg.Done()
		<-wait
	}()
}

func TestGet(t *testing.T) {
	defer NoLeaks(t, leakWaitTime)()
	var wg sync.WaitGroup
	wg.Add(3)
	wait := make(chan struct{})
	runGoA(&wg, wait)
	runGoB(&wg, wait)
	runGoC(&wg, wait)
	wg.Wait()
	gs, err := Get()
	if err != nil {
		t.Fatal(err)
	}
	close(wait)

	if len(gs) < 4 {
		t.Errorf("Got %d goroutines, expected at least 4", len(gs))
	}
	bycreator := map[string]*Goroutine{}
	for _, g := range gs {
		key := ""
		if g.Creator != nil {
			key = g.Creator.Call
		}
		bycreator[key] = g
	}
	a := bycreator["v.io/x/ref/test/goroutines.runGoA"]
	switch {
	case a == nil:
		t.Errorf("runGoA is missing")
	case len(a.Stack) < 1:
		t.Errorf("got %d expected at least 1: %s", len(a.Stack), Format(a))
	case !strings.HasPrefix(a.Stack[0].Call, "v.io/x/ref/test/goroutines.waitForIt"):
		t.Errorf("got %s, wanted it to start with v.io/x/ref/test/goroutines.waitForIt",
			a.Stack[0].Call)
	}
	b := bycreator["v.io/x/ref/test/goroutines.runGoB"]
	if b == nil {
		t.Errorf("runGoB is missing")
	} else if len(b.Stack) < 5 {
		t.Errorf("got %d expected at least 5: %s", len(b.Stack), Format(b))
	}
	c := bycreator["v.io/x/ref/test/goroutines.runGoC"]
	if c == nil {
		t.Errorf("runGoC is missing")
	} else if len(c.Stack) < 1 {
		t.Errorf("got %d expected at least 1: %s", len(c.Stack), Format(c))
	}
}

func TestFormat(t *testing.T) {
	defer NoLeaks(t, leakWaitTime)()

	var wg sync.WaitGroup
	wg.Add(3)
	wait := make(chan struct{})
	runGoA(&wg, wait)
	runGoB(&wg, wait)
	runGoC(&wg, wait)
	wg.Wait()

	buf := make([]byte, 1<<20)
	buf = buf[:runtime.Stack(buf, true)]
	close(wait)

	gs, err := Parse(buf, false)
	if err != nil {
		t.Fatal(err)
	}
	if formatted := Format(gs...); !bytes.Equal(buf, formatted) {
		t.Errorf("got:\n%s\nwanted:\n%s\n", string(formatted), string(buf))
	}
}

type fakeErrorReporter struct {
	calls     int
	extra     int
	formatted string
}

func (f *fakeErrorReporter) Errorf(format string, args ...interface{}) {
	f.calls++
	f.extra = args[0].(int)
	f.formatted = args[1].(string)
}

func TestNoLeaks(t *testing.T) {
	er := &fakeErrorReporter{}
	f := NoLeaks(er, leakWaitTime)

	var wg sync.WaitGroup
	wg.Add(3)
	wait := make(chan struct{})
	runGoA(&wg, wait)
	runGoB(&wg, wait)
	runGoC(&wg, wait)
	wg.Wait()

	f()
	if er.calls != 1 {
		t.Errorf("got %d, wanted 1: %s", er.calls, er.formatted)
	}
	if er.extra != 3 {
		t.Errorf("got %d, wanted 3: %s", er.extra, er.formatted)
	}
	close(wait)

	*er = fakeErrorReporter{}
	f()
	if er.calls != 0 {
		t.Errorf("got %d, wanted 0: %s", er.calls, er.formatted)
	}
	if er.extra != 0 {
		t.Errorf("got %d, wanted 0: %s", er.extra, er.formatted)
	}
}
