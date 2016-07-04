// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Binary leveldb_fsck performs consistency checking
// of a leveldb-based syncbase store.
package main

import "flag"
import "fmt"
import "os"

import "v.io/x/ref/services/syncbase/store/leveldb_fsck"

// The following imports give access to the relevant types in Syncbase.
import _ "v.io/x/ref/services/syncbase/server"
import _ "v.io/x/ref/services/syncbase/vsync"

func main() {
	var rc int

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s <directory>...\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	var nArgs int = flag.NArg()

	for i := 0; i != nArgs; i++ {
		var path string = flag.Arg(i)
		var errors []error
		leveldb_fsck.CheckDBRoot(path, &errors)
		for _, err := range errors {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		}
		// exit() with status 2 if there were errors.
		if len(errors) != 0 {
			rc = 2
		}
	}

	os.Exit(rc)
}
