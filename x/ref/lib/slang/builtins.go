package slang

import (
	"fmt"
	"os"
	"sort"

	"v.io/v23/context"
	"v.io/x/lib/textutil"
)

func (scr *Script) Context() *context.T {
	return scr.ctx
}

func (scr *Script) Printf(format string, args ...interface{}) {
	fmt.Fprintf(scr.Stdout, format, args...)
}

func (scr *Script) Help(cmd string) error {
	v, ok := supportedVerbs[cmd]
	if !ok {
		return fmt.Errorf("help: unrecognised function: %q", cmd)
	}
	fmt.Fprintln(scr.Stdout, v.String())
	wr := textutil.NewUTF8WrapWriter(scr.Stdout, 70)
	wr.SetIndents("  ", "  ")
	wr.Write([]byte(v.help))
	return wr.Flush()
}

func (scr *Script) ListFunctions() {
	names := make([]string, 0, len(supportedVerbs))
	for k := range supportedVerbs {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(scr.Stdout, "%s++\n", n)
	}
}

func (scr *Script) ExpandEnv(s string) string {
	if scr.envvars == nil {
		return os.ExpandEnv(s)
	}
	e := os.Expand(s, func(vn string) string {
		if v, ok := scr.envvars[vn]; ok {
			return v
		}
		return "${" + vn + "}"
	})
	return os.ExpandEnv(e)
}

func printf(rt Runtime, format string, args ...interface{}) error {
	rt.Printf(format, args...)
	return nil
}

func sprintf(rt Runtime, format string, args ...interface{}) (string, error) {
	return fmt.Sprintf(format, args...), nil
}

func listFunctions(rt Runtime) error {
	rt.ListFunctions()
	return nil
}

func expandEnv(rt Runtime, s string) (string, error) {
	return rt.ExpandEnv(s), nil
}

func help(rt Runtime, cmd string) error {
	return rt.Help(cmd)
}

func init() {
	RegisterFunction(printf, `equivalent to fmt.Printf`)
	RegisterFunction(sprintf, `equivalent to fmt.Sprintf`)
	RegisterFunction(listFunctions, `list available functions`)
	RegisterFunction(expandEnv, `perform shell environment variable expansion`)
	RegisterFunction(help, "display information for a specific function")
}
