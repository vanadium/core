// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package apilog provides functions to be used in conjunction with logcop.
// In particular, logcop will inject calls to these functions as the first
// statement in methods that implement the v23 API. The output can
// be controlled by vlog verbosity or vtrace.
// --vmodule=apilog=<level> can be used to globally control logging,
// and --vmodule=module=<level> can also be used to control logging
// on a per-file basis.
package apilog

import (
	"fmt"
	"path"
	"reflect"
	"runtime"
	"sync/atomic"

	"v.io/v23/context"
	"v.io/v23/logging"

	"v.io/x/ref/internal/logger"
)

// logCallLogLevel is the log level beyond which calls are logged.
const logCallLogLevel = 1

func callerLocation() string {
	var funcName string
	const stackSkip = 1
	pc, _, _, ok := runtime.Caller(stackSkip + 1)
	if ok {
		function := runtime.FuncForPC(pc)
		if function != nil {
			funcName = path.Base(function.Name())
		}
	}
	return funcName
}

func getLogger(ctx *context.T) logging.Logger {
	if ctx == nil {
		return logger.Global()
	}
	return ctx
}

// LogCall logs that its caller has been called given the arguments
// passed to it. It returns a function that is supposed to be called
// when the caller returns, logging the caller’s return along with the
// arguments it is provided with.
// File name and line number of the call site and a randomly generated
// invocation identifier is logged automatically.  The path through which
// the caller function returns will be logged automatically too.
//
// The canonical way to use LogCall is along the lines of the following:
//
//     func Function(ctx *context.T, a Type1, b Type2) ReturnType {
//         defer apilog.LogCall(ctx, a, b)(ctx)
//         // ... function body ...
//         return retVal
//     }
//
// To log the return value as the function returns, the following
// pattern should be used. Note that pointers to the output
// variables should be passed to the returning function, not the
// variables themselves. Also note that nil can be used when a context.T
// is not available:
//
//     func Function(a Type1, b Type2) (r ReturnType) {
//         defer apilog.LogCall(nil, a, b)(nil, &r)
//         // ... function body ...
//         return computeReturnValue()
//     }
//
// Note that when using this pattern, you do not need to actually
// assign anything to the named return variable explicitly.  A regular
// return statement would automatically do the proper return variable
// assignments.
//
// The log injector tool will automatically insert a LogCall invocation
// into all implementations of the public API it runs, unless a Valid
// Log Construct is found.  A Valid Log Construct is defined as one of
// the following at the beginning of the function body (i.e. should not
// be preceded by any non-whitespace or non-comment tokens):
//     1. defer apilog.LogCall(optional arguments)(optional pointers to return values)
//     2. defer apilog.LogCallf(argsFormat, optional arguments)(returnValuesFormat, optional pointers to return values)
//     3. // nologcall
//
// The comment "// nologcall" serves as a hint to log injection and
// checking tools to exclude the function from their consideration.
// It is used as follows:
//
//     func FunctionWithoutLogging(args ...interface{}) {
//         // nologcall
//         // ... function body ...
//     }
//
func LogCall(ctx *context.T, v ...interface{}) func(*context.T, ...interface{}) {
	l := getLogger(ctx)
	if !l.V(logCallLogLevel) { // TODO(mattr): add call to vtrace.
		return func(*context.T, ...interface{}) {}
	}
	callerLocation := callerLocation()
	invocationId := newInvocationIdentifier()
	var output string
	if len(v) > 0 {
		output = fmt.Sprintf("call[%s %s]: args:%v", callerLocation, invocationId, v)
	} else {
		output = fmt.Sprintf("call[%s %s]", callerLocation, invocationId)
	}
	l.InfoDepth(1, output)

	// TODO(mattr): annotate vtrace span.
	return func(ctx *context.T, v ...interface{}) {
		var output string
		if len(v) > 0 {
			output = fmt.Sprintf("return[%s %s]: %v", callerLocation, invocationId, derefSlice(v))
		} else {
			output = fmt.Sprintf("return[%s %s]", callerLocation, invocationId)
		}
		getLogger(ctx).InfoDepth(1, output)
		// TODO(mattr): annotate vtrace span.
	}
}

// LogCallf behaves identically to LogCall, except it lets the caller to
// customize the log messages via format specifiers, like the following:
//
//     func Function(a Type1, b Type2) (r, t ReturnType) {
//         defer apilog.LogCallf(nil, "a: %v, b: %v", a, b)(nil, "(r,t)=(%v,%v)", &r, &t)
//         // ... function body ...
//         return finalR, finalT
//     }
//
func LogCallf(ctx *context.T, format string, v ...interface{}) func(*context.T, string, ...interface{}) {
	l := getLogger(ctx)
	if !l.V(logCallLogLevel) { // TODO(mattr): add call to vtrace.
		return func(*context.T, string, ...interface{}) {}
	}
	callerLocation := callerLocation()
	invocationId := newInvocationIdentifier()
	output := fmt.Sprintf("call[%s %s]: %s", callerLocation, invocationId, fmt.Sprintf(format, v...))
	l.InfoDepth(1, output)
	// TODO(mattr): annotate vtrace span.
	return func(ctx *context.T, format string, v ...interface{}) {
		output := fmt.Sprintf("return[%s %s]: %v", callerLocation, invocationId, fmt.Sprintf(format, derefSlice(v)...))
		getLogger(ctx).InfoDepth(1, output)
		// TODO(mattr): annotate vtrace span.
	}
}

func derefSlice(slice []interface{}) []interface{} {
	o := make([]interface{}, 0, len(slice))
	for _, x := range slice {
		o = append(o, reflect.Indirect(reflect.ValueOf(x)).Interface())
	}
	return o
}

var invocationCounter uint64 = 0

// newInvocationIdentifier generates a unique identifier for a method invocation
// to make it easier to match up log lines for the entry and exit of a function
// when looking at a log transcript.
func newInvocationIdentifier() string {
	const (
		charSet    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789abcdefghijklmnopqrstuvwxyz"
		charSetLen = uint64(len(charSet))
	)
	r := []byte{'@'}
	for n := atomic.AddUint64(&invocationCounter, 1); n > 0; n /= charSetLen {
		r = append(r, charSet[n%charSetLen])
	}
	return string(r)
}
