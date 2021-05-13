// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slang_test

import (
	"v.io/v23/context"
	"v.io/x/ref/lib/slang"
)

func ExampleScript() {
	ctx, cancel := context.RootContext()
	defer cancel()
	scr := &slang.Script{}
	err := scr.ExecuteBytes(ctx, []byte(`
	s:=sprintf("hello %v", "world")
	printf("%s\n",s)
	`))
	if err != nil {
		panic(err)
	}

	// Output:
	// hello world
}
