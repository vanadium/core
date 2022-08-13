// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"sync"
)

// RegisterNative registers conversion functions between a VDL wire type and a
// Go native type.  This is typically used when there is a more idiomatic native
// representation for a given wire type; e.g. the VDL standard Time type is
// converted into the Go standard time.Time.
//
// The signatures of the conversion functions is expected to be:
//
//	func ToNative(wire W, native *N) error
//	func FromNative(wire *W, native N) error
//
// The VDL conversion routines automatically apply these conversion functions,
// to avoid manual marshaling by the user.  The "dst" values (i.e. native in
// ToNative, and wire in FromNative) are guaranteed to be an allocated zero
// value of the respective type.
//
// As a special-case, RegisterNative is called by the verror package to
// register conversions between vdl.WireError and the standard Go error type.
// This is required for error conversions to work correctly.
//
// RegisterNative is not intended to be called by end users; calls are
// auto-generated for types with native conversions in *.vdl files.
func RegisterNative(toFn, fromFn interface{}) {
	ni, err := deriveNativeInfo(toFn, fromFn)
	if err != nil {
		panic(fmt.Errorf("vdl: RegisterNative invalid (%v)", err))
	}
	if err := niReg.addNativeInfo(ni); err != nil {
		panic(fmt.Errorf("vdl: RegisterNative invalid (%v)", err))
	}
}

// RegisterNativeIsZero registers the supplied native type as implementing
// vdl.NativeIsZero with the expected semantics. This is purely a performance
// optimization.
func RegisterNativeIsZero(native interface{}) {
	rt := reflect.TypeOf(native)
	if !perfReflectCache.implementsBuiltinInterface(perfReflectInfo{}, rt, rtNativeIsZeroBitMask) {
		panic(fmt.Errorf("vdl: RegisterNativeIsZero: %T type does not implement: %v", native, rtNativeIsZero))
	}
}

// RegisterNativeConverter registers a 'converter' that implements
// vdl.NativeConverer for converting to/from native types. This is a performance
// optimization since invoking the methods via the interface is much faster
// than doing so via reflect.Call. Typical usage is as follows. Note
// that the traditional native conversion functions and registrations are
// required to ensure valid conversions and this is purely a performance
// optimisation.
//
//	   vdl.RegisterNativeConverter(Time{}, nativeConverter{})
//
//	   type nativeConverter struct{}
//
//	   func (f nativeConverter) ToNative(wire, native interface{}) error {
//		    return TimeToNative(wire.(Time), native.(*time.Time))
//	   }
//
//	   func (f nativeConverter) FromNative(wire, native interface{}) error {
//	     return TimeFromNative(wire.(*Time), native.(time.Time))
//	   }
func RegisterNativeConverter(wireType interface{}, converter NativeConverter) {
	ni := nativeInfoFromWire(reflect.TypeOf(wireType))
	if ni == nil {
		panic(fmt.Errorf("vdl: RegisterNativeConverter: wire type %T is not registered", wireType))
	}
	ni.converter = converter
}

// RegisterNativeAnyType registers the native type to use for when
// decoding into an any type from a registered wire type. This is required
// when the native type associated with the specified wire type is an interface
// and hence there is ambiguity as to which concrete type to use when
// decoding into an any.
//
// As a special-case, RegisterNativeAnyType is called by the verror package to
// register conversions between vdl.WireError and the standard Go error type.
// This is required for error conversions to work correctly. It can also
// be used for any wire/native conversions that need to enforce a specific
// type when decoding into an any type.
//
// RegisterNativeAnyType is not intended to be called by end users; calls are
// auto-generated for types with native conversions in *.vdl files.
func RegisterNativeAnyType(wireType interface{}, anyType interface{}) {
	wt := reflect.TypeOf(wireType)
	niany := &nativeAnyTypeInfo{}
	niany.nativeAnyType = reflect.TypeOf(anyType)
	if niany.nativeAnyType.Kind() == reflect.Interface {
		panic(fmt.Errorf("%v cannot be an interface type", niany.nativeAnyType))
	}
	niany.nativeAnyElemType = niany.nativeAnyType.Elem()
	if err := niAnyReg.addNativeAnyType(wt, niany); err != nil {
		panic(fmt.Errorf("vdl: RegisterNativeAnyType invalid (%v)", err))
	}
}

// nativeType holds the mapping between a VDL wire type and a Go native type,
// along with functions to convert between values of these types.
type nativeInfo struct {
	WireType       reflect.Type  // Wire type from the conversion funcs.
	NativeType     reflect.Type  // Native type from the conversion funcs.
	toNativeFunc   reflect.Value // ToNative conversion func.
	fromNativeFunc reflect.Value // FromNative conversion func.
	converter      NativeConverter
	stack          []byte
}

// nativeAnyTypeInfo holds the mapping from a wire type to its any native
// Go type.
type nativeAnyTypeInfo struct {
	nativeAnyType     reflect.Type
	nativeAnyElemType reflect.Type
}

func (ni *nativeInfo) ToNative(wire, native reflect.Value) error {
	if ni.converter != nil {
		return ni.converter.ToNative(wire.Interface(), native.Interface())
	}
	callArray := [2]reflect.Value{wire, native}
	if ierr := ni.toNativeFunc.Call(callArray[:])[0].Interface(); ierr != nil {
		return ierr.(error)
	}
	return nil
}

func (ni *nativeInfo) FromNative(wire, native reflect.Value) error {
	if ni.converter != nil {
		return ni.converter.FromNative(wire.Interface(), native.Interface())
	}
	callArray := [2]reflect.Value{wire, native}
	if ierr := ni.fromNativeFunc.Call(callArray[:])[0].Interface(); ierr != nil {
		return ierr.(error)
	}
	return nil
}

