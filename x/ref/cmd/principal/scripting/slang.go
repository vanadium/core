package scripting

import (
	"fmt"
	"io"
	"os"
	"strings"

	"v.io/v23/context"
	"v.io/x/ref/lib/slang"
)

func RunScript(ctx *context.T, buf []byte) error {
	scr := &slang.Script{}
	registerTimeFormats(scr)
	return scr.ExecuteBytes(ctx, buf)
}

func RunFrom(ctx *context.T, rd io.Reader) error {
	buf, err := io.ReadAll(rd)
	if err != nil {
		return err
	}
	return RunScript(ctx, buf)
}

func RunFile(ctx *context.T, name string) error {
	if name == "-" || len(name) == 0 {
		return RunFrom(ctx, os.Stdin)
	}
	rd, err := os.Open(name)
	if err != nil {
		return err
	}
	return RunFrom(ctx, rd)
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
