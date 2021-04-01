package slang

import (
	"fmt"
	"go/scanner"
	"go/token"
	"os"
)

type tokPos struct {
	tok token.Token
	pos token.Position
	lit string
}

var supportedTokens = map[token.Token]bool{}

func init() {
	for _, tok := range []token.Token{
		token.IDENT, token.SEMICOLON, token.DEFINE,
		token.LPAREN, token.RPAREN, token.COMMA,
		token.STRING, token.INT,
	} {
		supportedTokens[tok] = true
	}
}

func scan(fset *token.FileSet, file *token.File, src []byte) ([]tokPos, error) {
	errs := &scanner.ErrorList{}
	s := &scanner.Scanner{}
	errHandler := func(pos token.Position, msg string) {
		errs.Add(pos, msg)
	}
	tokens := make([]tokPos, 0, 100)
	s.Init(file, src, errHandler, scanner.ScanComments)
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if supportedTokens[tok] {
			tokens = append(tokens, tokPos{tok: tok, pos: file.Position(pos), lit: lit})
			continue
		}
		if tok == token.COMMENT {
			continue
		}
		s.ErrorCount++
		if tok.IsKeyword() {
			errs.Add(fset.Position(pos), fmt.Sprintf("reserved keyword '%s'", tok))
		} else {
			errs.Add(fset.Position(pos), fmt.Sprintf("invalid token '%s'", tok))
		}
		if s.ErrorCount > 5 {
			break
		}
	}
	return tokens, errs.Err()
}

func scanFile(filename string) ([]tokPos, error) {
	src, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	file := fset.AddFile(filename, fset.Base(), len(src))
	return scan(fset, file, src)
}

func scanBytes(src []byte) ([]tokPos, error) {
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(src))
	return scan(fset, file, src)
}
