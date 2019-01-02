// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package rpc

import (
	"io"
	"unsafe"

	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/vdl"
	"v.io/v23/verror"
	"v.io/v23/vom"
	"v.io/x/lib/vlog"

	jble "v.io/x/jni/impl/google/rpc/protocols/ble"
	jbt "v.io/x/jni/impl/google/rpc/protocols/bt"
	jutil "v.io/x/jni/util"
	jcontext "v.io/x/jni/v23/context"
	jnaming "v.io/x/jni/v23/naming"
	jopts "v.io/x/jni/v23/options"
	jsecurity "v.io/x/jni/v23/security"
)

// #include "jni.h"
import "C"

var (
	contextSign          = jutil.ClassSign("io.v.v23.context.VContext")
	dispatcherSign       = jutil.ClassSign("io.v.v23.rpc.Dispatcher")
	invokerSign          = jutil.ClassSign("io.v.v23.rpc.Invoker")
	serverCallSign       = jutil.ClassSign("io.v.v23.rpc.ServerCall")
	streamServerCallSign = jutil.ClassSign("io.v.v23.rpc.StreamServerCall")
	listenAddrSign       = jutil.ClassSign("io.v.v23.rpc.ListenSpec$Address")
	addressChooserSign   = jutil.ClassSign("io.v.v23.rpc.AddressChooser")
	serverStateSign      = jutil.ClassSign("io.v.v23.rpc.ServerState")
	streamSign           = jutil.ClassSign("io.v.v23.rpc.Stream")

	// Global reference for io.v.impl.google.rpc.AddressChooserImpl class.
	jAddressChooserImplClass jutil.Class
	// Global reference for io.v.impl.google.rpc.ServerImpl class.
	jServerImplClass jutil.Class
	// Global reference for io.v.impl.google.rpc.ClientImpl class.
	jClientImplClass jutil.Class
	// Global reference for io.v.impl.google.rpc.ClientCallImpl class.
	jClientCallImplClass jutil.Class
	// Global reference for io.v.impl.google.rpc.StreamServerCallImpl class.
	jStreamServerCallImplClass jutil.Class
	// Global reference for io.v.impl.google.rpc.ServerCallImpl class.
	jServerCallImplClass jutil.Class
	// Global reference for io.v.impl.google.rpc.StreamImpl class.
	jStreamImplClass jutil.Class
	// Global reference for io.v.impl.google.rpc.ServerRPCHelper class.
	jServerRPCHelperClass jutil.Class
	// Global reference for io.v.v23.rpc.Invoker class.
	jInvokerClass jutil.Class
	// Global reference for io.v.v23.rpc.ListenSpec class.
	jListenSpecClass jutil.Class
	// Global reference for io.v.v23.rpc.ListenSpec$Address class.
	jListenSpecAddressClass jutil.Class
	// Global reference for io.v.v23.rpc.PublisherEntry class.
	jPublisherEntryClass jutil.Class
	// Global reference for io.v.v23.rpc.NetworkAddress class.
	jNetworkAddressClass jutil.Class
	// Global reference for io.v.v23.rpc.ProxyStatus class.
	jProxyStatusClass jutil.Class
	// Global reference for io.v.v23.rpc.ReflectInvoker class.
	jReflectInvokerClass jutil.Class
	// Global reference for io.v.v23.rpc.ServerStatus class.
	jServerStatusClass jutil.Class
	// Global reference for io.v.v23.rpc.ServerState class.
	jServerStateClass jutil.Class
	// Global reference for io.v.v23.OptionDefs class.
	jOptionDefsClass jutil.Class
	// Global reference for io.v.v23.naming.Endpoint.
	jEndpointClass jutil.Class
	// Global reference for io.v.v23.vdlroot.signature.Interface class.
	jInterfaceClass jutil.Class
	// Global reference for io.v.v23.vdlroot.signature.Method class.
	jMethodClass jutil.Class
	// Global reference for io.v.v23.naming.GlobReply
	jGlobReplyClass jutil.Class
	// Global reference for java.lang.Object class.
	jObjectClass jutil.Class
)

