// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package rpc

import (
	"fmt"
	"runtime"

	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/vdl"
	"v.io/v23/vdlroot/signature"
	"v.io/v23/vom"

	jchannel "v.io/x/jni/impl/google/channel"
	jutil "v.io/x/jni/util"
	jcontext "v.io/x/jni/v23/context"
)

// #include "jni.h"
import "C"

func goInvoker(env jutil.Env, jInvoker jutil.Object) (rpc.Invoker, error) {
	// Reference Java invoker; it will be de-referenced when the go invoker
	// created below is garbage-collected (through the finalizer callback we
	// setup just below).
	jInvoker = jutil.NewGlobalRef(env, jInvoker)
	i := &invoker{
		jInvoker: jInvoker,
	}
	runtime.SetFinalizer(i, func(i *invoker) {
		env, freeFunc := jutil.GetEnv()
		defer freeFunc()
		jutil.DeleteGlobalRef(env, i.jInvoker)
	})
	return i, nil
}

type invoker struct {
	jInvoker jutil.Object
}

func (i *invoker) Prepare(ctx *context.T, method string, numArgs int) (argptrs []interface{}, tags []*vdl.Value, err error) {
	env, freeFunc := jutil.GetEnv()
	jContext, err := jcontext.JavaContext(env, ctx, nil)
	if err != nil {
		freeFunc()
		return nil, nil, err
	}
	// Have all input arguments be decoded into *vdl.Value.
	argptrs = make([]interface{}, numArgs)
	for i := 0; i < numArgs; i++ {
		value := new(vdl.Value)
		argptrs[i] = &value
	}
	// This method will invoke the freeFunc().
	jVomTags, err := jutil.CallStaticFutureMethod(env, freeFunc, jServerRPCHelperClass, "prepare", []jutil.Sign{invokerSign, contextSign, jutil.StringSign}, i.jInvoker, jContext, jutil.CamelCase(method))
	if err != nil {
		return nil, nil, err
	}
	env, freeFunc = jutil.GetEnv()
	defer freeFunc()
	defer jutil.DeleteGlobalRef(env, jVomTags)
	vomTags, err := jutil.GoByteArrayArray(env, jVomTags)
	if err != nil {
		return nil, nil, err
	}
	tags = make([]*vdl.Value, len(vomTags))
	for i, vomTag := range vomTags {
		var err error
		if tags[i], err = jutil.VomDecodeToValue(vomTag); err != nil {
			return nil, nil, err
		}
	}
	return
}

func (i *invoker) Invoke(ctx *context.T, call rpc.StreamServerCall, method string, argptrs []interface{}) (results []interface{}, err error) {
	env, freeFunc := jutil.GetEnv()
	jContext, err := jcontext.JavaContext(env, ctx, nil)
	if err != nil {
		freeFunc()
		return nil, err
	}
	jStreamServerCall, err := javaStreamServerCall(env, jContext, call)
	if err != nil {
		freeFunc()
		return nil, err
	}
	vomArgs := make([][]byte, len(argptrs))
	for i, argptr := range argptrs {
		arg := interface{}(jutil.DerefOrDie(argptr))
		var err error
		if vomArgs[i], err = vom.Encode(arg); err != nil {
			freeFunc()
			return nil, err
		}
	}
	// This method will invoke the freeFunc().
	jResult, err := jutil.CallStaticFutureMethod(env, freeFunc, jServerRPCHelperClass, "invoke", []jutil.Sign{invokerSign, contextSign, streamServerCallSign, jutil.StringSign, jutil.ArraySign(jutil.ArraySign(jutil.ByteSign))}, i.jInvoker, jContext, jStreamServerCall, jutil.CamelCase(method), vomArgs)
	if err != nil {
		return nil, err
	}
	env, freeFunc = jutil.GetEnv()
	defer freeFunc()
	defer jutil.DeleteGlobalRef(env, jResult)
	vomResults, err := jutil.GoByteArrayArray(env, jResult)
	if err != nil {
		return nil, err
	}
	results = make([]interface{}, len(vomResults))
	for i, vomResult := range vomResults {
		var err error
		if results[i], err = jutil.VomDecodeToValue(vomResult); err != nil {
			return nil, err
		}
	}
	return results, nil
}

