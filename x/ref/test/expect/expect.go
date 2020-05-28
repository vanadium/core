// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package expect implements support for checking expectations against a
// buffered input stream. It supports literal and pattern based matching. It is
// line oriented; all of the methods (expect ReadAll) strip trailing newlines
// from their return values. It places a timeout on all its operations.  It will
// generally be used to read from the stdout stream of subprocesses in tests and
// other situations and to make 'assertions' about what is to be read.
//
// A Session type is used to store state, in particular error state, across
// consecutive invocations of its method set. If a particular method call
// encounters an error then subsequent calls on that Session will have no
// effect. This allows for a series of assertions to be made, one per line,
// and for errors to be checked at the end. In addition Session is designed
// to be easily used with the testing package; passing a *testing.T instance
// to NewSession allows it to set errors directly and hence tests will pass or
// fail according to whether the expect assertions are met or not.
//
// Care is taken to ensure that the file and line number of the first
// failed assertion in the session are recorded in the error stored in
// the Session.
//
// Examples
//
// func TestSomething(t *testing.T) {
//     buf := []byte{}
//     buffer := bytes.NewBuffer(buf)
//     buffer.WriteString("foo\n")
//     buffer.WriteString("bar\n")
//     buffer.WriteString("baz\n")
//     s := expect.NewSession(t, bufio.NewReader(buffer), time.Second)
//     s.Expect("foo")
//     s.Expect("bars)
//     if got, want := s.ReadLine(), "baz"; got != want {
//         t.Errorf("got %v, want %v", got, want)
//     }
// }
//
package expect

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"v.io/x/ref/internal/logger"
)

var (
	//nolint:golint // API change required.
	Timeout = errors.New("timeout")
)

// Session represents the state of an expect session.
type Session struct {
	input           *bufio.Reader
	timeout         time.Duration
	t               testing.TB
	verbose         bool
	continueOnError bool
	oerr, err       error
}

// NewSession creates a new Session. The parameter t may be safely be nil.
func NewSession(t testing.TB, input io.Reader, timeout time.Duration) *Session {
	return &Session{t: t, timeout: timeout, input: bufio.NewReader(input)}
}

// Failed returns true if an error has been encountered by a prior call.
func (s *Session) Failed() bool {
	return s.err != nil
}

// Error returns the error code (possibly nil) currently stored in the Session.
// This will include the file and line of the calling function that experienced
// the error. Use OriginalError to obtain the original error code.
func (s *Session) Error() error {
	return s.err
}

// OriginalError returns any error code (possibly nil) returned by the
// underlying library routines called.
func (s *Session) OriginalError() error {
	return s.oerr
}

// TODO(caprita): Consider removing these setters and instead expose the
// corresponding fields in Session.

// SetVerbosity enables/disables verbose debugging information, in particular,
// every line of input read will be logged via t.Logf or, if t is nil, to
// stderr.
func (s *Session) SetVerbosity(v bool) {
	s.verbose = v
}

// SetContinueOnError controls whether to invoke TB.FailNow on error, i.e.
// whether to panic on error.
func (s *Session) SetContinueOnError(value bool) {
	s.continueOnError = value
}

// SetTimeout sets the timeout for subsequent operations on this session.
func (s *Session) SetTimeout(timeout time.Duration) {
	s.timeout = timeout
}

func (s *Session) log(err error, format string, args ...interface{}) {
	s.logWithDepth(err, 2, format, args...)
}

func (s *Session) logWithDepth(err error, depth int, format string, args ...interface{}) {
	if !s.verbose {
		return
	}
	_, path, line, _ := runtime.Caller(depth + 1)
	errstr := ""
	if err != nil {
		errstr = err.Error() + ": "
	}
	loc := fmt.Sprintf("%s:%d", filepath.Base(path), line)
	o := strings.TrimRight(fmt.Sprintf(format, args...), "\n\t ")
	if s.t == nil {
		logger.Global().Infof("%s: %s%s\n", loc, errstr, o)
	} else {
		s.t.Logf("%s: %s%s", loc, errstr, o)
	}
}

