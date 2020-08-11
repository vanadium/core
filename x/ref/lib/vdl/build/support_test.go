// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package build

// SetFilePathSeparator is for use from within tests to fake out working
// on windows style filesystems.
func SetFilePathSeparator(sep string) {
	filePathSeparator = sep
}
