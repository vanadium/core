// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package context

import (
	"fmt"
	"runtime"

	"v.io/v23/context"
	jutil "v.io/x/jni/util"
)

// #include "jni.h"
import "C"

type goContextKey string

type goContextValue struct {
	jObj jutil.Object
}

// JavaContext converts the provided Go Context into a Java VContext.
// If the provided cancel function is nil, Java VContext won't be
// cancelable.
func JavaContext(env jutil.Env, ctx *context.T, cancel context.CancelFunc) (jutil.Object, error) {
	ctxRef := jutil.GoNewRef(ctx) // Un-refed when the Java context object is finalized.
	cancelRef := jutil.NullRef
	if cancel != nil {
		cancelRef = jutil.GoNewRef(&cancel) // Un-refed when the Java context object is finalized.
	}
	jCtx, err := jutil.NewObject(env, jVContextClass, []jutil.Sign{jutil.LongSign, jutil.LongSign}, int64(ctxRef), int64(cancelRef))
	if err != nil {
		jutil.GoDecRef(ctxRef)
		if cancel != nil {
			jutil.GoDecRef(cancelRef)
		}
		return jutil.NullObject, err
	}
	return jCtx, err
}

// GoContext converts the provided Java VContext into a Go context and
// a cancel function (if any) that can be used to cancel the context.
func GoContext(env jutil.Env, jContext jutil.Object) (*context.T, context.CancelFunc, error) {
	if jContext.IsNull() {
		return nil, nil, nil
	}
	goCtxRefVal, err := jutil.CallLongMethod(env, jContext, "nativeRef", nil)
	if err != nil {
		return nil, nil, err
	}
	goCtxRef := jutil.Ref(goCtxRefVal)
	goCancelRefVal, err := jutil.CallLongMethod(env, jContext, "nativeCancelRef", nil)
	if err != nil {
		return nil, nil, err
	}
	goCancelRef := jutil.Ref(goCancelRefVal)
	var cancel context.CancelFunc
	if goCancelRef != jutil.NullRef {
		cancel = *(*context.CancelFunc)(jutil.GoRefValue(goCancelRef))
	}
	return (*context.T)(jutil.GoRefValue(goCtxRef)), cancel, nil
}

// GoContextValue returns the Go Context value given the Java Context value.
func GoContextValue(env jutil.Env, jValue jutil.Object) (interface{}, error) {
	// Reference Java object; it will be de-referenced when the Go wrapper
	// object created below is garbage-collected (via the finalizer we setup
	// just below.)
	jValue = jutil.NewGlobalRef(env, jValue)
	value := &goContextValue{
		jObj: jValue,
	}
	runtime.SetFinalizer(value, func(value *goContextValue) {
		env, freeFunc := jutil.GetEnv()
		defer freeFunc()
		jutil.DeleteGlobalRef(env, value.jObj)
	})
	return value, nil
}

// JavaContextValue returns the Java Context value given the Go Context value.
func JavaContextValue(env jutil.Env, value interface{}) (jutil.Object, error) {
	if value == nil {
		return jutil.NullObject, nil
	}
	val, ok := value.(*goContextValue)
	if !ok {
		return jutil.NullObject, fmt.Errorf("Invalid type %T for value %v, wanted goContextValue", value, value)
	}
	return val.jObj, nil
}

// JavaContextDoneReason return the Java DoneReason given the Go error returned
// by ctx.Error().
func JavaContextDoneReason(env jutil.Env, err error) (jutil.Object, error) {
	var name string
	switch err {
	case context.Canceled:
		name = "CANCELED"
	case context.DeadlineExceeded:
		name = "DEADLINE_EXCEEDED"
	default:
		return jutil.NullObject, fmt.Errorf("Unrecognized context done reason: %v", err)
	}
	return jutil.CallStaticObjectMethod(env, jDoneReasonClass, "valueOf", []jutil.Sign{jutil.StringSign}, doneReasonSign, name)
}
