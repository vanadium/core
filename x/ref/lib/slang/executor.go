package slang

import (
	"fmt"
	"go/scanner"
	"go/token"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"

	"v.io/v23/context"
)

// Script represents the execution of a single script.
type Script struct {
	Stdout      io.Writer
	envvars     map[string]string
	ctx         *context.T
	symbols     *symbolTable
	invocations []invocation
}

// SetEnv sets an environment variable that will be available to the
// script that's preferred to any value in the processes environment.
func (scr *Script) SetEnv(name, value string) {
	if scr.envvars == nil {
		scr.envvars = map[string]string{}
	}
	scr.envvars[name] = value
}

// String returns a string representation of the compiled script
// and any variables defined during its execution.
func (scr *Script) String() string {
	out := &strings.Builder{}
	scr.formatEnv(out)
	scr.formatCompiledState(out)
	scr.symbols.formatValues(out)
	return out.String()
}

// Variables returns a copy of the currently defined variables.
func (scr *Script) Variables() map[string]interface{} {
	vals := make(map[string]interface{}, len(scr.symbols.values))
	for k, v := range scr.symbols.values {
		vals[k] = v.Interface()
	}
	return vals
}

// ExecuteBytes will parse, compile and execute the supplied script.
func (scr *Script) ExecuteBytes(ctx *context.T, src []byte) error {
	stmts, err := parseBytes(src)
	if err != nil {
		return err
	}
	return scr.compileAndRun(ctx, stmts)
}

// ExecuteFile will parse, compile and execute the script contained in
// the specified file.
func (scr *Script) ExecuteFile(ctx *context.T, filename string) error {
	stmts, err := parseFile(filename)
	if err != nil {
		return err
	}
	return scr.compileAndRun(ctx, stmts)
}

func (scr *Script) compileAndRun(ctx *context.T, stmts statements) error {
	if err := scr.compile(stmts); err != nil {
		return err
	}
	scr.ctx = ctx
	if scr.Stdout == nil {
		scr.Stdout = os.Stdout
	}
	return scr.run()
}

// value is the interface implemented by either runtime variables
// or literals.
type value interface {
	value(st *symbolTable) reflect.Value
	varOrLiteral() string
}

// symbolTable tracks the type of all compile-time defined
// variables and the values of these variables at runtime.
type symbolTable struct {
	types  map[string]reflect.Type
	values map[string]reflect.Value
}

// invocation is the compiled form of a single statement - i.e.
// function invocation.
type invocation struct {
	pos     token.Position
	verb    verb
	args    []value
	results []value
}

func (scr *Script) formatEnv(out *strings.Builder) {
	names := make([]string, 0, len(scr.envvars))
	for k := range scr.envvars {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		fmt.Fprintf(out, "$%v=%v\n", k, scr.envvars[k])
	}
}

func (st *symbolTable) formatTypes(out *strings.Builder) {
	names := make([]string, 0, len(st.types))
	for k := range st.types {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		fmt.Fprintf(out, "%v: %v\n", k, st.types[k])
	}
}

func (st *symbolTable) formatValues(out *strings.Builder) {
	names := make([]string, 0, len(st.values))
	for k := range st.values {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		v := st.values[k]
		fmt.Fprintf(out, "%v: %v: %v\n", k, v.Type(), fmt.Sprintf("%v", v.Interface()))
	}
}

// compiledState returns a string representation of the compiled state.
func (scr *Script) formatCompiledState(out *strings.Builder) {
	scr.symbols.formatTypes(out)
	scr.symbols.formatValues(out)
	for _, inv := range scr.invocations {
		inv.format(out)
		out.WriteRune('\n')
	}
}

// runtimeState returns a string representation of the current
// execution state.
func (scr *Script) runtimeState() string {
	out := &strings.Builder{}
	scr.symbols.formatValues(out)
	return out.String()
}

// format the invocation into a readable form.
func (inv invocation) format(out *strings.Builder) {
	out.WriteString(inv.pos.String())
	out.WriteString(": ")
	for i, r := range inv.results {
		out.WriteString(r.varOrLiteral())
		if i < len(inv.results)-1 {
			out.WriteString(", ")
		}
	}
	if len(inv.results) > 0 {
		out.WriteString(" := ")
	}
	fmt.Fprintf(out, "%s(", inv.verb.name)
	for i, a := range inv.args {
		out.WriteString(a.varOrLiteral())
		if i < len(inv.args)-1 {
			out.WriteString(", ")
		}
	}
	out.WriteRune(')')
	out.WriteString(" :: ")
	out.WriteString(inv.verb.String())
}

type literalValue reflect.Value

func (lv literalValue) value(*symbolTable) reflect.Value {
	return reflect.Value(lv)
}

func (lv literalValue) varOrLiteral() string {
	rv := reflect.Value(lv)
	if rv.Type().Kind() == reflect.String {
		return fmt.Sprintf("%q", rv.Interface())
	}
	return fmt.Sprintf("%v", rv.Interface())
}

type variableValue string

func (rv variableValue) value(st *symbolTable) reflect.Value {
	v, ok := st.values[string(rv)]
	if !ok {
		panic(fmt.Sprintf("variable %v has not been initialized", rv))
	}
	return v
}

func (rv variableValue) varOrLiteral() string {
	return string(rv)
}

func (scr *Script) run() error {
	errs := scanner.ErrorList{}
	rt := reflect.ValueOf(scr)
	for _, inv := range scr.invocations {
		args := make([]reflect.Value, len(inv.args)+1)
		args[0] = rt
		for i, arg := range inv.args {
			args[i+1] = arg.value(scr.symbols)
		}
		results := inv.verb.implementation.Call(args)
		if len(results) < 1 {
			panic("internal error: unexpected number of results")
		}
		if ierr := results[len(results)-1].Interface(); ierr != nil {
			errs.Add(inv.pos, ierr.(error).Error())
			return errs.Err()
		}
		if len(results) == 1 {
			continue
		}
		for i, v := range results[:len(results)-1] {
			scr.symbols.values[inv.results[i].varOrLiteral()] = v
		}
	}
	return nil
}
