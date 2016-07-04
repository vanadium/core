// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"unsafe"
)

// Tests of TypeFromReflect success.
type rtTest struct {
	rt reflect.Type
	t  *Type
}

// rtKeyTests contains types that may be used as map keys.
var rtKeyTests = []rtTest{
	// Unnamed scalars
	{reflect.TypeOf(bool(false)), BoolType},
	{reflect.TypeOf(byte(0)), ByteType},
	{reflect.TypeOf(uint16(0)), Uint16Type},
	{reflect.TypeOf(uint32(0)), Uint32Type},
	{reflect.TypeOf(uint64(0)), Uint64Type},
	{reflect.TypeOf(uint(0)), testUintType()},
	{reflect.TypeOf(uintptr(0)), testUintptrType()},
	{reflect.TypeOf(int8(0)), Int8Type},
	{reflect.TypeOf(int16(0)), Int16Type},
	{reflect.TypeOf(int32(0)), Int32Type},
	{reflect.TypeOf(int64(0)), Int64Type},
	{reflect.TypeOf(int(0)), testIntType()},
	{reflect.TypeOf(float32(0)), Float32Type},
	{reflect.TypeOf(float64(0)), Float64Type},
	{reflect.TypeOf(string("")), StringType},
	// Named scalars
	{reflect.TypeOf(NBool(false)), NameN("Bool", BoolType)},
	{reflect.TypeOf(NByte(0)), NameN("Byte", ByteType)},
	{reflect.TypeOf(NUint16(0)), NameN("Uint16", Uint16Type)},
	{reflect.TypeOf(NUint32(0)), NameN("Uint32", Uint32Type)},
	{reflect.TypeOf(NUint64(0)), NameN("Uint64", Uint64Type)},
	{reflect.TypeOf(NUint(0)), NameN("Uint", testUintType())},
	{reflect.TypeOf(NUintptr(0)), NameN("Uintptr", testUintptrType())},
	{reflect.TypeOf(NInt8(0)), NameN("Int8", Int8Type)},
	{reflect.TypeOf(NInt16(0)), NameN("Int16", Int16Type)},
	{reflect.TypeOf(NInt32(0)), NameN("Int32", Int32Type)},
	{reflect.TypeOf(NInt64(0)), NameN("Int64", Int64Type)},
	{reflect.TypeOf(NInt(0)), NameN("Int", testIntType())},
	{reflect.TypeOf(NFloat32(0)), NameN("Float32", Float32Type)},
	{reflect.TypeOf(NFloat64(0)), NameN("Float64", Float64Type)},
	{reflect.TypeOf(NString("")), NameN("String", StringType)},
	// Unnamed arrays
	{reflect.TypeOf([3]bool{}), ArrayType(3, BoolType)},
	{reflect.TypeOf([3]byte{}), ArrayType(3, ByteType)},
	{reflect.TypeOf([3]uint16{}), ArrayType(3, Uint16Type)},
	{reflect.TypeOf([3]uint32{}), ArrayType(3, Uint32Type)},
	{reflect.TypeOf([3]uint64{}), ArrayType(3, Uint64Type)},
	{reflect.TypeOf([3]uint{}), ArrayType(3, testUintType())},
	{reflect.TypeOf([3]uintptr{}), ArrayType(3, testUintptrType())},
	{reflect.TypeOf([3]int8{}), ArrayType(3, Int8Type)},
	{reflect.TypeOf([3]int16{}), ArrayType(3, Int16Type)},
	{reflect.TypeOf([3]int32{}), ArrayType(3, Int32Type)},
	{reflect.TypeOf([3]int64{}), ArrayType(3, Int64Type)},
	{reflect.TypeOf([3]int{}), ArrayType(3, testIntType())},
	{reflect.TypeOf([3]float32{}), ArrayType(3, Float32Type)},
	{reflect.TypeOf([3]float64{}), ArrayType(3, Float64Type)},
	{reflect.TypeOf([3]string{}), ArrayType(3, StringType)},
	// Named arrays
	{reflect.TypeOf(NArray3Bool{}), NameNArray("Bool", BoolType)},
	{reflect.TypeOf(NArray3Byte{}), NameNArray("Byte", ByteType)},
	{reflect.TypeOf(NArray3Uint16{}), NameNArray("Uint16", Uint16Type)},
	{reflect.TypeOf(NArray3Uint32{}), NameNArray("Uint32", Uint32Type)},
	{reflect.TypeOf(NArray3Uint64{}), NameNArray("Uint64", Uint64Type)},
	{reflect.TypeOf(NArray3Uint{}), NameNArray("Uint", testUintType())},
	{reflect.TypeOf(NArray3Uintptr{}), NameNArray("Uintptr", testUintptrType())},
	{reflect.TypeOf(NArray3Int8{}), NameNArray("Int8", Int8Type)},
	{reflect.TypeOf(NArray3Int16{}), NameNArray("Int16", Int16Type)},
	{reflect.TypeOf(NArray3Int32{}), NameNArray("Int32", Int32Type)},
	{reflect.TypeOf(NArray3Int64{}), NameNArray("Int64", Int64Type)},
	{reflect.TypeOf(NArray3Int{}), NameNArray("Int", testIntType())},
	{reflect.TypeOf(NArray3Float32{}), NameNArray("Float32", Float32Type)},
	{reflect.TypeOf(NArray3Float64{}), NameNArray("Float64", Float64Type)},
	{reflect.TypeOf(NArray3String{}), NameNArray("String", StringType)},
	// Unnamed structs
	{reflect.TypeOf(struct{ X bool }{}), StructType(Field{"X", BoolType})},
	{reflect.TypeOf(struct{ X byte }{}), StructType(Field{"X", ByteType})},
	{reflect.TypeOf(struct{ X uint16 }{}), StructType(Field{"X", Uint16Type})},
	{reflect.TypeOf(struct{ X uint32 }{}), StructType(Field{"X", Uint32Type})},
	{reflect.TypeOf(struct{ X uint64 }{}), StructType(Field{"X", Uint64Type})},
	{reflect.TypeOf(struct{ X uint }{}), StructType(Field{"X", testUintType()})},
	{reflect.TypeOf(struct{ X uintptr }{}), StructType(Field{"X", testUintptrType()})},
	{reflect.TypeOf(struct{ X int8 }{}), StructType(Field{"X", Int8Type})},
	{reflect.TypeOf(struct{ X int16 }{}), StructType(Field{"X", Int16Type})},
	{reflect.TypeOf(struct{ X int32 }{}), StructType(Field{"X", Int32Type})},
	{reflect.TypeOf(struct{ X int64 }{}), StructType(Field{"X", Int64Type})},
	{reflect.TypeOf(struct{ X int }{}), StructType(Field{"X", testIntType()})},
	{reflect.TypeOf(struct{ X float32 }{}), StructType(Field{"X", Float32Type})},
	{reflect.TypeOf(struct{ X float64 }{}), StructType(Field{"X", Float64Type})},
	{reflect.TypeOf(struct{ X string }{}), StructType(Field{"X", StringType})},
	// Named structs
	{reflect.TypeOf(NStructBool{}), NameNStruct("Bool", BoolType)},
	{reflect.TypeOf(NStructByte{}), NameNStruct("Byte", ByteType)},
	{reflect.TypeOf(NStructUint16{}), NameNStruct("Uint16", Uint16Type)},
	{reflect.TypeOf(NStructUint32{}), NameNStruct("Uint32", Uint32Type)},
	{reflect.TypeOf(NStructUint64{}), NameNStruct("Uint64", Uint64Type)},
	{reflect.TypeOf(NStructUint{}), NameNStruct("Uint", testUintType())},
	{reflect.TypeOf(NStructUintptr{}), NameNStruct("Uintptr", testUintptrType())},
	{reflect.TypeOf(NStructInt8{}), NameNStruct("Int8", Int8Type)},
	{reflect.TypeOf(NStructInt16{}), NameNStruct("Int16", Int16Type)},
	{reflect.TypeOf(NStructInt32{}), NameNStruct("Int32", Int32Type)},
	{reflect.TypeOf(NStructInt64{}), NameNStruct("Int64", Int64Type)},
	{reflect.TypeOf(NStructInt{}), NameNStruct("Int", testIntType())},
	{reflect.TypeOf(NStructFloat32{}), NameNStruct("Float32", Float32Type)},
	{reflect.TypeOf(NStructFloat64{}), NameNStruct("Float64", Float64Type)},
	{reflect.TypeOf(NStructString{}), NameNStruct("String", StringType)},
	// Special-case types
	{reflect.TypeOf(NEnum(0)), NameN("Enum", EnumType("A", "B", "C", "ABC"))},
	{reflect.TypeOf((*NUnionABC)(nil)).Elem(), UnionABCTypeN},
	{reflect.TypeOf(NUnionABCA{}), UnionABCTypeN},
	{reflect.TypeOf(NUnionABCB{}), UnionABCTypeN},
	{reflect.TypeOf(NUnionABCC{}), UnionABCTypeN},
	{reflect.TypeOf(NNative(0)), WireTypeN},
	{reflect.TypeOf(NWire{}), WireTypeN},
}

