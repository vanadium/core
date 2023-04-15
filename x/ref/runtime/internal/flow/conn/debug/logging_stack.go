// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build debug

package debug

import (
	"fmt"
	"runtime"
	"strings"
)

func Callers(skip, limit int) []uintptr {
	pc := make([]uintptr, limit)
	n := runtime.Callers(skip, pc)
	return pc[:n]
}

func FormatFramesFunctionsOnly(stack []uintptr, exclusions []string) string {
	out := strings.Builder{}
	if len(stack) == 0 {
		return ""
	}
	if exclusions == nil {
		exclusions = callerExclusions
	}
	frames := runtime.CallersFrames(stack)
	for {
		frame, more := frames.Next()
		skip := false
		for _, exclude := range exclusions {
			if strings.HasSuffix(frame.Function, exclude) {
				skip = true
				break
			}
		}
		if !skip {
			fmt.Fprintf(&out, "\t%v:\t%3v: %v\n", strings.TrimPrefix(frame.File, stripFilePrefix), frame.Line, strings.TrimPrefix(frame.Function, stripFunctionPrefix))
		}
		if !more {
			break
		}
	}
	return out.String()
}

var (
	callerExclusions = []string{
		"runtime.Callers",
		"flow/conn/debug.Callers",
		"runtime.goexit",
	}

	stripFilePrefix string

	stripFunctionPrefix = "v.io/x/ref/runtime/internal/"
)

func init() {
	_, file, _, _ := runtime.Caller(0)
	idx := strings.Index(file, "flow/conn/debug/logging_stack.go")
	stripFilePrefix = file[:idx]
}
