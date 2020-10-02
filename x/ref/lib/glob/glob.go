// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package glob defines a globbing syntax and implements matching routines.
//
// Globs match a slash separated series of glob expressions.
//
//   // Patterns:
//   term ['/' term]*
//   term:
//   '*'         matches any sequence of non-Separator characters
//   '?'         matches any single non-Separator character
//   '[' [ '^' ] { character-range } ']'
//   // Character classes (must be non-empty):
//   c           matches character c (c != '*', '?', '\\', '[', '/')
//   '\\' c      matches character c
//   // Character-ranges:
//   c           matches character c (c != '\\', '-', ']')
//   '\\' c      matches character c
//   lo '-' hi   matches character c for lo <= c <= hi
//
// This package is DEPRECATED. Use v.io/v23/glob instead.
package glob

import (
	"v.io/v23/glob"
)

// Glob represents a slash separated path glob pattern.
// Deprecated: use v.io/v23/glob.Glob instead.
type Glob struct {
	*glob.Glob
}

// Parse returns a new Glob.
// Deprecated: use v.io/v23/glob.Parse instead.
func Parse(pattern string) (*Glob, error) {
	g, err := glob.Parse(pattern)
	return &Glob{g}, err
}

// Tail returns the suffix of g starting at the second element.
// Deprecated: use v.io/v23/glob.Tail instead.
func (g *Glob) Tail() *Glob {
	return &Glob{g.Glob.Tail()}
}

// Finished returns true if the pattern cannot match anything.
// Deprecated: use v.io/v23/glob.Finished instead.
func (g *Glob) Finished() bool {
	return g.Empty()
}

// Split returns the suffix of g starting at the path element corresponding to
// start.
// Deprecated: use v.io/v23/glob.Split instead.
func (g *Glob) Split(start int) *Glob {
	suffix := g
	for i := start - 1; i >= 0; i-- {
		suffix = suffix.Tail()
	}
	return suffix
}

// MatchInitialSegment tries to match segment against the initial element of g.
// Returns:
// matched, a boolean indicating whether the match was successful;
// exact, a boolean indicating whether segment matched a fixed string pattern;
// remainder, a Glob representing the unmatched remainder of g.
// Deprecated: use v.io/v23/glob.MatchInitialSegment instead.
func (g *Glob) MatchInitialSegment(segment string) (matched bool, exact bool, remainder *Glob) {
	m := g.Head()
	matched = m.Match(segment)
	_, exact = m.FixedPrefix()
	remainder = g.Tail()
	return
}

// PartialMatch tries matching elems against part of a glob pattern.
// Returns:
// matched, a boolean indicating whether each element e_i of elems matches the
// (start + i)th element of the glob pattern;
// exact, a boolean indicating whether elems matched a fixed string pattern.
// <path> is considered an exact match for pattern <path>/...;
// remainder, a Glob representing the unmatched remainder of g. remainder will
// be empty if the pattern is completely matched.
//
// Note that if the glob is recursive elems can have more elements then
// the glob pattern and still get a true result.
// Deprecated: use v.io/v23/glob.PartialMatch instead.
func (g *Glob) PartialMatch(start int, elems []string) (matched bool, exact bool, remainder *Glob) {
	g = g.Split(start)
	allExact := true
	for i := 0; i < len(elems); i++ {
		var matched, exact bool
		if matched, exact, g = g.MatchInitialSegment(elems[i]); !matched {
			return false, false, nil
		} else if !exact {
			allExact = false
		}
	}
	return true, allExact, g
}

// SplitFixedElements returns the part of the glob pattern that contains only
// fixed elements, and the glob that follows it.
// Deprecated: use v.io/v23/glob.SplitFixedElements instead.
func (g *Glob) SplitFixedElements() ([]string, *Glob) {
	prefix, left := g.Glob.SplitFixedElements()
	return prefix, &Glob{left}
}

// SplitFixedPrefix returns the part of the glob pattern that contains only
// fixed elements, and the glob that follows it.
// Deprecated: use v.io/v23/glob.SplitFixedPrefix instead.
func (g *Glob) SplitFixedPrefix() ([]string, *Glob) {
	return g.SplitFixedElements()
}
