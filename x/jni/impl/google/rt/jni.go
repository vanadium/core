// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package rt

import (
	"unsafe"

	"v.io/v23"
	"v.io/v23/context"

	jdiscovery "v.io/x/jni/impl/google/discovery"
	jns "v.io/x/jni/impl/google/namespace"
	jrpc "v.io/x/jni/impl/google/rpc"
	jutil "v.io/x/jni/util"
	jcontext "v.io/x/jni/v23/context"
	jopts "v.io/x/jni/v23/options"
	jsecurity "v.io/x/jni/v23/security"
)

// #include "jni.h"
import "C"

var (
	contextSign = jutil.ClassSign("io.v.v23.context.VContext")
	serverSign  = jutil.ClassSign("io.v.v23.rpc.Server")

	// Global reference io.v.impl.google.rt.VRuntimeImpl
	jVRuntimeImplClass jutil.Class
)

// Init initializes the JNI code with the given Java environment.  This method
// must be invoked before any other method in this package and must be called
// from the main Java thread (e.g., On_Load()).
func Init(env jutil.Env) error {
	var err error
	jVRuntimeImplClass, err = jutil.JFindClass(env, "io/v/impl/google/rt/VRuntimeImpl")
	if err != nil {
		return err
	}

	return nil
}

type shutdownKey struct{}

//export Java_io_v_impl_google_rt_VRuntimeImpl_nativeInit
func Java_io_v_impl_google_rt_VRuntimeImpl_nativeInit(jenv *C.JNIEnv, jRuntime C.jclass, jOptions C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	ctx, shutdownFunc := v23.Init()
	ctx = context.WithValue(ctx, shutdownKey{}, shutdownFunc)
	jCtx, err := jcontext.JavaContext(env, ctx, nil)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jCtx))
}

//export Java_io_v_impl_google_rt_VRuntimeImpl_nativeShutdown
func Java_io_v_impl_google_rt_VRuntimeImpl_nativeShutdown(jenv *C.JNIEnv, jRuntime C.jclass, jContext C.jobject, jCallbackObj C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	jCallback := jutil.Object(uintptr(unsafe.Pointer(jCallbackObj)))
	ctx, _, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		jutil.JThrowV(env, err)
	}
	value := ctx.Value(shutdownKey{})
	shutdownFunc, ok := value.(v23.Shutdown)
	if !ok {
		panic("shutdown function not found")
	}
	jutil.DoAsyncCall(env, jCallback, func() (jutil.Object, error) {
		shutdownFunc()
		return jutil.NullObject, nil
	})
}

//export Java_io_v_impl_google_rt_VRuntimeImpl_nativeWithNewClient
func Java_io_v_impl_google_rt_VRuntimeImpl_nativeWithNewClient(jenv *C.JNIEnv, jRuntime C.jclass, jContext C.jobject, jOptions C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	ctx, cancel, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	// No options supported yet.
	newCtx, _, err := v23.WithNewClient(ctx)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	jNewCtx, err := jcontext.JavaContext(env, newCtx, cancel)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jNewCtx))
}

//export Java_io_v_impl_google_rt_VRuntimeImpl_nativeGetClient
func Java_io_v_impl_google_rt_VRuntimeImpl_nativeGetClient(jenv *C.JNIEnv, jRuntime C.jclass, jContext C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	ctx, _, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	client := v23.GetClient(ctx)
	jClient, err := jrpc.JavaClient(env, client)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jClient))
}

