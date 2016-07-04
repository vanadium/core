// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package checker defines checkers for syncbase longevity tests.
package checker

import (
	"v.io/v23/context"
	"v.io/x/ref/services/syncbase/longevity_tests/model"
)

// Checker is the interface that all checkers must implement.
type Checker interface {
	// Run should run the checker logic.
	// TODO(nlacasse): Consider making this return a slice of errors rather
	// than just one.
	Run(*context.T, model.Universe) error
}
