// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file contains tunable parameters of the local blobstore implementation.

package fs_cablobstore

var (
	// maxFragmentSize is the maxiumum number of bytes placed in a fragment.
	maxFragmentSize int64 = 1024 * 1024
)