//export Java_io_v_impl_google_rt_VRuntimeImpl_nativeWithNewServer
func Java_io_v_impl_google_rt_VRuntimeImpl_nativeWithNewServer(jenv *C.JNIEnv, jRuntime C.jclass, jContext C.jobject, jName C.jstring, jDispatcher C.jobject, jOptions C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	ctx, cancel, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	name := jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jName))))
	d, err := jrpc.GoDispatcher(env, jutil.Object(uintptr(unsafe.Pointer(jDispatcher))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	opts, err := jopts.GoRpcServerOpts(env, jutil.Object(uintptr(unsafe.Pointer(jOptions))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	newCtx, server, err := v23.WithNewDispatchingServer(ctx, name, d, opts...)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	jServer, err := jrpc.JavaServer(env, server)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	jNewCtx, err := jcontext.JavaContext(env, newCtx, cancel)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	// Explicitly attach a server to the new context.
	jServerAttCtx, err := jutil.CallStaticObjectMethod(env, jVRuntimeImplClass, "withServer", []jutil.Sign{contextSign, serverSign}, contextSign, jNewCtx, jServer)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jServerAttCtx))
}

//export Java_io_v_impl_google_rt_VRuntimeImpl_nativeWithPrincipal
func Java_io_v_impl_google_rt_VRuntimeImpl_nativeWithPrincipal(jenv *C.JNIEnv, jRuntime C.jclass, jContext C.jobject, jPrincipal C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	ctx, cancel, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	principal, err := jsecurity.GoPrincipal(env, jutil.Object(uintptr(unsafe.Pointer(jPrincipal))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	newCtx, err := v23.WithPrincipal(ctx, principal)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	jNewCtx, err := jcontext.JavaContext(env, newCtx, cancel)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jNewCtx))
}

//export Java_io_v_impl_google_rt_VRuntimeImpl_nativeGetPrincipal
func Java_io_v_impl_google_rt_VRuntimeImpl_nativeGetPrincipal(jenv *C.JNIEnv, jRuntime C.jclass, jContext C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	ctx, _, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	principal := v23.GetPrincipal(ctx)
	jPrincipal, err := jsecurity.JavaPrincipal(env, principal)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jPrincipal))
}

//export Java_io_v_impl_google_rt_VRuntimeImpl_nativeWithNewNamespace
func Java_io_v_impl_google_rt_VRuntimeImpl_nativeWithNewNamespace(jenv *C.JNIEnv, jRuntime C.jclass, jContext C.jobject, jRoots C.jobjectArray) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	ctx, cancel, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	roots, err := jutil.GoStringArray(env, jutil.Object(uintptr(unsafe.Pointer(jRoots))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}

	newCtx, _, err := v23.WithNewNamespace(ctx, roots...)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	jNewCtx, err := jcontext.JavaContext(env, newCtx, cancel)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jNewCtx))
}

//export Java_io_v_impl_google_rt_VRuntimeImpl_nativeGetNamespace
func Java_io_v_impl_google_rt_VRuntimeImpl_nativeGetNamespace(jenv *C.JNIEnv, jRuntime C.jclass, jContext C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	ctx, _, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	namespace := v23.GetNamespace(ctx)
	jNamespace, err := jns.JavaNamespace(env, namespace)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jNamespace))
}

//export Java_io_v_impl_google_rt_VRuntimeImpl_nativeWithListenSpec
func Java_io_v_impl_google_rt_VRuntimeImpl_nativeWithListenSpec(jenv *C.JNIEnv, jRuntime C.jclass, jContext C.jobject, jSpec C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	ctx, cancel, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	spec, err := jrpc.GoListenSpec(env, jutil.Object(uintptr(unsafe.Pointer(jSpec))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	newCtx := v23.WithListenSpec(ctx, spec)
	jNewCtx, err := jcontext.JavaContext(env, newCtx, cancel)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jNewCtx))
}

//export Java_io_v_impl_google_rt_VRuntimeImpl_nativeGetListenSpec
func Java_io_v_impl_google_rt_VRuntimeImpl_nativeGetListenSpec(jenv *C.JNIEnv, jRuntime C.jclass, jContext C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	ctx, _, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	spec := v23.GetListenSpec(ctx)
	jSpec, err := jrpc.JavaListenSpec(env, spec)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jSpec))
}

//export Java_io_v_impl_google_rt_VRuntimeImpl_nativeNewDiscovery
func Java_io_v_impl_google_rt_VRuntimeImpl_nativeNewDiscovery(jenv *C.JNIEnv, jRuntime C.jclass, jContext C.jobject) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	ctx, _, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}

	discovery, err := v23.NewDiscovery(ctx)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	jDiscovery, err := jdiscovery.JavaDiscovery(env, discovery)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jDiscovery))
}
