// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"fmt"

	"v.io/x/lib/netstate"
)

func main() {
	al, err := netstate.GetAll()
	if err != nil {
		fmt.Printf("error getting networking state: %s", err)
		return
	}
	for _, a := range al {
		fmt.Println(a)
	}
}
