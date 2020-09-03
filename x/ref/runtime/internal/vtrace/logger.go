// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vtrace

import (
	"fmt"
	"path/filepath"
	"runtime"

	"v.io/v23/context"
)

const (
	initialMaxStackBufSize = 128 * 1024
)

type Logger struct{}

func (*Logger) InfoDepth(ctx *context.T, depth int, args ...interface{}) {
	span := getSpan(ctx)
	if span == nil {
		return
	}
	file, line := computeFileAndLine(depth + 1)
	output := fmt.Sprintf("%s:%d] %s", file, line, fmt.Sprint(args...))
	span.Annotate(output)
}

func (*Logger) InfoStack(ctx *context.T, all bool) {
	span := getSpan(ctx)
	if span == nil {
		return
	}
	span.Annotate(printStack(all))
}

func computeFileAndLine(depth int) (file string, line int) {
	var ok bool
	_, file, line, ok = runtime.Caller(depth + 1)
	if !ok {
		file = "???"
		line = 1
	} else {
		file = filepath.Base(file)
	}
	return
}

func printStack(all bool) string {
	n := initialMaxStackBufSize
	trace := make([]byte, n)
	nbytes := runtime.Stack(trace, all)
	if nbytes < len(trace) {
		return string(trace[:nbytes])
	}
	return string(trace)
}

func (*Logger) VDepth(ctx *context.T, depth int, level int) bool {
	return GetVTraceLevel(ctx) >= level
}

func (v *Logger) VIDepth(ctx *context.T, depth int, level int) context.Logger {
	// InfoStack logs the current goroutine's stack if the all parameter
	// is false, or the stacks of all goroutine{
	if v.VDepth(ctx, depth, level) {
		return v
	}
	return nil
}

func (v *Logger) FlushLog() {}
