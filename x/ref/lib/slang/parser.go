package slang

import (
	"fmt"
	"go/scanner"
	"go/token"
	"reflect"
	"runtime"
	"strings"
)

type statement struct {
	pos           token.Position
	results, args []tokPos
	verbName      tokPos
}

type statements []statement

func parseBytes(src []byte) (statements, error) {
	tokens, err := scanBytes(src)
	if err != nil {
		return nil, err
	}
	return parse(tokens)
}

func parseFile(filename string) (statements, error) {
	tokens, err := scanFile(filename)
	if err != nil {
		return nil, err
	}
	return parse(tokens)
}

func writeCommaSep(out *strings.Builder, idents []tokPos) {
	for i, tp := range idents {
		out.WriteString(tp.lit)
		if i < len(idents)-1 {
			out.WriteString(", ")
		}
	}
}

func (s statement) String() string {
	out := &strings.Builder{}
	formatStatement(out, s)
	return out.String()
}

func formatStatement(out *strings.Builder, s statement) {
	if len(s.results) > 0 {
		writeCommaSep(out, s.results)
		out.WriteString(" := ")
	}
	out.WriteString(s.verbName.lit)
	out.WriteRune('(')
	writeCommaSep(out, s.args)
	out.WriteRune(')')

}

func (stmts statements) String() string {
	out := &strings.Builder{}
	for _, s := range stmts {
		formatStatement(out, s)
		out.WriteRune('\n')
	}
	return out.String()
}

type parseState struct {
	scratch    []tokPos
	results    []tokPos
	verbName   tokPos
	args       []tokPos
	statements statements
	errs       scanner.ErrorList
}

type parseStateFn func(tokPos) parseStateFn

func (ps *parseState) resultsIdent(tp tokPos) parseStateFn {
	switch tp.tok {
	case token.IDENT:
		ps.scratch = append(ps.scratch, tp)
		return ps.resultsCommaOrDefineOrArgs
	}
	ps.errs = append(ps.errs, &scanner.Error{
		Pos: tp.pos,
		Msg: fmt.Sprintf("expected variable name, got '%s'", tp.tok),
	})
	return ps.resultsIdent
}

func (ps *parseState) resultsCommaOrDefineOrArgs(tp tokPos) parseStateFn {
	switch tp.tok {
	case token.COMMA:
		return ps.resultsIdent
	case token.DEFINE:
		ps.results = make([]tokPos, len(ps.scratch))
		copy(ps.results, ps.scratch)
		ps.scratch = ps.scratch[:0]
		return ps.funcName
	case token.LPAREN:
		// function call with no results.
		if len(ps.scratch) == 1 {
			ps.verbName = ps.scratch[0]
			ps.scratch = ps.scratch[:0]
			return ps.argIdentOrRightParen
		}
	}
	ps.errs = append(ps.errs, &scanner.Error{
		Pos: tp.pos,
		Msg: fmt.Sprintf("expected ',' or ':=', got '%s'", tp.tok),
	})
	return ps.resultsCommaOrDefineOrArgs
}

func (ps *parseState) funcName(tp tokPos) parseStateFn {
	switch tp.tok {
	case token.IDENT:
		ps.verbName = tp
		return ps.argsLeftParen
	}
	ps.errs = append(ps.errs, &scanner.Error{
		Pos: tp.pos,
		Msg: fmt.Sprintf("expected function name, got '%s'", tp.tok),
	})
	return ps.resultsIdent
}

func (ps *parseState) argsLeftParen(tp tokPos) parseStateFn {
	if tp.tok == token.LPAREN {
		return ps.argIdentOrRightParen
	}
	ps.errs = append(ps.errs, &scanner.Error{
		Pos: tp.pos,
		Msg: fmt.Sprintf("expected '(', got '%s'", tp.tok),
	})
	return ps.argsLeftParen
}

func (ps *parseState) argIdentOrRightParen(tp tokPos) parseStateFn {
	if tp.tok == token.RPAREN {
		return ps.done
	}
	return ps.argIdent(tp)
}

func (ps *parseState) argIdent(tp tokPos) parseStateFn {
	switch tp.tok {
	case token.IDENT, token.STRING, token.INT:
		ps.scratch = append(ps.scratch, tp)
		return ps.argsCommaOrRightParen
	}
	ps.errs = append(ps.errs, &scanner.Error{
		Pos: tp.pos,
		Msg: fmt.Sprintf("expected variable or literal, got '%s'", tp.tok),
	})
	return ps.resultsIdent
}

func (ps *parseState) argsCommaOrRightParen(tp tokPos) parseStateFn {
	switch tp.tok {
	case token.COMMA:
		return ps.argIdent
	case token.RPAREN:
		ps.args = make([]tokPos, len(ps.scratch))
		copy(ps.args, ps.scratch)
		ps.scratch = ps.scratch[:0]
		return ps.done
	}
	ps.errs = append(ps.errs, &scanner.Error{
		Pos: tp.pos,
		Msg: fmt.Sprintf("expected ',' or ')', got '%s'", tp.tok),
	})
	return ps.argsCommaOrRightParen
}

func (ps *parseState) copyStatementAndReset() statement {
	stmt := statement{
		results:  ps.results,
		args:     ps.args,
		verbName: ps.verbName,
	}
	if len(ps.results) > 0 {
		stmt.pos = ps.results[0].pos
	} else {
		stmt.pos = ps.verbName.pos
	}
	ps.results, ps.args = nil, nil
	ps.verbName = tokPos{}
	return stmt
}

func (ps *parseState) done(tp tokPos) parseStateFn {
	if tp.tok == token.SEMICOLON {
		stmt := ps.copyStatementAndReset()
		ps.statements = append(ps.statements, stmt)
		return ps.resultsIdent
	}
	ps.errs = append(ps.errs, &scanner.Error{
		Pos: tp.pos,
		Msg: fmt.Sprintf("expected '; or '\\n', got '%s'", tp.tok),
	})
	return ps.done
}

var parseTrace = false

func parse(tokens []tokPos) (statements, error) {
	ps := &parseState{}
	fn := ps.resultsIdent
	for _, tp := range tokens {
		if parseTrace {
			fmt.Printf(">> %v: %v: %v\n", tp.pos, tp.tok, runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name())
		}
		fn = fn(tp)
		if len(ps.errs) > 5 {
			return nil, ps.errs.Err()
		}
	}
	return ps.statements, ps.errs.Err()
}