// Init initializes the JNI code with the given Java environment. This method
// must be called from the main Java thread.
func Init(env jutil.Env) error {
	if err := jbt.Init(env); err != nil {
		return err
	}
	if err := jble.Init(env); err != nil {
		// The BLE protocol isn't always compiled in, so don't fail if it
		// isn't available.
		vlog.Infof("Unable to initialize BLE protocol: %v", err)
	}
	// Cache global references to all Java classes used by the package.  This is
	// necessary because JNI gets access to the class loader only in the system
	// thread, so we aren't able to invoke FindClass in other threads.
	var err error
	jAddressChooserImplClass, err = jutil.JFindClass(env, "io/v/impl/google/rpc/AddressChooserImpl")
	if err != nil {
		return err
	}
	jServerImplClass, err = jutil.JFindClass(env, "io/v/impl/google/rpc/ServerImpl")
	if err != nil {
		return err
	}
	jClientImplClass, err = jutil.JFindClass(env, "io/v/impl/google/rpc/ClientImpl")
	if err != nil {
		return err
	}
	jClientCallImplClass, err = jutil.JFindClass(env, "io/v/impl/google/rpc/ClientCallImpl")
	if err != nil {
		return err
	}
	jStreamServerCallImplClass, err = jutil.JFindClass(env, "io/v/impl/google/rpc/StreamServerCallImpl")
	if err != nil {
		return err
	}
	jServerCallImplClass, err = jutil.JFindClass(env, "io/v/impl/google/rpc/ServerCallImpl")
	if err != nil {
		return err
	}
	jStreamImplClass, err = jutil.JFindClass(env, "io/v/impl/google/rpc/StreamImpl")
	if err != nil {
		return err
	}
	jServerRPCHelperClass, err = jutil.JFindClass(env, "io/v/impl/google/rpc/ServerRPCHelper")
	if err != nil {
		return err
	}
	jInvokerClass, err = jutil.JFindClass(env, "io/v/v23/rpc/Invoker")
	if err != nil {
		return err
	}
	jListenSpecClass, err = jutil.JFindClass(env, "io/v/v23/rpc/ListenSpec")
	if err != nil {
		return err
	}
	jListenSpecAddressClass, err = jutil.JFindClass(env, "io/v/v23/rpc/ListenSpec$Address")
	if err != nil {
		return err
	}
	jPublisherEntryClass, err = jutil.JFindClass(env, "io/v/v23/rpc/PublisherEntry")
	if err != nil {
		return err
	}
	jNetworkAddressClass, err = jutil.JFindClass(env, "io/v/v23/rpc/NetworkAddress")
	if err != nil {
		return err
	}
	jProxyStatusClass, err = jutil.JFindClass(env, "io/v/v23/rpc/ProxyStatus")
	if err != nil {
		return err
	}
	jReflectInvokerClass, err = jutil.JFindClass(env, "io/v/v23/rpc/ReflectInvoker")
	if err != nil {
		return err
	}
	jServerStatusClass, err = jutil.JFindClass(env, "io/v/v23/rpc/ServerStatus")
	if err != nil {
		return err
	}
	jServerStateClass, err = jutil.JFindClass(env, "io/v/v23/rpc/ServerState")
	if err != nil {
		return err
	}
	jOptionDefsClass, err = jutil.JFindClass(env, "io/v/v23/OptionDefs")
	if err != nil {
		return err
	}
	jEndpointClass, err = jutil.JFindClass(env, "io/v/v23/naming/Endpoint")
	if err != nil {
		return err
	}
	jInterfaceClass, err = jutil.JFindClass(env, "io/v/v23/vdlroot/signature/Interface")
	if err != nil {
		return err
	}
	jMethodClass, err = jutil.JFindClass(env, "io/v/v23/vdlroot/signature/Method")
	if err != nil {
		return err
	}
	jGlobReplyClass, err = jutil.JFindClass(env, "io/v/v23/naming/GlobReply")
	if err != nil {
		return err
	}
	jObjectClass, err = jutil.JFindClass(env, "java/lang/Object")
	if err != nil {
		return err
	}
	return nil
}

//export Java_io_v_impl_google_rpc_ServerImpl_nativeAddName
func Java_io_v_impl_google_rpc_ServerImpl_nativeAddName(jenv *C.JNIEnv, jServer C.jobject, goRef C.jlong, jName C.jstring) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	name := jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jName))))
	if err := (*(*rpc.Server)(jutil.GoRefValue(jutil.Ref(goRef)))).AddName(name); err != nil {
		jutil.JThrowV(env, err)
		return
	}
}

