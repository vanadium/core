package slang

import (
	"fmt"
	"go/scanner"
	"go/token"
	"reflect"
	"strconv"
	"time"
)

func (scr *Script) compile(stmts statements) error {
	if scr.symbols == nil {
		scr.symbols = newSymbolTable()
	}
	scr.invocations = make([]invocation, 0, len(stmts))
	errs := scanner.ErrorList{}
	for _, stmt := range stmts {
		inv, err := compileStatement(scr.symbols, stmt)
		if err != nil {
			if err.Pos == stmt.pos {
				errs.Add(stmt.pos, err.Msg)
			} else {
				errs.Add(stmt.pos, err.Error())
			}
			continue
		}
		scr.invocations = append(scr.invocations, inv)
	}
	return errs.Err()
}

func parseBool(v string) (bool, error) {
	switch v {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	return false, fmt.Errorf("must be one of 'true' or 'false'")
}

func unquote(s string) string {
	uq, err := strconv.Unquote(s)
	if err != nil {
		return s
	}
	return uq
}

func handleLiteral(lit tokPos, tt reflect.Type) (reflect.Value, error) {
	switch lit.tok {
	case token.INT:
		iv, err := strconv.ParseInt(lit.lit, 10, 64)
		if err != nil {
			// The go parser has already made sure this token is an int.
			panic(fmt.Sprintf("%v: not an int: %v", lit.lit, err))
		}
		if iv > math.MaxInt32 {
			return reflect.ValueOf(iv), nil
		}
		return reflect.ValueOf(int(iv)), nil
	case token.IDENT:
		bv, err := parseBool(lit.lit)
		if err == nil {
			return reflect.ValueOf(bv), nil
		}
		if tt.Kind() == reflect.Bool {
			return reflect.Value{}, fmt.Errorf("not a bool: must be one of 'true' or 'false'")
		}
		// Turn function names into strings.
		if _, ok := supportedVerbs[lit.lit]; ok {
			return reflect.ValueOf(lit.lit), nil
		}
		return reflect.Value{}, fmt.Errorf("invalid identifier")
	case token.STRING:
		uq := unquote(lit.lit)
		if tt == reflect.TypeOf(time.Hour) {
			d, err := time.ParseDuration(uq)
			if err != nil {
				return reflect.Value{}, err
			}
			return reflect.ValueOf(d), nil
		}
		return reflect.ValueOf(uq), nil
	}
	panic(fmt.Sprintf("unsupported literal type: expected %v", tt))
}

func compileResults(symbols *symbolTable, td reflect.Type, results []tokPos, assignment bool) ([]value, *scanner.Error) {
	resultValues := []value{}
	for i, r := range results {
		name := r.lit
		et, existing := symbols.types[name]
		if existing && symbols.constants[name] {
			return nil, &scanner.Error{
				Pos: r.pos,
				Msg: fmt.Sprintf("cannot assign or define result '%v' which is a constant", name),
			}
		}
		tt := td.Out(i)
		if !existing && assignment {
			return nil, &scanner.Error{
				Pos: r.pos,
				Msg: fmt.Sprintf("result '%v' is not defined", name),
			}
		}
		if existing && !assignment {
			if et != tt {
				return nil, &scanner.Error{
					Pos: r.pos,
					Msg: fmt.Sprintf("result '%v' is redefined with a new type %v, previous type %v", name, tt, et),
				}
			}
			return nil, &scanner.Error{
				Pos: r.pos,
				Msg: fmt.Sprintf("result '%v' is redefined", name),
			}
		}
		if !assignment {
			symbols.types[name] = tt
		}
		resultValues = append(resultValues, variableValue{name: name, hasEllipsis: false})
	}
	return resultValues, nil
}

func compileArguments(symbols *symbolTable, td reflect.Type, args []tokPos) ([]value, *scanner.Error) {
	argValues := []value{}
	for i, a := range args {
		name := a.lit
		et, existing := symbols.types[name]
		var tt reflect.Type
		if td.IsVariadic() && i+1 >= td.NumIn()-1 {
			tt = td.In(td.NumIn() - 1)
			if !a.ellipsis {
				tt = tt.Elem()
			}
		} else {
			tt = td.In(i + 1)
		}
		if existing {
			if !et.AssignableTo(tt) {
				return nil, &scanner.Error{
					Pos: a.pos,
					Msg: fmt.Sprintf("arg '%v' is of the wrong type %v, should be %v", name, et, tt),
				}
			}
			argValues = append(argValues, variableValue{name: name, hasEllipsis: a.ellipsis})
			continue
		}
		// Must be a literal.
		val, err := handleLiteral(a, tt)
		if err != nil {
			return nil, &scanner.Error{
				Pos: a.pos,
				Msg: fmt.Sprintf("arg '%v' is an invalid literal: %v", name, err),
			}
		}
		if !val.Type().AssignableTo(tt) {
			return nil, &scanner.Error{
				Pos: a.pos,
				Msg: fmt.Sprintf("literal arg '%v' of type %v is not assignable to %v", name, val.Type(), tt),
			}
		}
		argValues = append(argValues, literalValue(val))
	}
	return argValues, nil
}

func compileStatement(symbols *symbolTable, stmt statement) (invocation, *scanner.Error) {
	inv := invocation{pos: stmt.pos}
	v, ok := supportedVerbs[stmt.verbName.lit]
	if !ok {
		return inv, &scanner.Error{
			Pos: inv.pos,
			Msg: fmt.Sprintf("unrecognised function name: %s", stmt.verbName.lit),
		}
	}
	td := v.protoype
	if len(stmt.results)+1 != td.NumOut() {
		return inv, &scanner.Error{
			Pos: inv.pos,
			Msg: fmt.Sprintf("wrong # of results for %v", v),
		}
	}
	if len(stmt.args)+1 != td.NumIn() {
		if !td.IsVariadic() {
			return inv, &scanner.Error{
				Pos: inv.pos,
				Msg: fmt.Sprintf("wrong # of arguments for %v", v),
			}
		}
		if len(stmt.args)+1 < td.NumIn()-1 {
			return inv, &scanner.Error{
				Pos: inv.pos,
				Msg: fmt.Sprintf("need at least %v arguments for variadic function %v", td.NumIn()-2, v),
			}
		}
	}

	resultValues, err := compileResults(symbols, td, stmt.results, stmt.assign)
	if err != nil {
		return inv, err
	}

	argValues, err := compileArguments(symbols, td, stmt.args)
	if err != nil {
		return inv, err
	}

	return invocation{
		pos:        stmt.pos,
		verb:       v,
		results:    resultValues,
		args:       argValues,
		assignment: stmt.assign,
	}, nil
}
