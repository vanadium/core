// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package servicetest

import (
	"fmt"
	"sync"
)

// Tape holds a record of function call stimuli and each function call's
// response. Use Tape to help build a mock framework by first creating a
// new Tape, then SetResponses to define the mock responses and then
// Record each function invocation. Play returns the function invocations
// for verification in a test.
type Tape struct {
	sync.Mutex
	stimuli   []interface{}
	responses []interface{}
}

// Record stores a new function invocation in a Tape and returns the
// response for that function interface.
func (t *Tape) Record(call interface{}) interface{} {
	t.Lock()
	defer t.Unlock()
	t.stimuli = append(t.stimuli, call)

	if len(t.responses) < 1 {
		// Returning an error at this point will likely cause the mock
		// using the tape to panic when it tries to cast the response
		// to the desired type.
		// Panic'ing here at least makes the issue more
		// apparent.
		// TODO(caprita): Don't panic.
		panic(fmt.Errorf("Record(%#v) had no response", call))
	}
	resp := t.responses[0]
	t.responses = t.responses[1:]
	return resp
}

// SetResponses updates the Tape's associated responses.
func (t *Tape) SetResponses(responses ...interface{}) {
	t.Lock()
	defer t.Unlock()
	t.responses = make([]interface{}, len(responses))
	copy(t.responses, responses)
}

// Rewind resets the tape to the beginning so that it could be used again
// for further tests.
func (t *Tape) Rewind() {
	t.Lock()
	defer t.Unlock()
	t.stimuli = make([]interface{}, 0)
	t.responses = make([]interface{}, 0)
}

// Play returns the function call stimuli recorded to this Tape.
func (t *Tape) Play() []interface{} {
	t.Lock()
	defer t.Unlock()
	resp := make([]interface{}, len(t.stimuli))
	copy(resp, t.stimuli)
	return resp
}

// NewTape creates a new Tape.
func NewTape() *Tape {
	t := new(Tape)
	t.Rewind()
	return t
}

// TapeMap provides multiple tapes for different strings. Use TapeMap to
// record separate Tapes for each suffix in a service.
type TapeMap struct {
	sync.Mutex
	tapes map[string]*Tape
}

// NewTapeMap creates a new empty TapeMap.
func NewTapeMap() *TapeMap {
	tm := &TapeMap{
		tapes: make(map[string]*Tape),
	}
	return tm
}

// ForSuffix returns the Tape for suffix s.
func (tm *TapeMap) ForSuffix(s string) *Tape {
	tm.Lock()
	defer tm.Unlock()
	t, ok := tm.tapes[s]
	if !ok {
		t = new(Tape)
		tm.tapes[s] = t
	}
	return t
}

// Rewind rewinds all of the Tapes in the TapeMap.
func (tm *TapeMap) Rewind() {
	tm.Lock()
	defer tm.Unlock()
	for _, t := range tm.tapes {
		t.Rewind()
	}
}