// error reports the error. It must be called directly from every public
// function that can produce an error; otherwise, the file:line info will be
// incorrect.
func (s *Session) error(err error) {
	_, file, line, _ := runtime.Caller(2)
	s.oerr = err
	s.err = fmt.Errorf("%s:%d: %s", filepath.Base(file), line, err)
	if s.t != nil {
		s.t.Error(s.err)
		if !s.continueOnError {
			s.t.FailNow()
		}
	}
}

type reader func(r *bufio.Reader) (string, error)

func readAll(r *bufio.Reader) (string, error) {
	all := ""
	for {
		l, err := r.ReadString('\n')
		all += l
		if err != nil {
			if err == io.EOF {
				return all, nil
			}
			return all, err
		}
	}
}

func readLine(r *bufio.Reader) (string, error) {
	return r.ReadString('\n')
}

func (s *Session) read(f reader) (string, error) {
	ch := make(chan string, 1)
	ech := make(chan error, 1)
	go func(fn reader, io *bufio.Reader) {
		str, err := fn(io)
		if err != nil {
			ech <- err
			return
		}
		ch <- str
	}(f, s.input)
	select {
	case err := <-ech:
		return "", err
	case m := <-ch:
		return m, nil
	case <-time.After(s.timeout):
		return "", Timeout
	}
}

// Expect asserts that the next line in the input matches the supplied string.
func (s *Session) Expect(expected string) {
	if s.Failed() {
		return
	}
	line, err := s.read(readLine)
	s.log(err, "Expect: %s", line)
	if err != nil {
		s.error(err)
		return
	}
	line = strings.TrimRight(line, "\n")
	if line != expected {
		s.error(fmt.Errorf("got %q, want %q", line, expected))
	}
}

// Expectf asserts that the next line in the input matches the result of
// formatting the supplied arguments. It's equivalent to
// Expect(fmt.Sprintf(args)).
func (s *Session) Expectf(format string, args ...interface{}) {
	if s.Failed() {
		return
	}
	line, err := s.read(readLine)
	s.log(err, "Expect: %s", line)
	if err != nil {
		s.error(err)
		return
	}
	line = strings.TrimRight(line, "\n")
	expected := fmt.Sprintf(format, args...)
	if line != expected {
		s.error(fmt.Errorf("got %q, want %q", line, expected))
	}
}

func (s *Session) expectRE(pattern string, n int) (string, [][]string, error) {
	if s.Failed() {
		return "", nil, s.err
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", nil, err
	}
	line, err := s.read(readLine)
	if err != nil {
		return "", nil, err
	}
	line = strings.TrimRight(line, "\n")
	return line, re.FindAllStringSubmatch(line, n), err
}

// ExpectRE asserts that the next line in the input matches the pattern using
// regexp.MustCompile(pattern).FindAllStringSubmatch(..., n).
func (s *Session) ExpectRE(pattern string, n int) [][]string {
	if s.Failed() {
		return [][]string{}
	}
	l, m, err := s.expectRE(pattern, n)
	s.log(err, "ExpectRE: %s", l)
	if err != nil {
		s.error(err)
		return [][]string{}
	}
	if len(m) == 0 {
		s.error(fmt.Errorf("%q found no match in %q", pattern, l))
	}
	return m
}

// ExpectVar asserts that the next line in the input matches the pattern
// <name>=<value> and returns <value>.
func (s *Session) ExpectVar(name string) string {
	if s.Failed() {
		return ""
	}
	l, m, err := s.expectRE(name+"=(.*)", 1)
	s.log(err, "ExpectVar: %s", l)
	if err != nil {
		s.error(err)
		return ""
	}
	if len(m) != 1 || len(m[0]) != 2 {
		s.error(fmt.Errorf("failed to find value for %q in %q", name, l))
		return ""
	}
	return m[0][1]
}

// ExpectSetRE verifies whether the supplied set of regular expression
// parameters matches the next n (where n is the number of parameters)
// lines of input. Each line is read and matched against the supplied
// patterns in the order that they are supplied as parameters. Consequently
// the set may contain repetitions if the same pattern is expected multiple
// times. The value returned is either:
//   * nil in the case of an error, or
//   * nil if n lines are read or EOF is encountered before all expressions are
//       matched, or
//   * an array of length len(expected), whose ith element contains the result
//       of FindStringSubmatch of expected[i] on the matching string (never
//       nil). If there are no capturing groups in expected[i], the return
//       value's [i][0] element will be the entire matching string
func (s *Session) ExpectSetRE(expected ...string) [][]string {
	if s.Failed() {
		return nil
	}
	match, err := s.expectSetRE(len(expected), expected...)
	if err != nil {
		s.error(err)
		return nil
	}
	return match
}

