package slang

import (
	"fmt"
	"strings"
	"testing"
	"time"

	vcontext "v.io/v23/context"
)

func fn1(rt Runtime) error {
	return nil
}

func fn2(rt Runtime) (int, error) {
	return 3, nil
}

func fn3(rt Runtime, v int) (int, error) {
	return v, nil
}

type tt1 struct {
	msg string
}

func (t *tt1) String() string {
	return t.msg
}

func fn4(rt Runtime, s string) (*tt1, error) {
	return &tt1{msg: s}, nil
}

func fn5(rt Runtime, v *tt1) error {
	return fmt.Errorf("%v", v.msg)
}

func fn6(rt Runtime, duration time.Duration, flag bool) error {
	return nil
}

func fn7(rt Runtime, format string, args ...interface{}) (string, error) {
	return fmt.Sprintf(format, args...), nil
}

func fn8(rt Runtime, format string, second int, args ...interface{}) (string, error) {
	return fmt.Sprintf(format, args...), nil
}

func fn9(rt Runtime, args ...string) (string, error) {
	return strings.Join(args, ", "), nil
}

func fn10(rt Runtime, duration time.Duration, flag bool) (time.Duration, bool, error) {
	return duration, flag, nil
}

func stringv(rt Runtime, args ...string) ([]string, error) {
	return args, nil
}

func init() {
	RegisterFunction(fn1, "test", "")
	RegisterFunction(fn2, "test", "")
	RegisterFunction(fn3, "test", "", "v")
	RegisterFunction(fn4, "test", "", "s", "msg")
	RegisterFunction(fn5, "test", "", "v")
	RegisterFunction(fn6, "test", "", "duration", "flag")
	RegisterFunction(fn7, "test", "", "format", "args")
	RegisterFunction(fn8, "test", "", "format", "second", "args")
	RegisterFunction(fn9, "test", "", "args")
	RegisterFunction(fn10, "test", "", "duration", "flag")
	RegisterFunction(stringv, "test", "", "args")

}

func parseAndCompile(src string) (string, error) {
	stmts, err := parseBytes([]byte(src))
	if err != nil {
		return "", err
	}
	out := &strings.Builder{}
	scr := &Script{}
	if err := scr.compile(stmts); err != nil {
		return "", err
	}
	scr.formatCompiledState(out)
	return out.String(), nil
}

