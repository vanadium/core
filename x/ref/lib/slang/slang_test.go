// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package slang_test

import (
	"os"
	"runtime"
	"strings"
	"testing"

	vcontext "v.io/v23/context"
	"v.io/x/ref/lib/slang"
)

func TestBuiltins(t *testing.T) {
	ctx, cancel := vcontext.RootContext()
	defer cancel()

	out := &strings.Builder{}
	var scr *slang.Script

	execute := func(script string) {
		out.Reset()
		scr.SetStdout(out)
		if err := scr.ExecuteBytes(ctx, []byte(script)); err != nil {
			_, _, line, _ := runtime.Caller(1)
			t.Fatalf("line %v: script error: %v", line, err)
		}
	}

	scr = &slang.Script{}
	execute("listFunctions()")
	if got, want := out.String(), "fn10"; !strings.Contains(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	scr = &slang.Script{}
	execute(`printf("format %q %v", "msg", true)`)
	if got, want := out.String(), `format "msg" true`; got != want {
		t.Errorf("got %v does not contain %v", got, want)
	}

	scr = &slang.Script{}
	execute(`x:=sprintf("format %q %v", "oops", 42); y := expandEnv("$HOME/dummy")`)
	vars := scr.Variables()
	if got, want := vars["x"].(string), `format "oops" 42`; !strings.Contains(got, want) {
		t.Errorf("got %v does not contain %v", got, want)
	}
	if got, want := vars["y"].(string), "$HOME"; strings.Contains(got, want) && len(got) > len("/dummy") {
		t.Errorf("got %v does not contain %v", got, want)
	}

	scr = &slang.Script{}
	execute(`help(listFunctions)`)
	if got, want := out.String(), "listFunctions(tags ...string)\n  list available functions\n"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	scr = &slang.Script{}
	scr.SetEnv("MY_VAR", "my_val")
	execute(`v := expandEnv("$MY_VAR/foo"); printf("%s", v)`)
	if got, want := out.String(), "my_val/foo"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	scr = &slang.Script{}
	execute(`v := expandEnv("$MY_NON_VAR/foo"); printf("%s", v)`)
	if got, want := out.String(), "/foo"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	scr = &slang.Script{}
	home := os.Getenv("HOME")
	execute(`v := expandEnv("$HOME/foo"); printf("%s", v)`)
	if got, want := out.String(), home+"/foo"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	scr = &slang.Script{}
	scr.SetEnv("MY_VAR", "my_val")
	scr.SetEnv("HOME", "myhome")
	execute(`v := expandEnv("$HOME/foo"); printf("%s", v)`)
	if got, want := out.String(), "myhome/foo"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := scr.String(), `$HOME=myhome
$MY_VAR=my_val
v: string
v: string: myhome/foo
1:1: v := expandEnv("$HOME/foo") :: expandEnv(value string) string
1:30: printf("%s", v) :: printf(format string, args ...interface {})
v: string: myhome/foo
`; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

}

func TestExecute(t *testing.T) {
	ctx, cancel := vcontext.RootContext()
	defer cancel()

	scr := &slang.Script{}
	err := scr.ExecuteBytes(ctx, []byte("help(undefined)"))
	if err == nil || err.Error() != "1:1: 1:6: arg 'undefined' is an invalid literal: invalid identifier" {
		t.Errorf("missing or unexpected error: %v", err)
	}

	scr = &slang.Script{}
	err = scr.ExecuteBytes(ctx, []byte(`help("undefined")`))
	if err == nil || err.Error() != `1:1: help: unrecognised function: "undefined"` {
		t.Errorf("missing or unexpected error: %v", err)
	}

	if got, want := scr.Context(), ctx; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRegistered(t *testing.T) {
	rf := slang.RegisteredFunctions()
	if got, want := len(rf), 18; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := rf[0].Function, "expandEnv(value string) string"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	rf = slang.RegisteredFunctions("test")
	if got, want := len(rf), 11; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := rf[0].Function, "fn1()"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
