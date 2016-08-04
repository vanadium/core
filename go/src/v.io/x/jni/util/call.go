// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package util

import (
	"fmt"
	"unsafe"
)

// #include <stdlib.h>
// #include "jni_wrapper.h"
//
// static jvalue* allocJValueArray(int elements) {
//   return malloc(sizeof(jvalue) * elements);
// }
//
// static void setJValueArrayElement(jvalue* arr, int index, jvalue val) {
//   arr[index] = val;
// }
//
import "C"

// jArgArray converts a slice of Go args to an array of Java args.  It uses the provided slice of
// Signs to validate that the arguments are of compatible types.
func jArgArray(env Env, args []interface{}, argSigns []Sign) (jArr *C.jvalue, freeFunc func(), err error) {
	if len(argSigns) != len(args) {
		return nil, nil, fmt.Errorf("mismatch in number of arguments, want %d, got %d", len(argSigns), len(args))
	}
	jArr = C.allocJValueArray(C.int(len(args)))
	freeFunc = func() {
		C.free(unsafe.Pointer(jArr))
	}
	for i, arg := range args {
		sign := argSigns[i]
		jVal, err := jValue(env, arg, sign)
		if err != nil {
			freeFunc()
			return nil, nil, fmt.Errorf("couldn't get Java value for argument #%d [%#v] of expected type %v: %v", i, arg, sign, err)
		}
		C.setJValueArrayElement(jArr, C.int(i), jVal)
	}
	return
}

// NewObject invokes a java constructor with the given arguments, returning the
// newly created object.
func NewObject(env Env, class Class, argSigns []Sign, args ...interface{}) (Object, error) {
	jcid, err := jMethodID(env, class, "<init>", FuncSign(argSigns, VoidSign))
	if err != nil {
		return NullObject, err
	}
	jArr, freeFunc, err := jArgArray(env, args, argSigns)
	if err != nil {
		return NullObject, err
	}
	defer freeFunc()
	obj := C.NewObjectA(env.value(), class.value(), jcid, jArr)
	err = JExceptionMsg(env)
	return Object(uintptr(unsafe.Pointer(obj))), err
}

// jMethodID returns the Java method ID for the given instance (non-static)
// method, or an error if the method couldn't be found.
func jMethodID(env Env, class Class, name string, signature Sign) (C.jmethodID, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	cSignature := C.CString(string(signature))
	defer C.free(unsafe.Pointer(cSignature))
	mid := C.GetMethodID(env.value(), class.value(), cName, cSignature)
	if err := JExceptionMsg(env); err != nil || mid == C.jmethodID(nil) {
		return nil, fmt.Errorf("couldn't find method %q with signature %v.", name, signature)
	}
	return mid, nil
}

// jStaticMethodID returns the Java method ID for the given static method, or an
// error if the method couldn't be found.
func jStaticMethodID(env Env, class Class, name string, signature Sign) (C.jmethodID, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	cSignature := C.CString(string(signature))
	defer C.free(unsafe.Pointer(cSignature))
	mid := C.GetStaticMethodID(env.value(), class.value(), cName, cSignature)
	if err := JExceptionMsg(env); err != nil || mid == C.jmethodID(nil) {
		return nil, fmt.Errorf("couldn't find method %s with a given signature: %s", name, signature)
	}
	return mid, nil
}

// setupMethodCall performs the shared preparation operations between various
// Java method invocation functions.
func setupMethodCall(env Env, obj Object, name string, argSigns []Sign, retSign Sign, args ...interface{}) (mid C.jmethodID, jArgArr *C.jvalue, freeFunc func(), err error) {
	class := GetClass(env, obj)
	mid, err = jMethodID(env, class, name, FuncSign(argSigns, retSign))
	if err != nil {
		return
	}
	jArgArr, freeFunc, err = jArgArray(env, args, argSigns)
	if err != nil {
		err = fmt.Errorf("error creating arguments for method %s: %v", name, err)
	}
	return

}

