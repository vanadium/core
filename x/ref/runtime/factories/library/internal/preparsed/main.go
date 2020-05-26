// Copyright 2017 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// An example using the 'library' factory which is configured via exported
// variables rather than by command line flags.
package main

import (
	"flag"
	"fmt"
	"os"

	v23 "v.io/v23"
	"v.io/x/ref/runtime/factories/library"
)

func main() {
	myfs := &flag.FlagSet{}
	bv := false
	myfs.BoolVar(&bv, "test", true, "set to true")
	if err := library.EnableFlags(myfs, true); err != nil {
		os.Exit(10)
	}
	_, shutdown := v23.Init()
	defer shutdown()
	myfs.VisitAll(func(f *flag.Flag) { fmt.Printf("%v\n", f.Name) })
}
