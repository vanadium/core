// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflectutil

import (
	"reflect"
	"sort"
)

// AreComparable is a helper to call AreComparableTypes.
func AreComparable(a, b interface{}) bool {
	return AreComparableTypes(reflect.TypeOf(a), reflect.TypeOf(b))
}

// AreComparableTypes returns true iff a and b are comparable types: bools,
// strings and numbers, and composites using arrays, slices, structs or
// pointers.
func AreComparableTypes(a, b reflect.Type) bool {
	return areComparable(a, b, make(map[reflect.Type]bool))
}

func areComparable(a, b reflect.Type, seen map[reflect.Type]bool) bool { //nolint:gocyclo
	if a.Kind() != b.Kind() {
		if isUint(a) && isUint(b) || isInt(a) && isInt(b) || isFloat(a) && isFloat(b) || isComplex(a) && isComplex(b) {
			return true // Special-case for comparable numbers.
		}
		return false // Different kinds are incomparable.
	}

	// Deal with cyclic types.
	if seen[a] {
		return true
	}
	seen[a] = true

	switch a.Kind() {
	case reflect.Bool, reflect.String,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return true
	case reflect.Array, reflect.Slice, reflect.Ptr:
		return areComparable(a.Elem(), b.Elem(), seen)
	case reflect.Struct:
		if a.NumField() != b.NumField() {
			return false
		}
		for fx := 0; fx < a.NumField(); fx++ {
			af := a.Field(fx)
			bf := b.Field(fx)
			if af.Name != bf.Name || af.PkgPath != bf.PkgPath {
				return false
			}
			if !areComparable(af.Type, bf.Type, seen) {
				return false
			}
		}
		return true
	default:
		// Unhandled: Map, Interface, Chan, Func, UnsafePointer
		return false
	}
}

func isUint(rt reflect.Type) bool {
	switch rt.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return true
	}
	return false
}

func isInt(rt reflect.Type) bool {
	switch rt.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	}
	return false
}