// setupStaticMethodCall performs the shared preparation operations between
// various Java static method invocation functions.
func setupStaticMethodCall(env Env, class Class, name string, argSigns []Sign, retSign Sign, args ...interface{}) (mid C.jmethodID, jArgArr *C.jvalue, freeFunc func(), err error) {
	mid, err = jStaticMethodID(env, class, name, FuncSign(argSigns, retSign))
	if err != nil {
		return
	}
	jArgArr, freeFunc, err = jArgArray(env, args, argSigns)
	if err != nil {
		err = fmt.Errorf("error creating arguments for method %s: %v", name, err)
	}
	return
}

// CallObjectMethod calls a Java method that returns a java object.
func CallObjectMethod(env Env, obj Object, name string, argSigns []Sign, retSign Sign, args ...interface{}) (Object, error) {
	switch retSign {
	case ByteSign, CharSign, ShortSign, LongSign, FloatSign, DoubleSign, BoolSign, IntSign, VoidSign:
		panic(fmt.Sprintf("Illegal call to CallObjectMethod on method with return sign %s", retSign))
	}
	jmid, jArgArr, freeFunc, err := setupMethodCall(env, obj, name, argSigns, retSign, args...)
	if err != nil {
		return NullObject, err
	}
	defer freeFunc()
	ret := C.CallObjectMethodA(env.value(), obj.value(), jmid, jArgArr)
	return Object(uintptr(unsafe.Pointer(ret))), JExceptionMsg(env)
}

// CallStringMethod calls a Java method that returns a string.
func CallStringMethod(env Env, obj Object, name string, argSigns []Sign, args ...interface{}) (string, error) {
	strObj, err := CallObjectMethod(env, obj, name, argSigns, StringSign, args...)
	return GoString(env, strObj), err
}

// CallByteArrayMethod calls a Java method that returns a byte array.
func CallByteArrayMethod(env Env, obj Object, name string, argSigns []Sign, args ...interface{}) ([]byte, error) {
	arrObj, err := CallObjectMethod(env, obj, name, argSigns, ArraySign(ByteSign), args...)
	if err != nil {
		return nil, err
	}
	return GoByteArray(env, arrObj), nil
}

// CallObjectArrayMethod calls a Java method that returns an object array.
func CallObjectArrayMethod(env Env, obj Object, name string, argSigns []Sign, retElemSign Sign, args ...interface{}) ([]Object, error) {
	arrObj, err := CallObjectMethod(env, obj, name, argSigns, ArraySign(retElemSign), args...)
	if err != nil {
		return nil, err
	}
	return GoObjectArray(env, arrObj)
}

// CallStringArrayMethod calls a Java method that returns an string array.
func CallStringArrayMethod(env Env, obj Object, name string, argSigns []Sign, args ...interface{}) ([]string, error) {
	arrObj, err := CallObjectMethod(env, obj, name, argSigns, ArraySign(StringSign), args...)
	if err != nil {
		return nil, err
	}
	return GoStringArray(env, arrObj)
}

// CallMapMethod calls a Java method that returns a map.
func CallMapMethod(env Env, obj Object, name string, argSigns []Sign, args ...interface{}) (map[Object]Object, error) {
	mapObj, err := CallObjectMethod(env, obj, name, argSigns, MapSign, args...)
	if err != nil {
		return nil, err
	}
	return GoObjectMap(env, mapObj)
}

// CallMultimapMethod calls a Java method that returns a multimap.
func CallMultimapMethod(env Env, obj Object, name string, argSigns []Sign, args ...interface{}) (map[Object][]Object, error) {
	multiMapObj, err := CallObjectMethod(env, obj, name, argSigns, MultimapSign, args...)
	if err != nil {
		return nil, err
	}
	return GoObjectMultimap(env, multiMapObj)
}

// CallBooleanMethod calls a Java method that returns a boolean.
func CallBooleanMethod(env Env, obj Object, name string, argSigns []Sign, args ...interface{}) (bool, error) {
	jmid, jArgArr, freeFunc, err := setupMethodCall(env, obj, name, argSigns, BoolSign, args...)
	if err != nil {
		return false, err
	}
	defer freeFunc()
	ret := C.CallBooleanMethodA(env.value(), obj.value(), jmid, jArgArr) != C.JNI_OK
	return ret, JExceptionMsg(env)
}

