// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scripting

import (
	"fmt"
	"io"
	"os"
	"strings"

	"v.io/v23/context"
	"v.io/x/ref/lib/slang"
)

func NewScript() *slang.Script {
	scr := &slang.Script{}
	registerTimeFormats(scr)
	return scr
}

func RunScript(ctx *context.T, compileOnly bool, buf []byte) error {
	scr := NewScript()
	if compileOnly {
		return scr.CompileBytes(buf)
	}
	return scr.ExecuteBytes(ctx, buf)

}

func RunFrom(ctx *context.T, compileOnly bool, rd io.Reader) error {
	buf, err := io.ReadAll(rd)
	if err != nil {
		return err
	}
	return RunScript(ctx, compileOnly, buf)
}

func RunFile(ctx *context.T, compileOnly bool, name string) error {
	if name == "-" || len(name) == 0 {
		return RunFrom(ctx, compileOnly, os.Stdin)
	}
	rd, err := os.Open(name)
	if err != nil {
		return err
	}
	return RunFrom(ctx, compileOnly, rd)
}

func Documentation() string {
	out := &strings.Builder{}

	underline(out, "Summary")
	fmt.Fprintln(out, format(slang.Summary), "")

	underline(out, "Literals")
	fmt.Fprintln(out, format(slang.Literals))

	underline(out, "Examples")
	fmt.Fprintln(out, slang.Examples)

	underline(out, "Available Functions")
	for _, tag := range []struct {
		tag, title string
	}{
		{"builtin", "builtin functions"},
		{"time", "time related functions"},
		{"print", "print/debug related functions"},
		{"principal", "security.Principal related functions"},
		{"blessings", "security.Blessings related functions"},
		{"caveats", "security.Caveat related functions"},
	} {
		underline(out, tag.title)
		for _, fn := range slang.RegisteredFunctions(tag.tag) {
			fmt.Fprintf(out, "%s\n", fn.Function)
			fmt.Fprint(out, format(fn.Help, "  "))
			fmt.Fprintln(out)
		}
	}
	return out.String()
}
