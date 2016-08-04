// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build java android

package util

import (
	"unsafe"
)

// #include "jni_wrapper.h"
import "C"

// Env represents a native *C.JNIEnv type.  The sole reason for this type is the
// fact that CGO creates package-local types: type *C.JNIEnv in package A is
// different from type *C.JNIEnv in package B.  Env type therefore unifies all
// *C.JNIEnv types across all packages, and we routinely convert from a
// package-local *C.JNIEnv type to Env and vice-versa.
type Env uintptr

// Object represents a native C.jobject type.  The sole reason for this type is
// the fact that CGO creates package-local types: type C.jobject in package A is
// different from type C.jobject in package B.  Object type therefore unifies
// all C.jobject types across all packages, and we routinely convert from a
// package-local C.jobject type to Object and vice-versa.
type Object uintptr

// Class represents a native C.jclass type.  The sole reason for this type is
// the fact that CGO creates package-local types: type C.jclass in package A is
// different from type C.jclass in package B.  Class type therefore unifies
// all C.jclass types across all packages, and we routinely convert from a
// package-local C.jclass type to Class and vice-versa.
type Class uintptr

// Ref represents a reference to a Go value that can be passed to the Java code
// and then safely re-casted to a Go value back again.
type Ref uint64

const (
	// NullObject represents an Object that holds a null C.jobject value.
	NullObject = Object(0)
	// NullClas represents a Class that holds a null C.jclass value.
	NullClass = Class(0)
	// NullRef represents a reference that points to nothing.
	NullRef = Ref(0)
)

func (e Env) value() *C.JNIEnv {
	return (*C.JNIEnv)(unsafe.Pointer(e))
}

// IsNull returns true iff the Object holds a null C.jobject value.
func (o Object) IsNull() bool {
	return o == NullObject
}

func (o Object) value() C.jobject {
	return C.jobject(unsafe.Pointer(o))
}

// IsNull returns true iff the Object holds a null C.jobject value.
func (c Class) IsNull() bool {
	return c == NullClass
}

func (c Class) value() C.jclass {
	return C.jclass(unsafe.Pointer(c))
}
