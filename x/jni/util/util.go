// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

// Package util provides various JNI utilities shared across our JNI code.
package util

import (
	"errors"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"v.io/v23/vdl"
	"v.io/v23/vom"
)

// #include <stdlib.h>
// #include "jni_wrapper.h"
// static jstring CallGetExceptionMessage(JNIEnv* env, jobject obj, jmethodID id) {
//   return (jstring)(*env)->CallObjectMethod(env, obj, id);
// }
import "C"

type skipOptionType struct{}

var (
	// SkipOption is a special error that should be returned by the option
	// processing function passed to GoOptions. It indicates that the
	// option being processed should be skipped.
	SkipOption = skipOptionType{}
	envs       = newEnvCounter()
)

func (skipOptionType) Error() string {
	return "ignored option"
}

// CamelCase converts ThisString to thisString.
func CamelCase(s string) string {
	if s == "" {
		return ""
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[n:]
}

// UpperCamelCase converts thisString to ThisString.
func UpperCamelCase(s string) string {
	if s == "" {
		return ""
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}

// GoString returns a Go string given the Java string.
func GoString(env Env, str Object) string {
	if str.IsNull() {
		return ""
	}
	cString := C.GetStringUTFChars(env.value(), C.jstring(str.value()), nil)
	defer C.ReleaseStringUTFChars(env.value(), C.jstring(str.value()), cString)
	return C.GoString(cString)
}

// GetClass returns the class of the given object.
func GetClass(env Env, obj Object) Class {
	return Class(uintptr(unsafe.Pointer(C.GetObjectClass(env.value(), obj.value()))))
}

var envRefs = newSafeRefCounter()

// GetEnv returns the Java environment for the running thread, creating a new
// one if it doesn't already exist.  This method also returns a function which
// must be invoked when the returned environment is no longer needed. The
// returned environment can only be used by the thread that invoked this method,
// and the function must be invoked by the same thread as well.
func GetEnv() (env Env, free func()) {
	// Lock the goroutine to the current OS thread.  This is necessary as
	// *C.JNIEnv must not be shared across threads.  The scenario that can break
	// this requrement is:
	//   - goroutine A executing on thread X, obtaining a *C.JNIEnv pointer P.
	//   - goroutine A gets re-scheduled on thread Y, maintaining the pointer P.
	//   - goroutine B starts executing on thread X, obtaining pointer P.
	//
	// By locking the goroutines to their OS thread while they hold the pointer
	// to *C.JNIEnv, the above scenario can never occur.
	runtime.LockOSThread()
	var jenv *C.JNIEnv
	if C.GetEnv(jVM, &jenv, C.JNI_VERSION_1_6) != C.JNI_OK {
		// Couldn't get env - attach the thread.  Note that we never detach
		// the thread so the next call to GetEnv on this thread will succeed.
		C.AttachCurrentThreadAsDaemon(jVM, &jenv, nil)
	}
	env = Env(uintptr(unsafe.Pointer(jenv)))
	// GetEnv is called by Go code that wishes to call Java methods. In
	// this case, JNI cannot automatically free unused local refererences.
	// We must do it manually by pushing a new local reference frame. The
	// frame will be popped in the env's cleanup function below, at which
	// point JNI will free the unused references.
	// http://developer.android.com/training/articles/perf-jni.html states
	// that the JNI implementation is only required to provide a local
	// reference table with a capacity of 16, so here we provide a table of
	// that size.
	localRefCapacity := 16
	if newCapacity := PushLocalFrame(env, localRefCapacity); newCapacity < 0 {
		panic("PushLocalFrame(" + string(localRefCapacity) + ") returned < 0 (was " + string(newCapacity) + ")")
	}
	// Count *env.  This is necessary to make GetEnv() re-entrant:
	// same goroutine should be allowed to call GetEnv() multiple times and
	// the OS thread should only be released after the last call to the free
	// function.
	envs.inc(jenv)
	return env, func() {
		PopLocalFrame(env, NullObject)
		if envs.dec(jenv) == 0 {
			// Last call to free the env out of possibly many successive
			// GetEnv() calls by the same goroutine: it is now safe to release
			// the thread.
			runtime.UnlockOSThread()
		}
	}
}

// IsInstanceOf returns true iff the provided Java object is an instance of the
// provided Java class.
func IsInstanceOf(env Env, obj Object, class Class) bool {
	return C.IsInstanceOf(env.value(), obj.value(), class.value()) == C.JNI_TRUE
}

// JString returns a Java string given the Go string.
func JString(env Env, str string) Object {
	cString := C.CString(str)
	defer C.free(unsafe.Pointer(cString))
	return Object(uintptr(unsafe.Pointer(C.NewStringUTF(env.value(), cString))))
}

// JThrow throws a new Java exception of the provided type with the given message.
func JThrow(env Env, class Class, msg string) {
	s := C.CString(msg)
	defer C.free(unsafe.Pointer(s))
	C.ThrowNew(env.value(), class.value(), s)
}

// JThrowV throws a new Java VException corresponding to the given error.
func JThrowV(env Env, native error) {
	if native == nil {
		log.Printf("Couldn't throw exception: nil error")
		return
	}
	obj, err := JVomCopy(env, native, jVExceptionClass)
	if err != nil {
		log.Printf("Couldn't throw exception %#v: %v", native, err)
		return
	}
	C.Throw(env.value(), C.jthrowable(obj.value()))
}

// JVException returns the Java VException given the Go error.
func JVException(env Env, native error) (Object, error) {
	if native == nil {
		return NullObject, nil
	}
	return JVomCopy(env, native, jVExceptionClass)
}

// JExceptionMsg returns the exception message as a Go error, if an exception
// occurred, or nil otherwise.
func JExceptionMsg(env Env) error {
	jException := Object(uintptr(unsafe.Pointer(C.ExceptionOccurred(env.value()))))
	if jException.IsNull() { // no exception
		return nil
	}
	C.ExceptionDescribe(env.value())
	C.ExceptionClear(env.value())
	return GoError(env, jException)
}

// GoError converts the provided Java Exception into a Go error, converting VException into
// verror.T and all other exceptions into a Go error.
func GoError(env Env, jException Object) error {
	if jException.IsNull() {
		return nil
	}
	if IsInstanceOf(env, jException, jVExceptionClass) {
		// VException: convert it into a verror.
		// Note that we can't use CallStaticObjectMethod below as it may lead to
		// an infinite loop.
		jmid, jArgArr, freeFunc, err := setupStaticMethodCall(env, jVomUtilClass, "encode", []Sign{ObjectSign, TypeSign}, ByteArraySign, jException, jVExceptionClass)
		if err != nil {
			return fmt.Errorf("error converting VException: " + err.Error())
		}
		defer freeFunc()
		dataObj := C.CallStaticObjectMethodA(env.value(), jVomUtilClass.value(), jmid, jArgArr)
		if e := C.ExceptionOccurred(env.value()); e != 0 {
			C.ExceptionClear(env.value())
			return fmt.Errorf("error converting VException: exception during VomUtil.encode()")
		}
		data := GoByteArray(env, Object(uintptr(unsafe.Pointer(dataObj))))
		var verr error
		if err := vom.Decode(data, &verr); err != nil {
			return fmt.Errorf("error converting VException: " + err.Error())
		}
		return verr
	}
	// Not a VException: convert it into a Go error.
	// Note that we can't use CallObjectMethod below, as it may lead to an
	// infinite loop.
	jmid, jArgArr, freeFunc, err := setupMethodCall(env, jException, "getMessage", nil, StringSign)
	if err != nil {
		return fmt.Errorf("error converting exception: " + err.Error())
	}
	defer freeFunc()
	strObj := C.CallObjectMethodA(env.value(), jException.value(), jmid, jArgArr)
	if e := C.ExceptionOccurred(env.value()); e != 0 {
		C.ExceptionClear(env.value())
		return fmt.Errorf("error converting exception: exception during Throwable.getMessage()")
	}
	return errors.New(GoString(env, Object(uintptr(unsafe.Pointer(strObj)))))
}

// JObjectField returns the value of the provided Java object's Object field, or
// error if the field value couldn't be retrieved.
func JObjectField(env Env, obj Object, field string, sign Sign) (Object, error) {
	fid, err := jFieldID(env, GetClass(env, obj), field, sign)
	if err != nil {
		return NullObject, err
	}
	return Object(uintptr(unsafe.Pointer(C.GetObjectField(env.value(), obj.value(), fid)))), nil
}

// JBoolField returns the value of the provided Java object's boolean field, or
// error if the field value couldn't be retrieved.
func JBoolField(env Env, obj Object, field string) (bool, error) {
	fid, err := jFieldID(env, GetClass(env, obj), field, BoolSign)
	if err != nil {
		return false, err
	}
	return C.GetBooleanField(env.value(), obj.value(), fid) != C.JNI_FALSE, nil
}

// JIntField returns the value of the provided Java object's int field, or
// error if the field value couldn't be retrieved.
func JIntField(env Env, obj Object, field string) (int, error) {
	fid, err := jFieldID(env, GetClass(env, obj), field, IntSign)
	if err != nil {
		return -1, err
	}
	return int(C.GetIntField(env.value(), obj.value(), fid)), nil
}

// JLongField returns the value of the provided Java object's long field, or
// error if the field value couldn't be retrieved.
func JLongField(env Env, obj Object, field string) (int64, error) {
	fid, err := jFieldID(env, GetClass(env, obj), field, LongSign)
	if err != nil {
		return -1, err
	}
	return int64(C.GetLongField(env.value(), obj.value(), fid)), nil
}

// JStringField returns the value of the provided Java object's String field, or
// error if the field value couldn't be retrieved.
func JStringField(env Env, obj Object, field string) (string, error) {
	strObj, err := JObjectField(env, obj, field, StringSign)
	if err != nil {
		return "", err
	}
	return GoString(env, strObj), nil
}

// JStringArrayField returns the value of the provided object's String[] field,
// or error if the field value couldn't be retrieved.
func JStringArrayField(env Env, obj Object, field string) ([]string, error) {
	arrObj, err := JObjectField(env, obj, field, ArraySign(StringSign))
	if err != nil {
		return nil, err
	}
	return GoStringArray(env, arrObj)
}

// JByteArrayField returns the value of the provided object's byte[] field, or
// error if the field value couldn't be retrieved.
func JByteArrayField(env Env, obj Object, field string) ([]byte, error) {
	arrObj, err := JObjectField(env, obj, field, ArraySign(ByteSign))
	if err != nil {
		return nil, err
	}
	return GoByteArray(env, arrObj), nil
}

// JByteArrayArrayField returns the value of the provided object's byte[][]
// field, or error if the field value couldn't be retrieved.
func JByteArrayArrayField(env Env, obj Object, field string) ([][]byte, error) {
	arrObj, err := JObjectField(env, obj, field, ArraySign(ArraySign(ByteSign)))
	if err != nil {
		return nil, err
	}
	return GoByteArrayArray(env, arrObj)
}

func JStaticObjectField(env Env, class Class, field string, sign Sign) (Object, error) {
	fid, err := jStaticFieldID(env, class, field, sign)
	if err != nil {
		return NullObject, err
	}
	return Object(uintptr(unsafe.Pointer(C.GetStaticObjectField(env.value(), class.value(), fid)))), nil
}

// JStaticStringField returns the value of the static String field of the
// provided Java class, or error if the field value couldn't be retrieved.
func JStaticStringField(env Env, class Class, field string) (string, error) {
	strObj, err := JStaticObjectField(env, class, field, StringSign)
	if err != nil {
		return "", err
	}
	return GoString(env, strObj), nil
}

// JObjectArray converts the provided slice of objects into a Java object
// array of the provided element type.
func JObjectArray(env Env, arr []Object, elemClass Class) (Object, error) {
	arrObj := C.NewObjectArray(env.value(), C.jsize(len(arr)), elemClass.value(), 0)
	for i, elem := range arr {
		C.SetObjectArrayElement(env.value(), arrObj, C.jsize(i), elem.value())
		if err := JExceptionMsg(env); err != nil {
			return NullObject, err
		}
	}
	return Object(uintptr(unsafe.Pointer(arrObj))), nil
}

// GoObjectArray converts a Java object array to a Go slice of Java objects.
func GoObjectArray(env Env, arr Object) ([]Object, error) {
	if arr.IsNull() {
		return nil, nil
	}
	length := int(C.GetArrayLength(env.value(), C.jarray(arr.value())))
	ret := make([]Object, length)
	for i := 0; i < length; i++ {
		ret[i] = Object(uintptr(unsafe.Pointer(C.GetObjectArrayElement(env.value(), C.jobjectArray(arr.value()), C.jsize(i)))))
		if err := JExceptionMsg(env); err != nil {
			// Out-of-bounds index.
			return nil, err
		}
	}
	return ret, nil
}

// JObjectList converts the provided slice of Java objects into a Java List
// of the provided element type.
func JObjectList(env Env, arr []Object, elemClass Class) (Object, error) {
	arrObj, err := JObjectArray(env, arr, elemClass)
	if err != nil {
		return NullObject, err
	}
	listObj, err := CallStaticObjectMethod(env, jArraysClass, "asList", []Sign{ArraySign(ObjectSign)}, ListSign, arrObj)
	if err != nil {
		return NullObject, err
	}
	return NewObject(env, jArrayListClass, []Sign{CollectionSign}, listObj)
}

// GoObjectList converts the provided Java list of objects into a Go slice
// of Java objects.
func GoObjectList(env Env, list Object) ([]Object, error) {
	if list.IsNull() {
		return nil, nil
	}
	arrObj, err := CallObjectMethod(env, list, "toArray", nil, ArraySign(ObjectSign))
	if err != nil {
		return nil, err
	}
	return GoObjectArray(env, arrObj)
}

// GoStringList converts the provided Java List<String> Strings into a Go slice of
// strings.
func GoStringList(env Env, list Object) ([]string, error) {
	if list.IsNull() {
		return nil, nil
	}
	strArr, err := GoObjectList(env, list)
	if err != nil {
		return nil, err
	}
	ret := make([]string, len(strArr))
	for i, strObj := range strArr {
		ret[i] = GoString(env, strObj)
	}
	return ret, nil
}

// JStringList converts the provided slice of Go strings into a Java
// List<String>.
func JStringList(env Env, strs []string) (Object, error) {
	strArr := make([]Object, len(strs))
	for i, str := range strs {
		strArr[i] = JString(env, str)
	}
	return JObjectList(env, strArr, jStringClass)
}

// JStringArray converts the provided slice of Go strings into a Java array of
// strings.
func JStringArray(env Env, strs []string) (Object, error) {
	strArr := make([]Object, len(strs))
	for i, str := range strs {
		strArr[i] = JString(env, str)
	}
	return JObjectArray(env, strArr, jStringClass)
}

// GoStringArray converts a Java string array to a Go string slice.
func GoStringArray(env Env, arr Object) ([]string, error) {
	if arr.IsNull() {
		return nil, nil
	}
	strArr, err := GoObjectArray(env, arr)
	if err != nil {
		return nil, err
	}
	ret := make([]string, len(strArr))
	for i, strObj := range strArr {
		ret[i] = GoString(env, strObj)
	}
	return ret, nil
}

// JByteArray converts the provided Go byte slice into a Java byte array.
func JByteArray(env Env, bytes []byte) (Object, error) {
	arr := C.NewByteArray(env.value(), C.jsize(len(bytes)))
	if len(bytes) > 0 {
		C.SetByteArrayRegion(env.value(), arr, 0, C.jsize(len(bytes)), (*C.jbyte)(unsafe.Pointer(&bytes[0])))
		if err := JExceptionMsg(env); err != nil {
			return NullObject, err
		}
	}
	return Object(uintptr(unsafe.Pointer(arr))), nil
}

// GoByteArray converts the provided Java byte array into a Go byte slice.
func GoByteArray(env Env, arr Object) (ret []byte) {
	if arr.IsNull() {
		return nil
	}
	length := int(C.GetArrayLength(env.value(), C.jarray(arr.value())))
	ret = make([]byte, length)
	bytes := C.GetByteArrayElements(env.value(), C.jbyteArray(arr.value()), nil)
	defer C.ReleaseByteArrayElements(env.value(), C.jbyteArray(arr.value()), bytes, 2 /* C.JNI_ABORT */)
	ptr := bytes
	for i := 0; i < length; i++ {
		ret[i] = byte(*ptr)
		ptr = (*C.jbyte)(unsafe.Pointer(uintptr(unsafe.Pointer(ptr)) + unsafe.Sizeof(*ptr)))
	}
	return
}

// GoLongArray converts the provided Java long array into a Go int64 slice.
func GoLongArray(env Env, arr Object) (ret []int64) {
	if arr.IsNull() {
		return
	}
	length := int(C.GetArrayLength(env.value(), C.jarray(arr.value())))
	ret = make([]int64, length)
	elems := C.GetLongArrayElements(env.value(), C.jlongArray(arr.value()), nil)
	defer C.ReleaseLongArrayElements(env.value(), C.jlongArray(arr.value()), elems, 2 /* C.JNI_ABORT */)
	ptr := elems
	for i := 0; i < length; i++ {
		ret[i] = int64(*ptr)
		ptr = (*C.jlong)(unsafe.Pointer(uintptr(unsafe.Pointer(ptr)) + unsafe.Sizeof(*ptr)))
	}
	return
}

// JByteArrayArray converts the provided [][]byte value into a Java array of
// byte arrays.
func JByteArrayArray(env Env, arr [][]byte) (Object, error) {
	objArr := make([]Object, len(arr))
	for i, elem := range arr {
		var err error
		if objArr[i], err = JByteArray(env, elem); err != nil {
			return NullObject, err
		}
	}
	return JObjectArray(env, objArr, jByteArrayClass)
}

// GoByteArrayArray converts the provided Java array of byte arrays into a Go
// [][]byte value.
func GoByteArrayArray(env Env, arr Object) ([][]byte, error) {
	objArr, err := GoObjectArray(env, arr)
	if err != nil {
		return nil, err
	}
	ret := make([][]byte, len(objArr))
	for i, obj := range objArr {
		ret[i] = GoByteArray(env, obj)
	}
	return ret, nil
}

// JVDLValueArray converts the provided Go slice of *vdl.Value values into a
// Java array of VdlValue objects.
func JVDLValueArray(env Env, arr []*vdl.Value) (Object, error) {
	objArr := make([]Object, len(arr))
	for i, val := range arr {
		var err error
		if objArr[i], err = JVomCopy(env, val, jVdlValueClass); err != nil {
			return NullObject, err
		}
	}
	return JObjectArray(env, objArr, jVdlValueClass)
}

// GoVDLValueArray converts the provided Java array of VdlValue objects into a
// Go slice of *vdl.Value values.
func GoVDLValueArray(env Env, arr Object) ([]*vdl.Value, error) {
	objArr, err := GoObjectArray(env, arr)
	if err != nil {
		return nil, err
	}
	vals := make([]*vdl.Value, len(objArr))
	for i, obj := range objArr {
		var err error
		if vals[i], err = GoVomCopyValue(env, obj); err != nil {
			return nil, err
		}
	}
	return vals, nil
}

// JObjectMap converts the provided Go map of Java objects into a Java
// object map.
func JObjectMap(env Env, m map[Object]Object) (Object, error) {
	mapObj, err := NewObject(env, jHashMapClass, nil)
	if err != nil {
		return NullObject, err
	}
	for keyObj, valObj := range m {
		if _, err := CallObjectMethod(env, mapObj, "put", []Sign{ObjectSign, ObjectSign}, ObjectSign, keyObj, valObj); err != nil {
			return NullObject, err
		}
	}
	return mapObj, nil
}

// GoObjectMap converts the provided Java object map into a Go map of Java
// objects.
func GoObjectMap(env Env, mapObj Object) (map[Object]Object, error) {
	if mapObj.IsNull() {
		return nil, nil
	}
	keysObj, err := CallObjectMethod(env, mapObj, "keySet", nil, SetSign)
	if err != nil {
		return nil, err
	}
	keysArr, err := CallObjectArrayMethod(env, keysObj, "toArray", nil, ObjectSign)
	if err != nil {
		return nil, err
	}
	ret := make(map[Object]Object)
	for _, keyObj := range keysArr {
		valObj, err := CallObjectMethod(env, mapObj, "get", []Sign{ObjectSign}, ObjectSign, keyObj)
		if err != nil {
			return nil, err
		}
		ret[keyObj] = valObj
	}
	return ret, nil
}

// JObjectMultimap converts the provided Go map of Java objects into a Java
// object multimap.
func JObjectMultimap(env Env, goMap map[Object][]Object) (Object, error) {
	mapObj, err := NewObject(env, jHashMultimapClass, nil)
	if err != nil {
		return NullObject, err
	}
	for keyObj, vals := range goMap {
		for _, valObj := range vals {
			if _, err := CallBooleanMethod(env, mapObj, "put", []Sign{ObjectSign, ObjectSign}, keyObj, valObj); err != nil {
				return NullObject, err
			}
		}
	}
	return mapObj, nil
}

// GoObjectMultimap converts the provided Java Multimap object into a Go map
// of Java objects.
func GoObjectMultimap(env Env, multiMap Object) (map[Object][]Object, error) {
	if multiMap.IsNull() {
		return nil, nil
	}
	keysObj, err := CallObjectMethod(env, multiMap, "keySet", nil, SetSign)
	if err != nil {
		return nil, err
	}
	keysArr, err := CallObjectArrayMethod(env, keysObj, "toArray", nil, ObjectSign)
	if err != nil {
		return nil, err
	}
	ret := make(map[Object][]Object)
	for _, keyObj := range keysArr {
		valsObj, err := CallObjectMethod(env, multiMap, "get", []Sign{ObjectSign}, CollectionSign, keyObj)
		if err != nil {
			return nil, err
		}
		valsArr, err := CallObjectArrayMethod(env, valsObj, "toArray", nil, ObjectSign)
		if err != nil {
			return nil, err
		}
		ret[keyObj] = valsArr
	}
	return ret, nil
}

// JFindClass returns the global reference to the Java class with the
// given pathname, or an error if the class cannot be found.
func JFindClass(env Env, name string) (Class, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	class := C.FindClass(env.value(), cName)
	if err := JExceptionMsg(env); err != nil || class == 0 {
		return NullClass, fmt.Errorf("couldn't find class %s: %v", name, err)
	}
	obj := NewGlobalRef(env, Object(uintptr(unsafe.Pointer(class))))
	return Class(uintptr(unsafe.Pointer(C.jclass(obj.value())))), nil
}

// GoOptions converts a Java io.v.v23.Options instance into a slice of Go
// options (stored as interface{}).
//
// For each entry in the Java Option map, the user-supplied
// optionFunc is called to turn the entry into its corresponding Go option. It
// is up to the caller to cast the returned value to the appropriate Go type.
// If optionFunc returns a non-nil error, the err and a nil slice will be
// returned.
//
// The only exception to this rule is the special SkipOption error:
// if optionFunc returns this, the option will not be added to the result and
// option processing will continue.
func GoOptions(env Env, opts Object, optionFunc func(env Env, key string, opt Object) (interface{}, error)) ([]interface{}, error) {
	if opts.IsNull() {
		return []interface{}{}, nil
	}
	mapObj, err := CallMapMethod(env, opts, "asMap", []Sign{})
	if err != nil {
		return nil, err
	}
	var result []interface{}
	for keyObj, valObj := range mapObj {
		key := GoString(env, keyObj)
		value, err := optionFunc(env, key, valObj)
		if err == SkipOption {
			continue
		}
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

// JTime converts the provided Go time.Time value into a Java DateTime
// object.
func JTime(env Env, t time.Time) (Object, error) {
	millis := t.UnixNano() / 1000000
	return NewObject(env, jDateTimeClass, []Sign{LongSign}, millis)
}

// GoTime converts the provided Java DateTime object into a Go time.Time value.
func GoTime(env Env, timeObj Object) (time.Time, error) {
	if timeObj.IsNull() {
		return time.Time{}, nil
	}
	millis, err := CallLongMethod(env, timeObj, "getMillis", nil)
	if err != nil {
		return time.Time{}, err
	}
	sec := millis / 1000
	nsec := (millis % 1000) * 1000000
	return time.Unix(sec, nsec), nil
}

// JDuration converts the provided Go time.Duration value into a Java
// Duration object.
func JDuration(env Env, d time.Duration) (Object, error) {
	millis := d.Nanoseconds() / 1000000
	return NewObject(env, jDurationClass, []Sign{LongSign}, int64(millis))
}

// GoDuration converts the provided Java Duration object into a Go time.Duration
// value.
func GoDuration(env Env, duration Object) (time.Duration, error) {
	millis, err := CallLongMethod(env, duration, "getMillis", nil)
	if err != nil {
		return 0, err
	}
	return time.Duration(millis) * time.Millisecond, nil
}

// PushLocalFrame pushes a new local reference frame onto the reference frame
// stack. If the return value is >= 0, the new frame will have capacity for at
// least the specified number of local references. If the return value is < 0,
// the reference frame could not be created.
func PushLocalFrame(env Env, capacity int) int {
	return int(C.PushLocalFrame(env.value(), C.jint(capacity)))
}

// PopLocalFrame pops the most recent local reference frame off the reference
// frame stack. Returns a local reference in the previous reference frame to
// the given jFramePtr object. If you do not require a reference to the
// previous frame, you may pass nil for the jFramePtr parameter.
func PopLocalFrame(env Env, result Object) Object {
	if result.IsNull() {
		return Object(uintptr(unsafe.Pointer(C.PopLocalFrame(env.value(), 0))))
	}
	return Object(uintptr(unsafe.Pointer(C.PopLocalFrame(env.value(), result.value()))))
}

// jFieldID returns the Java field ID for the given object (i.e., non-static)
// field, or an error if the field couldn't be found.
// TODO(rosswang): Make these exported/cacheable (likely warrants refactor of corresponding
// GetXField methods)
// https://rkennke.wordpress.com/2007/07/24/efficient-jni-programming-ii-field-and-method-access/
func jFieldID(env Env, class Class, name string, sign Sign) (C.jfieldID, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	cSign := C.CString(string(sign))
	defer C.free(unsafe.Pointer(cSign))
	fid := C.GetFieldID(env.value(), class.value(), cName, cSign)
	if err := JExceptionMsg(env); err != nil || fid == nil {
		return nil, fmt.Errorf("couldn't find field %s: %v", name, err)
	}
	return fid, nil
}

// jStaticFieldID returns the Java field ID for the given static field,
// or an error if the field couldn't be found.
func jStaticFieldID(env Env, class Class, name string, sign Sign) (C.jfieldID, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))
	cSign := C.CString(string(sign))
	defer C.free(unsafe.Pointer(cSign))
	fid := C.GetStaticFieldID(env.value(), class.value(), cName, cSign)
	if err := JExceptionMsg(env); err != nil || fid == nil {
		return nil, fmt.Errorf("couldn't find field %s: %v", name, err)
	}
	return fid, nil
}

func newEnvCounter() *envCounter {
	return &envCounter{
		envs: make(map[*C.JNIEnv]*int),
	}
}

// envCounter counts, for each JNI environment, how many times has it been
// requested (by the same goroutine).
type envCounter struct {
	lock sync.Mutex
	envs map[*C.JNIEnv]*int
}

func (c *envCounter) inc(jenv *C.JNIEnv) {
	c.lock.Lock()
	defer c.lock.Unlock()
	count, ok := c.envs[jenv]
	if !ok {
		count := int(1)
		c.envs[jenv] = &count
	} else {
		(*count)++
	}
}

func (c *envCounter) dec(jenv *C.JNIEnv) int {
	c.lock.Lock()
	defer c.lock.Unlock()
	count, ok := c.envs[jenv]
	if !ok {
		panic("env entry with zero count")
	}
	(*count)--
	if *count == 0 {
		delete(c.envs, jenv)
	}
	return *count
}
