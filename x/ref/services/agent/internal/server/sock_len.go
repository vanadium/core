// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

// #include <sys/un.h>
import "C"

func GetMaxSockPathLen() int {
	var t C.struct_sockaddr_un
	return len(t.sun_path) - 1 // -1 for the \0 that's part of c strings.
}