//export Java_io_v_impl_google_rpc_ServerImpl_nativeRemoveName
func Java_io_v_impl_google_rpc_ServerImpl_nativeRemoveName(jenv *C.JNIEnv, jServer C.jobject, goRef C.jlong, jName C.jstring) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	name := jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jName))))
	(*(*rpc.Server)(jutil.GoRefValue(jutil.Ref(goRef)))).RemoveName(name)
}

//export Java_io_v_impl_google_rpc_ServerImpl_nativeGetStatus
func Java_io_v_impl_google_rpc_ServerImpl_nativeGetStatus(jenv *C.JNIEnv, jServer C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	status := (*(*rpc.Server)(jutil.GoRefValue(jutil.Ref(goRef)))).Status()
	jStatus, err := JavaServerStatus(env, status)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jStatus))
}

//export Java_io_v_impl_google_rpc_ServerImpl_nativeAllPublished
func Java_io_v_impl_google_rpc_ServerImpl_nativeAllPublished(jenv *C.JNIEnv, jServer C.jobject, goRef C.jlong, jContext C.jobject, jCallbackObj C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	server := *(*rpc.Server)(jutil.GoRefValue(jutil.Ref(goRef)))
	jCallback := jutil.Object(uintptr(unsafe.Pointer(jCallbackObj)))
	jutil.DoAsyncCall(env, jCallback, func() (jutil.Object, error) {
		for {
			status := server.Status()
			done := true
			for _, pub := range status.PublisherStatus {
				if pub.LastState != pub.DesiredState {
					done = false
					break
				}
			}
			if done {
				break
			}
			<-status.Dirty
		}
		return jutil.NullObject, nil
	})
}

//export Java_io_v_impl_google_rpc_ServerImpl_nativeFinalize
func Java_io_v_impl_google_rpc_ServerImpl_nativeFinalize(jenv *C.JNIEnv, jServer C.jobject, goRef C.jlong) {
	jutil.GoDecRef(jutil.Ref(goRef))
}

func decodeArgs(env jutil.Env, jVomArgs C.jobjectArray) ([]interface{}, error) {
	vomArgs, err := jutil.GoByteArrayArray(env, jutil.Object(uintptr(unsafe.Pointer(jVomArgs))))
	if err != nil {
		return nil, err
	}
	// VOM-decode each arguments into a *vdl.Value.
	args := make([]interface{}, len(vomArgs))
	for i := 0; i < len(vomArgs); i++ {
		var err error
		if args[i], err = jutil.VomDecodeToValue(vomArgs[i]); err != nil {
			return nil, err
		}
	}
	return args, nil
}

func doStartCall(context *context.T, cancel func(), name, method string, opts []rpc.CallOpt, goRef C.jlong, args []interface{}) (jutil.Object, error) {
	// Invoke StartCall
	call, err := (*(*rpc.Client)(jutil.GoRefValue(jutil.Ref(goRef)))).StartCall(context, name, method, args, opts...)
	if err != nil {
		return jutil.NullObject, err
	}
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	jContext, err := jcontext.JavaContext(env, context, cancel)
	if err != nil {
		return jutil.NullObject, err
	}
	jCall, err := javaCall(env, jContext, call)
	if err != nil {
		return jutil.NullObject, err
	}
	// Must grab a global reference as we free up the env and all local references that come along
	// with it.
	return jutil.NewGlobalRef(env, jCall), nil // Un-refed in DoAsyncCall
}

//export Java_io_v_impl_google_rpc_ClientImpl_nativeStartCall
func Java_io_v_impl_google_rpc_ClientImpl_nativeStartCall(jenv *C.JNIEnv, jClientObj C.jobject, goRef C.jlong,
	jContext C.jobject, jName C.jstring, jMethod C.jstring, jVomArgs C.jobjectArray, jOptionsObj C.jobject, jCallbackObj C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	name := jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jName))))
	method := jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jMethod))))
	jOptions := jutil.Object(uintptr(unsafe.Pointer(jOptionsObj)))
	jCallback := jutil.Object(uintptr(unsafe.Pointer(jCallbackObj)))
	ctx, cancel, err := jcontext.GoContext(env, jutil.Object(uintptr(unsafe.Pointer(jContext))))
	if err != nil {
		jutil.CallbackOnFailure(env, jCallback, err)
		return
	}
	args, err := decodeArgs(env, jVomArgs)
	if err != nil {
		jutil.CallbackOnFailure(env, jCallback, err)
		return
	}

	opts, err := jopts.GoRpcOpts(env, jOptions)
	if err != nil {
		jutil.CallbackOnFailure(env, jCallback, err)
		return
	}

	// Create a global reference to the client object for the duration of the call
	// so that the java object doesn't get garbage collected while the async call
	// is happening. This is an issue because if the java object is garbage collected,
	// the go ref will also be collected causing doStartCall to panic.
	jClient := jutil.NewGlobalRef(env, jutil.Object(uintptr(unsafe.Pointer(jClientObj))))
	jutil.DoAsyncCall(env, jCallback, func() (jutil.Object, error) {
		obj, err := doStartCall(ctx, cancel, name, method, opts, goRef, args)
		env, freeFunc := jutil.GetEnv()
		jutil.DeleteGlobalRef(env, jClient)
		freeFunc()
		return obj, err
	})
}

