// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testutil

// TODO(sadovsky): RetryFor is not really testing-specific; maybe move it
// elsewhere.

import (
	"fmt"
	"time"
)

// TryAgain should be returned by the function provided to RetryFor to indicate
// that the function should be retried. If the function returns a non-TryAgain
// error, RetryFor will exit.
func TryAgain(err error) error {
	return tryAgain{error: err}
}

type tryAgain struct{ error }

// TimeoutError is returned by RetryFor when the specified timeout is reached.
type TimeoutError struct {
	// Timeout is the user-specified timeout.
	Timeout time.Duration
	// LastErr is the most recent TryAgain error returned by the user-provided
	// function.
	LastErr error
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("timed out after %v: %v", e.Timeout, e.LastErr)
}

// RetryFor calls fn repeatedly, with a brief delay after each invocation, until
// either (1) fn returns something other than TryAgain or (2) the specified
// timeout is reached. RetryFor returns nil, TimeoutError, or the non-TryAgain
// error returned by fn.
func RetryFor(timeout time.Duration, fn func() error) error {
	deadline := time.Now().Add(timeout)
	for {
		err := fn()
		tryAgainErr, ok := err.(tryAgain)
		if !ok {
			return err
		}
		if time.Now().After(deadline) {
			return &TimeoutError{Timeout: timeout, LastErr: tryAgainErr.error}
		}
		time.Sleep(100 * time.Millisecond)
	}
}
