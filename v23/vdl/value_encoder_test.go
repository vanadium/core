// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import (
	"reflect"
	"runtime"
	"testing"
)

func runEncoder(t *testing.T, enc Encoder, tt *Type, assigners ...func() error) {
	_, _, line, _ := runtime.Caller(2)
	if err := enc.StartValue(tt); err != nil {
		t.Errorf("line %v: %v", line, err)
	}
	for i, assigner := range assigners {
		if err := assigner(); err != nil {
			t.Errorf("line %v: assigner %v: %v", line, i, err)
		}
	}
	if err := enc.FinishValue(); err != nil {
		t.Errorf("line %v: %v", line, err)
	}
}

func assertEncoderResult(t *testing.T, enc Encoder, got, want *Value) {
	if !reflect.DeepEqual(got, want) {
		_, _, line, _ := runtime.Caller(2)
		t.Errorf("line %v: got %v, want %v", line, got, want)
	}
}

func typeFromReflect(t *testing.T, rt reflect.Type) *Type {
	tt, err := TypeFromReflect(rt)
	if err != nil {
		_, _, line, _ := runtime.Caller(2)
		t.Fatalf("line %v: %v", line, err)

	}
	return tt
}

func TestEncodeToValuePrimitive(t *testing.T) {
	var value Value
	enc := newToValueEncoder(&value)

	for i, tc := range []struct {
		value *Value
		src   interface{}
		rep   interface{}
	}{
		{ZeroValue(BoolType), false, false},
		{ZeroValue(BoolType), true, true},
		{ZeroValue(ByteType), byte(0x00), uint64(0)},
		{ZeroValue(ByteType), byte(0x80), uint64(0x80)},
		{ZeroValue(Uint16Type), uint16(0x0001), uint64(0x0001)},
		{ZeroValue(Uint16Type), uint16(0x1000), uint64(0x1000)},
		{ZeroValue(Uint32Type), uint32(0x0001), uint64(0x0001)},
		{ZeroValue(Uint32Type), uint32(0x1000), uint64(0x1000)},
		{ZeroValue(Uint64Type), uint64(0x0001), uint64(0x0001)},
		{ZeroValue(Uint64Type), uint64(0x1000), uint64(0x1000)},
		{ZeroValue(Int16Type), int16(0x0001), int64(0x0001)},
		{ZeroValue(Int16Type), int16(0x1000), int64(0x1000)},
		{ZeroValue(Int32Type), int32(0x0001), int64(0x0001)},
		{ZeroValue(Int32Type), int32(0x1000), int64(0x1000)},
		{ZeroValue(Int64Type), int64(0x0001), int64(0x0001)},
		{ZeroValue(Int64Type), int64(0x1000), int64(0x1000)},
		{ZeroValue(Float32Type), float32(1.0), float64(1.0)},
		{ZeroValue(Float64Type), float64(1.0), float64(1.0)},
		{ZeroValue(StringType), "hello", "hello"},
		{ZeroValue(TypeObjectType), ByteType, ByteType},
	} {
		tc.value.rep = tc.rep
		if err := Write(enc, tc.src); err != nil {
			t.Fatalf("%v: %v: failed: %v", i, tc.value, err)
		}
		if got, want := &value, tc.value; !reflect.DeepEqual(got, want) {
			t.Errorf("%v: %v: got %v, want %v", i, tc.value, got, want)
		}
	}

	runEncoder := func(tt *Type, assigner func() error) {
		runEncoder(t, enc, tt, assigner)
	}
	assertValue := func(want *Value) {
		assertEncoderResult(t, enc, &value, want)
	}

	runEncoder(BoolType, func() error { return enc.EncodeBool(true) })
	assertValue(&Value{t: BoolType, rep: true})

	runEncoder(Uint32Type, func() error { return enc.EncodeUint(32) })
	assertValue(&Value{t: Uint32Type, rep: uint64(32)})

	runEncoder(Int16Type, func() error { return enc.EncodeInt(33) })
	assertValue(&Value{t: Int16Type, rep: int64(33)})

	runEncoder(StringType, func() error { return enc.EncodeString("oh my") })
	assertValue(&Value{t: StringType, rep: "oh my"})

	runEncoder(ByteType, func() error { return enc.EncodeUint('c') })
	assertValue(&Value{t: ByteType, rep: uint64('c')})

	runEncoder(TypeObjectType, func() error { return enc.EncodeTypeObject(ByteType) })
	assertValue(&Value{t: TypeObjectType, rep: ByteType})
}

