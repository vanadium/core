// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package context provides an implementation of context.Context with
// additional functionality used within the Vanadium code base. The functions
// in this package mirror those from the go library's context package with
// two key differences as documented below.
//
// 1. context.T provides the concept of a 'root' context that is typically
//    created by the runtime and made available to the application code.
//    The WithRootCancel function creates a context from the root context that
//    is detached from all of its parent contexts, except for the root, in terms
//    of cancelation (both explicit and time based cancelation) but otherwise
//    inherits all other state from its parent. Such contexts are used for
//    asynchronous operations that persist past the return of the function tha
//    created function. A typical use case would be a goroutine listening for
//    new network connections. Canceling the immediate parent of these contexts
//    has no effect on them; canceling the root context will lead to their
//    cancelation and is therefore a convenient mechanism for the runtime to
//    terminate all asynchronous/background processing when it is shutdown
//    This gives these background processes the opportunity to clean up any
//    external state.
// 2. context.T provides access to logging functions and thus allows for
//    different packages or code paths to be configured to use different
//    loggers.
//
// Note that since context.T implements context.Context (it embeds the interface)
// it can be passed in to code that expects context.Context. In addition APIs
// that do not manipulate the context using the functions in this package should
// be written to expect a context.Context rather than *context.T.
//
// Application code receives contexts in two main ways:
//
// 1) A context.T is returned from v23.Init().  This will generally be
// used to set up servers in main, or for stand-alone client programs.
//    func main() {
//      ctx, shutdown := v23.Init()
//      defer shutdown()
//
//      doSomething(ctx)
//    }
//
// 2) A context.T is passed to every server method implementation as the first
// parameter.
//    func (m *myServer) Method(ctx *context.T, call rpc.ServerCall) error {
//      doSomething(ctx)
//    }
//
// Once you have a context you can derive further contexts to change settings.
// for example to adjust a deadline you might do:
//    func main() {
//      ctx, shutdown := v23.Init()
//      defer shutdown()
//      // We'll use cacheCtx to lookup data in memcache
//      // if it takes more than a second to get data from
//      // memcache we should just skip the cache and perform
//      // the slow operation.
//      cacheCtx, cancel := WithTimeout(ctx, time.Second)
//      if err := FetchDataFromMemcache(cacheCtx, key); err != nil {
//        // Here we use the original ctx, not the derived cacheCtx
//        // so we aren't constrained by the 1 second timeout.
//        RecomputeData(ctx, key)
//      }
//    }
//
package context

import (
	"context"
	"sync/atomic"
	"time"

	"v.io/v23/logging"
	"v.io/x/lib/vlog"
)

type internalKey int

const (
	rootKey = internalKey(iota)
	loggerKey
	ctxLoggerKey
	prefixKey
)

// Logger is a logger that uses a passed in T to configure the logging behavior;
// it is called by the correspoding logging methods of *T.
type Logger interface {
	// InfoDepth logs to the INFO log. depth is used to determine which call frame to log.
	InfoDepth(ctx *T, depth int, args ...interface{})

	// InfoStack logs the current goroutine's stack if the all parameter
	// is false, or the stacks of all goroutines if it's true.
	InfoStack(ctx *T, all bool)

	// VDepth returns true if the configured logging level is greater than or equal to its parameter. depth
	// is used to determine which call frame to test against.
	VDepth(ctx *T, depth int, level int) bool

	// VIDepth is like VDepth, except that it returns nil if there level is greater than the
	// configured log level.
	VIDepth(ctx *T, depth int, level int) Logger

	// Flush flushes all pending log I/O.
	FlushLog()
}

// CancelFunc is the signature of the function used to cancel a context.
type CancelFunc context.CancelFunc

// Canceled is returned by contexts which have been canceled.
var Canceled = context.Canceled

// DeadlineExceeded is returned by contexts that have exceeded their
// deadlines and therefore been canceled automatically.
var DeadlineExceeded = context.DeadlineExceeded

// T carries deadlines, cancellation and data across API boundaries.
// It is safe to use a T from multiple goroutines simultaneously.  The
// zero-type of context is uninitialized and will panic if used
// directly by application code. It also implements v23/logging.Logger and
// hence can be used directly for logging (e.g. ctx.Infof(...)).
type T struct {
	context.Context
	logger    logging.Logger
	ctxLogger Logger
	// parent and key are used to keep track of all the keys that are used
	// WithValue so that WithRootCancel can copy the key/value pairs to
	// the new background context that it creates.
	parent *T
	key    interface{}
}

// Value implements context.Value.
func (t *T) Value(key interface{}) interface{} {
	if val := t.Context.Value(key); val != nil {
		return val
	}
	// Make sure to chain the call to Value to any parent
	// that was set to handle the case where the vanadium
	// context was (re)created from a go context.
	if t.parent != nil {
		return t.parent.Value(key)
	}
	return nil
}

// RootContext creates a new root context with no data attached.
// A RootContext is cancelable (see WithCancel).
// Typically you should not call this function, instead you should derive
// contexts from other contexts, such as the context returned from v23.Init
// or the result of the Context() method on a ServerCall.  This function
// is sometimes useful in tests, where it is undesirable to initialize a
// runtime to test a function that reads from a T.
func RootContext() (*T, CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	nctx := newT(ctx)
	nctx = WithValue(nctx, loggerKey, nctx.logger)
	nctx = WithValue(nctx, ctxLoggerKey, nctx.ctxLogger)
	nctx = WithValue(nctx, rootKey, nctx)
	return nctx, CancelFunc(cancel)
}