// CallIntMethod calls a Java method that returns an int.
func CallIntMethod(env Env, obj Object, name string, argSigns []Sign, args ...interface{}) (int, error) {
	jmid, jArgArr, freeFunc, err := setupMethodCall(env, obj, name, argSigns, IntSign, args...)
	if err != nil {
		return 0, err
	}
	defer freeFunc()
	ret := int(C.CallIntMethodA(env.value(), obj.value(), jmid, jArgArr))
	return ret, JExceptionMsg(env)
}

// CallLongMethod calls a Java method that returns an int64.
func CallLongMethod(env Env, obj Object, name string, argSigns []Sign, args ...interface{}) (int64, error) {
	jmid, jArgArr, freeFunc, err := setupMethodCall(env, obj, name, argSigns, LongSign, args...)
	if err != nil {
		return 0, err
	}
	defer freeFunc()
	ret := int64(C.CallLongMethodA(env.value(), obj.value(), jmid, jArgArr))
	return ret, JExceptionMsg(env)
}

// CallVoidMethod calls a Java method that doesn't return anything.
func CallVoidMethod(env Env, obj Object, name string, argSigns []Sign, args ...interface{}) error {
	jmid, jArgArr, freeFunc, err := setupMethodCall(env, obj, name, argSigns, VoidSign, args...)
	if err != nil {
		return err
	}
	defer freeFunc()
	C.CallVoidMethodA(env.value(), obj.value(), jmid, jArgArr)
	return JExceptionMsg(env)
}

// CallStaticObjectMethod calls a static Java method that returns a Java object.
func CallStaticObjectMethod(env Env, class Class, name string, argSigns []Sign, retSign Sign, args ...interface{}) (Object, error) {
	switch retSign {
	case ByteSign, CharSign, ShortSign, LongSign, FloatSign, DoubleSign, BoolSign, IntSign, VoidSign:
		panic(fmt.Sprintf("Illegal call to CallStaticObjectMethod on method with return sign %s", retSign))
	}
	jmid, jArgArr, freeFunc, err := setupStaticMethodCall(env, class, name, argSigns, retSign, args...)
	if err != nil {
		return NullObject, err
	}
	defer freeFunc()
	ret := C.CallStaticObjectMethodA(env.value(), class.value(), jmid, jArgArr)
	return Object(uintptr(unsafe.Pointer(ret))), JExceptionMsg(env)
}

// CallStaticStringMethod calls a static Java method that returns a string.
func CallStaticStringMethod(env Env, class Class, name string, argSigns []Sign, args ...interface{}) (string, error) {
	strObj, err := CallStaticObjectMethod(env, class, name, argSigns, StringSign, args...)
	return GoString(env, strObj), err
}

// CallStaticByteArrayMethod calls a static Java method that returns a byte array.
func CallStaticByteArrayMethod(env Env, class Class, name string, argSigns []Sign, args ...interface{}) ([]byte, error) {
	arrObj, err := CallStaticObjectMethod(env, class, name, argSigns, ArraySign(ByteSign), args...)
	if err != nil {
		return nil, err
	}
	return GoByteArray(env, arrObj), nil
}

// CallStaticLongArrayMethod calls a static Java method that returns a array of long.
func CallStaticLongArrayMethod(env Env, class Class, name string, argSigns []Sign, args ...interface{}) ([]int64, error) {
	arrObj, err := CallStaticObjectMethod(env, class, name, argSigns, ArraySign(LongSign), args...)
	if err != nil {
		return nil, err
	}
	return GoLongArray(env, arrObj), nil
}

// CallStaticIntMethod calls a static Java method that returns an int.
func CallStaticIntMethod(env Env, class Class, name string, argSigns []Sign, args ...interface{}) (int, error) {
	jmid, jArgArr, freeFunc, err := setupStaticMethodCall(env, class, name, argSigns, IntSign, args...)
	if err != nil {
		return 0, err
	}
	defer freeFunc()
	ret := int(C.CallStaticIntMethodA(env.value(), class.value(), jmid, jArgArr))
	return ret, JExceptionMsg(env)
}

// CallStaticVoidMethod calls a static Java method doesn't return anything.
func CallStaticVoidMethod(env Env, class Class, name string, argSigns []Sign, args ...interface{}) error {
	jmid, jArgArr, freeFunc, err := setupStaticMethodCall(env, class, name, argSigns, VoidSign, args...)
	if err != nil {
		return err
	}
	defer freeFunc()
	C.CallStaticVoidMethodA(env.value(), class.value(), jmid, jArgArr)
	return JExceptionMsg(env)
}