func TestEncodeToValueBytes(t *testing.T) {
	var value Value
	enc := newToValueEncoder(&value)
	runEncoder := func(tt *Type, assigner func() error) {
		runEncoder(t, enc, tt, assigner)
	}
	assertValue := func(want *Value) {
		assertEncoderResult(t, enc, &value, want)
	}

	vdlWrite := func(val interface{}) {
		if err := Write(enc, val); err != nil {
			_, _, line, _ := runtime.Caller(1)
			t.Fatalf("line %v: val %v: %v", line, val, err)
		}
	}

	checkCap := func(val int) {
		_, _, line, _ := runtime.Caller(1)
		if got, want := cap(*(value.rep.(*repBytes))), val; got != want {
			t.Errorf("line %v: got %v, want %v", line, got, want)
		}
	}

	data := make([]byte, 0, 4)
	data = append(data, 0x22, 0x33)
	// Note that the encoder will internall resize to the length of the slice
	// unless SetLenHint is used.
	adata := [2]byte{0x22, 0x66}
	runEncoder(ListType(ByteType), func() error { return enc.EncodeBytes(data) })
	assertValue(BytesValue(ListType(ByteType), data))
	checkCap(2)
	vdlWrite(data)
	assertValue(BytesValue(ListType(ByteType), data))
	checkCap(2)

	runEncoder(ArrayType(2, ByteType), func() error { return enc.EncodeBytes(data) })
	assertValue(BytesValue(ArrayType(2, ByteType), data))
	checkCap(2)
	vdlWrite(adata)
	assertValue(BytesValue(ArrayType(2, ByteType), adata[:]))
	checkCap(2)

	runEncoder(ListType(ByteType), func() error {
		enc.SetLenHint(3)
		return enc.EncodeBytes(data)
	})
	assertValue(BytesValue(ListType(ByteType), data))
	checkCap(3)

	vdlWrite(data)
	checkCap(2)
}

func TestEncodeToValueArray(t *testing.T) {
	var value Value
	enc := newToValueEncoder(&value)
	runEncoder := func(tt *Type, assigners ...func() error) {
		runEncoder(t, enc, tt, assigners...)
	}

	runEncoder(typeFromReflect(t, reflect.TypeOf([4]int32{})),
		func() error { return enc.NextEntryValueInt(Int32Type, 11) },
		func() error { return enc.NextEntry(false) },
		func() error { return enc.StartValue(Int32Type) },
		func() error { return enc.EncodeInt(22) },
		enc.FinishValue,
		func() error { return enc.NextEntryValueInt(Int32Type, 33) },
		func() error { return enc.NextEntry(true) },
	)

	if got, want := value.String(), `[4]int32{11, 22, 33, 0}`; got != want {
		t.Errorf("got %v, want %v\n", got, want)
	}

	runEncoder(typeFromReflect(t, reflect.TypeOf([2]float64{})),
		func() error { return enc.NextEntryValueFloat(Float64Type, 2.0) },
		func() error { return enc.NextEntryValueFloat(Float64Type, 1.0) },
		func() error { return enc.NextEntry(true) },
	)

	if got, want := value.String(), `[2]float64{2, 1}`; got != want {
		t.Errorf("got %v, want %v\n", got, want)
	}

	runEncoder(typeFromReflect(t, reflect.TypeOf([1][]float64{})),
		func() error { return enc.NextEntry(false) },
		func() error { return enc.StartValue(ListType(Float64Type)) },
		func() error { return enc.SetLenHint(2) },
		func() error { return enc.NextEntryValueFloat(Float64Type, 2.0) },
		func() error { return enc.NextEntryValueFloat(Float64Type, 1.0) },
		func() error { return enc.NextEntry(true) },
		enc.FinishValue,
		func() error { return enc.NextEntry(true) },
	)

	if got, want := value.String(), `[1][]float64{{2, 1}}`; got != want {
		t.Errorf("got %v, want %v\n", got, want)
	}

	runEncoder(typeFromReflect(t, reflect.TypeOf([1][]byte{})),
		func() error { return enc.NextEntryValueBytes(ListByteType, []byte{'a', 'b', 'c'}) },
		func() error { return enc.NextEntry(true) },
	)

	if got, want := value.String(), `[1][]byte{"abc"}`; got != want {
		t.Errorf("got %v, want %v\n", got, want)
	}

	runEncoder(typeFromReflect(t, reflect.TypeOf([1][]byte{})),
		func() error { return enc.NextEntry(false) },
		func() error { return enc.StartValue(ListByteType) },
		func() error { return enc.SetLenHint(4) },
		func() error { return enc.EncodeBytes([]byte{'a', 'b', 'c'}) },
		enc.FinishValue,
		func() error { return enc.NextEntry(true) },
	)

	if got, want := value.String(), `[1][]byte{"abc"}`; got != want {
		t.Errorf("got %v, want %v\n", got, want)
	}

}