func newT(ctx context.Context) *T {
	return &T{
		Context:   ctx,
		logger:    logging.Discard,
		ctxLogger: &loggerDiscard{},
	}
}

func newChild(ctx context.Context, parent *T) *T {
	return &T{
		parent:    parent,
		Context:   ctx,
		logger:    parent.logger,
		ctxLogger: parent.ctxLogger,
		key:       nil,
	}
}

// WithLogger returns a child of the current context that embeds the supplied
// logger.
func WithLogger(parent *T, logger logging.Logger) *T {
	if !parent.Initialized() {
		return nil
	}
	child := WithValue(parent, loggerKey, logger)
	child.logger = logger
	return child
}

// WithContextLogger returns a child of the current context that embeds the
// supplied context logger.
func WithContextLogger(parent *T, logger Logger) *T {
	if !parent.Initialized() {
		return nil
	}
	child := WithValue(parent, ctxLoggerKey, logger)
	child.ctxLogger = logger
	return child
}

// WithLoggingPrefix returns a child of the current context that embeds the
// supplied prefix. The prefix will be prepended to all log output, both
// formated and unformatted.
func WithLoggingPrefix(parent *T, prefix interface{}) *T {
	if !parent.Initialized() {
		return nil
	}
	return WithValue(parent, prefixKey, prefix)
}

// LoggingPrefix returns the value set by the most recent call of
// WithLoggingPrefix.
func LoggingPrefix(ctx *T) interface{} {
	return ctx.Value(prefixKey)
}

// LoggerFromContext returns the implementation of the logger
// associated with this context. It should almost never need to be used by application
// code.
func LoggerFromContext(ctx context.Context) logging.Logger {
	l, _ := ctx.Value(loggerKey).(logging.Logger)
	return l
}

// Initialized returns true if this context has been properly initialized
// by a runtime.
func (t *T) Initialized() bool {
	return t != nil && t.Context != nil
}

// WithValue returns a child of the current context that will return
// the given val when Value(key) is called.
func WithValue(parent *T, key interface{}, val interface{}) *T {
	child := newChild(context.WithValue(parent.Context, key, val), parent)
	child.key = key
	return child
}

// WithCancel returns a child of the current context along with
// a function that can be used to cancel it.  After cancel() is
// called the channels returned by the Done() methods of the new context
// (and all context further derived from it) will be closed.
func WithCancel(parent *T) (*T, CancelFunc) {
	ctx, cancel := context.WithCancel(parent.Context)
	child := newChild(ctx, parent)
	return child, CancelFunc(cancel)
}

// FromGoContext creates a Vanadium Context object from a generic Context. If
// the implementation of ctx is a *T it will be returned, if not, a new *T will
// be created with a default logger that discards its output.
func FromGoContext(ctx context.Context) *T {
	if vctx, ok := ctx.(*T); ok {
		return vctx
	}
	return newT(ctx)
}

// FromGoContextWithValues is like FromGoContext except that it will
// copy values from the specified 'peer' *T to a newly created
// *T. Note that if the supplied context is already a *T it will
// returned directly and no values will be copied to it.
func FromGoContextWithValues(ctx context.Context, peer *T) *T {
	if vctx, ok := ctx.(*T); ok {
		return vctx
	}
	nctx := newT(ctx)
	nctx.Context = copyValues(ctx, peer)
	return nctx
}

var nRootCancelWarning int32

func copyValues(root context.Context, ctx *T) context.Context {
	if ctx == nil {
		return root
	}
	nctx := copyValues(root, ctx.parent)
	if ctx.key == nil {
		return nctx
	}
	if val := ctx.Value(ctx.key); val != nil {
		return context.WithValue(nctx, ctx.key, val)
	}
	return nctx
}

// WithRootCancel returns a context derived from parent, but that is
// detached from the deadlines and cancellation hierarchy so that this
// context will only ever be canceled when the returned CancelFunc is
// called, or the RootContext from which this context is ultimately
// derived is canceled.
func WithRootCancel(parent *T) (*T, CancelFunc) {
	// Create a new context and copy over the keys.
	nctx := copyValues(context.Background(), parent)
	ctx, cancel := context.WithCancel(nctx)
	if val := parent.Value(rootKey); val != nil {
		rootCtx := val.(context.Context)
		// Forward the cancelation from the root context to the newly
		// created context.
		go func() {
			<-rootCtx.Done()
			cancel()
		}()
	} else if atomic.AddInt32(&nRootCancelWarning, 1) < 3 {
		vlog.Errorf("context.WithRootCancel: context %+v is not derived from root v23 context.\n", parent)
	}
	child := newChild(ctx, parent)
	return child, CancelFunc(cancel)
}

// WithDeadline returns a child of the current context along with a
// function that can be used to cancel it at any time (as from
// WithCancel).  When the deadline is reached the context will be
// automatically cancelled.
// Contexts should be cancelled when they are no longer needed
// so that resources associated with their timers may be released.
func WithDeadline(parent *T, deadline time.Time) (*T, CancelFunc) {
	ctx, cancel := context.WithDeadline(parent.Context, deadline)
	child := newChild(ctx, parent)
	return child, CancelFunc(cancel)
}

// WithTimeout is similar to WithDeadline except a Duration is given
// that represents a relative point in time from now.
func WithTimeout(parent *T, timeout time.Duration) (*T, CancelFunc) {
	ctx, cancel := context.WithTimeout(parent.Context, timeout)
	child := newChild(ctx, parent)
	return child, CancelFunc(cancel)
}