var (
	nonPtrError error = NonPtrError{}
	ptrError    error = &PtrError{}
)

// rtNonKeyTests contains types that may not be used as map keys.
var rtNonKeyTests = []rtTest{
	// Unnamed scalars
	{reflect.Type(nil), AnyType},
	{reflect.TypeOf((*interface{})(nil)), AnyType},
	{reflect.TypeOf((*interface{})(nil)).Elem(), AnyType},
	{reflect.TypeOf((*error)(nil)), ErrorType},
	{reflect.TypeOf((*error)(nil)).Elem(), ErrorType},
	{reflect.TypeOf(&nonPtrError), ErrorType},
	{reflect.TypeOf(&ptrError), ErrorType},
	{reflect.TypeOf((*Type)(nil)), TypeObjectType},
	// Named scalars (we cannot detect the error type if it is named)
	{reflect.TypeOf((*NInterface)(nil)), AnyType},
	{reflect.TypeOf((*NInterface)(nil)).Elem(), AnyType},
	// Unnamed arrays
	{reflect.TypeOf([3]interface{}{}), ArrayType(3, AnyType)},
	{reflect.TypeOf([3]error{}), ArrayType(3, ErrorType)},
	{reflect.TypeOf([3]*Type{}), ArrayType(3, TypeObjectType)},
	// Named arrays
	{reflect.TypeOf(NArray3Interface{}), NameNArray("Interface", AnyType)},
	{reflect.TypeOf(NArray3TypeObject{}), NameNArray("TypeObject", TypeObjectType)},
	// Unnamed structs
	{reflect.TypeOf(struct{ X interface{} }{}), StructType(Field{"X", AnyType})},
	{reflect.TypeOf(struct{ X error }{}), StructType(Field{"X", ErrorType})},
	{reflect.TypeOf(struct{ X *Type }{}), StructType(Field{"X", TypeObjectType})},
	// Named structs
	{reflect.TypeOf(NStructInterface{}), NameNStruct("Interface", AnyType)},
	{reflect.TypeOf(NStructTypeObject{}), NameNStruct("TypeObject", TypeObjectType)},
	// Unnamed slices
	{reflect.TypeOf([]interface{}{}), ListType(AnyType)},
	{reflect.TypeOf([]error{}), ListType(ErrorType)},
	{reflect.TypeOf([]*Type{}), ListType(TypeObjectType)},
	{reflect.TypeOf([]bool{}), ListType(BoolType)},
	{reflect.TypeOf([]byte{}), ListType(ByteType)},
	{reflect.TypeOf([]uint16{}), ListType(Uint16Type)},
	{reflect.TypeOf([]uint32{}), ListType(Uint32Type)},
	{reflect.TypeOf([]uint64{}), ListType(Uint64Type)},
	{reflect.TypeOf([]uint{}), ListType(testUintType())},
	{reflect.TypeOf([]uintptr{}), ListType(testUintptrType())},
	{reflect.TypeOf([]int8{}), ListType(Int8Type)},
	{reflect.TypeOf([]int16{}), ListType(Int16Type)},
	{reflect.TypeOf([]int32{}), ListType(Int32Type)},
	{reflect.TypeOf([]int64{}), ListType(Int64Type)},
	{reflect.TypeOf([]int{}), ListType(testIntType())},
	{reflect.TypeOf([]float32{}), ListType(Float32Type)},
	{reflect.TypeOf([]float64{}), ListType(Float64Type)},
	{reflect.TypeOf([]string{}), ListType(StringType)},
	// Named slices
	{reflect.TypeOf(NSliceInterface{}), NameNSlice("Interface", AnyType)},
	{reflect.TypeOf(NSliceTypeObject{}), NameNSlice("TypeObject", TypeObjectType)},
	{reflect.TypeOf(NSliceBool{}), NameNSlice("Bool", BoolType)},
	{reflect.TypeOf(NSliceByte{}), NameNSlice("Byte", ByteType)},
	{reflect.TypeOf(NSliceUint16{}), NameNSlice("Uint16", Uint16Type)},
	{reflect.TypeOf(NSliceUint32{}), NameNSlice("Uint32", Uint32Type)},
	{reflect.TypeOf(NSliceUint64{}), NameNSlice("Uint64", Uint64Type)},
	{reflect.TypeOf(NSliceUint{}), NameNSlice("Uint", testUintType())},
	{reflect.TypeOf(NSliceUintptr{}), NameNSlice("Uintptr", testUintptrType())},
	{reflect.TypeOf(NSliceInt8{}), NameNSlice("Int8", Int8Type)},
	{reflect.TypeOf(NSliceInt16{}), NameNSlice("Int16", Int16Type)},
	{reflect.TypeOf(NSliceInt32{}), NameNSlice("Int32", Int32Type)},
	{reflect.TypeOf(NSliceInt64{}), NameNSlice("Int64", Int64Type)},
	{reflect.TypeOf(NSliceInt{}), NameNSlice("Int", testIntType())},
	{reflect.TypeOf(NSliceFloat32{}), NameNSlice("Float32", Float32Type)},
	{reflect.TypeOf(NSliceFloat64{}), NameNSlice("Float64", Float64Type)},
	{reflect.TypeOf(NSliceString{}), NameNSlice("String", StringType)},
	// Unnamed sets
	{reflect.TypeOf(map[bool]struct{}{}), rtSet(BoolType)},
	{reflect.TypeOf(map[byte]struct{}{}), rtSet(ByteType)},
	{reflect.TypeOf(map[uint16]struct{}{}), rtSet(Uint16Type)},
	{reflect.TypeOf(map[uint32]struct{}{}), rtSet(Uint32Type)},
	{reflect.TypeOf(map[uint64]struct{}{}), rtSet(Uint64Type)},
	{reflect.TypeOf(map[uint]struct{}{}), rtSet(testUintType())},
	{reflect.TypeOf(map[uintptr]struct{}{}), rtSet(testUintptrType())},
	{reflect.TypeOf(map[int8]struct{}{}), rtSet(Int8Type)},
	{reflect.TypeOf(map[int16]struct{}{}), rtSet(Int16Type)},
	{reflect.TypeOf(map[int32]struct{}{}), rtSet(Int32Type)},
	{reflect.TypeOf(map[int64]struct{}{}), rtSet(Int64Type)},
	{reflect.TypeOf(map[int]struct{}{}), rtSet(testIntType())},
	{reflect.TypeOf(map[float32]struct{}{}), rtSet(Float32Type)},
	{reflect.TypeOf(map[float64]struct{}{}), rtSet(Float64Type)},
	{reflect.TypeOf(map[string]struct{}{}), rtSet(StringType)},
	// Named sets
	{reflect.TypeOf(NSetBool{}), NameNSet("Bool", BoolType)},
	{reflect.TypeOf(NSetByte{}), NameNSet("Byte", ByteType)},
	{reflect.TypeOf(NSetUint16{}), NameNSet("Uint16", Uint16Type)},
	{reflect.TypeOf(NSetUint32{}), NameNSet("Uint32", Uint32Type)},
	{reflect.TypeOf(NSetUint64{}), NameNSet("Uint64", Uint64Type)},
	{reflect.TypeOf(NSetUint{}), NameNSet("Uint", testUintType())},
	{reflect.TypeOf(NSetUintptr{}), NameNSet("Uintptr", testUintptrType())},
	{reflect.TypeOf(NSetInt8{}), NameNSet("Int8", Int8Type)},
	{reflect.TypeOf(NSetInt16{}), NameNSet("Int16", Int16Type)},
	{reflect.TypeOf(NSetInt32{}), NameNSet("Int32", Int32Type)},
	{reflect.TypeOf(NSetInt64{}), NameNSet("Int64", Int64Type)},
	{reflect.TypeOf(NSetInt{}), NameNSet("Int", testIntType())},
	{reflect.TypeOf(NSetFloat32{}), NameNSet("Float32", Float32Type)},
	{reflect.TypeOf(NSetFloat64{}), NameNSet("Float64", Float64Type)},
	{reflect.TypeOf(NSetString{}), NameNSet("String", StringType)},
	// Unnamed maps
	{reflect.TypeOf(map[bool]bool{}), rtMap(BoolType)},
	{reflect.TypeOf(map[byte]byte{}), rtMap(ByteType)},
	{reflect.TypeOf(map[uint16]uint16{}), rtMap(Uint16Type)},
	{reflect.TypeOf(map[uint32]uint32{}), rtMap(Uint32Type)},
	{reflect.TypeOf(map[uint64]uint64{}), rtMap(Uint64Type)},
	{reflect.TypeOf(map[uint]uint{}), rtMap(testUintType())},
	{reflect.TypeOf(map[uintptr]uintptr{}), rtMap(testUintptrType())},
	{reflect.TypeOf(map[int8]int8{}), rtMap(Int8Type)},
	{reflect.TypeOf(map[int16]int16{}), rtMap(Int16Type)},
	{reflect.TypeOf(map[int32]int32{}), rtMap(Int32Type)},
	{reflect.TypeOf(map[int64]int64{}), rtMap(Int64Type)},
	{reflect.TypeOf(map[int]int{}), rtMap(testIntType())},
	{reflect.TypeOf(map[float32]float32{}), rtMap(Float32Type)},
	{reflect.TypeOf(map[float64]float64{}), rtMap(Float64Type)},
	{reflect.TypeOf(map[string]string{}), rtMap(StringType)},
	// Named maps
	{reflect.TypeOf(NMapBool{}), NameNMap("Bool", BoolType)},
	{reflect.TypeOf(NMapByte{}), NameNMap("Byte", ByteType)},
	{reflect.TypeOf(NMapUint16{}), NameNMap("Uint16", Uint16Type)},
	{reflect.TypeOf(NMapUint32{}), NameNMap("Uint32", Uint32Type)},
	{reflect.TypeOf(NMapUint64{}), NameNMap("Uint64", Uint64Type)},
	{reflect.TypeOf(NMapUint{}), NameNMap("Uint", testUintType())},
	{reflect.TypeOf(NMapUintptr{}), NameNMap("Uintptr", testUintptrType())},
	{reflect.TypeOf(NMapInt8{}), NameNMap("Int8", Int8Type)},
	{reflect.TypeOf(NMapInt16{}), NameNMap("Int16", Int16Type)},
	{reflect.TypeOf(NMapInt32{}), NameNMap("Int32", Int32Type)},
	{reflect.TypeOf(NMapInt64{}), NameNMap("Int64", Int64Type)},
	{reflect.TypeOf(NMapInt{}), NameNMap("Int", testIntType())},
	{reflect.TypeOf(NMapFloat32{}), NameNMap("Float32", Float32Type)},
	{reflect.TypeOf(NMapFloat64{}), NameNMap("Float64", Float64Type)},
	{reflect.TypeOf(NMapString{}), NameNMap("String", StringType)},
	// Recursive types
	{reflect.TypeOf(NRecurseSelf{}), RecurseSelfType()},
	{reflect.TypeOf(NRecurseA{}), RecurseAType()},
	{reflect.TypeOf(NRecurseB{}), RecurseBType()},
}