func TestEncodeToValueComposite(t *testing.T) {

	type structTypeComposite struct {
		A int
		B string
		C []byte
		D map[string]int
		E map[int]struct{} // a set.
		F []int
	}

	var value Value
	enc := newToValueEncoder(&value)
	runEncoder := func(tt *Type, assigners ...func() error) {
		runEncoder(t, enc, tt, assigners...)
	}

	tt := typeFromReflect(t, reflect.TypeOf(structTypeComposite{}))
	runEncoder(tt,
		func() error { return enc.NextField(0) },
		func() error { return enc.StartValue(Int64Type) },
		func() error { return enc.EncodeInt(2) },
		enc.FinishValue,

		func() error { return enc.NextField(1) },
		func() error { return enc.StartValue(StringType) },
		func() error { return enc.EncodeString("bar") },
		enc.FinishValue,

		func() error { return enc.NextField(2) },
		func() error { return enc.StartValue(ListType(ByteType)) },
		func() error { return enc.EncodeBytes([]byte{0x77}) },
		enc.FinishValue,

		func() error { return enc.NextField(3) },
		func() error { return enc.StartValue(MapType(StringType, Int64Type)) },
		func() error { return enc.NextEntry(false) },
		func() error { return enc.StartValue(StringType) },
		func() error { return enc.EncodeString("bar") },
		enc.FinishValue,
		func() error { return enc.StartValue(Int64Type) },
		func() error { return enc.EncodeInt(33) },
		enc.FinishValue,
		func() error { return enc.NextEntry(true) },
		enc.FinishValue,

		func() error { return enc.NextField(4) },
		func() error { return enc.StartValue(SetType(Int64Type)) },
		func() error { return enc.NextEntry(false) },
		func() error { return enc.StartValue(Int64Type) },
		func() error { return enc.EncodeInt(3) },
		enc.FinishValue,
		func() error { return enc.NextEntry(false) },
		func() error { return enc.StartValue(Int64Type) },
		func() error { return enc.EncodeInt(5) },
		enc.FinishValue,
		func() error { return enc.NextEntry(true) },
		enc.FinishValue,

		func() error { return enc.NextField(5) },
		func() error { return enc.StartValue(ListType(Int64Type)) },
		func() error { return enc.SetLenHint(2) },
		func() error { return enc.NextEntry(false) },
		func() error { return enc.StartValue(Int64Type) },
		func() error { return enc.EncodeInt(13) },
		enc.FinishValue,
		func() error { return enc.NextEntry(false) },
		func() error { return enc.StartValue(Int64Type) },
		func() error { return enc.EncodeInt(14) },
		enc.FinishValue,
		func() error { return enc.NextEntry(true) },
		enc.FinishValue,

		func() error { return enc.NextField(-1) },
	)
	empty := `v.io/v23/vdl.structTypeComposite struct{A int64;B string;C []byte;D map[string]int64;E set[int64];F []int64}{A: 0, B: "", C: "", D: {}, E: {}, F: {}}`
	initializedA := `v.io/v23/vdl.structTypeComposite struct{A int64;B string;C []byte;D map[string]int64;E set[int64];F []int64}{A: 2, B: "bar", C: "w", D: {"bar": 33}, E: {3, 5}, F: {13, 14}}`
	// map keys are randomly ordered.
	initializedB := `v.io/v23/vdl.structTypeComposite struct{A int64;B string;C []byte;D map[string]int64;E set[int64];F []int64}{A: 2, B: "bar", C: "w", D: {"bar": 33}, E: {5, 3}, F: {13, 14}}`
	if got, wantA, wantB := value.String(), initializedA, initializedB; got != wantA && got != wantB {
		t.Errorf("got %v, want %v or %v", got, wantA, wantB)
	}

	vdlWrite := func(val interface{}) {
		if err := Write(enc, val); err != nil {
			_, _, line, _ := runtime.Caller(1)
			t.Fatalf("line %v: val %v: %v", line, val, err)
		}
	}
	vdlWrite(structTypeComposite{})
	if got, want := value.String(), empty; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	original := structTypeComposite{
		A: 2,
		B: "bar",
		C: []byte{0x77},
		D: map[string]int{"bar": 33},
		E: map[int]struct{}{3: {}, 5: {}},
		F: []int{13, 14},
	}
	vdlWrite(original)
	if got, wantA, wantB := value.String(), initializedA, initializedB; got != wantA && got != wantB {
		t.Errorf("got %v, want %v or %v", got, wantA, wantB)
	}

	dec := value.Decoder()
	var w structTypeComposite
	if err := Read(dec, &w); err != nil {
		t.Fatal(err)
	}
	if got, want := w, original; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

}

