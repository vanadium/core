// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dependency

import (
	"testing"
	"time"
)

var nextID = 0

type Dep struct {
	deps    []*Dep
	stopped bool
	id      int
}

func NewDep(deps ...*Dep) *Dep {
	d := &Dep{deps: deps, id: nextID}
	nextID++
	return d
}

func (d *Dep) Use(t *testing.T, by *Dep) {
	if d.stopped {
		t.Errorf("Object %d using %d after stop.", by.id, d.id)
	}
}

func (d *Dep) Stop(t *testing.T) {
	d.Use(t, d)
	d.stopped = true
	for _, dd := range d.deps {
		dd.Use(t, d)
	}
}

func TestGraph(t *testing.T) {
	a := NewDep()
	b, c := NewDep(a), NewDep(a)
	d := NewDep(c)

	g := NewGraph()
	if err := g.Depend(a); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if err := g.Depend(b, a); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if err := g.Depend(c, a); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if err := g.Depend(d, c); err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	alldone := make(chan struct{})
	go func() {
		g.CloseAndWaitForAll()
		close(alldone)
	}()

	// Close d, which is a leaf.
	finish := g.CloseAndWait(d)
	d.Stop(t)
	finish()

	// Set a to close and wait which should wait for b and c.
	done := make(chan struct{})
	go func() {
		finish := g.CloseAndWait(a)
		a.Stop(t)
		finish()
		close(done)
	}()

	// done and alldone shouldn't be finished yet.
	select {
	case <-time.After(time.Second):
	case <-done:
		t.Errorf("done is finished before it's time")
	case <-alldone:
		t.Errorf("alldone is finished before it's time")
	}

	// Now close b and c.
	finish = g.CloseAndWait(b)
	b.Stop(t)
	finish()
	finish = g.CloseAndWait(c)
	c.Stop(t)
	finish()

	<-done
	<-alldone
}
