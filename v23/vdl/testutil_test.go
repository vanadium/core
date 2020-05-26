// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

// This file contains a collection of types, constants and functions used for
// testing.  All identifiers are exported, so they may be accessed via tests in
// the vdl_test package.  Note that since this is a *_test.go file, these
// identifiers are still only visible in tests.
//
// TODO(toddw): Merge with vdl/opconst/testutil_test.go

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

const (
	// These constants are the same as the ones defined in overflow.go.
	Float64MaxInt = (1 << 53)
	Float64MinInt = -(1 << 53)
	Float32MaxInt = (1 << 24)
	Float32MinInt = -(1 << 24)
)

// CallAndRecover calls the function f and returns the result of recover().
// This minimizes the scope of the deferred recover, to ensure f is actually the
// function that paniced.
func CallAndRecover(f func()) (result interface{}) {
	defer func() {
		result = recover()
	}()
	f()
	return
}

func ExpectErr(t *testing.T, err error, wantstr string, format string, args ...interface{}) bool {
	gotstr := fmt.Sprint(err)
	msg := fmt.Sprintf(format, args...)
	if wantstr != "" && !strings.Contains(gotstr, wantstr) {
		t.Errorf(`%s got error %q, want substr %q`, msg, gotstr, wantstr)
		return false
	}
	if wantstr == "" && err != nil {
		t.Errorf(`%s got error %q, want nil`, msg, gotstr)
		return false
	}
	return true
}

func ExpectPanic(t *testing.T, f func(), wantstr string, format string, args ...interface{}) {
	got := CallAndRecover(f)
	gotstr := fmt.Sprint(got)
	msg := fmt.Sprintf(format, args...)
	if wantstr != "" && !strings.Contains(gotstr, wantstr) {
		t.Errorf(`%s got panic %q, want substr %q`, msg, gotstr, wantstr)
	}
	if wantstr == "" && got != nil {
		t.Errorf(`%s got panic %q, want nil`, msg, gotstr)
	}
}

func ExpectMismatchedKind(t *testing.T, f func()) {
	ExpectPanic(t, f, "mismatched kind", "")
}

