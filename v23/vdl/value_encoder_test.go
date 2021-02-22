package vdl

import (
	"fmt"
	"reflect"
	"runtime"
	"testing"
)

func runEncoder(t *testing.T, enc Encoder, tt *Type, assigners ...func() error) {
	_, _, line, _ := runtime.Caller(2)
	if err := enc.StartValue(tt); err != nil {
		t.Fatalf("line %v: %v", line, err)
	}
	for i, assigner := range assigners {
		if err := assigner(); err != nil {
			t.Fatalf("line %v: assigner %v: %v", line, i, err)
		}
	}
	if err := enc.FinishValue(); err != nil {
		t.Fatalf("line %v: %v", line, err)
	}
}

func assertEncoderResult(t *testing.T, enc *toValueEncoder, want *Value) {
	_, _, line, _ := runtime.Caller(2)
	v := enc.Value()
	fmt.Printf("%#v\n", v)
	if got := &v; !reflect.DeepEqual(got, want) {
		t.Errorf("line %v: got %v, want %v", line, got, want)
	}
}

func TestEncodeToValuePrimitive(t *testing.T) {
	enc := newToValueEncoder()

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
		v := enc.Value()
		if got, want := &v, tc.value; !reflect.DeepEqual(got, want) {
			t.Errorf("%v: %v: got %v, want %v", i, tc.value, got, want)
		}
	}

	runEncoder := func(tt *Type, assigner func() error) {
		runEncoder(t, enc, tt, assigner)
	}
	assertValue := func(want *Value) {
		assertEncoderResult(t, enc, want)
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
	enc := newToValueEncoder()
	runEncoder := func(tt *Type, assigner func() error) {
		runEncoder(t, enc, tt, assigner)
	}
	assertValue := func(want *Value) {
		assertEncoderResult(t, enc, want)
	}

	vdlWrite := func(val interface{}) {
		if err := Write(enc, val); err != nil {
			_, _, line, _ := runtime.Caller(1)
			t.Fatalf("line %v: val %v: %v", line, val, err)
		}
	}

	checkCap := func(val int) {
		_, _, line, _ := runtime.Caller(1)
		if got, want := cap(*(enc.Value().rep.(*repBytes))), val; got != want {
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

type structType struct {
	A int
	B string
	C []byte
	D map[string]int
}

type mapType map[string]structType

func TestEncodeToValueComposite(t *testing.T) {
	enc := newToValueEncoder()
	runEncoder := func(tt *Type, assigners ...func() error) {
		runEncoder(t, enc, tt, assigners...)
	}
	/*assertValue := func(want *Value) {
		assertEncoderResult(t, enc, want)
	}*/
	/*
		func() error { return enc.EncodeBytes(data) })
	runEncoder(tt)*/

	tt, _ := TypeFromReflect(reflect.TypeOf(structType{}))
	runEncoder(tt,
		func() error { return enc.NextField(0) },
		func() error { return enc.StartValue(Int64Type) },
		func() error { return enc.EncodeInt(2) },
		func() error { return enc.FinishValue() },
		func() error { return enc.NextField(1) },
		func() error { return enc.StartValue(StringType) },
		func() error { return enc.EncodeString("bar") },
		func() error { return enc.NextField(2) },
		func() error { return enc.FinishValue() },
		func() error { return enc.StartValue(ListType(ByteType)) },
		func() error { return enc.EncodeBytes([]byte{0x77}) },
		func() error { return enc.FinishValue() },
		func() error { return enc.NextField(3) },
		func() error { return enc.StartValue(MapType(StringType, Int64Type)) },
		func() error { return enc.NextEntry(false) },
		func() error { return enc.StartValue(StringType) },
		func() error { return enc.EncodeString("bar") },
		func() error { return enc.FinishValue() },
		func() error { return enc.StartValue(Int64Type) },
		func() error { return enc.EncodeInt(33) },
		func() error { return enc.FinishValue() },
		func() error { return enc.NextEntry(true) },
		func() error { return enc.NextField(-1) },
	)
	v := enc.Value()
	if got, want := v.String(), ""; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	vdlWrite := func(val interface{}) {
		if err := Write(enc, val); err != nil {
			_, _, line, _ := runtime.Caller(1)
			t.Fatalf("line %v: val %v: %v", line, val, err)
		}
	}

	vdlWrite(structType{A: 3, B: "bar", C: []byte{0x71}})
	v = enc.Value()
	if got, want := v.String(), ""; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

}

/*
	fmt.Printf("%p .. %p .. %p\n", BoolType, StringType, TypeObjectType)
	fmt.Printf("%p .. %p .. %p\n", Uint16Type, Uint32Type, Uint64Type)
*/
