// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflectutil

import (
	"fmt"
	"reflect"
)

// Equal is similar to reflect.DeepEqual, except that it also
// considers the sharing structure for pointers.  When reflect.DeepEqual
// encounters pointers it just compares the dereferenced values; we also keep
// track of the pointers themselves and require that if a pointer appears
// multiple places in a, it appears in the same places in b.
func DeepEqual(a, b interface{}, options *DeepEqualOpts) bool {
	return deepEqual(reflect.ValueOf(a), reflect.ValueOf(b), &orderInfo{}, options)
}

// TODO(bprosnitz) Implement the debuggable deep equal option.
// TODO(bprosnitz) Add an option to turn pointer sharing on/off

// DeepEqualOpts represents the options configuration for DeepEqual.
type DeepEqualOpts struct {
	SliceEqNilEmpty bool
}

// orderInfo tracks pointer ordering information.  As we encounter new pointers
// in our a and b values we maintain their ordering information in the slices.
// We use slices rather than maps for efficiency; we typically have a small
// number of pointers and sequential lookup is fast.
type orderInfo struct {
	orderA, orderB []uintptr
}

func lookupPtr(items []uintptr, target uintptr) (int, bool) { // (index, seen)
	for index, item := range items {
		if item == target {
			return index, true
		}
	}
	return -1, false
}

// sharingEqual returns equal=true iff the sharing structure between a and b is
// the same, and returns seen=true iff we've seen either a or b before.
func (info *orderInfo) sharingEqual(a, b uintptr) (bool, bool) { // (equal, seen)
	indexA, seenA := lookupPtr(info.orderA, a)
	indexB, seenB := lookupPtr(info.orderB, b)
	if seenA || seenB {
		return seenA == seenB && indexA == indexB, seenA || seenB
	}
	// Neither type has been seen - add to our order slices and return.
	info.orderA = append(info.orderA, a)
	info.orderB = append(info.orderB, b)
	return true, false
}

func deepEqual(a, b reflect.Value, info *orderInfo, options *DeepEqualOpts) bool { //nolint:gocyclo
	// We only consider sharing via explicit pointers, and ignore sharing via
	// slices, maps or pointers to internal data.
	if !a.IsValid() || !b.IsValid() {
		return a.IsValid() == b.IsValid()
	}
	if a.Type() != b.Type() {
		return false
	}
	switch a.Kind() {
	case reflect.Ptr:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() == b.IsNil()
		}
		equal, seen := info.sharingEqual(a.Pointer(), b.Pointer())
		if !equal {
			return false // a and b are not equal if their sharing isn't equal.
		}
		if seen {
			// Skip the deepEqual call if we've already seen the pointers and they're
			// equal, otherwise we'll have an infinite loop for cyclic values.
			return true
		}
		return deepEqual(a.Elem(), b.Elem(), info, options)
	case reflect.Array:
		if a.Len() != b.Len() {
			return false
		}
		for ix := 0; ix < a.Len(); ix++ {
			if !deepEqual(a.Index(ix), b.Index(ix), info, options) {
				return false
			}
		}
		return true
	case reflect.Slice:
		if !options.SliceEqNilEmpty {
			if a.IsNil() || b.IsNil() {
				return a.IsNil() == b.IsNil()
			}
		}
		if a.Len() != b.Len() {
			return false
		}
		for ix := 0; ix < a.Len(); ix++ {
			if !deepEqual(a.Index(ix), b.Index(ix), info, options) {
				return false
			}
		}
		return true
	case reflect.Map:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() == b.IsNil()
		}
		if a.Len() != b.Len() {
			return false
		}
		for _, key := range a.MapKeys() {
			if !deepEqual(a.MapIndex(key), b.MapIndex(key), info, options) {
				return false
			}
		}
		return true
	case reflect.Struct:
		for fx := 0; fx < a.NumField(); fx++ {
			if !deepEqual(a.Field(fx), b.Field(fx), info, options) {
				return false
			}
		}
		return true
	case reflect.Interface:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() == b.IsNil()
		}
		return deepEqual(a.Elem(), b.Elem(), info, options)

		// Ideally we would add a default clause here that would just return
		// a.Interface() == b.Interface(), but that panics if we're dealing with
		// unexported fields.  Instead we check each primitive type.

	case reflect.Bool:
		return a.Bool() == b.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return a.Int() == b.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return a.Uint() == b.Uint()
	case reflect.Float32, reflect.Float64:
		return a.Float() == b.Float()
	case reflect.Complex64, reflect.Complex128:
		return a.Complex() == b.Complex()
	case reflect.String:
		return a.String() == b.String()
	case reflect.UnsafePointer:
		return a.Pointer() == b.Pointer()
	default:
		panic(fmt.Errorf("SharingDeepEqual unhandled kind %v type %q", a.Kind(), a.Type()))
	}
}