// Define a bunch of regular Go types used in tests.
type (
	// Scalars
	NInterface interface{}
	NBool      bool
	NByte      byte
	NUint16    uint16
	NUint32    uint32
	NUint64    uint64
	NUint      uint
	NUintptr   uintptr
	NInt8      int8
	NInt16     int16
	NInt32     int32
	NInt64     int64
	NInt       int
	NFloat32   float32
	NFloat64   float64
	NString    string
	// Arrays
	NArray3Interface  [3]NInterface
	NArray3TypeObject [3]*Type
	NArray3Bool       [3]bool
	NArray3Byte       [3]byte
	NArray3Uint16     [3]uint16
	NArray3Uint32     [3]uint32
	NArray3Uint64     [3]uint64
	NArray3Uint       [3]uint
	NArray3Uintptr    [3]uintptr
	NArray3Int8       [3]int8
	NArray3Int16      [3]int16
	NArray3Int32      [3]int32
	NArray3Int64      [3]int64
	NArray3Int        [3]int
	NArray3Float32    [3]float32
	NArray3Float64    [3]float64
	NArray3String     [3]string
	// Structs
	NStructInterface      struct{ X NInterface }
	NStructTypeObject     struct{ X *Type }
	NStructBool           struct{ X bool }
	NStructByte           struct{ X byte }
	NStructUint16         struct{ X uint16 }
	NStructUint32         struct{ X uint32 }
	NStructUint64         struct{ X uint64 }
	NStructUint           struct{ X uint }
	NStructUintptr        struct{ X uintptr }
	NStructInt8           struct{ X int8 }
	NStructInt16          struct{ X int16 }
	NStructInt32          struct{ X int32 }
	NStructInt64          struct{ X int64 }
	NStructInt            struct{ X int }
	NStructFloat32        struct{ X float32 }
	NStructFloat64        struct{ X float64 }
	NStructString         struct{ X string }
	NStructOptionalStruct struct{ X *NStructInt }
	NStructOptionalAny    struct{ X interface{} }
	// Slices
	NSliceInterface  []NInterface
	NSliceTypeObject []*Type
	NSliceBool       []bool
	NSliceByte       []byte
	NSliceUint16     []uint16
	NSliceUint32     []uint32
	NSliceUint64     []uint64
	NSliceUint       []uint
	NSliceUintptr    []uintptr
	NSliceInt8       []int8
	NSliceInt16      []int16
	NSliceInt32      []int32
	NSliceInt64      []int64
	NSliceInt        []int
	NSliceFloat32    []float32
	NSliceFloat64    []float64
	NSliceString     []string
	// Sets
	NSetInterface  map[NInterface]struct{} //nolint:deadcode,unused,varcheck
	NSetTypeObject map[*Type]struct{}      //nolint:deadcode,unused,varcheck
	NSetBool       map[bool]struct{}
	NSetByte       map[byte]struct{}
	NSetUint16     map[uint16]struct{}
	NSetUint32     map[uint32]struct{}
	NSetUint64     map[uint64]struct{}
	NSetUint       map[uint]struct{}
	NSetUintptr    map[uintptr]struct{}
	NSetInt8       map[int8]struct{}
	NSetInt16      map[int16]struct{}
	NSetInt32      map[int32]struct{}
	NSetInt64      map[int64]struct{}
	NSetInt        map[int]struct{}
	NSetFloat32    map[float32]struct{}
	NSetFloat64    map[float64]struct{}
	NSetString     map[string]struct{}
	// Maps
	NMapInterface  map[NInterface]NInterface //nolint:deadcode,unused,varcheck
	NMapTypeObject map[*Type]*Type           //nolint:deadcode,unused,varcheck
	NMapBool       map[bool]bool
	NMapByte       map[byte]byte
	NMapUint16     map[uint16]uint16
	NMapUint32     map[uint32]uint32
	NMapUint64     map[uint64]uint64
	NMapUint       map[uint]uint
	NMapUintptr    map[uintptr]uintptr
	NMapInt8       map[int8]int8
	NMapInt16      map[int16]int16
	NMapInt32      map[int32]int32
	NMapInt64      map[int64]int64
	NMapInt        map[int]int
	NMapFloat32    map[float32]float32
	NMapFloat64    map[float64]float64
	NMapString     map[string]string
	// Recursive types
	NRecurseSelf struct{ X []NRecurseSelf }
	NRecurseA    struct{ B []NRecurseB }
	NRecurseB    struct{ A []NRecurseA }

	// Composite types representing sets of numbers.
	NMapByteEmpty    map[NByte]struct{}
	NMapUint64Empty  map[NUint64]struct{}
	NMapInt64Empty   map[NUint64]struct{}
	NMapFloat64Empty map[NUint64]struct{}
	NMapByteBool     map[NByte]NBool
	NMapUint64Bool   map[NUint64]NBool
	NMapInt64Bool    map[NInt64]NBool
	NMapFloat64Bool  map[NFloat64]NBool
	// Composite types representing sets of strings.
	NMapStringEmpty          map[NString]struct{}
	NMapStringBool           map[NString]NBool
	NStructXYZBool           struct{ X, Y, Z NBool }
	NStructXYZBoolUnexported struct{ a, X, b, Y, c, Z, d NBool } //nolint:structcheck,unused
	NStructWXBool            struct{ W, X NBool }
	// Composite types representing maps of strings to numbers.
	NMapStringByte    map[NString]NByte
	NMapStringUint64  map[NString]NUint64
	NMapStringInt64   map[NString]NInt64
	NMapStringFloat64 map[NString]NFloat64
	NStructVWXByte    struct{ V, W, X NByte }
	NStructVWXUint64  struct{ V, W, X NUint64 }
	NStructVWXInt64   struct{ V, W, X NInt64 }
	NStructVWXFloat64 struct{ V, W, X NFloat64 }
	NStructVWXMixed   struct {
		// Interleave unexported fields, which are ignored.
		a bool //nolint:structcheck,unused
		V int64
		b string //nolint:structcheck,unused
		W float64
		X float32
		c []byte      //nolint:structcheck,unused
		d interface{} //nolint:structcheck,unused
	}
	NStructUVByte    struct{ U, V NByte }
	NStructUVUint64  struct{ U, V NUint64 }
	NStructUVInt64   struct{ U, V NInt64 }
	NStructUVFloat64 struct{ U, V NFloat64 }
	NStructUVMixed   struct {
		// Interleave unexported fields, which are ignored.
		a bool //nolint:structcheck,unused
		U int64
		b string //nolint:structcheck,unused
		V float64
		c []byte //nolint:structcheck,unused
	}
	// Types that cannot be converted to sets.  We represent sets as
	// map[key]struct{} on the Go side, but don't allow map[key]NEmpty.
	NEmpty           struct{}
	NMapStringNEmpty map[NString]NEmpty
	NStructXYZEmpty  struct{ X, Y, Z struct{} }
	NStructXYZNEmpty struct{ X, Y, Z NEmpty }
)

