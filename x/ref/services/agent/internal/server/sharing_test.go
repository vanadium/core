// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"testing"

	_ "v.io/x/ref/runtime/factories/generic"
)

func doRead(c chan struct{}) string {
	select {
	case _, ok := <-c:
		if !ok {
			return "closed"
		}
		return "ok"
	default:
		return "nada"
	}
}

func TestNotify(t *testing.T) {
	p := newWatchers()
	a1, a2 := p.newID(), p.newID()
	c1, c2 := p.register(a1), p.register(a2)

	if got := doRead(c1); got != "nada" {
		t.Errorf("agent1: Unexpected %s", got)
	}
	if got := doRead(c2); got != "nada" {
		t.Errorf("agent2: Unexpected %s", got)
	}
	p.lock()
	p.unlock(a1)

	if got := doRead(c1); got != "nada" {
		t.Errorf("agent1: Unexpected %s", got)
	}
	if got := doRead(c2); got != "ok" {
		t.Errorf("agent2: Unexpected %s, wanted ok", got)
	}

	p.lock()
	p.unlock(a2)
	if got := doRead(c2); got != "nada" {
		t.Errorf("agent2: Unexpected %s", got)
	}
	if got := doRead(c1); got != "ok" {
		t.Errorf("agent1: Unexpected %s, wanted ok", got)
	}

	p.unregister(a2, c2)
	if got := doRead(c2); got != "closed" {
		t.Errorf("agent2: Unexpected %s", got)
	}

	p.lock()
	p.unlock(a1)
	if got := doRead(c1); got != "nada" {
		t.Errorf("agent1: Unexpected %s", got)
	}

	p.lock()
	p.unlock(a2)
	if got := doRead(c1); got != "ok" {
		t.Errorf("agent1: Unexpected %s, wanted ok", got)
	}
}
