// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build debug

package debug

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
)

type caller struct {
	stack  []uintptr
	record string
}

type CallerCircularList struct {
	mu      sync.Mutex
	callers [100]caller
	e       int
}

func (cl *CallerCircularList) Append(record string) {
	if !traceWriteq {
		return
	}
	cl.mu.Lock()
	defer cl.mu.Unlock()
	cl.callers[cl.e] = caller{stack: Callers(0, 15), record: record}
	cl.e = (cl.e + 1) % len(cl.callers)
}

func (cl *CallerCircularList) String() string {
	if !traceWriteq {
		return ""
	}
	cl.mu.Lock()
	defer cl.mu.Unlock()
	out := strings.Builder{}
	prev := func(e int) int {
		if e == 0 {
			e = len(cl.callers)
		}
		return e - 1
	}
	e := prev(cl.e)
	for {
		c := cl.callers[e]
		if len(c.stack) == 0 {
			break
		}
		out.WriteString(c.record)
		out.WriteByte('\n')
		out.WriteString(FormatFramesFunctionsOnly(c.stack, writeqExclusions))
		if e == cl.e {
			break
		}
		e = prev(e)
	}
	return out.String()
}

var writeqExclusions = append(callerExclusions,
	"flow/conn/debug.(*CallerCircularList).Append",
)

func (cl *CallerCircularList) DumpAndExit(msg string) {
	if !traceWriteq {
		return
	}
	fmt.Fprintf(os.Stderr, "------ %s ------\n", msg)
	fmt.Fprintf(os.Stderr, "------ Circular List ------\n")
	fmt.Fprintf(os.Stderr, "%s\n", cl.String())
	fmt.Fprintf(os.Stderr, "------ Goroutine Dump ------\n")
	buf := make([]byte, 1<<16)
	n := runtime.Stack(buf, true)
	fmt.Fprintf(os.Stderr, "%s\n\n", buf[:n])
	panic(msg)
}