func RecurseSelfType() *Type {
	var builder TypeBuilder
	n := builder.Named("v.io/v23/vdl.NRecurseSelf")
	n.AssignBase(builder.Struct().AppendField("X", builder.List().AssignElem(n)))
	builder.Build()
	t, err := n.Built()
	if err != nil {
		panic(err)
	}
	return t
}

func RecurseABTypes() [2]*Type {
	var builder TypeBuilder
	a := builder.Named("v.io/v23/vdl.NRecurseA")
	b := builder.Named("v.io/v23/vdl.NRecurseB")
	a.AssignBase(builder.Struct().AppendField("B", builder.List().AssignElem(b)))
	b.AssignBase(builder.Struct().AppendField("A", builder.List().AssignElem(a)))
	builder.Build()
	aT, err := a.Built()
	if err != nil {
		panic(err)
	}
	bT, err := b.Built()
	if err != nil {
		panic(err)
	}
	return [2]*Type{aT, bT}
}

func RecurseAType() *Type { return RecurseABTypes()[0] }
func RecurseBType() *Type { return RecurseABTypes()[1] }

// Special case enum isn't regularly expressible in Go.
type NEnum int

const (
	NEnumA NEnum = iota
	NEnumB
	NEnumC
	NEnumABC
)

func (x *NEnum) Set(label string) error {
	switch label {
	case "A":
		*x = NEnumA
		return nil
	case "B":
		*x = NEnumB
		return nil
	case "C":
		*x = NEnumC
		return nil
	case "ABC":
		*x = NEnumABC
		return nil
	}
	*x = -1
	return fmt.Errorf("unknown label %q in NEnum", label)
}

func (x NEnum) String() string {
	switch x {
	case NEnumA:
		return "A"
	case NEnumB:
		return "B"
	case NEnumC:
		return "C"
	case NEnumABC:
		return "ABC"
	}
	return ""
}

func (NEnum) VDLReflect(struct{ Enum struct{ A, B, C, ABC string } }) {}

var EnumTypeN = NamedType("NEnum", EnumType("A", "B", "C", "ABC"))

// union{A bool;B string;C NStructInt64}
type (
	NUnionABC interface {
		Index() int
		Name() string
		VDLReflect(privateNUnionABCReflect)
	}
	NUnionABCA struct{ Value bool }
	NUnionABCB struct{ Value string }
	NUnionABCC struct{ Value NStructInt64 }

	privateNUnionABCReflect struct {
		Type  NUnionABC
		Union struct {
			A NUnionABCA
			B NUnionABCB
			C NUnionABCC
		}
	}
)

func (NUnionABCA) Name() string                       { return "A" }
func (NUnionABCA) Index() int                         { return 0 }
func (NUnionABCA) VDLReflect(privateNUnionABCReflect) {}
func (NUnionABCB) Name() string                       { return "B" }
func (NUnionABCB) Index() int                         { return 1 }
func (NUnionABCB) VDLReflect(privateNUnionABCReflect) {}
func (NUnionABCC) Name() string                       { return "C" }
func (NUnionABCC) Index() int                         { return 2 }
func (NUnionABCC) VDLReflect(privateNUnionABCReflect) {}

// union{B string;C NStructInt64;D int64}
type (
	NUnionBCD interface {
		Index() int
		Name() string
		VDLReflect(privateNUnionBCDDesc)
	}
	NUnionBCDB struct{ Value string }
	NUnionBCDC struct{ Value NStructInt64 }
	NUnionBCDD struct{ Value int64 }

	privateNUnionBCDDesc struct {
		Type  NUnionBCD
		Union struct {
			B NUnionBCDB
			C NUnionBCDC
			D NUnionBCDD
		}
	}
)