// CallFutureMethod invokes a Java method that returns a ListenableFuture.
// This method will wait for the future to complete and return the
// result/error to the caller.  The returned result Object, if not-null, is guaranteed
// to be globally referenced and this reference must be deleted.
//
// The provided freeFunc is guaranteed to be invoked, regardless on whether the method
// succeeds or fails.  Hence, the caller should get a new JNI environment after this
// method returns.
func CallFutureMethod(env Env, freeFunc func(), obj Object, name string, argSigns []Sign, args ...interface{}) (Object, error) {
	jFuture, err := CallObjectMethod(env, obj, name, argSigns, futureSign, args...)
	if err != nil {
		freeFunc()
		return NullObject, err
	}
	return CallStaticCallbackMethod(env, freeFunc, jUtilClass, "asCallback", []Sign{futureSign}, jFuture)
}

// CallStaticFutureMethod invokes a Java static method that returns a ListenableFuture.
// This method will wait for the future to complete and return the
// result/error to the caller.  The returned result Object, if not-null, is guaranteed
// to be globally referenced and this reference must be deleted.
//
// The provided freeFunc is guaranteed to be invoked, regardless on whether the method
// succeeds or fails.  Hence, the caller should get a new JNI environment after this
// method returns.
func CallStaticFutureMethod(env Env, freeFunc func(), class Class, name string, argSigns []Sign, args ...interface{}) (Object, error) {
	jFuture, err := CallStaticObjectMethod(env, class, name, argSigns, futureSign, args...)
	if err != nil {
		freeFunc()
		return NullObject, err
	}
	return CallStaticCallbackMethod(env, freeFunc, jUtilClass, "asCallback", []Sign{futureSign}, jFuture)
}

// CallCallbackMethod invokes a Java method whose last argument is a
// Java Callback.  This method will wait for the callback to complete and return the
// result/error to the caller.  The returned result Object, if not-null, is guaranteed
// to be globally referenced and this reference must be deleted.
//
// The invoked Java method must return "void" and have
// its last argument be a Java Callback.  The provided list of argument signs and
// arguments should include all arguments except the Callback.
//
// The provided freeFunc is guaranteed to be invoked, regardless on whether the method
// succeeds or fails.  Hence, the caller should get a new JNI environment after this
// method returns.
func CallCallbackMethod(env Env, freeFunc func(), obj Object, name string, argSigns []Sign, args ...interface{}) (Object, error) {
	jCallback, succCh, errCh, err := setupJavaCallback(env)
	if err != nil {
		freeFunc()
		return NullObject, err
	}
	if err := CallVoidMethod(env, obj, name, append(argSigns, callbackSign), append(args, jCallback)...); err != nil {
		freeFunc()
		return NullObject, err
	}
	// Blocking below, so must free the env.
	freeFunc()
	return blockOnJavaCallback(succCh, errCh)
}

// CallStaticCallbackMethod invokes a Java static method whose last argument is a
// Java Callback.  This method will wait for the callback to complete and return the
// result/error to the caller.  The returned result Object, if not-null, is guaranteed
// to be globally referenced and this reference must be deleted.
//
// The invoked Java method must return "void" and have
// its last argument be a Java Callback.  The provided list of argument signs and
// arguments should include all arguments except the Callback.
//
// The provided freeFunc is guaranteed to be invoked, regardless on whether the method
// succeeds or fails.  Hence, the caller should get a new JNI environment after this
// method returns.
func CallStaticCallbackMethod(env Env, freeFunc func(), class Class, name string, argSigns []Sign, args ...interface{}) (Object, error) {
	jCallback, succCh, errCh, err := setupJavaCallback(env)
	if err != nil {
		freeFunc()
		return NullObject, err
	}
	if err := CallStaticVoidMethod(env, class, name, append(argSigns, callbackSign), append(args, jCallback)...); err != nil {
		freeFunc()
		return NullObject, err
	}
	// Blocking below, so must free the env.
	freeFunc()
	return blockOnJavaCallback(succCh, errCh)
}

