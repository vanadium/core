// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package swift

import (
	"regexp"
	"strings"
)

var (
	// File-wide
	reCeilNewlines *regexp.Regexp
	// Per line
	reIsClosingBracket     *regexp.Regexp
	reIsComment            *regexp.Regexp
	reIsSwiftdoc           *regexp.Regexp
	reOnlyWhitespaceRegexp *regexp.Regexp
)

type parseState int

const (
	stateClosingBracket parseState = iota
	stateCode
	stateComment
	stateNull
	stateSwiftdoc
	stateWhitespace
)

func init() {
	// File-wide
	reCeilNewlines = regexp.MustCompile("(?m)(\\n([ ]|\t)*){2,}")
	// Per line
	reIsClosingBracket = regexp.MustCompile(`^\s*}$`)
	reIsComment = regexp.MustCompile(`\s*(//|/\*)`)
	reIsSwiftdoc = regexp.MustCompile(`\s*(///|//:)`)
	reOnlyWhitespaceRegexp = regexp.MustCompile(`^\s*$`)
}

// categorizeLine examines a line of Swift and categorizes it into a few rough buckets
func categorizeLine(line string) parseState {
	switch {
	case reIsClosingBracket.MatchString(line):
		return stateClosingBracket
	case reIsSwiftdoc.MatchString(line):
		return stateSwiftdoc
	case reIsComment.MatchString(line):
		return stateComment
	case reOnlyWhitespaceRegexp.MatchString(line):
		return stateWhitespace
	}
	return stateCode
}

// formatSwiftCode takes generated Swift code and cleans & reformats it
// like gofmt to match the appropriate Swift style.
// TODO(zinman) Integrate a proper Swift formatter
func formatSwiftCode(code string) string {
	// Across file
	code = strings.TrimSpace(code)
	code = reCeilNewlines.ReplaceAllStringFunc(code, func(lines string) string {
		splitUp := strings.Split(lines, "\n")
		if splitUp[len(splitUp)-1] == "" {
			return "\n\n"
		}
		return "\n\n" + splitUp[len(splitUp)-1]
	})
	// Per line
	lines := strings.Split(code, "\n")
	cleanedLines := []string{}
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		line = strings.TrimRight(line, " ")
		state := categorizeLine(line)
		stateAbove := stateNull
		if len(cleanedLines) > 0 {
			stateAbove = categorizeLine(cleanedLines[len(cleanedLines)-1])
		}
		switch state {
		case stateWhitespace:
			if stateAbove == stateSwiftdoc {
				// Parent was a Swift-doc comment, so kill extra space by ignoring this
				continue
			}
		case stateSwiftdoc:
			if strings.TrimSpace(line) == "///" && (stateAbove == stateWhitespace || stateAbove == stateNull) {
				continue
			}
		}
		cleanedLines = append(cleanedLines, line)
	}
	return strings.Join(cleanedLines, "\n")
}