func (NUnionBCDB) Name() string                    { return "B" }
func (NUnionBCDB) Index() int                      { return 0 }
func (NUnionBCDB) VDLReflect(privateNUnionBCDDesc) {}
func (NUnionBCDC) Name() string                    { return "C" }
func (NUnionBCDC) Index() int                      { return 1 }
func (NUnionBCDC) VDLReflect(privateNUnionBCDDesc) {}
func (NUnionBCDD) Name() string                    { return "D" }
func (NUnionBCDD) Index() int                      { return 2 }
func (NUnionBCDD) VDLReflect(privateNUnionBCDDesc) {}

// Special-case error types
type NonPtrError struct{}
type PtrError struct{}

func (NonPtrError) Error() string { return "" }
func (*PtrError) Error() string   { return "" }

// NWire and NNative are used to test native type support.
type NWire struct{ Str string }
type NNative int64

func nWireToNative(x NWire, n *NNative) error {
	*n = 0
	i, err := strconv.ParseInt(x.Str, 10, 64)
	if err != nil {
		return err
	}
	*n = NNative(i)
	return nil
}

func nWireFromNative(x *NWire, n NNative) error {
	x.Str = strconv.FormatInt(int64(n), 10)
	return nil
}

func init() {
	RegisterNative(nWireToNative, nWireFromNative)
	Register(NWire{})
}

// NUnionWire and NUnionNative are used to test native type support for unions.
type (
	NUnionWire interface {
		Index() int
		Interface() interface{}
		Name() string
		VDLReflect(privateNUnionWireReflect)
	}
	NUnionWireA struct{ Value bool }
	NUnionWireB struct{ Value int64 }

	privateNUnionWireReflect struct {
		Type  NUnionWire
		Union struct {
			A NUnionWireA
			B NUnionWireB
		}
	}

	NUnionNative string
)

func (x NUnionWireA) Name() string                        { return "A" }
func (x NUnionWireA) Interface() interface{}              { return x.Value }
func (x NUnionWireA) Index() int                          { return 0 }
func (x NUnionWireA) VDLReflect(privateNUnionWireReflect) {}
func (x NUnionWireB) Name() string                        { return "B" }
func (x NUnionWireB) Interface() interface{}              { return x.Value }
func (x NUnionWireB) Index() int                          { return 1 }
func (x NUnionWireB) VDLReflect(privateNUnionWireReflect) {}

func nUnionWireToNative(w NUnionWire, n *NUnionNative) error {
	*n = NUnionNative(fmt.Sprintf("%s=%v", w.Name(), w.Interface()))
	return nil
}

func nUnionWireFromNative(w *NUnionWire, n NUnionNative) error {
	kv := strings.Split(string(n), "=")
	if len(kv) != 2 {
		return fmt.Errorf("invalid NUnionNative, no '=': %v", n)
	}
	switch kv[0] {
	case "A":
		var value bool
		if _, err := fmt.Sscan(kv[1], &value); err != nil {
			return err
		}
		*w = NUnionWireA{value}
		return nil
	case "B":
		var value int64
		if _, err := fmt.Sscan(kv[1], &value); err != nil {
			return err
		}
		*w = NUnionWireB{value}
		return nil
	}
	return fmt.Errorf("invalid NUnionNative, unknown key: %v", n)
}

func init() {
	RegisterNative(nUnionWireToNative, nUnionWireFromNative)
	Register((*NUnionWire)(nil))
}

var (
	StructInt64TypeN = NamedType("v.io/v23/vdl.NStructInt64", StructType(Field{"X", Int64Type}))
	UnionABCTypeN    = NamedType("v.io/v23/vdl.NUnionABC", UnionType([]Field{{"A", BoolType}, {"B", StringType}, {"C", StructInt64TypeN}}...))
	UnionBCDTypeN    = NamedType("v.io/v23/vdl.NUnionBCD", UnionType([]Field{{"B", StringType}, {"C", StructInt64TypeN}, {"D", Int64Type}}...))
	UnionXYTypeN     = NamedType("v.io/v23/vdl.NUnionXY", UnionType([]Field{{"X", StringType}, {"Y", StructInt64TypeN}}...))
)

