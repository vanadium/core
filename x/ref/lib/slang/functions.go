package slang

import (
	"fmt"
	"io"
	"reflect"
	"runtime"
	"sort"
	"strings"

	"v.io/v23/context"
)

type verb struct {
	name           string
	implementation reflect.Value
	protoype       reflect.Type
	parameters     []string
	results        []string
	help           string
}

func (v verb) formatParameters(out *strings.Builder) {
	rt := v.protoype
	out.WriteRune('(')
	variadic := rt.IsVariadic()
	nIn := rt.NumIn()
	for i := 1; i < nIn; i++ {
		if pn := v.parameters[i-1]; len(pn) > 0 {
			out.WriteString(pn)
			out.WriteRune(' ')
		}
		rti := rt.In(i)
		if variadic && i == nIn-1 {
			out.WriteString("...")
			out.WriteString(rti.Elem().String())
		} else {
			out.WriteString(rti.String())
		}
		if i < nIn-1 {
			out.WriteString(", ")
		}
	}
	out.WriteRune(')')
}

func (v verb) formatResults(out *strings.Builder) {
	rt := v.protoype
	if rt.NumOut() == 1 {
		return
	}
	if rt.NumOut() > 2 {
		out.WriteString(" (")
	} else {
		out.WriteRune(' ')
	}
	nOut := rt.NumOut() - 1
	for i := 0; i < nOut; i++ {
		if len(v.results) > 0 {
			if i == 0 {
				out.WriteRune('(')
			}
			if rn := v.results[i]; len(rn) > 0 {
				out.WriteString(rn)
				out.WriteRune(' ')
			}
		}
		out.WriteString(rt.Out(i).String())
		if i < nOut-1 {
			out.WriteString(", ")
		}
		if len(v.results) > 0 && i == nOut-1 {
			out.WriteRune(')')
		}
	}
	if rt.NumOut() > 2 {
		out.WriteRune(')')
	}
}

func (v verb) String() string {
	out := &strings.Builder{}
	out.WriteString(v.name)
	v.formatParameters(out)
	v.formatResults(out)
	return out.String()
}

var (
	errorIfc   = reflect.TypeOf((*error)(nil)).Elem()
	runtimeIfc = reflect.TypeOf((*Runtime)(nil)).Elem()

	supportedVerbs = map[string]verb{}
)

// RegisterFunction registers a new function that can be called from a
// slang script and an associated 'help' describe describing its use.
// The names of each parameter and named results must be explicitly provided
// since they cannot be obtained via the reflect package. They should
// be provided in the order that they are defined, i.e. left-to-right,
// parameters first, then optionally any named results.
//
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
func RegisterFunction(fn interface{}, help string, parameterAndResultNames ...string) {
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

	npr := len(parameterAndResultNames)
	if npr < ni-1 {
		panic(fmt.Sprintf("%v: too few parameter names: found %v, expected %v", fullname, npr, ni-1))
	}
	v.parameters = parameterAndResultNames[:ni-1]
	npr -= ni - 1
	if npr > 0 {
		if npr != nr-1 {
			panic(fmt.Sprintf("%v: wrong number of results names: found %v, expected %v", fullname, npr, nr-1))
		}
		v.results = parameterAndResultNames[ni-1:]
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
	// Stdout returns the appropriate io.Writer for accessing stdout.
	Stdout() io.Writer
}

// RegisteredFunction represents the name and help message for a registered
// function.
type RegisteredFunction struct {
	Function string
	Help     string
}

// RegisteredFunctions returns all currently registered functions.
func RegisteredFunctions() []RegisteredFunction {
	rf := make([]RegisteredFunction, 0, len(supportedVerbs))

	for _, v := range supportedVerbs {
		rf = append(rf, RegisteredFunction{
			Function: v.String(),
			Help:     v.help,
		})
	}
	sort.Slice(rf, func(i, j int) bool {
		return rf[i].Function < rf[j].Function
	})
	return rf
}
