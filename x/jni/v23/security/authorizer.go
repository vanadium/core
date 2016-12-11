// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package security

import (
	"runtime"

	"v.io/v23/context"
	"v.io/v23/security"
	jutil "v.io/x/jni/util"
	jcontext "v.io/x/jni/v23/context"
)

// #include "jni.h"
import "C"

// GoAuthorizer converts the given Java authorizer into a Go authorizer.
func GoAuthorizer(env jutil.Env, jAuth jutil.Object) (security.Authorizer, error) {
	if jAuth.IsNull() {
		return nil, nil
	}
	if jutil.IsInstanceOf(env, jAuth, jPermissionsAuthorizerClass) {
		// Called with our implementation of Authorizer, which maintains a Go reference - use it.
		ref, err := jutil.JLongField(env, jAuth, "nativeRef")
		if err != nil {
			return nil, err
		}
		return *(*security.Authorizer)(jutil.GoRefValue(jutil.Ref(ref))), nil
	}
	// Reference Java dispatcher; it will be de-referenced when the go
	// dispatcher created below is garbage-collected (through the finalizer
	// callback we setup below).
	jAuth = jutil.NewGlobalRef(env, jAuth)
	a := &authorizer{
		jAuth: jAuth,
	}
	runtime.SetFinalizer(a, func(a *authorizer) {
		env, freeFunc := jutil.GetEnv()
		defer freeFunc()
		jutil.DeleteGlobalRef(env, a.jAuth)
	})
	return a, nil
}

type authorizer struct {
	jAuth jutil.Object
}

func (a *authorizer) Authorize(ctx *context.T, call security.Call) error {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()

	jCtx, err := jcontext.JavaContext(env, ctx, nil)
	if err != nil {
		return err
	}

	jCall, err := JavaCall(env, call)
	if err != nil {
		return err
	}

	// Run Java Authorizer.
	return jutil.CallVoidMethod(env, a.jAuth, "authorize", []jutil.Sign{contextSign, callSign}, jCtx, jCall)
}
