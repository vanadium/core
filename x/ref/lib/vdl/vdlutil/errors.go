// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdlutil

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
)

// Errors holds a buffer of encountered errors.  The point is to try displaying
// all errors to the user rather than just the first.  We cutoff at MaxErrors to
// ensure if something's really messed up we won't spew errors forever.  Set
// MaxErrors=-1 to effectively continue despite any number of errors.  The zero
// Errors struct stops at the first error encountered.
type Errors struct {
	MaxErrors int
	buf       bytes.Buffer
	num       int
}

// NewErrors returns a new Errors object, holding up to max errors.
func NewErrors(max int) *Errors {
	return &Errors{MaxErrors: max}
}

// Error adds the error described by msg to the buffer.  Returns true iff we're
// still under the MaxErrors cutoff.
func (e *Errors) Error(msg string) bool {
	if e.num == e.MaxErrors {
		return false
	}
	msg1 := "#" + strconv.Itoa(e.num) + " " + msg + "\n"
	e.buf.WriteString(msg1)
	if e.num++; e.num == e.MaxErrors {
		msg2 := fmt.Sprintf("...stopping after %d error(s)...\n", e.num)
		e.buf.WriteString(msg2)
		return false
	}
	return true
}

// Errorf is like Error, and takes the same args as fmt.Printf.
func (e *Errors) Errorf(format string, v ...interface{}) bool {
	return e.Error(fmt.Sprintf(format, v...))
}

// String returns the buffered errors as a single human-readable string.
func (e *Errors) String() string {
	return e.buf.String()
}

// ToError returns the buffered errors as a single error, or nil if there
// weren't any errors.
func (e *Errors) ToError() error {
	if e.num == 0 {
		return nil
	}
	return errors.New(e.buf.String())
}

// IsEmpty returns true iff there weren't any errors.
func (e *Errors) IsEmpty() bool {
	return e.num == 0
}

// IsFull returns true iff we hit the MaxErrors cutoff.
func (e *Errors) IsFull() bool {
	return e.num == e.MaxErrors
}

// NumErrors returns the number of errors we've seen.
func (e *Errors) NumErrors() int {
	return e.num
}

// Reset clears the internal state so you start with no buffered errors.
// MaxErrors remains the same; if you want to change it you should create a new
// Errors struct.
func (e *Errors) Reset() {
	e.buf = bytes.Buffer{}
	e.num = 0
}