func testUintType() *Type {
	switch bitlen := 8 * unsafe.Sizeof(uint(0)); bitlen {
	case 32:
		return Uint32Type
	case 64:
		return Uint64Type
	default:
		panic(fmt.Errorf("testUintType unhandled bitlen %d", bitlen))
	}
}

func testUintptrType() *Type {
	switch bitlen := 8 * unsafe.Sizeof(uintptr(0)); bitlen {
	case 32:
		return Uint32Type
	case 64:
		return Uint64Type
	default:
		panic(fmt.Errorf("testUintptrType unhandled bitlen %d", bitlen))
	}
}

func testIntType() *Type {
	switch bitlen := 8 * unsafe.Sizeof(int(0)); bitlen {
	case 32:
		return Int32Type
	case 64:
		return Int64Type
	default:
		panic(fmt.Errorf("testIntType unhandled bitlen %d", bitlen))
	}
}

func allTests() []rtTest {
	// Start with all keys and non keys
	tests := make([]rtTest, len(rtKeyTests)+len(rtNonKeyTests))
	n := copy(tests, rtKeyTests)
	copy(tests[n:], rtNonKeyTests)
	// Add all types we can generate via reflect.
	for _, test := range rtKeyTests {
		if test.t.CanBeOptional() {
			tests = append(tests, rtTest{reflect.PtrTo(test.rt), OptionalType(test.t)})
		} else {
			tests = append(tests, rtTest{reflect.PtrTo(test.rt), test.t})
		}
		tests = append(tests, rtTest{reflect.SliceOf(test.rt), ListType(test.t)})
		tests = append(tests, rtTest{reflect.MapOf(test.rt, test.rt), MapType(test.t, test.t)})
	}
	// Now generate types from everything we have so far, for more complicated subtypes.
	for _, test := range tests {
		if test.rt == nil {
			continue
		}
		if test.t.CanBeOptional() {
			tests = append(tests, rtTest{reflect.PtrTo(test.rt), OptionalType(test.t)})
		} else {
			tests = append(tests, rtTest{reflect.PtrTo(test.rt), test.t})
		}
		tests = append(tests, rtTest{reflect.SliceOf(test.rt), ListType(test.t)})
		for _, key := range rtKeyTests {
			// Only generate maps with valid keys.
			tests = append(tests, rtTest{reflect.MapOf(key.rt, reflect.SliceOf(test.rt)), MapType(key.t, ListType(test.t))})
		}
	}
	return tests
}