// Define a bunch of *Type types used in tests.
var (
	// Named scalar types
	BoolTypeN    = NamedType("NBool", BoolType)
	ByteTypeN    = NamedType("NByte", ByteType)
	Uint16TypeN  = NamedType("NUint16", Uint16Type)
	Uint32TypeN  = NamedType("NUint32", Uint32Type)
	Uint64TypeN  = NamedType("NUint64", Uint64Type)
	Int8TypeN    = NamedType("NInt8", Int8Type)
	Int16TypeN   = NamedType("NInt16", Int16Type)
	Int32TypeN   = NamedType("NInt32", Int32Type)
	Int64TypeN   = NamedType("NInt64", Int64Type)
	Float32TypeN = NamedType("NFloat32", Float32Type)
	Float64TypeN = NamedType("NFloat64", Float64Type)
	StringTypeN  = NamedType("NString", StringType)

	// Composite types representing sequences of numbers.
	Array3ByteType     = ArrayType(3, ByteType)
	Array3ByteTypeN    = NamedType("NArray3Byte", ArrayType(3, ByteTypeN))
	Array3Uint64Type   = ArrayType(3, Uint64Type)
	Array3Uint64TypeN  = NamedType("NArray3Uint64", ArrayType(3, Uint64TypeN))
	Array3Int64Type    = ArrayType(3, Int64Type)
	Array3Int64TypeN   = NamedType("NArray3Int64", ArrayType(3, Int64TypeN))
	Array3Float64Type  = ArrayType(3, Float64Type)
	Array3Float64TypeN = NamedType("NArray3Float64", ArrayType(3, Float64TypeN))
	ListByteType       = ListType(ByteType)
	ListByteTypeN      = NamedType("NListByte", ListType(ByteTypeN))
	ListUint64Type     = ListType(Uint64Type)
	ListUint64TypeN    = NamedType("NListUint64", ListType(Uint64TypeN))
	ListInt64Type      = ListType(Int64Type)
	ListInt64TypeN     = NamedType("NListInt64", ListType(Int64TypeN))
	ListFloat64Type    = ListType(Float64Type)
	ListFloat64TypeN   = NamedType("NListFloat64", ListType(Float64TypeN))
	// Composite types representing sets of numbers.
	SetByteType         = SetType(ByteType)
	SetByteTypeN        = NamedType("NSetByte", SetType(ByteTypeN))
	SetUint64Type       = SetType(Uint64Type)
	SetUint64TypeN      = NamedType("NSetUint64", SetType(Uint64TypeN))
	SetInt64Type        = SetType(Int64Type)
	SetInt64TypeN       = NamedType("NSetInt64", SetType(Int64TypeN))
	SetFloat64Type      = SetType(Float64Type)
	SetFloat64TypeN     = NamedType("NSetFloat64", SetType(Float64TypeN))
	MapByteBoolType     = MapType(ByteType, BoolType)
	MapByteBoolTypeN    = NamedType("NMapByteBool", MapType(ByteTypeN, BoolTypeN))
	MapUint64BoolType   = MapType(Uint64Type, BoolType)
	MapUint64BoolTypeN  = NamedType("NMapUint64Bool", MapType(Uint64TypeN, BoolTypeN))
	MapInt64BoolType    = MapType(Int64Type, BoolType)
	MapInt64BoolTypeN   = NamedType("NMapInt64Bool", MapType(Int64TypeN, BoolTypeN))
	MapFloat64BoolType  = MapType(Float64Type, BoolType)
	MapFloat64BoolTypeN = NamedType("NMapFloat64Bool", MapType(Float64TypeN, BoolTypeN))
	// Composite types representing sets of strings.
	SetStringType      = SetType(StringType)
	SetStringTypeN     = NamedType("NSetString", SetType(StringTypeN))
	MapStringBoolType  = MapType(StringType, BoolType)
	MapStringBoolTypeN = NamedType("NMapStringBool", MapType(StringTypeN, BoolTypeN))
	StructXYZBoolType  = StructType(Field{"X", BoolType}, Field{"Y", BoolType}, Field{"Z", BoolType})
	StructXYZBoolTypeN = NamedType("NStructXYZBool", StructType(Field{"X", BoolTypeN}, Field{"Y", BoolTypeN}, Field{"Z", BoolTypeN}))
	StructWXBoolType   = StructType(Field{"W", BoolType}, Field{"X", BoolType})
	StructWXBoolTypeN  = NamedType("NStructWXBool", StructType(Field{"W", BoolTypeN}, Field{"X", BoolTypeN}))
	// Composite types representing maps of strings to numbers.
	MapStringByteType     = MapType(StringType, ByteType)
	MapStringByteTypeN    = NamedType("NMapStringByte", MapType(StringTypeN, ByteTypeN))
	MapStringUint64Type   = MapType(StringType, Uint64Type)
	MapStringUint64TypeN  = NamedType("NMapStringUint64", MapType(StringTypeN, Uint64TypeN))
	MapStringInt64Type    = MapType(StringType, Int64Type)
	MapStringInt64TypeN   = NamedType("NMapStringInt64", MapType(StringTypeN, Int64TypeN))
	MapStringFloat64Type  = MapType(StringType, Float64Type)
	MapStringFloat64TypeN = NamedType("NMapStringFloat64", MapType(StringTypeN, Float64TypeN))
	StructVWXByteType     = StructType(Field{"V", ByteType}, Field{"W", ByteType}, Field{"X", ByteType})
	StructVWXByteTypeN    = NamedType("NStructVWXByte", StructType(Field{"V", ByteTypeN}, Field{"W", ByteTypeN}, Field{"X", ByteTypeN}))
	StructVWXUint64Type   = StructType(Field{"V", Uint64Type}, Field{"W", Uint64Type}, Field{"X", Uint64Type})
	StructVWXUint64TypeN  = NamedType("NStructVWXUint64", StructType(Field{"V", Uint64TypeN}, Field{"W", Uint64TypeN}, Field{"X", Uint64TypeN}))
	StructVWXInt64Type    = StructType(Field{"V", Int64Type}, Field{"W", Int64Type}, Field{"X", Int64Type})
	StructVWXInt64TypeN   = NamedType("NStructVWXInt64", StructType(Field{"V", Int64TypeN}, Field{"W", Int64TypeN}, Field{"X", Int64TypeN}))
	StructVWXFloat64Type  = StructType(Field{"V", Float64Type}, Field{"W", Float64Type}, Field{"X", Float64Type})
	StructVWXFloat64TypeN = NamedType("NStructVWXFloat64", StructType(Field{"V", Float64TypeN}, Field{"W", Float64TypeN}, Field{"X", Float64TypeN}))
	StructUVByteType      = StructType(Field{"U", ByteType}, Field{"V", ByteType})
	StructUVByteTypeN     = NamedType("NStructUVByte", StructType(Field{"U", ByteTypeN}, Field{"V", ByteTypeN}))
	StructUVUint64Type    = StructType(Field{"U", Uint64Type}, Field{"V", Uint64Type})
	StructUVUint64TypeN   = NamedType("NStructUVUint64", StructType(Field{"U", Uint64TypeN}, Field{"V", Uint64TypeN}))
	StructUVInt64Type     = StructType(Field{"U", Int64Type}, Field{"V", Int64Type})
	StructUVInt64TypeN    = NamedType("NStructUVInt64", StructType(Field{"U", Int64TypeN}, Field{"V", Int64TypeN}))
	StructUVFloat64Type   = StructType(Field{"U", Float64Type}, Field{"V", Float64Type})
	StructUVFloat64TypeN  = NamedType("NStructUVFloat64", StructType(Field{"U", Float64TypeN}, Field{"V", Float64TypeN}))

	//nolint:deadcode,unused,varcheck
	StructAIntType = StructType(Field{"A", Int64Type})

	WireTypeN = NameN("Wire", StructType(Field{"Str", StringType}))

	// Types that cannot be converted to sets.  Although we represent sets as
	// map[key]struct{} on the Go side, we don't allow these as general
	// conversions for val.Value.
	EmptyType           = StructType()
	EmptyTypeN          = NamedType("NEmpty", StructType())
	MapStringEmptyType  = MapType(StringType, EmptyType)
	MapStringEmptyTypeN = NamedType("NMapStringEmpty", MapType(StringTypeN, EmptyTypeN))
	StructXYZEmptyType  = StructType(Field{"X", EmptyType}, Field{"Y", EmptyType}, Field{"Z", EmptyType})
	StructXYZEmptyTypeN = NamedType("NStructXYZEmpty", StructType(Field{"X", EmptyTypeN}, Field{"Y", EmptyTypeN}, Field{"Z", EmptyTypeN}))
)

