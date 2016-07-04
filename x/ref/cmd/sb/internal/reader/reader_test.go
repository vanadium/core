// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reader

import (
	"io"
	"reflect"
	"testing"
)

type stringPrompter struct {
	lines []string
	curr  int
}

func (s *stringPrompter) Close() {
}

func (s *stringPrompter) InitialPrompt() (string, error) {
	if s.curr >= len(s.lines) {
		return "", io.EOF
	}
	q := s.lines[s.curr]
	s.curr++
	return q, nil
}

func (s *stringPrompter) ContinuePrompt() (string, error) {
	return s.InitialPrompt()
}

func (s *stringPrompter) AppendHistory(query string) {
}

func TestGetQuery(t *testing.T) {
	type testCase struct {
		lines   []string
		queries []string
	}

	tests := []testCase{
		{ // Single query.
			[]string{"select k from C;"},
			[]string{"select k from C"},
		},
		{ // Multiple queries.
			[]string{"select k from C;", "select bar from C;"},
			[]string{"select k from C", "select bar from C"},
		},
		{ // Multiple queries on one line.
			[]string{"select k from C; select bar from C;"},
			[]string{"select k from C", " select bar from C"},
		},
		{ // Multiple queries without a ; are just one query.
			[]string{"select k from C select bar from C;"},
			[]string{"select k from C select bar from C"},
		},
		{ // Multiple queries without a ; are just one query.
			[]string{"select k from C", "select bar from C;"},
			[]string{"select k from C\nselect bar from C"},
		},
		{
			[]string{"select\tfoo.bar from\nC;"},
			[]string{"select\tfoo.bar from\nC"},
		},
	}
	for _, test := range tests {
		r := newT(&stringPrompter{lines: test.lines})
		var queries []string
		for true {
			if q, err := r.GetQuery(); err != nil {
				if err == io.EOF {
					break
				}
				t.Errorf("test %v: unexpected error: %v", test.lines, err)
				break
			} else {
				queries = append(queries, q)
			}
		}
		if got, want := queries, test.queries; !reflect.DeepEqual(got, want) {
			t.Errorf("test %#v: got %#v, want %#v", test.lines, got, want)
		}
	}
}