func TestEncodeToValueAny(t *testing.T) {
	type structTypeAny struct {
		A int
		B interface{}
		C interface{}
	}

	var value Value
	enc := newToValueEncoder(&value)
	runEncoder := func(tt *Type, assigners ...func() error) {
		runEncoder(t, enc, tt, assigners...)
	}

	tt := typeFromReflect(t, reflect.TypeOf(structTypeAny{}))
	runEncoder(tt,
		func() error { return enc.NextFieldValueInt(0, Int64Type, 33) },
		func() error { return enc.NextFieldValueFloat(1, Float64Type, 33.0) },
		func() error { return enc.NextField(2) },
		func() error { return enc.NilValue(AnyType) },
		func() error { return enc.NextField(-1) },
	)

	initialized := `v.io/v23/vdl.structTypeAny struct{A int64;B any;C any}{A: 33, B: float64(33), C: nil}`
	if got, want := value.String(), initialized; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	original := structTypeAny{A: 33, B: 33.0, C: nil}
	if err := Write(enc, original); err != nil {
		t.Fatal(err)
	}

	if got, want := value.String(), initialized; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	dec := value.Decoder()
	var w structTypeAny
	if err := Read(dec, &w); err != nil {
		t.Fatal(err)
	}
	if got, want := w, original; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

}