func NameN(suffix string, base *Type) *Type {
	return NamedType("v.io/v23/vdl.N"+suffix, base)
}

func NameNArray(suffix string, base *Type) *Type {
	return NamedType("v.io/v23/vdl.NArray3"+suffix, ArrayType(3, base))
}

func NameNStruct(suffix string, base *Type) *Type {
	return NamedType("v.io/v23/vdl.NStruct"+suffix, StructType(Field{"X", base}))
}

func NameNSlice(suffix string, base *Type) *Type {
	return NamedType("v.io/v23/vdl.NSlice"+suffix, ListType(base))
}

func rtSet(base *Type) *Type {
	return SetType(base)
}

func NameNSet(suffix string, base *Type) *Type {
	return NamedType("v.io/v23/vdl.NSet"+suffix, rtSet(base))
}

func rtMap(base *Type) *Type {
	return MapType(base, base)
}

func NameNMap(suffix string, base *Type) *Type {
	return NamedType("v.io/v23/vdl.NMap"+suffix, rtMap(base))
}

func SetStringValue(t *Type, x ...string) *Value {
	res := ZeroValue(t)
	for _, vx := range x {
		key := StringValue(t.Key(), vx)
		res.AssignSetKey(key)
	}
	return res
}

type SB struct {
	S string
	B bool
}

