// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package exec implements configuration and secret-sharing between parent and
// child processes via anoymous pipes.  Anonymous pipes are used since they are
// the most secure communication channel available.
//
// Once a parent starts a child process it can use WaitForReady to wait for the
// child to reach its 'Ready' state.  Operations are provided to wait for the
// child to terminate, and to terminate the child, cleaning up any state
// associated with it.
//
// A child process uses the GetChildHandle function to complete the initial
// authentication handshake.  The child must call SetReady to indicate that it
// is fully initialized and ready for whatever purpose it is intended to
// fulfill.  This handshake is referred as the 'exec protocol'.
package exec
