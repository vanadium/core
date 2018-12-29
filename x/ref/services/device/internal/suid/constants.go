// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package suid

const (
	// fd of the pipe to be used to return the pid of the forked child to the
	// device manager.
	PipeToParentFD = 3
)
