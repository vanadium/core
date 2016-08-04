// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package security

import (
	"log"
	"runtime"
	"time"

	jutil "v.io/x/jni/util"
	jcontext "v.io/x/jni/v23/context"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/vdl"
)

// #include "jni.h"
import "C"

// JavaCall converts the provided Go (security) Call into a Java Call object.
func JavaCall(env jutil.Env, call security.Call) (jutil.Object, error) {
	ref := jutil.GoNewRef(&call) // Un-refed when the Java CallImpl object is finalized.
	jCall, err := jutil.NewObject(env, jCallImplClass, []jutil.Sign{jutil.LongSign}, int64(ref))
	if err != nil {
		jutil.GoDecRef(ref)
		return jutil.NullObject, err
	}
	return jCall, nil
}

// GoCall creates instance of security.Call that uses the provided Java
// Call as its underlying implementation.
func GoCall(env jutil.Env, jCall jutil.Object) (security.Call, error) {
	if jCall.IsNull() {
		return nil, nil
	}
	if jutil.IsInstanceOf(env, jCall, jCallImplClass) {
		// Called with our implementation of Call, which maintains a Go reference - use it.
		ref, err := jutil.CallLongMethod(env, jCall, "nativeRef", nil)
		if err != nil {
			return nil, err
		}
		return (*(*security.Call)(jutil.GoRefValue(jutil.Ref(ref)))), nil
	}
	// Reference Java call; it will be de-referenced when the go call
	// created below is garbage-collected (through the finalizer callback we
	// setup just below).
	jCall = jutil.NewGlobalRef(env, jCall)
	call := &callImpl{
		jCall: jCall,
	}
	runtime.SetFinalizer(call, func(c *callImpl) {
		env, freeFunc := jutil.GetEnv()
		defer freeFunc()
		jutil.DeleteGlobalRef(env, c.jCall)
	})
	return call, nil
}

// callImpl is the go interface to the java implementation of security.Call
type callImpl struct {
	jCall jutil.Object
}

func (c *callImpl) Timestamp() time.Time {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	jTime, err := jutil.CallObjectMethod(env, c.jCall, "timestamp", nil, jutil.DateTimeSign)
	if err != nil {
		log.Println("Couldn't call Java timestamp method: ", err)
		return time.Time{}
	}
	t, err := jutil.GoTime(env, jTime)
	if err != nil {
		log.Println("Couldn't convert Java time to Go: ", err)
		return time.Time{}
	}
	return t
}

func (c *callImpl) Method() string {
	return jutil.UpperCamelCase(c.callStringMethod("method"))
}

func (c *callImpl) MethodTags() []*vdl.Value {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	jTags, err := jutil.CallObjectMethod(env, c.jCall, "methodTags", nil, jutil.ArraySign(jutil.VdlValueSign))
	if err != nil {
		log.Println("Couldn't call Java methodTags method: ", err)
		return nil
	}
	tags, err := jutil.GoVDLValueArray(env, jTags)
	if err != nil {
		log.Println("Couldn't convert Java tags to Go: ", err)
		return nil
	}
	return tags
}

func (c *callImpl) Suffix() string {
	return c.callStringMethod("suffix")
}

func (c *callImpl) LocalDischarges() map[string]security.Discharge {
	return c.callDischargeMapMethod("localDischarges")
}

func (c *callImpl) RemoteDischarges() map[string]security.Discharge {
	return c.callDischargeMapMethod("remoteDischarges")
}

func (c *callImpl) LocalEndpoint() naming.Endpoint {
	epStr := c.callStringMethod("localEndpoint")
	ep, err := naming.ParseEndpoint(epStr)
	if err != nil {
		log.Printf("Couldn't parse endpoint string %q: %v", epStr, err)
		return naming.Endpoint{}
	}
	return ep
}

func (c *callImpl) LocalPrincipal() security.Principal {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	jPrincipal, err := jutil.CallObjectMethod(env, c.jCall, "localPrincipal", nil, principalSign)
	if err != nil {
		log.Printf("Couldn't call Java localPrincipal method: %v", err)
		return nil
	}
	principal, err := GoPrincipal(env, jPrincipal)
	if err != nil {
		log.Printf("Couldn't convert Java principal to Go: %v", err)
		return nil
	}
	return principal
}

func (c *callImpl) LocalBlessings() security.Blessings {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	jBlessings, err := jutil.CallObjectMethod(env, c.jCall, "localBlessings", nil, blessingsSign)
	if err != nil {
		log.Printf("Couldn't call Java localBlessings method: %v", err)
		return security.Blessings{}
	}
	blessings, err := GoBlessings(env, jBlessings)
	if err != nil {
		log.Printf("Couldn't convert Java Blessings into Go: %v", err)
		return security.Blessings{}
	}
	return blessings
}

func (c *callImpl) RemoteBlessings() security.Blessings {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	jBlessings, err := jutil.CallObjectMethod(env, c.jCall, "remoteBlessings", nil, blessingsSign)
	if err != nil {
		log.Printf("Couldn't call Java remoteBlessings method: %v", err)
		return security.Blessings{}
	}
	blessings, err := GoBlessings(env, jBlessings)
	if err != nil {
		log.Printf("Couldn't convert Java Blessings into Go: %v", err)
		return security.Blessings{}
	}
	return blessings
}

func (c *callImpl) RemoteEndpoint() naming.Endpoint {
	epStr := c.callStringMethod("remoteEndpoint")
	ep, err := naming.ParseEndpoint(epStr)
	if err != nil {
		log.Printf("Couldn't parse endpoint string %q: %v", epStr, err)
		return naming.Endpoint{}
	}
	return ep
}

func (c *callImpl) Context() *context.T {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	contextSign := jutil.ClassSign("io.v.v23.context.VContext")
	jCtx, err := jutil.CallObjectMethod(env, c.jCall, "context", nil, contextSign)
	if err != nil {
		log.Printf("Couldn't get Java Vanadium context: %v", err)
	}
	ctx, _, err := jcontext.GoContext(env, jCtx)
	if err != nil {
		log.Printf("Couldn't convert Java Vanadium context to Go: %v", err)
	}
	return ctx
}

func (c *callImpl) callStringMethod(methodName string) string {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	ret, err := jutil.CallStringMethod(env, c.jCall, methodName, nil)
	if err != nil {
		log.Printf("Couldn't call Java %q method: %v", methodName, err)
		return ""
	}
	return ret
}

func (c *callImpl) callDischargeMapMethod(methodName string) map[string]security.Discharge {
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	javaObjectMap, err := jutil.CallMapMethod(env, c.jCall, methodName, nil)
	if err != nil {
		log.Printf("Couldn't call Java %q method: %v", methodName, err)
		return nil
	}
	discharges := make(map[string]security.Discharge)
	for jKey, jValue := range javaObjectMap {
		key := jutil.GoString(env, jKey)
		discharge, err := GoDischarge(env, jValue)
		if err != nil {
			log.Printf("Couldn't convert Java Discharge to Go Discharge: %v", err)
			return nil
		}
		discharges[key] = discharge
	}
	return discharges
}