func setupJavaCallback(env Env) (jCallback Object, succCh chan Object, errCh chan error, err error) {
	succCh = make(chan Object, 1)
	errCh = make(chan error, 1)
	success := func(jResult Object) {
		env, freeFunc := GetEnv()
		defer freeFunc()
		succCh <- NewGlobalRef(env, jResult) // Un-refed by the callers of the callback methods above
	}
	failure := func(err error) {
		errCh <- err
	}
	jCallback, err = JavaNativeCallback(env, success, failure)
	return
}

func blockOnJavaCallback(succCh chan Object, errCh chan error) (Object, error) {
	select {
	case jResult := <-succCh:
		return jResult, nil
	case err := <-errCh:
		return NullObject, err
	}
}

// JavaNativeCallback creates a new Java Callback object that calls the provided Go functions
// on success/failures.
func JavaNativeCallback(env Env, success func(jResult Object), failure func(err error)) (Object, error) {
	succRef := GoNewRef(&success) // Un-refed when jCallback is finalized
	failRef := GoNewRef(&failure) // Un-refed when jCallback is finalized
	jCallback, err := NewObject(env, jNativeCallbackClass, []Sign{LongSign, LongSign}, int64(succRef), int64(failRef))
	if err != nil {
		GoDecRef(succRef)
		GoDecRef(failRef)
		return NullObject, err
	}
	return jCallback, nil
}

// DoAsyncCall invokes the given fnToWrap in a goroutine. If fnToWrap returns an
// error, the given callback's onFailure method is invoked with the error as a
// parameter. If fnToWrap succeeds, its Object result is passed as a
// parameter to the callback's onSuccess method.  fnToWrap must return a global reference
// to any non-null objects - this reference will be deleted by DoAsyncCall.
//
// The caller of doAsyncCall must take care that no local JNI references are
// used in fnToWrap's closure. For example:
//
// func myNativeCall(env Env, jCallback Object, someJObject C.jobject) {
//     doAsyncCallback(env, jCallback, func(env Env) (Object, error) {
//         callSomeMethodOn(someJObject)  // not OK, someJObject is a local JNI reference
//                                        // in the caller's scope.
//         ...
//     }
// }
//
// fnToWrap is run in on an arbitrary thread and local
// JNI references are only valid in the scope of a particular thread. You are
// free to capture any pure-Go variables and we recommend that you use that
// approach to pass parameters through to fnToWrap.
func DoAsyncCall(env Env, jCallback Object, fnToWrap func() (Object, error)) {
	go func(jCallback Object) {
		jResult, err := fnToWrap() // probably blocking, so don't call GetEnv() before this line
		env, freeFunc := GetEnv()
		defer freeFunc()
		defer DeleteGlobalRef(env, jCallback)
		if !jResult.IsNull() {
			if !IsGlobalRef(env, jResult) {
				CallbackOnFailure(env, jCallback, fmt.Errorf("Function passed to DoAsyncCall must return global object references"))
				return
			}
			defer DeleteGlobalRef(env, jResult)
		}
		if err != nil {
			CallbackOnFailure(env, jCallback, err)
			return
		}
		CallbackOnSuccess(env, jCallback, jResult)
	}(NewGlobalRef(env, jCallback))
}

// CallbackOnFailure calls the given callback's "onFailure" method with the given error and
// panic-s if the method couldn't be invoked.
func CallbackOnFailure(env Env, jCallback Object, err error) {
	if err := CallVoidMethod(env, jCallback, "onFailure", []Sign{VExceptionSign}, err); err != nil {
		panic(fmt.Sprintf("couldn't call Java onFailure method: %v", err))
	}
}

// CalbackOnSuccess calls the given callback's "onSuccess" method with the given result
// and panic-s if the method couldn't be invoked.
func CallbackOnSuccess(env Env, jCallback Object, jResult Object) {
	if err := CallVoidMethod(env, jCallback, "onSuccess", []Sign{ObjectSign}, jResult); err != nil {
		panic(fmt.Sprintf("couldn't call Java onSuccess method: %v", err))
	}
}

func handleError(name string, err error) {
	if err != nil {
		panic(fmt.Sprintf("error while calling jni method %q: %v", name, err))
	}
}

func isSignOneOf(sign Sign, set []Sign) bool {
	for _, s := range set {
		if sign == s {
			return true
		}
	}
	return false
}