func TestEncodeToValueAnySlice(t *testing.T) {
	type structTypeAnySlice []interface{}
	var value Value
	enc := newToValueEncoder(&value)
	runEncoder := func(tt *Type, assigners ...func() error) {
		runEncoder(t, enc, tt, assigners...)
	}

	tt := typeFromReflect(t, reflect.TypeOf(structTypeAnySlice{}))
	runEncoder(tt,
		func() error { return enc.SetLenHint(3) },
		func() error { return enc.NextEntryValueInt(Int64Type, 33) },
		func() error { return enc.NextEntryValueFloat(Float32Type, 33.0) },
		func() error { return enc.NextEntry(true) },
	)
	typeStr := `v.io/v23/vdl.structTypeAnySlice []any`
	initialized := typeStr + `{int64(33), float32(33), nil}`
	if got, want := value.String(), initialized; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	runEncoder(tt,
		func() error { return enc.SetLenHint(3) },
		func() error { return enc.NextEntry(false) },
		func() error { return enc.NilValue(AnyType) },
		func() error { return enc.NextEntryValueFloat(Float32Type, 33.0) },
		func() error { return enc.NextEntry(true) },
	)
	initialized = typeStr + `{nil, float32(33), nil}`
	if got, want := value.String(), initialized; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	original := structTypeAnySlice{int64(32), &WireError{RetryCode: WireRetryCodeRetryRefetch}}
	if err := Write(enc, original); err != nil {
		t.Fatal(err)
	}
	initialized = typeStr + `{int64(32), v.io/v23/vdl.WireError struct{Id string;RetryCode v.io/v23/vdl.WireRetryCode enum{NoRetry;RetryConnection;RetryRefetch;RetryBackoff};Msg string;ParamList []any}{Id: "", RetryCode: RetryRefetch, Msg: "", ParamList: {}}}`
	if got, want := value.String(), initialized; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
func TestEncodeToValueOptional(t *testing.T) {

	type optionalStructType struct {
		A int
		B string
	}

	type structTypeOptional struct {
		A *optionalStructType
		B interface{}
		C interface{}
		D *optionalStructType
	}

	var value Value
	enc := newToValueEncoder(&value)
	runEncoder := func(tt *Type, assigners ...func() error) {
		runEncoder(t, enc, tt, assigners...)
	}

	tt := typeFromReflect(t, reflect.TypeOf(structTypeOptional{}))
	ott := typeFromReflect(t, reflect.TypeOf(optionalStructType{}))
	typeStr := `v.io/v23/vdl.structTypeOptional struct{A ?v.io/v23/vdl.optionalStructType struct{A int64;B string};B any;C any;D ?v.io/v23/vdl.optionalStructType}`

	runEncoder(tt,
		func() error { return enc.NextField(0) },
		func() error { enc.SetNextStartValueIsOptional(); return nil },
		func() error { return enc.NilValue(OptionalType(ott)) },

		func() error { return enc.NextField(1) },
		func() error { return enc.NilValue(AnyType) },

		func() error { return enc.NextField(2) },
		func() error { return enc.StartValue(AnyType) },
		func() error { return enc.EncodeString("hello") },
		enc.FinishValue,

		func() error { return enc.NextField(3) },
		func() error { enc.SetNextStartValueIsOptional(); return nil },
		func() error { return enc.StartValue(ott) },

		func() error { return enc.NextFieldValueInt(0, Int64Type, 11) },
		func() error { return enc.NextFieldValueString(1, StringType, "world") },
		func() error { return enc.NextField(-1) },
		enc.FinishValue,

		func() error { return enc.NextField(-1) },
	)

	initialized := typeStr + `{A: nil, B: nil, C: "hello", D: {A: 11, B: "world"}}`
	if got, want := value.String(), initialized; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	for _, tc := range []struct {
		input  structTypeOptional
		output string
	}{
		{structTypeOptional{}, `{A: nil, B: nil, C: nil, D: nil}`},
		{structTypeOptional{
			C: "hello",
			D: &optionalStructType{A: 11, B: "world"},
		},
			`{A: nil, B: nil, C: "hello", D: {A: 11, B: "world"}}`},
	} {

		if err := Write(enc, tc.input); err != nil {
			t.Fatalf("val %v: %v", tc.input, err)
		}
		if got, want := value.String(), typeStr+tc.output; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		var w structTypeOptional
		if err := Read(value.Decoder(), &w); err != nil {
			t.Fatal(err)
		}
		if got, want := w, tc.input; !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}

		// optional version.
		tmp := tc.input
		if err := Write(enc, &tmp); err != nil {
			t.Fatalf("val %v: %v", tc.input, err)
		}
		if got, want := value.String(), "?"+typeStr+"("+tc.output+")"; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		if err := Read(value.Decoder(), &w); err != nil {
			t.Fatal(err)
		}
		if got, want := w, tc.input; !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	}

}

