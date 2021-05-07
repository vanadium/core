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
	scr := &slang.Script{Stdout: out}

	execute := func(script string) {
		out.Reset()
		if err := scr.ExecuteBytes(ctx, []byte(script)); err != nil {
			_, _, line, _ := runtime.Caller(1)
			t.Fatalf("line %v: script error: %v", line, err)
		}
	}

	execute("listFunctions()")
	if got, want := out.String(), "fn10"; !strings.Contains(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	execute(`printf("format %q %v", "msg", true)`)
	if got, want := out.String(), `format "msg" true`; got != want {
		t.Errorf("got %v does not contain %v", got, want)
	}

	execute(`x:=sprintf("format %q %v", "oops", 42); y := expandEnv("$HOME/dummy")`)
	vars := scr.Variables()
	if got, want := vars["x"].(string), `format "oops" 42`; !strings.Contains(got, want) {
		t.Errorf("got %v does not contain %v", got, want)
	}
	if got, want := vars["y"].(string), "$HOME"; strings.Contains(got, want) && len(got) > len("/dummy") {
		t.Errorf("got %v does not contain %v", got, want)
	}

	execute(`help(listFunctions)`)
	if got, want := out.String(), "listFunctions()\n  list available functions\n"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	scr.SetEnv("MY_VAR", "my_val")
	execute(`v := expandEnv("$MY_VAR/foo"); printf("%s", v)`)
	if got, want := out.String(), "my_val/foo"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	execute(`v := expandEnv("$MY_NON_VAR/foo"); printf("%s", v)`)
	if got, want := out.String(), "/foo"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	home := os.Getenv("HOME")
	execute(`v := expandEnv("$HOME/foo"); printf("%s", v)`)
	if got, want := out.String(), home+"/foo"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

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

	err := scr.ExecuteBytes(ctx, []byte("help(undefined)"))
	if err == nil || err.Error() != "1:1: 1:6: arg 'undefined' is an invalid literal: invalid identifier" {
		t.Errorf("missing or unexpected error: %v", err)
	}

}