//export Java_io_v_impl_google_rpc_ClientImpl_nativeClose
func Java_io_v_impl_google_rpc_ClientImpl_nativeClose(jenv *C.JNIEnv, jClient C.jobject, goRef C.jlong) {
	(*(*rpc.Client)(jutil.GoRefValue(jutil.Ref(goRef)))).Close()
}

//export Java_io_v_impl_google_rpc_ClientImpl_nativeFinalize
func Java_io_v_impl_google_rpc_ClientImpl_nativeFinalize(jenv *C.JNIEnv, jClient C.jobject, goRef C.jlong) {
	jutil.GoDecRef(jutil.Ref(goRef))
}

//export Java_io_v_impl_google_rpc_StreamImpl_nativeSend
func Java_io_v_impl_google_rpc_StreamImpl_nativeSend(jenv *C.JNIEnv, jStream C.jobject, goRef C.jlong, jVomItem C.jbyteArray, jCallbackObj C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	jCallback := jutil.Object(uintptr(unsafe.Pointer(jCallbackObj)))
	vomItem := jutil.GoByteArray(env, jutil.Object(uintptr(unsafe.Pointer(jVomItem))))
	jutil.DoAsyncCall(env, jCallback, func() (jutil.Object, error) {
		item, err := jutil.VomDecodeToValue(vomItem)
		if err != nil {
			return jutil.NullObject, err
		}
		return jutil.NullObject, (*(*rpc.Stream)(jutil.GoRefValue(jutil.Ref(goRef)))).Send(item)
	})
}

//export Java_io_v_impl_google_rpc_StreamImpl_nativeRecv
func Java_io_v_impl_google_rpc_StreamImpl_nativeRecv(jenv *C.JNIEnv, jStream C.jobject, goRef C.jlong, jCallbackObj C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	jCallback := jutil.Object(uintptr(unsafe.Pointer(jCallbackObj)))
	result := new(vdl.Value)
	jutil.DoAsyncCall(env, jCallback, func() (jutil.Object, error) {
		if err := (*(*rpc.Stream)(jutil.GoRefValue(jutil.Ref(goRef)))).Recv(&result); err != nil {
			if err == io.EOF {
				// Java uses EndOfFile error to detect EOF.
				err = verror.NewErrEndOfFile(nil)
			}
			return jutil.NullObject, err
		}
		vomResult, err := vom.Encode(result)
		if err != nil {
			return jutil.NullObject, err
		}
		env, freeFunc := jutil.GetEnv()
		defer freeFunc()
		jResult, err := jutil.JByteArray(env, vomResult)
		if err != nil {
			return jutil.NullObject, err
		}
		// Must grab a global reference as we free up the env and all local references that come along
		// with it.
		return jutil.NewGlobalRef(env, jResult), nil // Un-refed in DoAsyncCall
	})
}

//export Java_io_v_impl_google_rpc_StreamImpl_nativeFinalize
func Java_io_v_impl_google_rpc_StreamImpl_nativeFinalize(jenv *C.JNIEnv, jStream C.jobject, goRef C.jlong) {
	jutil.GoDecRef(jutil.Ref(goRef))
}

//export Java_io_v_impl_google_rpc_ClientCallImpl_nativeCloseSend
func Java_io_v_impl_google_rpc_ClientCallImpl_nativeCloseSend(jenv *C.JNIEnv, jCall C.jobject, goRef C.jlong, jCallbackObj C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	jCallback := jutil.Object(uintptr(unsafe.Pointer(jCallbackObj)))
	jutil.DoAsyncCall(env, jCallback, func() (jutil.Object, error) {
		return jutil.NullObject, (*(*rpc.ClientCall)(jutil.GoRefValue(jutil.Ref(goRef)))).CloseSend()
	})
}

