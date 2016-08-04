// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdlutil

import (
	"io/ioutil"
	"log"
	"os"
)

const (
	logPrefix = ""
	logFlags  = log.Lshortfile | log.Ltime | log.Lmicroseconds
)

var (
	// Vlog is a logger that discards output by default, and only outputs real
	// logs when SetVerbose is called.
	Vlog = log.New(ioutil.Discard, logPrefix, logFlags)
)

// SetVerbose tells the vdl package (and subpackages) to enable verbose logging.
func SetVerbose() {
	Vlog = log.New(os.Stderr, logPrefix, logFlags)
}
