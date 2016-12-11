// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package server

import (
	"testing"
)

func TestMaxSockPathLen(t *testing.T) {
	if length := GetMaxSockPathLen(); length != 103 {
		t.Errorf("Expected max socket path length to be 103 on darwin, got %d instead", length)
	}
}