// niRegistry holds the nativeInfo registry.  Unlike rtRegister (used for the
// rtCache), this information cannot be regenerated at will.  We expect a small
// number of native types to be registered within a single address space.
type niRegistry struct {
	sync.RWMutex
	fromWire   map[reflect.Type]*nativeInfo
	fromNative map[reflect.Type]*nativeInfo
}

type niAnyRegistry struct {
	sync.RWMutex
	fromWireAny map[string]*nativeAnyTypeInfo
}

var niReg = &niRegistry{
	fromWire:   make(map[reflect.Type]*nativeInfo),
	fromNative: make(map[reflect.Type]*nativeInfo),
}

var niAnyReg = &niAnyRegistry{
	fromWireAny: make(map[string]*nativeAnyTypeInfo),
}

func (reg *niRegistry) addNativeInfo(ni *nativeInfo) error {
	reg.Lock()
	defer reg.Unlock()
	// Require a bijective mapping.  Also reject chains of A <-> B <-> C..., which
	// would add unnecessary complexity to the already complex conversion logic.
	dup1 := reg.fromWire[ni.WireType]
	dup2 := reg.fromWire[ni.NativeType]
	dup3 := reg.fromNative[ni.WireType]
	dup4 := reg.fromNative[ni.NativeType]
	if dup1 != nil || dup2 != nil || dup3 != nil || dup4 != nil {
		return fmt.Errorf("non-bijective mapping, or chaining: %#v, stack: %s", ni, nonNilStack(dup1, dup2, dup3, dup4))
	}
	reg.fromWire[ni.WireType] = ni
	reg.fromNative[ni.NativeType] = ni
	return nil
}

func (reg *niAnyRegistry) addNativeAnyType(wireType reflect.Type, ninil *nativeAnyTypeInfo) error {
	reg.Lock()
	defer reg.Unlock()
	ri, err := TypeFromReflect(wireType)
	if err != nil {
		return err
	}
	name := ri.Name()
	if ri == nil || name == "" {
		return fmt.Errorf("%v is not registered as a wire type", wireType)
	}
	reg.fromWireAny[name] = ninil
	return nil
}

func nonNilStack(infos ...*nativeInfo) []byte {
	for _, ni := range infos {
		if ni != nil {
			return ni.stack
		}
	}
	return nil
}

// nativeInfoFromWire returns the nativeInfo for the given wire type, or nil if
// RegisterNative has not been called for the wire type.
func nativeInfoFromWire(wire reflect.Type) *nativeInfo {
	niReg.RLock()
	ni := niReg.fromWire[wire]
	niReg.RUnlock()
	return ni
}

func nativeAnyTypeFromWire(wire string) *nativeAnyTypeInfo {
	niAnyReg.RLock()
	ni := niAnyReg.fromWireAny[wire]
	niAnyReg.RUnlock()
	return ni
}

// nativeInfoFromNative returns the nativeInfo for the given native type, or nil
// if RegisterNative has not been called for the native type.
func nativeInfoFromNative(native reflect.Type) *nativeInfo {
	niReg.RLock()
	ni := niReg.fromNative[native]
	niReg.RUnlock()
	return ni
}

var errNoRegisterNativeError = errors.New(`vdl: RegisterNative must be called to register error<->WireError conversions.  Import the "v.io/v23/verror" package in your program`)

// deriveNativeInfo returns the nativeInfo corresponding to toFn and fromFn,
// which are expected to have the following signatures:
//
//	func ToNative(wire W, native *N) error
//	func FromNative(wire *W, native N) error
func deriveNativeInfo(toFn, fromFn interface{}) (*nativeInfo, error) { //nolint:gocyclo
	if toFn == nil || fromFn == nil {
		return nil, fmt.Errorf("nil arguments")
	}
	rvT, rvF := reflect.ValueOf(toFn), reflect.ValueOf(fromFn)
	rtT, rtF := rvT.Type(), rvF.Type()
	if rtT.Kind() != reflect.Func || rtF.Kind() != reflect.Func {
		return nil, fmt.Errorf("arguments must be functions")
	}
	if rtT.NumIn() != 2 || rtT.In(1).Kind() != reflect.Ptr || rtT.NumOut() != 1 || rtT.Out(0) != rtError {
		return nil, fmt.Errorf("toFn must have signature ToNative(wire W, native *N) error")
	}
	if rtF.NumIn() != 2 || rtF.In(0).Kind() != reflect.Ptr || rtF.NumOut() != 1 || rtF.Out(0) != rtError {
		return nil, fmt.Errorf("fromFn must have signature FromNative(wire *W, native N) error")
	}
	// Make sure the wire and native types match up between the two functions.
	rtWire, rtNative := rtT.In(0), rtT.In(1).Elem()
	if rtWire != rtF.In(0).Elem() || rtNative != rtF.In(1) {
		return nil, fmt.Errorf("mismatched wire/native types, want signatures ToNative(wire W, native *N) error, FromNative(wire *W, native N) error")
	}
	if rtWire == rtNative {
		return nil, fmt.Errorf("wire type == native type: %v", rtWire)
	}

	// Attach a stacktrace to the info, up to 1KB.
	stack := make([]byte, 1024)
	stack = stack[:runtime.Stack(stack, false)]
	return &nativeInfo{
		WireType:       rtWire,
		NativeType:     rtNative,
		toNativeFunc:   rvT,
		fromNativeFunc: rvF,
		stack:          stack}, nil
}
