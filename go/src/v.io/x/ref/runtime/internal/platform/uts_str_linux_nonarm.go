// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux,!arm

package platform

// str converts the input byte slice to a string, ignoring everything following
// a null character (including the null character).
func utsStr(c []int8) string {
	ret := make([]byte, 0, len(c))
	for _, v := range c {
		if v == 0 {
			break
		}
		ret = append(ret, byte(v))
	}
	return string(ret)
}