func doFinish(goRef C.jlong, numResults int) (jutil.Object, error) {
	// Have all the results be decoded into *vdl.Value.
	resultPtrs := make([]interface{}, numResults)
	for i := 0; i < numResults; i++ {
		value := new(vdl.Value)
		resultPtrs[i] = &value
	}
	if err := (*(*rpc.ClientCall)(jutil.GoRefValue(jutil.Ref(goRef)))).Finish(resultPtrs...); err != nil {
		// Invocation error.
		return jutil.NullObject, err
	}
	// VOM-encode the results.
	vomResults := make([][]byte, numResults)
	for i, resultPtr := range resultPtrs {
		// Remove the pointer from the result.  Simply *resultPtr doesn't work
		// as resultPtr is of type interface{}.
		result := interface{}(jutil.DerefOrDie(resultPtr))
		var err error
		if vomResults[i], err = vom.Encode(result); err != nil {
			return jutil.NullObject, err
		}
	}
	env, freeFunc := jutil.GetEnv()
	defer freeFunc()
	jArr, err := jutil.JByteArrayArray(env, vomResults)
	if err != nil {
		return jutil.NullObject, err
	}
	// Must grab a global reference as we free up the env and all local references that come along
	// with it.
	return jutil.NewGlobalRef(env, jArr), nil // Un-refed in DoAsyncCall
}

//export Java_io_v_impl_google_rpc_ClientCallImpl_nativeFinish
func Java_io_v_impl_google_rpc_ClientCallImpl_nativeFinish(jenv *C.JNIEnv, jCall C.jobject, goRef C.jlong, jNumResults C.jint, jCallbackObj C.jobject) {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	numResults := int(jNumResults)
	jCallback := jutil.Object(uintptr(unsafe.Pointer(jCallbackObj)))
	jutil.DoAsyncCall(env, jCallback, func() (jutil.Object, error) {
		return doFinish(goRef, numResults)
	})
}

//export Java_io_v_impl_google_rpc_ClientCallImpl_nativeFinalize
func Java_io_v_impl_google_rpc_ClientCallImpl_nativeFinalize(jenv *C.JNIEnv, jCall C.jobject, goRef C.jlong) {
	jutil.GoDecRef(jutil.Ref(goRef))
}

//export Java_io_v_impl_google_rpc_ServerCallImpl_nativeSecurity
func Java_io_v_impl_google_rpc_ServerCallImpl_nativeSecurity(jenv *C.JNIEnv, jServerCallClass C.jclass, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	securityCall := (*(*rpc.ServerCall)(jutil.GoRefValue(jutil.Ref(goRef)))).Security()
	if securityCall == nil {
		return 0
	}
	jSecurityCall, err := jsecurity.JavaCall(env, securityCall)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jSecurityCall))
}

//export Java_io_v_impl_google_rpc_ServerCallImpl_nativeSuffix
func Java_io_v_impl_google_rpc_ServerCallImpl_nativeSuffix(jenv *C.JNIEnv, jServerCall C.jobject, goRef C.jlong) C.jstring {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	jSuffix := jutil.JString(env, (*(*rpc.ServerCall)(jutil.GoRefValue(jutil.Ref(goRef)))).Suffix())
	return C.jstring(unsafe.Pointer(jSuffix))
}

//export Java_io_v_impl_google_rpc_ServerCallImpl_nativeLocalEndpoint
func Java_io_v_impl_google_rpc_ServerCallImpl_nativeLocalEndpoint(jenv *C.JNIEnv, jServerCall C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	jEndpoint, err := jnaming.JavaEndpoint(env, (*(*rpc.ServerCall)(jutil.GoRefValue(jutil.Ref(goRef)))).LocalEndpoint())
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jEndpoint))
}

//export Java_io_v_impl_google_rpc_ServerCallImpl_nativeRemoteEndpoint
func Java_io_v_impl_google_rpc_ServerCallImpl_nativeRemoteEndpoint(jenv *C.JNIEnv, jServerCall C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	jEndpoint, err := jnaming.JavaEndpoint(env, (*(*rpc.ServerCall)(jutil.GoRefValue(jutil.Ref(goRef)))).RemoteEndpoint())
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jEndpoint))
}

