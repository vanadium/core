package slang

import (
	"fmt"
	"testing"

	"v.io/v23/context"
)

func parseAndCompileAndExecute(ctx *context.T, src string) (*Script, error) {
	scr := &Script{}
	err := scr.ExecuteBytes(ctx, []byte(src))
	return scr, err
}

func TestExecute(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()
	for i, tc := range []struct {
		script  string
		runtime string
	}{
		{"fn1()", ""},
		{"a := fn2()", "a: int: 3\n"},
		{"b := fn3(44)", "b: int: 44\n"},
		{`c := fn4("oh my")`, `c: *slang.tt1: oh my
`},
		{`d,f := fn10("1h30s", false)`, `d: time.Duration: 1h0m30s
f: bool: false
`},
		{`d,f := fn10("1h30s", true); s := fn7("duration: %v, bool %v", d,f)`, `d: time.Duration: 1h0m30s
f: bool: true
s: string: duration: 1h0m30s, bool true
`},
	} {
		exec, err := parseAndCompileAndExecute(ctx, tc.script)
		if err != nil {
			t.Errorf("%v: failed to compile: %v", i, err)
			continue
		}
		if got, want := exec.runtimeState(), tc.runtime; got != want {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
	}
}

func fnError(rt Runtime) error {
	return fmt.Errorf("Ooops")
}

func init() {
	RegisterFunction(fnError, "")
}

func TestExecuteError(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()
	scr, err := parseAndCompileAndExecute(ctx, `
	x := sprintf("foo")
	fnError()
	y := sprintf("y %s", x)
	`)
	if err == nil || err.Error() != "3:2: Ooops" {
		t.Errorf("unexpected error: %v", err)
	}
	vars := scr.Variables()
	if _, ok := vars["y"]; ok {
		t.Errorf("made it past an error")
	}

}