func TestTypeFromReflect(t *testing.T) {
	// Make sure we can create all types without the cache.
	rtCacheEnabled = false
	testTypeFromReflect(t, "no cache")
	// Enable the cache, and make multiple goroutines update the same types
	// concurrently.  This should expose locking issues in the cache.
	rtCacheEnabled = true
	var done sync.WaitGroup
	for i := 0; i < 3; i++ {
		done.Add(1)
		go func(i int) {
			testTypeFromReflect(t, fmt.Sprintf("cache%d", i))
			done.Done()
		}(i)
	}
	done.Wait()
	// Final test with all types already cached.
	testTypeFromReflect(t, "all cached")
}

func testTypeFromReflect(t *testing.T, prefix string) {
	for _, test := range allTests() {
		got, err := TypeFromReflect(test.rt)
		ExpectErr(t, err, "", "%s TypeFromReflect(%v)", prefix, test.rt)
		if want := test.t; got != want {
			t.Errorf("%s TypeFromReflect(%v) got type %v, want %v", prefix, test.rt, got, want)
		}
		// Make sure that the type is automatically registered, if it is named.
		if test.rt != nil && test.t.Name() != "" {
			if rt := TypeToReflect(test.t); rt == nil {
				t.Errorf("%s TypeFromReflect(%v) got TypeToReflect nil, want non-nil", prefix, test.rt)
			}
		}
	}
}