func TestEncodeToValueEnum(t *testing.T) {
	type enumTypeStruct struct {
		A int
		C WireRetryCode
	}

	var value Value
	enc := newToValueEncoder(&value)

	runEncoder := func(tt *Type, assigners ...func() error) {
		runEncoder(t, enc, tt, assigners...)
	}

	runEncoder(EnumType("L1", "L2", "L3"),
		func() error { return enc.EncodeString("L2") },
	)

	if got, want := value.String(), `enum{L1;L2;L3}(L2)`; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	et := typeFromReflect(t, reflect.TypeOf(WireRetryCode(0)))
	runEncoder(typeFromReflect(t, reflect.TypeOf(enumTypeStruct{})),
		func() error { return enc.NextFieldValueInt(0, Int64Type, 3) },
		func() error {
			return enc.NextFieldValueString(1, et, "RetryRefetch")
		},
		func() error { return enc.NextField(-1) },
	)
	initialized := `v.io/v23/vdl.enumTypeStruct struct{A int64;C v.io/v23/vdl.WireRetryCode enum{NoRetry;RetryConnection;RetryRefetch;RetryBackoff}}{A: 3, C: RetryRefetch}`
	if got, want := value.String(), initialized; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	original := enumTypeStruct{A: 3, C: WireRetryCode(2)}
	if err := Write(enc, original); err != nil {
		t.Fatal(err)
	}
	if got, want := value.String(), initialized; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	dec := value.Decoder()
	var w enumTypeStruct
	if err := Read(dec, &w); err != nil {
		t.Fatal(err)
	}
	if got, want := w, original; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestEncodeToValueMap(t *testing.T) {
	var value Value
	enc := newToValueEncoder(&value)

	runEncoder := func(tt *Type, assigners ...func() error) {
		runEncoder(t, enc, tt, assigners...)
	}

	runEncoder(typeFromReflect(t, reflect.TypeOf(map[byte]bool{})),
		func() error { return enc.NextEntry(false) },
		func() error { return enc.StartValue(ByteType) },
		func() error { return enc.EncodeUint('c') },
		enc.FinishValue,
		func() error { return enc.StartValue(BoolType) },
		func() error { return enc.EncodeBool(false) },
		enc.FinishValue,

		func() error { return enc.NextEntry(false) },
		func() error { return enc.StartValue(ByteType) },
		func() error { return enc.EncodeUint('d') },
		enc.FinishValue,
		func() error { return enc.StartValue(BoolType) },
		func() error { return enc.EncodeBool(true) },
		enc.FinishValue,

		func() error { return enc.NextEntry(true) },
	)
	initializedA := `map[byte]bool{99: false, 100: true}`
	initializedB := `map[byte]bool{100: true, 99: false}`

	if got, wantA, wantB := value.String(), initializedA, initializedB; got != wantA && got != wantB {
		t.Errorf("got %v, want %v or %v", got, wantA, wantB)
	}

	if err := Write(enc, map[byte]bool{'c': false, 'd': true}); err != nil {
		t.Fatal(err)
	}

	if got, wantA, wantB := value.String(), initializedA, initializedB; got != wantA && got != wantB {
		t.Errorf("got %v, want %v or %v", got, wantA, wantB)
	}
}

func TestEncodeToValueUnion(t *testing.T) {
	ut := UnionType([]Field{
		{"A", Int16Type},
		{"B", Int64Type},
		{"C", StringType}}...)
	var value Value
	enc := newToValueEncoder(&value)

	runEncoder := func(tt *Type, assigners ...func() error) {
		runEncoder(t, enc, tt, assigners...)
	}

	runEncoder(ut)
	if got, want := value.String(), `union{A int16;B int64;C string}{A: 0}`; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	runEncoder(ut,
		func() error { return enc.NextFieldValueString(2, StringType, "world") },
		func() error { return enc.NextField(-1) },
	)
	if got, want := value.String(), `union{A int16;B int64;C string}{C: "world"}`; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestEncodeToValueTranscodeArray(t *testing.T) {
	var value Value
	enc := newToValueEncoder(&value)
	av := ZeroValue(ArrayType(3, Int64Type))
	av.AssignIndex(0, IntValue(Int64Type, 1))
	av.AssignIndex(1, IntValue(Int64Type, 2))
	av.AssignIndex(2, IntValue(Int64Type, 3))

	// Transcode incorrectly calls SetLenHint for an array, make
	// sure that it's ignored.
	if err := Transcode(enc, av.Decoder()); err != nil {
		t.Fatal(err)
	}

	if got, want := value.String(), av.String(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	if got, want := value.String(), "[3]int64{1, 2, 3}"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
