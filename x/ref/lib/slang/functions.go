package slang

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"v.io/v23/context"
)

type stype int

type verb struct {
	name           string
	implementation reflect.Value
	protoype       reflect.Type
	help           string
}

func (v verb) String() string {
	out := &strings.Builder{}
	out.WriteString(v.name)
	out.WriteRune('(')
	rt := v.protoype
	nIn := rt.NumIn()
	for i := 1; i < nIn; i++ {
		out.WriteString(rt.In(i).String())
		if i < nIn-1 {
			out.WriteString(", ")
		}
	}
	out.WriteRune(')')
	if rt.NumOut() == 1 {
		return out.String()
	}
	if rt.NumOut() > 2 {
		out.WriteString(" (")
	} else {
		out.WriteRune(' ')
	}
	nOut := rt.NumOut() - 1
	for i := 0; i < nOut; i++ {
		out.WriteString(rt.Out(i).String())
		if i < nOut-1 {
			out.WriteString(", ")
		}
	}
	if rt.NumOut() > 2 {
		out.WriteRune(')')
	}
	return out.String()
}

var (
	errorIfc   = reflect.TypeOf((*error)(nil)).Elem()
	runtimeIfc = reflect.TypeOf((*Runtime)(nil)).Elem()

	supportedVerbs = map[string]verb{}
)

// RegisterFunction registers a new function that can be called from a
// slang script and an associated 'help' describe describing its use.
// The type of registered functions must of the form:
//
//   func<Name>(Runtime, [zero-or-more-arguments]) ([zero-or-more-results] error)
//
// For example:
//   func save(Runtime, filename string) (int, error)
//
// RegisterFunction will panic if the type of the function does not meet
// these requirements.
//
// Runtime provides access to the underlying environment in which the
// function is run and includes access to a context and various builtin
// functions etc.
func RegisterFunction(fn interface{}, help string) {
	v := verb{
		implementation: reflect.ValueOf(fn),
		protoype:       reflect.TypeOf(fn),
		help:           help,
	}
	rt := v.protoype
	if rt.Kind() != reflect.Func {
		panic(fmt.Sprintf("%v: is not a function", rt.Name()))
	}
	fullname := runtime.FuncForPC(v.implementation.Pointer()).Name()
	nr := rt.NumOut()
	if nr == 0 {
		panic(fmt.Sprintf("%v: must have at least one result", fullname))
	}
	ni := rt.NumIn()
	if ni == 0 {
		panic(fmt.Sprintf("%v: must have at least one arg", fullname))
	}
	if et := rt.Out(nr - 1); !et.Implements(errorIfc) {
		panic(fmt.Sprintf("%v: right most result must implement error", fullname))
	}
	if st := rt.In(0); !st.Implements(runtimeIfc) {
		panic(fmt.Sprintf("%s: %s: first argument is of type %s, must implement %s", fullname, rt, st, runtimeIfc))

	}
	v.name = fullname[strings.LastIndex(fullname, ".")+1:]
	supportedVerbs[v.name] = v
}

// Runtime represents the set of functions available to script functions.
type Runtime interface {
	// Context returns the context specified when the script was executed.
	Context() *context.T
	// Printf is like fmt.Printf.
	Printf(format string, args ...interface{})
	// ListFunctions prints a list of the currently available functions.
	ListFunctions()
	// Help prints the function prototype and its help message.
	Help(function string) error
	// ExpandEnv provides access to the environment variables defined and
	// the process, preferring those defined for the script.
	ExpandEnv(name string) string
}