func MapStringBoolValue(t *Type, x ...SB) *Value {
	res := ZeroValue(t)
	for _, sb := range x {
		key := StringValue(t.Key(), sb.S)
		val := BoolValue(t.Elem(), sb.B)
		res.AssignMapIndex(key, val)
	}
	return res
}

func MapStringEmptyValue(t *Type, x ...string) *Value {
	res := ZeroValue(t)
	for _, vx := range x {
		key := StringValue(t.Key(), vx)
		val := ZeroValue(t.Elem())
		res.AssignMapIndex(key, val)
	}
	return res
}

func StructBoolValue(t *Type, x ...SB) *Value {
	res := ZeroValue(t)
	for _, sb := range x {
		_, index := t.FieldByName(sb.S)
		res.StructField(index).AssignBool(sb.B)
	}
	return res
}

func AssignNum(v *Value, num float64) *Value {
	switch v.Kind() {
	case Byte, Uint16, Uint32, Uint64:
		v.AssignUint(uint64(num))
	case Int8, Int16, Int32, Int64:
		v.AssignInt(int64(num))
	case Float32, Float64:
		v.AssignFloat(num)
	default:
		panic(fmt.Errorf("vdl: AssignNum unhandled %v", v.Type()))
	}
	return v
}

func SeqNumValue(t *Type, x ...float64) *Value {
	res := ZeroValue(t)
	if t.Kind() == List {
		res.AssignLen(len(x))
	}
	for index, n := range x {
		AssignNum(res.Index(index), n)
	}
	return res
}

func SetNumValue(t *Type, x ...float64) *Value {
	res := ZeroValue(t)
	for _, n := range x {
		res.AssignSetKey(AssignNum(ZeroValue(t.Key()), n))
	}
	return res
}

type NB struct {
	N float64
	B bool
}

func MapNumBoolValue(t *Type, x ...NB) *Value {
	res := ZeroValue(t)
	for _, nb := range x {
		key := AssignNum(ZeroValue(t.Key()), nb.N)
		val := BoolValue(t.Elem(), nb.B)
		res.AssignMapIndex(key, val)
	}
	return res
}

type SN struct {
	S string
	N float64
}

func MapStringNumValue(t *Type, x ...SN) *Value {
	res := ZeroValue(t)
	for _, sn := range x {
		key := StringValue(t.Key(), sn.S)
		val := AssignNum(ZeroValue(t.Elem()), sn.N)
		res.AssignMapIndex(key, val)
	}
	return res
}

func StructNumValue(t *Type, x ...SN) *Value {
	res := ZeroValue(t)
	for _, sn := range x {
		_, index := t.FieldByName(sn.S)
		AssignNum(res.StructField(index), sn.N)
	}
	return res
}
