// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package context

import "fmt"

// loggerDiscard implements Logger but silently does nothing.
type loggerDiscard struct{}

func (ld *loggerDiscard) InfoDepth(ctx *T, depth int, args ...interface{}) {}

func (ld *loggerDiscard) InfoStack(ctx *T, all bool) {}

func (ld *loggerDiscard) VDepth(ctx *T, depth int, level int) bool {
	return false
}

func (ld *loggerDiscard) VIDepth(ctx *T, depth int, level int) Logger {
	return ld
}

func (ld *loggerDiscard) FlushLog() {}

func (t *T) prefixedArgs(args []interface{}) []interface{} {
	prefix := t.Value(prefixKey)
	if prefix == nil {
		return args
	}
	return append([]interface{}{prefix}, args...)
}

func (t *T) prefixedFormat(format string) string {
	prefix := t.Value(prefixKey)
	if prefix == nil {
		return format
	}
	return "%v: " + format
}

// Info implements logging.InfoLog, it calls the registered Logger and then
// the registered ContextLogger.
func (t *T) Info(args ...interface{}) {
	t.logger.InfoDepth(1, t.prefixedArgs(args)...)
	t.ctxLogger.InfoDepth(t, 1, t.prefixedArgs(args)...)
}

// InfoDepth implements logging.InfoLog; it calls the registered Logger and then
// the registered ContextLogger.
func (t *T) InfoDepth(depth int, args ...interface{}) {
	t.logger.InfoDepth(depth+1, t.prefixedArgs(args)...)
	t.ctxLogger.InfoDepth(t, depth+1, t.prefixedArgs(args)...)
}

// Infof implements logging.InfoLog; it calls the registered Logger and then
// the registered ContextLogger.
func (t *T) Infof(format string, args ...interface{}) {
	line := fmt.Sprintf(t.prefixedFormat(format), t.prefixedArgs(args)...)
	t.logger.InfoDepth(1, line)
	t.ctxLogger.InfoDepth(t, 1, line)
}

// InfoStack implements logging.InfoLog; it calls the registered Logger and then
// the registered ContextLogger.
func (t *T) InfoStack(all bool) {
	t.logger.InfoStack(all)
	t.ctxLogger.InfoStack(t, all)
}

// Error immplements and calls logging.ErrorLog; it calls the registered Logger and then
// the registered ContextLogger.
func (t *T) Error(args ...interface{}) {
	t.logger.ErrorDepth(1, t.prefixedArgs(args)...)
	t.ctxLogger.InfoDepth(t, 1, t.prefixedArgs(args)...)
}

// ErrorDepth immplements and calls logging.ErrorLog; it calls the registered Logger and then
// the registered ContextLogger.
func (t *T) ErrorDepth(depth int, args ...interface{}) {
	t.logger.ErrorDepth(depth+1, t.prefixedArgs(args)...)
	t.ctxLogger.InfoDepth(t, depth+1, t.prefixedArgs(args)...)
}

// Errorf immplements and calls logging.ErrorLog; it calls the registered Logger and then
// the registered ContextLogger.
func (t *T) Errorf(format string, args ...interface{}) {
	line := fmt.Sprintf(t.prefixedFormat(format), t.prefixedArgs(args)...)
	t.logger.ErrorDepth(1, line)
	t.ctxLogger.InfoDepth(t, 1, line)
}

// Fatal implements logging.FatalLog; it calls the registered Logger but not
// the ContextLogger.
func (t *T) Fatal(args ...interface{}) {
	t.logger.FatalDepth(1, t.prefixedArgs(args)...)
}

// FatalDepth implements logging.FatalLog; it calls the registered Logger but not
// the ContextLogger.
func (t *T) FatalDepth(depth int, args ...interface{}) {
	t.logger.FatalDepth(depth+1, t.prefixedArgs(args)...)
}

