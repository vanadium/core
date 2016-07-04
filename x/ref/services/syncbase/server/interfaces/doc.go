// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO(sadovsky): Eliminate this directory. Most things should move into the
// syncbase/common directory, and the Service and Database interfaces should
// move into the vsync package.

// Package interfaces defines internal interfaces for various objects in the
// Syncbase server implementation. Defining these interfaces in a separate
// package helps prevent import cycles: all other packages can import the
// interfaces package, and individual modules can pass each other interfaces to
// enable bidirectional cross-package communication.
package interfaces
