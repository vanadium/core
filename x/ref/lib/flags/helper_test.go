// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

func MergedForTest() map[string]interface{} {
	return mergeDefaultValues()
}
