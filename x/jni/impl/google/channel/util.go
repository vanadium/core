// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package channel

import (
	"v.io/v23/context"

	jutil "v.io/x/jni/util"
	jcontext "v.io/x/jni/v23/context"
)

// #include "jni.h"
import "C"

// JavaInputChannel creates a new Java InputChannel object given the provided Go recv function.
//
// All objects returned by the recv function must be globally references.
//
// The recv function must return verror.ErrEndOfFile when there are no more elements
// to receive.
func JavaInputChannel(env jutil.Env, ctx *context.T, ctxCancel func(), recv func() (jutil.Object, error)) (jutil.Object, error) {
	jContext, err := jcontext.JavaContext(env, ctx, ctxCancel)
	if err != nil {
		return jutil.NullObject, err
	}
	ref := jutil.GoNewRef(&recv) // Un-refed when jInputChannel is finalized.
	jInputChannel, err := jutil.NewObject(env, jInputChannelImplClass, []jutil.Sign{contextSign, jutil.LongSign}, jContext, int64(ref))
	if err != nil {
		jutil.GoDecRef(ref)
		return jutil.NullObject, err
	}
	return jInputChannel, nil
}

// JavaOutputChannel creates a new Java OutputChannel object given the provided Go convert, send
// and close functions. Send is invoked with the result of convert, which must be non-blocking.
func JavaOutputChannel(env jutil.Env, ctx *context.T, ctxCancel func(), convert func(jutil.Object) (interface{}, error), send func(interface{}) error, close func() error) (jutil.Object, error) {
	jContext, err := jcontext.JavaContext(env, ctx, ctxCancel)
	if err != nil {
		return jutil.NullObject, err
	}
	convertRef := jutil.GoNewRef(&convert) // Un-refed when jOutputChannel is finalized.
	sendRef := jutil.GoNewRef(&send)       // Un-refed when jOutputChannel is finalized.
	closeRef := jutil.GoNewRef(&close)     // Un-refed when jOutputChannel is finalized.
	jOutputChannel, err := jutil.NewObject(env, jOutputChannelImplClass, []jutil.Sign{contextSign, jutil.LongSign, jutil.LongSign, jutil.LongSign}, jContext, int64(convertRef), int64(sendRef), int64(closeRef))
	if err != nil {
		jutil.GoDecRef(convertRef)
		jutil.GoDecRef(sendRef)
		jutil.GoDecRef(closeRef)
		return jutil.NullObject, err
	}
	return jOutputChannel, nil
}