// Fatalf implements logging.FatalLog; it calls the registered Logger but not
// the ContextLogger.
func (t *T) Fatalf(format string, args ...interface{}) {
	t.logger.FatalDepth(1, fmt.Sprintf(t.prefixedFormat(format), t.prefixedArgs(args)...))
}

// Panic implements logging.PanicLog; it calls the registered Logger but not
// the ContextLogger.
func (t *T) Panic(args ...interface{}) {
	t.logger.PanicDepth(1, t.prefixedArgs(args)...)
}

// PanicDepth implements logging.PanicLog; it calls the registered Logger but not
// the ContextLogger.
func (t *T) PanicDepth(depth int, args ...interface{}) {
	t.logger.PanicDepth(depth+1, t.prefixedArgs(args)...)
}

// Panicf implements logging.PanicLog; it calls the registered Logger but not
// the ContextLogger.
func (t *T) Panicf(format string, args ...interface{}) {
	t.logger.PanicDepth(1, fmt.Sprintf(t.prefixedFormat(format), t.prefixedArgs(args)...))
}

// V implements logging.Verbosity; it returns the 'or' of the values
// returned by the register Logger and ContextLogger.
func (t *T) V(level int) bool {
	return t.logger.VDepth(1, level) || t.ctxLogger.VDepth(t, 1, level)
}

// VDepth implements logging.Verbosity; it returns the 'or' of the values
// returned by the register Logger and ContextLogger.
func (t *T) VDepth(depth int, level int) bool {
	return t.logger.VDepth(depth+1, level) || t.ctxLogger.VDepth(t, depth+1, level)
}

type viLogger struct {
	ctx    *T
	logger interface {
		Info(args ...interface{})
		Infof(format string, args ...interface{})
		InfoDepth(depth int, args ...interface{})
		InfoStack(all bool)
	}
	ctxLogger Logger
}

func (v *viLogger) Info(args ...interface{}) {
	v.logger.InfoDepth(1, v.ctx.prefixedArgs(args)...)
	v.ctxLogger.InfoDepth(v.ctx, 1, v.ctx.prefixedArgs(args)...)
}

func (v *viLogger) Infof(format string, args ...interface{}) {
	line := fmt.Sprintf(v.ctx.prefixedFormat(format), v.ctx.prefixedArgs(args)...)
	v.logger.InfoDepth(1, line)
	v.ctxLogger.InfoDepth(v.ctx, 1, line)
}

func (v *viLogger) InfoDepth(depth int, args ...interface{}) {
	v.logger.InfoDepth(depth+1, v.ctx.prefixedArgs(args)...)
	v.ctxLogger.InfoDepth(v.ctx, depth+1, v.ctx.prefixedArgs(args)...)
}

func (v *viLogger) InfoStack(all bool) {
	v.logger.InfoStack(all)
	v.ctxLogger.InfoStack(v.ctx, all)
}

// VI implements logging.Verbosity.
func (t *T) VI(level int) interface {
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	InfoDepth(depth int, args ...interface{})
	InfoStack(all bool)
} {
	v := &viLogger{
		ctx:       t,
		logger:    t.logger.VIDepth(1, level),
		ctxLogger: t.ctxLogger.VIDepth(t, 1, level),
	}
	if v.ctxLogger == nil {
		v.ctxLogger = &loggerDiscard{}
	}
	return v
}

// VIDepth implements logging.Verbosity.
func (t *T) VIDepth(depth int, level int) interface {
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	InfoDepth(depth int, args ...interface{})
	InfoStack(all bool)
} {
	v := &viLogger{
		ctx:       t,
		logger:    t.logger.VIDepth(depth+1, level),
		ctxLogger: t.ctxLogger.VIDepth(t, depth+11, level),
	}
	if v.ctxLogger == nil {
		v.ctxLogger = &loggerDiscard{}
	}
	return v
}

// Flush flushes all pending log I/O.
func (t *T) FlushLog() {
	t.logger.FlushLog()
	t.ctxLogger.FlushLog()
}