func isFloat(rt reflect.Type) bool {
	switch rt.Kind() {
	case reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func isComplex(rt reflect.Type) bool {
	switch rt.Kind() {
	case reflect.Complex64, reflect.Complex128:
		return true
	}
	return false
}

// Less is a helper to call LessValues.
func Less(a, b interface{}) bool {
	return LessValues(reflect.ValueOf(a), reflect.ValueOf(b))
}

// LessValues returns true iff a and b are comparable and a < b.  If a and b are
// incomparable an arbitrary value is returned.  Cyclic values are not handled;
// if a and b are cyclic and equal, this will infinite loop.  Arrays, slices and
// structs use lexicographic ordering, and complex numbers compare real before
// imaginary.
func LessValues(a, b reflect.Value) bool { //nolint:gocyclo
	if a.Kind() != b.Kind() {
		return false // Different kinds are incomparable.
	}
	switch a.Kind() {
	case reflect.Bool:
		return lessBool(a.Bool(), b.Bool())
	case reflect.String:
		return a.String() < b.String()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return a.Uint() < b.Uint()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return a.Int() < b.Int()
	case reflect.Float32, reflect.Float64:
		return a.Float() < b.Float()
	case reflect.Complex64, reflect.Complex128:
		return lessComplex(a.Complex(), b.Complex())
	case reflect.Array:
		return compareArray(a, b) == -1
	case reflect.Slice:
		return compareSlice(a, b) == -1
	case reflect.Struct:
		return compareStruct(a, b) == -1
	case reflect.Ptr:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() && !b.IsNil() // nil is less than non-nil.
		}
		return LessValues(a.Elem(), b.Elem())
	default:
		return false
	}
}

func lessBool(a, b bool) bool {
	return !a && b // false < true
}

func lessComplex(a, b complex128) bool {
	// Compare lexicographically, real part before imaginary part.
	if real(a) == real(b) {
		return imag(a) < imag(b)
	}
	return real(a) < real(b)
}

// Compare is a helper to call CompareValues.
func Compare(a, b interface{}) int {
	return CompareValues(reflect.ValueOf(a), reflect.ValueOf(b))
}

// CompareValues returns an integer comparing two values.  If a and b are
// comparable, the result is 0 if a == b, -1 if a < b and +1 if a > b.  If a and
// b are incomparable an arbitrary value is returned.  Arrays, slices and
// structs use lexicographic ordering, and complex numbers compare real before
// imaginary.
func CompareValues(a, b reflect.Value) int {
	if a.Kind() != b.Kind() {
		return 0 // Different kinds are incomparable.
	}
	switch a.Kind() {
	case reflect.Array:
		return compareArray(a, b)
	case reflect.Slice:
		return compareSlice(a, b)
	case reflect.Struct:
		return compareStruct(a, b)
	case reflect.Ptr:
		if a.IsNil() || b.IsNil() {
			if a.IsNil() && !b.IsNil() {
				return -1
			}
			if !a.IsNil() && b.IsNil() {
				return +1
			}
			return 0
		}
		return CompareValues(a.Elem(), b.Elem())
	}
	if LessValues(a, b) {
		return -1 // a < b
	}
	if LessValues(b, a) {
		return +1 // a > b
	}
	return 0 // a == b, or incomparable.
}

func compareArray(a, b reflect.Value) int {
	// Return lexicographic ordering of the array elements.
	for ix := 0; ix < a.Len(); ix++ {
		if c := CompareValues(a.Index(ix), b.Index(ix)); c != 0 {
			return c
		}
	}
	return 0
}

func compareSlice(a, b reflect.Value) int {
	// Return lexicographic ordering of the slice elements.
	for ix := 0; ix < a.Len() && ix < b.Len(); ix++ {
		if c := CompareValues(a.Index(ix), b.Index(ix)); c != 0 {
			return c
		}
	}
	// Equal prefixes, shorter comes before longer.
	if a.Len() < b.Len() {
		return -1
	}
	if a.Len() > b.Len() {
		return +1
	}
	return 0
}

func compareStruct(a, b reflect.Value) int {
	// Return lexicographic ordering of the struct fields.
	for ix := 0; ix < a.NumField(); ix++ {
		if c := CompareValues(a.Field(ix), b.Field(ix)); c != 0 {
			return c
		}
	}
	return 0
}

// TrySortValues sorts a slice of reflect.Value if the value kind is supported.
// Supported kinds are bools, strings and numbers, and composites using arrays,
// slices, structs or pointers.  Arrays, slices and structs use lexicographic
// ordering, and complex numbers compare real before imaginary.  If the values
// in the slice aren't comparable or supported, the resulting ordering is
// arbitrary.
func TrySortValues(v []reflect.Value) []reflect.Value {
	if len(v) <= 1 {
		return v
	}
	switch v[0].Kind() {
	case reflect.Bool:
		sort.Sort(rvBools{v})
	case reflect.String:
		sort.Sort(rvStrings{v})
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		sort.Sort(rvUints{v})
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		sort.Sort(rvInts{v})
	case reflect.Float32, reflect.Float64:
		sort.Sort(rvFloats{v})
	case reflect.Complex64, reflect.Complex128:
		sort.Sort(rvComplexes{v})
	case reflect.Array:
		sort.Sort(rvArrays{v})
	case reflect.Slice:
		sort.Sort(rvSlices{v})
	case reflect.Struct:
		sort.Sort(rvStructs{v})
	case reflect.Ptr:
		sort.Sort(rvPtrs{v})
	}
	return v
}

// Sorting helpers, heavily inspired by similar code in text/template.

type rvs []reflect.Value

func (x rvs) Len() int      { return len(x) }
func (x rvs) Swap(i, j int) { x[i], x[j] = x[j], x[i] }

type rvBools struct{ rvs }
type rvStrings struct{ rvs }
type rvUints struct{ rvs }
type rvInts struct{ rvs }
type rvFloats struct{ rvs }
type rvComplexes struct{ rvs }
type rvArrays struct{ rvs }
type rvSlices struct{ rvs }
type rvStructs struct{ rvs }
type rvPtrs struct{ rvs }

func (x rvBools) Less(i, j int) bool {
	return lessBool(x.rvs[i].Bool(), x.rvs[j].Bool())
}
func (x rvStrings) Less(i, j int) bool {
	return x.rvs[i].String() < x.rvs[j].String()
}
func (x rvUints) Less(i, j int) bool {
	return x.rvs[i].Uint() < x.rvs[j].Uint()
}
func (x rvInts) Less(i, j int) bool {
	return x.rvs[i].Int() < x.rvs[j].Int()
}
func (x rvFloats) Less(i, j int) bool {
	return x.rvs[i].Float() < x.rvs[j].Float()
}
func (x rvComplexes) Less(i, j int) bool {
	return lessComplex(x.rvs[i].Complex(), x.rvs[j].Complex())
}
func (x rvArrays) Less(i, j int) bool {
	return compareArray(x.rvs[i], x.rvs[j]) == -1
}
func (x rvSlices) Less(i, j int) bool {
	return compareSlice(x.rvs[i], x.rvs[j]) == -1
}
func (x rvStructs) Less(i, j int) bool {
	return compareStruct(x.rvs[i], x.rvs[j]) == -1
}
func (x rvPtrs) Less(i, j int) bool {
	return LessValues(x.rvs[i], x.rvs[j])
}
