// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package reader provides an object that reads queries from various input
// sources (e.g. stdin, pipe).
package reader

import (
	"bufio"
	"os"
	"strings"
	"text/scanner"

	"github.com/peterh/liner"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/cmd/sb/commands"
)

type T struct {
	s      scanner.Scanner
	prompt prompter
}

func newT(prompt prompter) *T {
	t := &T{prompt: prompt}
	t.initScanner("")
	return t
}

// Close frees any resources acquired by this reader.
func (t *T) Close() {
	t.prompt.Close()
}

func (t *T) initScanner(input string) {
	t.s.Init(strings.NewReader(input))
	// Keep all whitespace.
	t.s.Whitespace = 0
}

// GetQuery returns an entire query where queries are delimited by semicolons.
// GetQuery returns the error io.EOF when there is no more input.
func (t *T) GetQuery() (string, error) {
	return t.GetQueryWithTerminator(';')
}

func (t *T) GetQueryWithTerminator(terminator rune) (string, error) {
	if t.s.Peek() == scanner.EOF {
		input, err := t.prompt.InitialPrompt()
		if err != nil {
			return "", err
		}
		t.initScanner(input)
	}
	var query string
WholeQuery:
	for true {
		for tok := t.s.Scan(); tok != scanner.EOF; tok = t.s.Scan() {
			if tok == terminator {
				break WholeQuery
			}
			query += t.s.TokenText()
		}
		if terminator == '\n' {
			break
		}
		input, err := t.prompt.ContinuePrompt()
		if err != nil {
			return "", err
		}
		t.initScanner(input)
		query += "\n" // User started a new line.
	}
	if terminator == '\n' {
		t.prompt.AppendHistory(query)
	} else {
		t.prompt.AppendHistory(query + string(terminator))
	}
	return query, nil
}

type prompter interface {
	Close()
	InitialPrompt() (string, error)
	ContinuePrompt() (string, error)
	AppendHistory(query string)
}

// noninteractive prompter just blindly reads from stdin.
type noninteractive struct {
	input *bufio.Reader
}

// NewNonInteractive returns a T that simply reads input from stdin. Useful
// for when the user is piping input from a file or another program.
func NewNonInteractive() *T {
	return newT(&noninteractive{bufio.NewReader(os.Stdin)})
}

func (i *noninteractive) Close() {
}

func (i *noninteractive) InitialPrompt() (string, error) {
	return i.input.ReadString('\n')
}

func (i *noninteractive) ContinuePrompt() (string, error) {
	return i.input.ReadString('\n')
}

func (i *noninteractive) AppendHistory(query string) {
}

// interactive prompter provides a nice prompt for a user to input queries.
type interactive struct {
	line *liner.State
}

// NewInteractive returns a T that prompts the user for input.
func NewInteractive() *T {
	i := &interactive{
		line: liner.NewLiner(),
	}
	i.line.SetCtrlCAborts(true)
	i.line.SetCompleter(complete)
	i.line.SetTabCompletionStyle(liner.TabPrints)
	return newT(i)
}

func (i *interactive) Close() {
	i.line.Close()
}

func (i *interactive) InitialPrompt() (string, error) {
	return i.line.Prompt("? ")
}

func (i *interactive) ContinuePrompt() (string, error) {
	return i.line.Prompt("  > ")
}

func (i *interactive) AppendHistory(query string) {
	i.line.AppendHistory(query)
}

func complete(line string) []string {
	// Get command list to tab-complete on.
	tokens := strings.Split(line, " ")
	cmds := commands.Commands
	for _, token := range tokens[:len(tokens)-1] {
		nextCommand := find(token, cmds)
		if nextCommand == nil {
			return nil
		} else {
			cmds = nextCommand.Children
		}
	}

	// Tab complete on command list.
	lastToken := tokens[len(tokens)-1]
	lineMinusLastToken := line[:len(line)-len(lastToken)]
	var ret []string
	for _, cmd := range cmds {
		name := cmd.Name
		if strings.HasPrefix(name, lastToken) {
			ret = append(ret, lineMinusLastToken+name)
		}
	}

	return ret
}

func find(name string, cmds []*cmdline.Command) *cmdline.Command {
	for _, cmd := range cmds {
		if cmd.Name == name {
			return cmd
		}
	}
	return nil
}
