// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"v.io/v23/context"
	"v.io/x/ref/services/ben"
)

type Store interface {
	Save(ctx *context.T, scenario ben.Scenario, code ben.SourceCode, uploader string, uploadTime time.Time, runs []ben.Run) error
	Benchmarks(query *Query) BenchmarkIterator
	Runs(benchmarkID string) (Benchmark, RunIterator)
	DescribeSource(id string) (ben.SourceCode, error)
}

type Query struct {
	Name     string
	CPU      string
	OS       string
	Uploader string
	Label    string
}

// Benchmark identifies a particular (Benchmark, Scenario, Uploader) tuple.
// Many ben.Run objects are associated with a single Benchmark.
type Benchmark struct {
	ID       string       // A unique identifier of this particular Benchmark
	Name     string       // Name (e.g. v.io/v23/security.BenchmarkSign) of the Benchmark.
	Scenario ben.Scenario // The scenario under which the benchmark was run.
	Uploader string       // Identity of the user that uploaded the results.

	// Results from most recently uploaded runs for this benchmark.
	NanoSecsPerOp   float64
	MegaBytesPerSec float64
	LastUpdate      time.Time
}

func (b Benchmark) PrettyTime() string {
	if b.NanoSecsPerOp < 100 {
		return fmt.Sprintf("%vns", b.NanoSecsPerOp)
	}
	return time.Duration(int64(b.NanoSecsPerOp)).String()
}

type Iterator interface {
	Advance() bool
	Err() error
	Close()
}

type BenchmarkIterator interface {
	Iterator
	Value() Benchmark
	Runs() RunIterator
}

type RunIterator interface {
	Iterator
	Value() (run ben.Run, sourceCodeID string, uploadTime time.Time)
}

func (q *Query) String() string {
	var fields []string
	quot := func(in string) string {
		if strings.ContainsRune(in, ' ') {
			return fmt.Sprintf("%q", in)
		}
		return in
	}
	if len(q.Name) > 0 {
		fields = append(fields, quot(q.Name))
	}
	if len(q.CPU) > 0 {
		fields = append(fields, "cpu:"+quot(q.CPU))
	}
	if len(q.OS) > 0 {
		fields = append(fields, "os:"+quot(q.OS))
	}
	if len(q.Uploader) > 0 {
		fields = append(fields, "uploader:"+quot(q.Uploader))
	}
	if len(q.Label) > 0 {
		fields = append(fields, "label:"+quot(q.Label))
	}
	return strings.Join(fields, " ")
}

// ParseQuery converts a query string into a structured Query object.
//
// The query language supports setting each field in the Query object at most
// once and the query will be an AND of all terms. A more expressive query
// language is left as an exercise to a future enthusiast.
func ParseQuery(query string) (*Query, error) {
	tokens, err := split(query)
	if err != nil {
		return nil, err
	}
	var ret Query
	for _, f := range tokens {
		var err error
		var set bool
		set = set || trySetField(&ret.CPU, &err, "cpu:", f)
		set = set || trySetField(&ret.OS, &err, "os:", f)
		set = set || trySetField(&ret.Uploader, &err, "uploader:", f)
		set = set || trySetField(&ret.Label, &err, "label:", f)
		if !set {
			trySetField(&ret.Name, &err, "", f)
		}
		if err != nil {
			return nil, err
		}
	}
	return &ret, nil
}

func trySetField(dst *string, err *error, prefix, value string) bool {
	if *err != nil {
		return false
	}
	if len(value) < len(prefix) || strings.ToLower(value[:len(prefix)]) != prefix {
		return false
	}
	if len(*dst) > 0 {
		*err = fmt.Errorf("operator %q already set", prefix)
	}
	*dst = strings.TrimSuffix(strings.TrimPrefix(value[len(prefix):], `"`), `"`)
	return true
}

// split is liked strings.Split except that it doesn't break quoted strings.
func split(input string) ([]string, error) {
	if len(input) == 0 {
		return nil, nil
	}
	var (
		tokens []string
		start  int
		inq    bool
		commit = func(end int) {
			if end > start {
				tokens = append(tokens, input[start:end])
			}
		}
	)
	for idx, r := range input {
		switch {
		case r == '"' && !inq:
			inq = true
		case r == '"' && inq:
			commit(idx + 1)
			start = idx + 1
			inq = false
		case unicode.IsSpace(r) && !inq:
			commit(idx)
			start = idx + 1
		}
	}
	if inq {
		return tokens, fmt.Errorf("unterminated quote")
	}
	commit(len(input))
	return tokens, nil
}