func TestCompile(t *testing.T) {
	for i, tc := range []struct {
		script   string
		compiled string
	}{
		{"fn1()", "1:1: fn1() :: fn1()\n"},
		{"a:=fn2()", "a: int\n1:1: a := fn2() :: fn2() int\n"},
		{"a:=fn2(); b:=fn3(a)", `a: int
b: int
1:1: a := fn2() :: fn2() int
1:11: b := fn3(a) :: fn3(v int) int
`},
		{"b:=fn3(33)", `b: int
1:1: b := fn3(33) :: fn3(v int) int
`},
		{`tt:=fn4("msg")`, `tt: *slang.tt1
1:1: tt := fn4("msg") :: fn4(s string) (msg *slang.tt1)
`},
		{`tt:=fn4("msg"); fn5(tt)`, `tt: *slang.tt1
1:1: tt := fn4("msg") :: fn4(s string) (msg *slang.tt1)
1:17: fn5(tt) :: fn5(v *slang.tt1)
`},
		{`fn6("1h", false)`, `1:1: fn6(1h0m0s, false) :: fn6(duration time.Duration, flag bool)
`},
		{`fn6("1h", true)`, `1:1: fn6(1h0m0s, true) :: fn6(duration time.Duration, flag bool)
`},
		{`s := fn7("format")`, `s: string
1:1: s := fn7("format") :: fn7(format string, args ...interface {}) string
`},
		{`s := fn7("format"); s = fn7("new format")`, `s: string
1:1: s := fn7("format") :: fn7(format string, args ...interface {}) string
1:21: s = fn7("new format") :: fn7(format string, args ...interface {}) string
`},
		{`s := fn7("%v", 3)`, `s: string
1:1: s := fn7("%v", 3) :: fn7(format string, args ...interface {}) string
`},
		{`s := fn7("%v %v", 3, 2)`, `s: string
1:1: s := fn7("%v %v", 3, 2) :: fn7(format string, args ...interface {}) string
`},
		{`s := fn7("%v %v", "msg", 2)`, `s: string
1:1: s := fn7("%v %v", "msg", 2) :: fn7(format string, args ...interface {}) string
`},
		{`d,f := fn10("1h30s", false); s := fn7("duration: %v, bool %v", d,f)`, `d: time.Duration
f: bool
s: string
1:1: d, f := fn10(1h0m30s, false) :: fn10(duration time.Duration, flag bool) (time.Duration, bool)
1:30: s := fn7("duration: %v, bool %v", d, f) :: fn7(format string, args ...interface {}) string
`},
		{`va := stringv("a", "b", "c"); vb := stringv(va...)`, `va: []string
vb: []string
1:1: va := stringv("a", "b", "c") :: stringv(args ...string) []string
1:31: vb := stringv(va...) :: stringv(args ...string) []string
`},
	} {
		out, err := parseAndCompile(tc.script)
		if err != nil {
			t.Errorf("%v: failed to compile: %v %v", i, tc.script, err)
			continue
		}
		if got, want := out, tc.compiled; got != want {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
	}
}

func TestCompileErrors(t *testing.T) {
	for i, tc := range []struct {
		script string
		msg    string
	}{
		{"fnx()", "1:1: unrecognised function name: fnx"},
		{"fn1(33)", "1:1: wrong # of arguments for fn1()"},
		{"fn6()", "1:1: wrong # of arguments for fn6(duration time.Duration, flag bool)"},
		{"x:=fn1()", "1:1: wrong # of results for fn1()"},
		{"fn2()", "1:1: wrong # of results for fn2() int"},
		{`a:=fn3("xxx")`, `1:1: 1:8: literal arg '"xxx"' of type string is not assignable to int`},
		{"a:=fn3(3); fn5(a)", `1:12: 1:16: arg 'a' is of the wrong type int, should be *slang.tt1`},
		{`fn6("1h", fals)`, `1:1: 1:11: arg 'fals' is an invalid literal: not a bool: must be one of 'true' or 'false'`},
		{`fn6("1d", fals)`, `1:1: 1:5: arg '"1d"' is an invalid literal: time: unknown unit "d" in duration "1d"`},
		{`a:=fn3(1); a := fn4("a")`, `1:12: result 'a' is redefined with a new type *slang.tt1, previous type int`},
		{`a:=fn3(1); a := fn3(2)`, `1:12: result 'a' is redefined`},
		{`a=fn3(1)`, `1:1: result 'a' is not defined`},
		{`s:=fn7()`, "1:1: need at least 1 arguments for variadic function fn7(format string, args ...interface {}) string"},
		{`s:=fn8("format")`, "1:1: need at least 2 arguments for variadic function fn8(format string, second int, args ...interface {}) string"},
		{`s:=fn9(13)`, `1:1: 1:8: literal arg '13' of type int is not assignable to string`},
		{`s:=fn9("a", 14)`, `1:1: 1:13: literal arg '14' of type int is not assignable to string`},
		{`va := stringv("a", "b", "c"); o := sprintf("%v", va...)`, `1:31: 1:50: arg 'va' is of the wrong type []string, should be []interface {}`},
	} {

		_, err := parseAndCompile(tc.script)
		if err == nil {
			t.Errorf("%v: should have failed to compile, no error returned", i)
			continue
		}
		if got, want := err.Error(), tc.msg; got != want {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
	}

}

func TestConsts(t *testing.T) {
	ctx, cancel := vcontext.RootContext()
	defer cancel()

	out := &strings.Builder{}
	scr := &Script{}
	scr.SetStdout(out)
	now := time.Now()
	scr.RegisterConst("startTime", now.Format(time.RFC1123))
	err := scr.ExecuteBytes(ctx, []byte(`printf("now: %s",startTime)`))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "now: "+now.Format(time.RFC1123); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	err = scr.ExecuteBytes(ctx, []byte(`startTime = sprintf("a")`))
	if err == nil || err.Error() != `1:1: cannot assign or define result 'startTime' which is a constant` {
		t.Errorf("missing or unexpected error: %v", err)
	}
	err = scr.ExecuteBytes(ctx, []byte(`a := fn3(startTime)`))
	if err == nil || err.Error() != `1:1: 1:10: arg 'startTime' is of the wrong type string, should be int` {
		t.Errorf("missing or unexpected error: %v", err)
	}
}