// ExpectSetEventuallyRE is like ExpectSetRE except that it reads as much
// output as required rather than just the next n lines. The value returned is
// either:
//   * nil in the case of an error, or
//   * nil if EOF is encountered before all expressions are matched, or
//   * an array of length len(expected), whose ith element contains the result
//       of FindStringSubmatch of expected[i] on the matching string (never
//       nil). If there are no capturing groups in expected[i], the return
//       value's [i][0] will contain the entire matching string
// This function stops consuming output as soon as all regular expressions are
// matched.
func (s *Session) ExpectSetEventuallyRE(expected ...string) [][]string {
	if s.Failed() {
		return nil
	}
	matches, err := s.expectSetRE(-1, expected...)
	if err != nil {
		s.error(err)
		return nil
	}
	return matches
}

// expectSetRE will look for the expected set of patterns in the next
// numLines of output or in all remaining output. If all expressions are
// matched, no more output is consumed.
func (s *Session) expectSetRE(numLines int, expected ...string) ([][]string, error) {
	matches := make([][]string, len(expected))
	regexps := make([]*regexp.Regexp, len(expected))
	for i, expRE := range expected {
		re, err := regexp.Compile(expRE)
		if err != nil {
			return nil, err
		}
		regexps[i] = re
	}
	i := 0
	matchCount := 0
	for {
		if matchCount == len(expected) {
			break
		}
		line, err := s.read(readLine)
		line = strings.TrimRight(line, "\n")
		s.logWithDepth(err, 2, "ExpectSetRE: %s", line)
		if err != nil {
			if numLines >= 0 {
				return nil, err
			}
			break
		}

		// Match the line against all regexp's and remove each regexp
		// that matches.
		for i, re := range regexps {
			if re == nil {
				continue
			}
			match := re.FindStringSubmatch(line)
			if match != nil {
				matchCount++
				regexps[i] = nil
				matches[i] = match
				// Don't allow this line to be matched by more than one re.
				break
			}
		}

		i++
		if numLines > 0 && i >= numLines {
			break
		}
	}

	// It's an error if there are any unmatched regexps.
	unmatchedRes := make([]string, 0)
	for i, re := range regexps {
		if re != nil {
			unmatchedRes = append(unmatchedRes, expected[i])
		}
	}
	if len(unmatchedRes) > 0 {
		return nil, fmt.Errorf("found no match for [%v]", strings.Join(unmatchedRes, ","))
	}
	return matches, nil
}

// ReadLine reads the next line, if any, from the input stream. It will set
// the error state to io.EOF if it has read past the end of the stream.
// ReadLine has no effect if an error has already occurred.
func (s *Session) ReadLine() string {
	if s.Failed() {
		return ""
	}
	l, err := s.read(readLine)
	s.log(err, "Readline: %s", l)
	if err != nil {
		s.error(err)
	}
	return strings.TrimRight(l, "\n")
}

// ReadAll reads all remaining input on the stream. Unlike all of the other
// methods it does not strip newlines from the input.
// ReadAll has no effect if an error has already occurred.
func (s *Session) ReadAll() (string, error) {
	if s.Failed() {
		return "", s.err
	}
	return s.read(readAll)
}

func (s *Session) ExpectEOF() error {
	if s.Failed() {
		return s.err
	}
	buf := [1024]byte{}
	n, err := s.input.Read(buf[:])
	if n != 0 || err == nil {
		s.error(fmt.Errorf("unexpected input %d bytes: %q", n, string(buf[:n])))
		return s.err
	}
	if err != io.EOF {
		s.error(err)
		return s.err
	}
	return nil
}

// Finish reads all remaining input on the stream regardless of any
// prior errors and writes it to the supplied io.Writer parameter if non-nil.
// It returns both the data read and the prior error, if any, otherwise it
// returns any error that occurred reading the rest of the input.
func (s *Session) Finish(w io.Writer) (string, error) {
	a, err := s.read(readAll)
	if w != nil {
		fmt.Fprint(w, a)
	}
	if s.Failed() {
		return a, s.err
	}
	return a, err
}
