// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package test implements initialization for unit and integration tests.
//
// V23Init can be used within test functions as a safe alternative to v23.Init.
// It sets up the context so that only localhost ports are used for
// communication.
//
// func TestFoo(t *testing.T) {
//    ctx, shutdown := test.V23Init()
//    defer shutdown()
//    ...
// }
//
// This package also defines flags for enabling and controlling the Vanadium
// integration tests in package v.io/x/ref/test/v23test:
//   --v23.tests - run the integration tests
//   --v23.tests.shell-on-fail - drop into a debug shell if the test fails
//
// Typical usage is:
// $ jiri go test . --v23.tests
//
// Note that, like all flags not recognized by the go testing package, the
// --v23.tests flags must follow the package spec.
//
// Subdirectories provide utilities for unit and integration tests.
//
// The subdirectories are:
// benchmark  - support for writing benchmarks.
// testutil   - utility functions used in tests.
// security   - security-related utility functions used in tests.
// timekeeper - an implementation of the timekeeper interface for use within
//              tests.
// expect     - support for testing the contents of of an input stream (an
//              io.Reader). v23test.Cmd contains an expect.Session, so this
//              package is generally not used directly.
// v23test    - defines Shell, which provides support for spawning and managing
//              subprocesses with configurable credentials.
package test
