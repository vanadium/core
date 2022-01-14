// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
)

func main() {
	contents, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	fmt.Println("buildtest/main")
	fmt.Println(string(contents))
}