func (i *invoker) Signature(ctx *context.T, call rpc.ServerCall) ([]signature.Interface, error) {
	env, freeFunc := jutil.GetEnv()
	jContext, err := jcontext.JavaContext(env, ctx, nil)
	if err != nil {
		freeFunc()
		return nil, err
	}
	// This method will invoke the freeFunc().
	jInterfaces, err := jutil.CallFutureMethod(env, freeFunc, i.jInvoker, "getSignature", []jutil.Sign{contextSign}, jContext)
	if err != nil {
		return nil, err
	}
	env, freeFunc = jutil.GetEnv()
	defer freeFunc()
	defer jutil.DeleteGlobalRef(env, jInterfaces)
	interfacesArr, err := jutil.GoObjectArray(env, jInterfaces)
	if err != nil {
		return nil, err
	}
	result := make([]signature.Interface, len(interfacesArr))
	for i, jInterface := range interfacesArr {
		err = jutil.GoVomCopy(env, jInterface, jInterfaceClass, &result[i])
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (i *invoker) MethodSignature(ctx *context.T, call rpc.ServerCall, method string) (signature.Method, error) {
	env, freeFunc := jutil.GetEnv()
	jContext, err := jcontext.JavaContext(env, ctx, nil)
	if err != nil {
		freeFunc()
		return signature.Method{}, err
	}
	// This method will invoke the freeFunc().
	jMethod, err := jutil.CallFutureMethod(env, freeFunc, i.jInvoker, "getMethodSignature", []jutil.Sign{contextSign, jutil.StringSign}, jContext, method)
	if err != nil {
		return signature.Method{}, err
	}
	env, freeFunc = jutil.GetEnv()
	defer freeFunc()
	defer jutil.DeleteGlobalRef(env, jMethod)
	var result signature.Method
	err = jutil.GoVomCopy(env, jMethod, jMethodClass, &result)
	if err != nil {
		return signature.Method{}, err
	}
	return result, nil
}

func (i *invoker) Globber() *rpc.GlobState {
	return &rpc.GlobState{AllGlobber: javaGlobber{i}}
}

type javaGlobber struct {
	i *invoker
}

func (j javaGlobber) Glob__(ctx *context.T, call rpc.GlobServerCall, g *glob.Glob) error {
	// TODO(sjr,rthellend): Update the Java API to match the new GO API.
	env, freeFunc := jutil.GetEnv()
	jContext, err := jcontext.JavaContext(env, ctx, nil)
	if err != nil {
		freeFunc()
		return err
	}
	jServerCall, err := JavaServerCall(env, call)
	if err != nil {
		freeFunc()
		return err
	}
	convert := func(input jutil.Object) (interface{}, error) {
		env, freeFunc := jutil.GetEnv()
		defer freeFunc()
		var reply naming.GlobReply
		if err := jutil.GoVomCopy(env, input, jGlobReplyClass, &reply); err != nil {
			return nil, err
		}
		return reply, nil
	}
	send := func(item interface{}) error {
		reply, ok := item.(naming.GlobReply)
		if !ok {
			return fmt.Errorf("Expected item of type naming.GlobReply, got: %T", reply)
		}
		return call.SendStream().Send(reply)
	}
	close := func() error {
		return nil
	}
	jOutputChannel, err := jchannel.JavaOutputChannel(env, ctx, nil, convert, send, close)
	if err != nil {
		freeFunc()
		return err
	}
	channelSign := jutil.ClassSign("io.v.v23.OutputChannel")
	// This method will invoke the freeFunc().
	_, err = jutil.CallStaticFutureMethod(env, freeFunc, jServerRPCHelperClass, "glob", []jutil.Sign{invokerSign, contextSign, serverCallSign, jutil.StringSign, channelSign}, j.i.jInvoker, jContext, jServerCall, g.String(), jOutputChannel)
	return err
}