var rtErrorTests = []rtErrorTest{
	{reflect.TypeOf(make(chan int64)), `type "chan int64" not supported`},
	{reflect.TypeOf(func() {}), `type "func()" not supported`},
	{reflect.TypeOf(unsafe.Pointer(nil)), `type "unsafe.Pointer" not supported`},
	{reflect.TypeOf(map[*int64]string{}), `invalid key "*int64" in "map[*int64]string"`},
	{reflect.TypeOf(struct{ a int64 }{}), `type "struct { a int64 }" only has unexported fields`},
}

func rtErrorTestsAll() []rtErrorTest {
	// Start with base error tests
	tests := make([]rtErrorTest, len(rtErrorTests))
	copy(tests, rtErrorTests)
	tests = append(tests, reflectInfoErrorTests...)
	// Add some types we can generate via reflect
	for _, test := range tests {
		if test.rt != nil {
			tests = append(tests, rtErrorTest{reflect.PtrTo(test.rt), test.errstr})
			tests = append(tests, rtErrorTest{reflect.SliceOf(test.rt), test.errstr})
		}
	}
	// Now generate types from everything we have so far, for more complicated subtypes.
	for _, test := range tests {
		if test.rt != nil {
			tests = append(tests, rtErrorTest{reflect.PtrTo(reflect.PtrTo(test.rt)), test.errstr})
			tests = append(tests, rtErrorTest{reflect.SliceOf(reflect.SliceOf(test.rt)), test.errstr})
		}
	}
	return tests
}

func TestTypeFromReflectError(t *testing.T) {
	for _, test := range rtErrorTestsAll() {
		got, err := TypeFromReflect(test.rt)
		ExpectErr(t, err, test.errstr, "TypeFromReflect(%v)", test.rt)
		if got != nil {
			t.Errorf("TypeFromReflect(%v) got type %v, want nil", test.rt, got)
		}
	}
}

type simpleNamedType struct {
	X int16
}

// BenchmarkRepeatedVdlTypeOf determines the time for a repeated
// vdl.TypeOf, i.e. skipping the initial vdl.Type of that builds
// the type.
func BenchmarkRepeatedVdlTypeOf(b *testing.B) {
	val := &simpleNamedType{}
	TypeOf(val)

	for i := 0; i < b.N; i++ {
		TypeOf(val)
	}
}

// BenchmarkRepeatedReflectTypeOf is the reflect analog of
// BenchmarkRepeatedVdlTypeOf, for comparison purposes.
func BenchmarkRepeatedReflectTypeOf(b *testing.B) {
	reflect.TypeOf(&simpleNamedType{})

	for i := 0; i < b.N; i++ {
		reflect.TypeOf(&simpleNamedType{})
	}
}
