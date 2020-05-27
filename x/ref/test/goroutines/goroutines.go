// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goroutines

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var goroutineHeaderRE = regexp.MustCompile(`^goroutine (\d+) \[([^\]]+)\]:$`)
var stackFileRE = regexp.MustCompile(`^\s+([^:]+):(\d+)(?: \+0x([0-9A-Fa-f]+))?$`)

var ignoredGoroutines = []string{
	"runtime.ensureSigM",
	"sync.(*WaitGroup).Done",
	"security.newOpenSSLSigner.func",
	"security.unmarshalPublicKeyImpl.func",
	"net/http.(*Transport).dialConn",
	"net/http.(*Transport).getConn.func2",
}

type Goroutine struct {
	ID      int
	State   string
	Stack   []*Frame
	Creator *Frame
}

// Get gets a set of currently running goroutines and parses them into a
// structured representation.
func Get() ([]*Goroutine, error) {
	bufsize, read := 1<<20, 0
	buf := make([]byte, bufsize)
	for {
		read = runtime.Stack(buf, true)
		if read < bufsize {
			buf = buf[:read]
			break
		}
		bufsize *= 2
		buf = make([]byte, bufsize)
	}
	return Parse(buf, true)
}

// Parse parses a stack trace into a structure representation.
func Parse(buf []byte, ignore bool) ([]*Goroutine, error) {
	scanner := bufio.NewScanner(bytes.NewReader(buf))
	var out []*Goroutine
	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		g, err := parseGoroutine(scanner)
		if err != nil {
			return out, fmt.Errorf("Error %v parsing trace:\n%s", err, string(buf))
		}
		if !ignore || !shouldIgnore(g) {
			out = append(out, g)
		}
	}
	return out, scanner.Err()
}

func shouldIgnore(g *Goroutine) bool {
	for _, ignored := range ignoredGoroutines {
		if c := g.Creator; c != nil && strings.Contains(c.Call, ignored) {
			return true
		}
		for _, f := range g.Stack {
			if strings.Contains(f.Call, ignored) {
				return true
			}
		}
	}
	return false
}

func parseGoroutine(scanner *bufio.Scanner) (*Goroutine, error) {
	g := &Goroutine{}
	matches := goroutineHeaderRE.FindSubmatch(scanner.Bytes())
	if len(matches) != 3 {
		return nil, fmt.Errorf("Could not parse goroutine header from: %s", scanner.Text())
	}
	id, err := strconv.ParseInt(string(matches[1]), 10, 64)
	if err != nil {
		return nil, err
	}
	g.ID = int(id)
	g.State = string(matches[2])

	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			break
		}
		frame, err := parseFrame(scanner)
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(frame.Call, "created by ") {
			frame.Call = frame.Call[len("created by "):]
			g.Creator = frame
			break
		}
		g.Stack = append(g.Stack, frame)
	}
	return g, nil
}

func (g *Goroutine) writeTo(w io.Writer) {
	fmt.Fprintf(w, "goroutine %d [%s]:\n", g.ID, g.State)
	for _, f := range g.Stack {
		f.writeTo(w)
	}
	if g.Creator != nil {
		fmt.Fprint(w, "created by ")
		g.Creator.writeTo(w)
	}
}

// Frame represents a single stack frame.
type Frame struct {
	Call   string
	File   string
	Line   int
	Offset int
}

func parseFrame(scanner *bufio.Scanner) (*Frame, error) {
	f := &Frame{Call: scanner.Text()}
	if !scanner.Scan() {
		return nil, fmt.Errorf("Frame lacked a second line %s", f.Call)
	}
	matches := stackFileRE.FindSubmatch(scanner.Bytes())
	if len(matches) < 4 {
		return nil, fmt.Errorf("Could not parse file reference from %s", scanner.Text())
	}
	f.File = string(matches[1])
	line, err := strconv.ParseInt(string(matches[2]), 10, 64)
	if err != nil {
		return nil, err
	}
	f.Line = int(line)
	if len(matches[3]) > 0 {
		offset, err := strconv.ParseInt(string(matches[3]), 16, 64)
		if err != nil {
			return nil, err
		}
		f.Offset = int(offset)
	}
	return f, nil
}

func (f *Frame) writeTo(w io.Writer) {
	fmt.Fprintln(w, f.Call)
	if f.Offset != 0 {
		fmt.Fprintf(w, "\t%s:%d +0x%x\n", f.File, f.Line, f.Offset)
	} else {
		fmt.Fprintf(w, "\t%s:%d\n", f.File, f.Line)
	}
}

// Format formats Goroutines back into the normal string representation.
func Format(gs ...*Goroutine) []byte {
	var buf bytes.Buffer
	for i, g := range gs {
		if i != 0 {
			buf.WriteRune('\n')
		}
		g.writeTo(&buf)
	}
	return buf.Bytes()
}

// ErrorReporter is used by NoLeaks to report errors.  testing.T implements
// this interface and is normally passed.
type ErrorReporter interface {
	Errorf(format string, args ...interface{})
}

// NoLeaks helps test that a test isn't leaving extra goroutines after it finishes.
//
// The normal way to use it is:
// func TestFoo(t *testing.T) {
//   defer goroutines.NoLeaks(t, time.Second)()
//
//   ... Normal test code here ...
//
// }
//
// The test will fail if there are goroutines running at the end of the test
// that weren't running at the beginning.
// Since testing for goroutines being finished can be racy, the detector
// can wait the specified duration for the set of goroutines to return to the
// initial set.
func NoLeaks(t ErrorReporter, wait time.Duration) func() {
	gs, err := Get()
	if err != nil {
		return func() {} // If we can't parse correctly we let the test pass.
	}
	bycreator := map[string]int{}
	for _, g := range gs {
		key := ""
		if g.Creator != nil {
			key = g.Creator.Call
		}
		bycreator[key]++
	}
	return func() {
		var left []*Goroutine
		backoff := 10 * time.Millisecond
		start := time.Now()
		until := start.Add(wait)
		for {
			cgs, err := Get()
			if err != nil {
				return // If we can't parse correctly we let the test pass.
			}
			left = left[:0]
			cbycreator := map[string]int{}
			for _, g := range cgs {
				key := ""
				if g.Creator != nil {
					key = g.Creator.Call
				}
				cbycreator[key]++
				if cbycreator[key] > bycreator[key] {
					left = append(left, g)
				}
			}
			if len(left) == 0 {
				return
			}
			if time.Now().After(until) {
				t.Errorf("%d extra Goroutines outstanding:\n %s", len(left),
					string(Format(left...)))
				return
			}
			time.Sleep(backoff)
			if backoff *= 2; backoff > time.Second {
				backoff = time.Second
			}
		}
	}
}
