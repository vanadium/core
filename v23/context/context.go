// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// redo documentation...

// Package context implements a mechanism to carry data across API boundaries.
// The context.T struct carries deadlines and cancellation as well as other
// arbitrary values.
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
// Contexts form a tree where derived contexts are children of the
// contexts from which they were derived.  Children inherit all the
// properties of their parent except for the property being replaced
// (the deadline in the example above).
//
// Contexts are extensible.  The Value/WithValue functions allow you to attach
// new information to the context and extend its capabilities.
// In the same way we derive new contexts via the 'With' family of functions
// you can create methods to attach new data:
//
//    package auth
//
//    import "v.io/v23/context"
//
//    type Auth struct{...}
//
//    type key int
//    const authKey = key(0)
//
//    function WithAuth(parent *context.T, data *Auth) *context.T {
//        return context.WithValue(parent, authKey, data)
//    }
//
//    function GetAuth(ctx *context.T) *Auth {
//        data, _ := ctx.Value(authKey).(*Auth)
//        return data
//    }
//
// Note that a value of any type can be used as a key, but you should
// use an unexported value of an unexported type to ensure that no
// collisions can occur.
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
)

// ContextLogger is a logger that uses a passed in T to configure
// the logging behavior.
//nolint:golint // API change required.
type ContextLogger interface {
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
	VIDepth(ctx *T, depth int, level int) ContextLogger
}

type CancelFunc context.CancelFunc

// Cancelled is returned by contexts which have been cancelled.
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
	ctxLogger ContextLogger
	parent    *T
	key       interface{}
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
	nctx := &T{Context: ctx, logger: logging.Discard}
	nctx = WithValue(nctx, rootKey, nctx)
	nctx = WithValue(nctx, loggerKey, logging.Discard)
	return nctx, CancelFunc(cancel)
}

func newChild(parent *T, ctx context.Context) *T {
	child := *parent
	child.parent = parent
	child.Context = ctx
	return &child
}

// WithLogger returns a child of the current context that embeds the supplied
// logger.
func WithLogger(parent *T, logger logging.Logger) *T {
	if !parent.Initialized() {
		return nil
	}
	child := newChild(parent, context.WithValue(parent.Context, loggerKey, logger))
	child.logger = logger
	return child
}

// WithContextLogger returns a child of the current context that embeds the supplied
// context logger.
// TODO(cnicolaou): consider getting rid of ContextLogger altogether.
func WithContextLogger(parent *T, logger ContextLogger) *T {
	if !parent.Initialized() {
		return nil
	}
	child := newChild(parent, context.WithValue(parent.Context, ctxLoggerKey, logger))
	child.ctxLogger = logger
	return child
}

// LoggerFromContext returns the implementation of the logger
// associated with this context. It should almost never need to be used by application
// code.
func LoggerFromContext(ctx context.Context) interface{} {
	if v := ctx.Value(loggerKey); v != nil {
		return v
	}
	return nil
}

// Initialized returns true if this context has been properly initialized
// by a runtime.
func (t *T) Initialized() bool {
	return t != nil && t.Context != nil
}

// WithValue returns a child of the current context that will return
// the given val when Value(key) is called.
func WithValue(parent *T, key interface{}, val interface{}) *T {
	child := newChild(parent, context.WithValue(parent.Context, key, val))
	child.key = key
	return child
}

// WithCancel returns a child of the current context along with
// a function that can be used to cancel it.  After cancel() is
// called the channels returned by the Done() methods of the new context
// (and all context further derived from it) will be closed.
func WithCancel(parent *T) (*T, CancelFunc) {
	ctx, cancel := context.WithCancel(parent.Context)
	child := newChild(parent, ctx)
	return child, CancelFunc(cancel)
}

// FromGoContext creates a Vanadium Context object from a generic Context.
func FromGoContext(ctx context.Context) *T {
	if vctx, ok := ctx.(*T); ok {
		return vctx
	}
	nctx := &T{Context: ctx, logger: logging.Discard}
	if v := ctx.Value(loggerKey); v != nil {
		nctx = WithValue(nctx, loggerKey, v)
	}
	if v := ctx.Value(ctxLoggerKey); v != nil {
		nctx = WithValue(nctx, ctxLoggerKey, v)
	}
	return nctx
}

var nRootCancelWarning int32

// WithRootCancel returns a context derived from parent, but that is
// detached from the deadlines and cancellation hierarchy so that this
// context will only ever be canceled when the returned CancelFunc is
// called, or the RootContext from which this context is ultimately
// derived is canceled.
func WithRootCancel(parent *T) (*T, CancelFunc) {
	// Create a new context and copy over the keys.
	nctx := context.Background()
	for p := parent; p != nil; p = p.parent {
		if val := p.Context.Value(p.key); val != nil {
			nctx = context.WithValue(nctx, p.key, val)
		}
	}
	ctx, cancel := context.WithCancel(nctx)

	if val := parent.Value(rootKey); val != nil {
		rootCtx := val.(context.Context)
		// Forward the cancelation from the root context to the newly
		// created one.
		go func() {
			select {
			case <-rootCtx.Done():
				cancel()
			}
		}()
	} else if atomic.AddInt32(&nRootCancelWarning, 1) < 3 {
		vlog.Errorf("context.WithRootCancel: context %+v is not derived from root v23 context.\n", parent)
	}
	child := newChild(parent, ctx)
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
	child := newChild(parent, ctx)
	return child, CancelFunc(cancel)
}

// WithTimeout is similar to WithDeadline except a Duration is given
// that represents a relative point in time from now.
func WithTimeout(parent *T, timeout time.Duration) (*T, CancelFunc) {
	ctx, cancel := context.WithTimeout(parent.Context, timeout)
	child := newChild(parent, ctx)
	return child, CancelFunc(cancel)
}
