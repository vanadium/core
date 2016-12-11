// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package util

import (
	"fmt"
	"reflect"
	"sync"
	"unsafe"
)

// #include "jni_wrapper.h"
import "C"

// NewGlobalRef creates a new global reference to the object referred to by the
// obj argument.  The obj argument may be a global or local reference. Global
// references must be explicitly disposed of by calling DeleteGlobalRef().
func NewGlobalRef(env Env, obj Object) Object {
	if obj.IsNull() {
		return obj
	}
	return Object(uintptr(unsafe.Pointer(C.NewGlobalRef(env.value(), obj.value()))))
}

// DeleteGlobalRef deletes the global reference pointed to by obj.
func DeleteGlobalRef(env Env, obj Object) {
	if !obj.IsNull() {
		C.DeleteGlobalRef(env.value(), obj.value())
	}
}

// NewLocalRef creates a new local reference that refers to the same object
// as obj. The given obj may be a global or local reference.
func NewLocalRef(env Env, obj Object) Object {
	if obj.IsNull() {
		return obj
	}
	return Object(uintptr(unsafe.Pointer(C.NewLocalRef(env.value(), obj.value()))))
}

// DeleteLocalRef deletes the local reference pointed to by obj.
func DeleteLocalRef(env Env, obj Object) {
	if !obj.IsNull() {
		C.DeleteLocalRef(env.value(), obj.value())
	}
}

// IsGlobalRef returns true iff the reference pointed to by obj is a global reference.
func IsGlobalRef(env Env, obj Object) bool {
	return !obj.IsNull() && C.GetObjectRefType(env.value(), obj.value()) == C.JNIGlobalRefType
}

// IsLocalRef returns true iff the reference pointed to by obj is a local reference.
func IsLocalRef(env Env, obj Object) bool {
	return obj.IsNull() || C.GetObjectRefType(env.value(), obj.value()) == C.JNILocalRefType
}

// GoNewRef creates a new reference for the given Go pointer, setting its
// reference count to 1.  The Go pointer isn't garbage collected as long as
// the reference count remains greater than zero.  The reference counts can
// be modified using GoIncRef and GoDecRef functions below.
//
// Note that it is legal to create multiple references for the same Go pointer:
// the Go pointer will not be garbage collected as long as some reference's
// ref count is greater than zero.
func GoNewRef(valptr interface{}) Ref {
	if !IsPointer(valptr) {
		panic(fmt.Sprintf("Must pass pointer value to GoNewRef; instead got %v of type %T", valptr, valptr))
	}
	return goRefs.newRef(valptr)
}

// GoIncRef increments the reference count for the given reference by 1.
func GoIncRef(ref Ref) {
	goRefs.incRef(ref)
}

// GoDecRef decrements the reference count for the given reference by 1.
func GoDecRef(ref Ref) {
	goRefs.decRef(ref)
}

// GoRefValue returns the Go pointer associated with the given reference
// as an unsafe.Pointer (so that it can be easily cast into its right type).
func GoRefValue(ref Ref) unsafe.Pointer {
	valptr := goRefs.getVal(ref)
	v := reflect.ValueOf(valptr)
	if v.Kind() != reflect.Ptr && v.Kind() != reflect.UnsafePointer {
		panic(fmt.Sprintf("must pass pointer value to PtrValue, was %v ", v.Type()))
	}
	return unsafe.Pointer(v.Pointer())
}

// IsPointer returns true iff the provided value is a pointer.
func IsPointer(val interface{}) bool {
	v := reflect.ValueOf(val)
	return v.Kind() == reflect.Ptr || v.Kind() == reflect.UnsafePointer
}

// DerefOrDie dereferences the provided (pointer) value, or panic-s if the value
// isn't of pointer type.
func DerefOrDie(i interface{}) interface{} {
	v := reflect.ValueOf(i)
	if v.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("want reflect.Ptr value for %v, have %v", i, v.Type()))
	}
	return v.Elem().Interface()
}

// goRefs stores references to instances of various Go types, namely instances
// that are referenced only by the Java code.  The only purpose of this store
// is to prevent Go runtime from garbage collecting those instances.
var goRefs = newSafeRefCounter()

type refData struct {
	instance interface{}
	count    int
}

// newSafeRefCounter returns a new instance of a thread-safe reference counter.
func newSafeRefCounter() *safeRefCounter {
	return &safeRefCounter{
		seq:  1, // must be > 0, as 0 is a special value
		refs: make(map[Ref]*refData),
	}
}

// safeRefCounter is a thread-safe reference counter.
type safeRefCounter struct {
	lock sync.RWMutex
	seq  Ref
	refs map[Ref]*refData
}

// newRef creates a new reference to a given Go pointer and sets its ref count to 1.
func (c *safeRefCounter) newRef(valptr interface{}) Ref {
	c.lock.Lock()
	defer c.lock.Unlock()
	ref := c.seq
	c.seq++
	c.refs[ref] = &refData{
		instance: valptr,
		count:    1,
	}
	return ref
}

// ref increases the reference count for the given reference by 1.
func (c *safeRefCounter) incRef(ref Ref) {
	c.lock.Lock()
	defer c.lock.Unlock()
	data, ok := c.refs[ref]
	if !ok {
		panic(fmt.Sprintf("Reference %d doesn't exist", ref))
	}
	data.count++
}

// unref decreases the reference count for the given reference by 1.
func (c *safeRefCounter) decRef(ref Ref) {
	c.lock.Lock()
	defer c.lock.Unlock()
	data, ok := c.refs[ref]
	if !ok {
		panic(fmt.Sprintf("Reference %d doesn't exist", ref))
	}
	if data.count == 0 {
		panic(fmt.Sprintf("Reference %d exists, but doesn't have a count of 0.", ref))
	}
	data.count--
	if data.count == 0 {
		delete(c.refs, ref)
	}
}

// getVal returns the value for the given reference.
func (c *safeRefCounter) getVal(ref Ref) interface{} {
	c.lock.RLock()
	defer c.lock.RUnlock()
	data, ok := c.refs[ref]
	if !ok {
		panic(fmt.Sprintf("Reference %d doesn't exist.", ref))
	}
	return data.instance
}
