// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package hello defines a simple client and server and uses them in a series
// of regression tests.  The idea is for these programs to have stable interfaces
// (command line flags, output, etc) so that the test code won't need to change
// as the underlying framework changes.
package hello