//export Java_io_v_impl_google_rpc_ServerCallImpl_nativeGrantedBlessings
func Java_io_v_impl_google_rpc_ServerCallImpl_nativeGrantedBlessings(jenv *C.JNIEnv, jServerCall C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	blessings := (*(*rpc.ServerCall)(jutil.GoRefValue(jutil.Ref(goRef)))).GrantedBlessings()
	jBlessings, err := jsecurity.JavaBlessings(env, blessings)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jBlessings))
}

//export Java_io_v_impl_google_rpc_ServerCallImpl_nativeServer
func Java_io_v_impl_google_rpc_ServerCallImpl_nativeServer(jenv *C.JNIEnv, jServerCall C.jobject, goRef C.jlong) C.jobject {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	server := (*(*rpc.ServerCall)(jutil.GoRefValue(jutil.Ref(goRef)))).Server()
	jServer, err := JavaServer(env, server)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobject(unsafe.Pointer(jServer))
}

//export Java_io_v_impl_google_rpc_ServerCallImpl_nativeFinalize
func Java_io_v_impl_google_rpc_ServerCallImpl_nativeFinalize(jenv *C.JNIEnv, jServerCall C.jobject, goRef C.jlong) {
	jutil.GoDecRef(jutil.Ref(goRef))
}

//export Java_io_v_impl_google_rpc_StreamServerCallImpl_nativeFinalize
func Java_io_v_impl_google_rpc_StreamServerCallImpl_nativeFinalize(jenv *C.JNIEnv, jStreamServerCall C.jobject, goRef C.jlong) {
	jutil.GoDecRef(jutil.Ref(goRef))
}

//export Java_io_v_impl_google_rpc_AddressChooserImpl_nativeChoose
func Java_io_v_impl_google_rpc_AddressChooserImpl_nativeChoose(jenv *C.JNIEnv, jAddressChooser C.jobject, goRef C.jlong, jProtocol C.jstring, jCandidates C.jobjectArray) C.jobjectArray {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	protocol := jutil.GoString(env, jutil.Object(uintptr(unsafe.Pointer(jProtocol))))
	candidates, err := GoNetworkAddressArray(env, jutil.Object(uintptr(unsafe.Pointer(jCandidates))))
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	addrs, err := (*(*rpc.AddressChooser)(jutil.GoRefValue(jutil.Ref(goRef)))).ChooseAddresses(protocol, candidates)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	jAddrs, err := JavaNetworkAddressArray(env, addrs)
	if err != nil {
		jutil.JThrowV(env, err)
		return 0
	}
	return C.jobjectArray(unsafe.Pointer(jAddrs))
}

//export Java_io_v_impl_google_rpc_AddressChooserImpl_nativeFinalize
func Java_io_v_impl_google_rpc_AddressChooserImpl_nativeFinalize(jenv *C.JNIEnv, jAddressChooser C.jobject, goRef C.jlong) {
	jutil.GoDecRef(jutil.Ref(goRef))
}

//export Java_io_v_impl_google_rpc_ServerRPCHelper_nativeGoInvoker
func Java_io_v_impl_google_rpc_ServerRPCHelper_nativeGoInvoker(jenv *C.JNIEnv, jServerRPCHelper C.jclass, jInvoker C.jobject) C.jlong {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	invoker, err := goInvoker(env, jutil.Object(uintptr(unsafe.Pointer(jInvoker))))
	if err != nil {
		jutil.JThrowV(env, err)
		return C.jlong(0)
	}
	ref := jutil.GoNewRef(&invoker) // Un-refed when the Go invoker is returned to the Go runtime
	return C.jlong(ref)
}

//export Java_io_v_impl_google_rpc_ServerRPCHelper_nativeGoAuthorizer
func Java_io_v_impl_google_rpc_ServerRPCHelper_nativeGoAuthorizer(jenv *C.JNIEnv, jServerRPCHelper C.jclass, jAuthorizer C.jobject) C.jlong {
	env := jutil.Env(uintptr(unsafe.Pointer(jenv)))
	auth, err := jsecurity.GoAuthorizer(env, jutil.Object(uintptr(unsafe.Pointer(jAuthorizer))))
	if err != nil {
		jutil.JThrowV(env, err)
		return C.jlong(0)
	}
	ref := jutil.GoNewRef(&auth) // Un-refed when the Go authorizer is returned to the Go runtime
	return C.jlong(ref)
}
