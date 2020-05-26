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

// CallerName returns the name of the calling function.
func CallerName() string {
	var funcName string
	pc, _, _, ok := runtime.Caller(1)
	if ok {
		function := runtime.FuncForPC(pc)
		if function != nil {
			funcName = path.Base(function.Name())
		}
	}
	return funcName
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

func getLogger(ctx *context.T) logging.Logger {
	if ctx == nil {
		return logger.Global()
	}
	return ctx
}

// LogCallf logs that its caller has been called given the arguments passed to
// it. It returns a function that is to be called when the caller returns,
// logging the caller’s return along with the arguments it is provided with
// which represent the named returns values from the function.
// It is primarily intended to be automotically added and removed using
// some form of annotation tool. If used manually, the apilog.CallerName
// function can be used to obtain the function name and file location. Generated
// calls to LogCallf will generally insert the actual value for callerName to
// avoid the run time overhead of obtaining them.
// LogCallf will also log an invocation identifier automatically.
//
// The canonical way to use LogCallf is as follows:
//
//     func Function(ctx *context.T, a Type1, b Type2) ReturnType {
//         defer apilog.LogCallf(ctx, "<package>.<function>", "%v, %v", a, b)(ctx)
//         // ... function body ...
//         return retVal
//     }
//
// In order for return values to be logged they must be named.
func LogCallf(ctx *context.T, callerName, format string, v ...interface{}) func(*context.T, string, ...interface{}) {
	l := getLogger(ctx)
	if !l.V(logCallLogLevel) { // TODO(mattr): add call to vtrace.
		return func(*context.T, string, ...interface{}) {}
	}
	invocationID := newInvocationIdentifier()
	output := fmt.Sprintf("call[%s %s]: %s", callerName, invocationID, fmt.Sprintf(format, v...))
	l.InfoDepth(1, output)
	// TODO(mattr): annotate vtrace span.
	return func(ctx *context.T, format string, v ...interface{}) {
		output := fmt.Sprintf("return[%s %s]: %v", callerName, invocationID, fmt.Sprintf(format, derefSlice(v)...))
		getLogger(ctx).InfoDepth(1, output)
		// TODO(mattr): annotate vtrace span.
	}
}
