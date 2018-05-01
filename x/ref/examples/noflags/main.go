// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $GOPATH/src/v.io/vendor/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"v.io/v23"
	"v.io/x/ref/runtime/factories/library"
)

func init() {
	library.AlsoLogToStderr = true
}

func main() {
	ctx, shutdown := v23.Init()
	ctx.Infof("hello world")
	defer shutdown()
}
