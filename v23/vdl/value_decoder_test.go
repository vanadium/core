// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import (
	"bytes"
	"reflect"
	"sort"
	"testing"
)

func TestValueDecoderDecodeBool(t *testing.T) {
	expected := true
	vd := ValueOf(expected).Decoder()
	if err := vd.StartValue(BoolType); err != nil {
		t.Errorf("error in StartValue: %v", err)
	}
	if got, want := vd.Type(), BoolType; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	switch val, err := vd.DecodeBool(); {
	case err != nil:
		t.Errorf("error decoding value: %v", err)
	case val != expected:
		t.Errorf("got %v, want %v", val, expected)
	}
	if err := vd.FinishValue(); err != nil {
		t.Errorf("error in FinishValue: %v", err)
	}
}

func TestValueDecoderDecodeUint(t *testing.T) {
	expected := uint32(5)
	vd := ValueOf(expected).Decoder()
	if err := vd.StartValue(Uint32Type); err != nil {
		t.Errorf("error in StartValue: %v", err)
	}
	if got, want := vd.Type(), Uint32Type; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	switch val, err := vd.DecodeUint(32); {
	case err != nil:
		t.Errorf("error decoding value: %v", err)
	case val != 5:
		t.Errorf("got %d, want %d", val, expected)
	}
	if err := vd.FinishValue(); err != nil {
		t.Errorf("error in FinishValue: %v", err)
	}
}

func TestValueDecoderDecodeInt(t *testing.T) {
	expected := int64(5)
	vd := ValueOf(expected).Decoder()
	if err := vd.StartValue(Int64Type); err != nil {
		t.Errorf("error in StartValue: %v", err)
	}
	if got, want := vd.Type(), Int64Type; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	switch val, err := vd.DecodeInt(64); {
	case err != nil:
		t.Errorf("error decoding value: %v", err)
	case val != 5:
		t.Errorf("got %d, want %d", val, expected)
	}
	if err := vd.FinishValue(); err != nil {
		t.Errorf("error in FinishValue: %v", err)
	}
}

func TestValueDecoderDecodeFloat(t *testing.T) {
	expected := float32(5)
	vd := ValueOf(expected).Decoder()
	if err := vd.StartValue(Float32Type); err != nil {
		t.Errorf("error in StartValue: %v", err)
	}
	if got, want := vd.Type(), Float32Type; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	switch val, err := vd.DecodeFloat(32); {
	case err != nil:
		t.Errorf("error decoding value: %v", err)
	case val != 5:
		t.Errorf("got %f, want %f", val, expected)
	}
	if err := vd.FinishValue(); err != nil {
		t.Errorf("error in FinishValue: %v", err)
	}
}

func TestValueDecoderDecodeString(t *testing.T) {
	expected := "abc"
	vd := ValueOf(expected).Decoder()
	if err := vd.StartValue(StringType); err != nil {
		t.Errorf("error in StartValue: %v", err)
	}
	if got, want := vd.Type(), StringType; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	switch val, err := vd.DecodeString(); {
	case err != nil:
		t.Errorf("error decoding value: %v", err)
	case val != expected:
		t.Errorf("got %v, want %v", val, expected)
	}
	if err := vd.FinishValue(); err != nil {
		t.Errorf("error in FinishValue: %v", err)
	}
}

func TestValueDecoderDecodeEnum(t *testing.T) {
	expectedType := EnumType("A", "B")
	expected := ZeroValue(expectedType)
	vd := expected.Decoder()
	if err := vd.StartValue(expectedType); err != nil {
		t.Errorf("error in StartValue: %v", err)
	}
	if got, want := vd.Type(), expectedType; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	switch val, err := vd.DecodeString(); {
	case err != nil:
		t.Errorf("error decoding value: %v", err)
	case val != expected.EnumLabel():
		t.Errorf("got %v, want %v", val, expected)
	}
	if err := vd.FinishValue(); err != nil {
		t.Errorf("error in FinishValue: %v", err)
	}
}

func TestValueDecoderDecodeByteList(t *testing.T) {
	expected := []byte("abc")
	expectedType := TypeOf(expected)
	vd := ValueOf(expected).Decoder()
	if err := vd.StartValue(expectedType); err != nil {
		t.Errorf("error in StartValue: %v", err)
	}
	if got, want := vd.Type(), expectedType; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	var out []byte
	if err := vd.DecodeBytes(-1, &out); err != nil {
		t.Errorf("error decoding value: %v", err)
	}
	if !bytes.Equal(expected, out) {
		t.Errorf("got %v, want %v", out, expected)
	}
	if err := vd.FinishValue(); err != nil {
		t.Errorf("error in FinishValue: %v", err)
	}
}

func TestValueDecoderDecodeByteArray(t *testing.T) {
	expected := [3]byte{byte('a'), byte('b'), byte('c')}
	expectedType := TypeOf(expected)
	vd := ValueOf(expected).Decoder()
	if err := vd.StartValue(expectedType); err != nil {
		t.Errorf("error in StartValue: %v", err)
	}
	if got, want := vd.Type(), expectedType; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	var out []byte
	if err := vd.DecodeBytes(3, &out); err != nil {
		t.Errorf("error decoding value: %v", err)
	}
	if !bytes.Equal(expected[:], out) {
		t.Errorf("got %v, want %v", out, expected)
	}
	if err := vd.FinishValue(); err != nil {
		t.Errorf("error in FinishValue: %v", err)
	}

	vd = ValueOf(expected).Decoder()
	if err := vd.StartValue(expectedType); err != nil {
		t.Errorf("error in StartValue: %v", err)
	}
	if err := vd.DecodeBytes(2, &out); err == nil {
		t.Errorf("expected error decoding with mismatched length")
	}
	if err := vd.FinishValue(); err != nil {
		t.Errorf("error in FinishValue: %v", err)
	}
}

func TestValueDecoderDecodeTypeObject(t *testing.T) {
	expected := TypeOf("abc")
	vd := ValueOf(expected).Decoder()
	if err := vd.StartValue(TypeObjectType); err != nil {
		t.Errorf("error in StartValue: %v", err)
	}
	if got, want := vd.Type(), TypeObjectType; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	switch val, err := vd.DecodeTypeObject(); {
	case err != nil:
		t.Errorf("error decoding value: %v", err)
	case val != expected:
		t.Errorf("got %v, want %v", val, expected)
	}
	if err := vd.FinishValue(); err != nil {
		t.Errorf("error in FinishValue: %v", err)
	}
}

func testValueDecoderDecodeSequence(t *testing.T, expected interface{}) { //nolint:gocyclo
	expectedType := TypeOf(expected)
	vd := ValueOf(expected).Decoder()
	if err := vd.StartValue(expectedType); err != nil {
		t.Fatalf("error in StartValue: %v", err)
	}
	if got, want := vd.Type(), expectedType; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	switch done, err := vd.NextEntry(); {
	case err != nil:
		t.Fatalf("error in call to NextEntry(): %v", err)
	case done:
		t.Fatalf("ended prematurely")
	}
	if err := vd.StartValue(StringType); err != nil {
		t.Fatalf("error in StartValue: %v", err)
	}
	if got, want := vd.Type(), StringType; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	switch val, err := vd.DecodeString(); {
	case err != nil:
		t.Errorf("error decoding value: %v", err)
	case val != "a":
		t.Errorf("got %v, want %v", val, "a")
	}
	if err := vd.FinishValue(); err != nil {
		t.Fatalf("error in FinishValue: %v", err)
	}

	switch done, err := vd.NextEntry(); {
	case err != nil:
		t.Fatalf("error in call to NextEntry(): %v", err)
	case done:
		t.Fatalf("ended prematurely")
	}
	if err := vd.StartValue(StringType); err != nil {
		t.Fatalf("error in StartValue: %v", err)
	}
	if got, want := vd.Type(), StringType; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	switch val, err := vd.DecodeString(); {
	case err != nil:
		t.Errorf("error decoding value: %v", err)
	case val != "b":
		t.Errorf("got %v, want %v", val, "b")
	}
	if err := vd.FinishValue(); err != nil {
		t.Fatalf("error in FinishValue: %v", err)
	}

	switch done, err := vd.NextEntry(); {
	case err != nil:
		t.Fatalf("error in call to NextEntry(): %v", err)
	case !done:
		t.Fatalf("expected end marker")
	}

	if _, err := vd.NextEntry(); err == nil {
		t.Errorf("expected error in final call to NextEntry()")
	}

	if err := vd.FinishValue(); err != nil {
		t.Fatalf("error in FinishValue: %v", err)
	}
}

func TestValueDecoderDecodeList(t *testing.T) {
	testValueDecoderDecodeSequence(t, []string{"a", "b"})
}

func TestValueDecoderDecodeArray(t *testing.T) {
	testValueDecoderDecodeSequence(t, [2]string{"a", "b"})
}

func TestValueDecoderDecodeSet(t *testing.T) { //nolint:gocyclo
	expected := map[string]struct{}{"a": {}, "b": {}}
	expectedType := TypeOf(expected)
	vd := ValueOf(expected).Decoder()
	if err := vd.StartValue(expectedType); err != nil {
		t.Fatalf("error in StartValue: %v", err)
	}
	if got, want := vd.Type(), expectedType; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	switch done, err := vd.NextEntry(); {
	case err != nil:
		t.Fatalf("error in call to NextEntry(): %v", err)
	case done:
		t.Fatalf("ended prematurely")
	}
	if err := vd.StartValue(StringType); err != nil {
		t.Fatalf("error in StartValue: %v", err)
	}
	if got, want := vd.Type(), StringType; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	key, err := vd.DecodeString()
	switch {
	case err != nil:
		t.Errorf("error decoding value: %v", err)
	case key != "a" && key != "b":
		t.Errorf(`got %v, expected "a" or "b"`, key)
	}
	if err := vd.FinishValue(); err != nil {
		t.Fatalf("error in FinishValue: %v", err)
	}

	switch done, err := vd.NextEntry(); {
	case err != nil:
		t.Fatalf("error in call to NextEntry(): %v", err)
	case done:
		t.Fatalf("ended prematurely")
	}
	if err := vd.StartValue(StringType); err != nil {
		t.Fatalf("error in StartValue: %v", err)
	}
	if got, want := vd.Type(), StringType; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	key, err = vd.DecodeString()
	switch {
	case err != nil:
		t.Errorf("error decoding value: %v", err)
	case key != "a" && key != "b":
		t.Errorf(`got %v, expected "a" or "b"`, key)
	}
	if err := vd.FinishValue(); err != nil {
		t.Fatalf("error in FinishValue: %v", err)
	}

	switch done, err := vd.NextEntry(); {
	case err != nil:
		t.Fatalf("error in call to NextEntry(): %v", err)
	case !done:
		t.Fatalf("expected end marker")
	}

	if _, err := vd.NextEntry(); err == nil {
		t.Errorf("expected error in final call to NextEntry()")
	}

	if err := vd.FinishValue(); err != nil {
		t.Fatalf("error in FinishValue: %v", err)
	}
}

func TestValueDecoderDecodeMap(t *testing.T) { //nolint:gocyclo
	expected := map[string]uint32{"a": 3, "b": 7}
	expectedType := TypeOf(expected)
	vd := ValueOf(expected).Decoder()
	if err := vd.StartValue(expectedType); err != nil {
		t.Fatalf("error in StartValue: %v", err)
	}
	if got, want := vd.Type(), expectedType; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	switch done, err := vd.NextEntry(); {
	case err != nil:
		t.Fatalf("error in call to NextEntry(): %v", err)
	case done:
		t.Fatalf("ended prematurely")
	}
	if err := vd.StartValue(StringType); err != nil {
		t.Fatalf("error in StartValue: %v", err)
	}
	if got, want := vd.Type(), StringType; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	key, err := vd.DecodeString()
	switch {
	case err != nil:
		t.Errorf("error decoding value: %v", err)
	case key != "a" && key != "b":
		t.Errorf(`got %v, expected "a" or "b"`, key)
	}
	if err := vd.FinishValue(); err != nil {
		t.Fatalf("error in FinishValue: %v", err)
	}
	if err := vd.StartValue(Uint32Type); err != nil {
		t.Fatalf("error in StartValue: %v", err)
	}
	if got, want := vd.Type(), Uint32Type; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	switch val, err := vd.DecodeUint(32); {
	case err != nil:
		t.Errorf("error decoding value: %v", err)
	case val != uint64(expected[key]):
		t.Errorf("got %v, want %v", expected[key], 3)
	}
	if err := vd.FinishValue(); err != nil {
		t.Fatalf("error in FinishValue: %v", err)
	}

	switch done, err := vd.NextEntry(); {
	case err != nil:
		t.Fatalf("error in call to NextEntry(): %v", err)
	case done:
		t.Fatalf("ended prematurely")
	}
	if err := vd.StartValue(StringType); err != nil {
		t.Fatalf("error in StartValue: %v", err)
	}
	if got, want := vd.Type(), StringType; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	key, err = vd.DecodeString()
	switch {
	case err != nil:
		t.Errorf("error decoding value: %v", err)
	case key != "a" && key != "b":
		t.Errorf(`got %v, expected "a" or "b"`, key)
	}
	if err := vd.FinishValue(); err != nil {
		t.Fatalf("error in FinishValue: %v", err)
	}
	if err := vd.StartValue(Uint32Type); err != nil {
		t.Fatalf("error in StartValue: %v", err)
	}
	if got, want := vd.Type(), Uint32Type; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	switch val, err := vd.DecodeUint(32); {
	case err != nil:
		t.Errorf("error decoding value: %v", err)
	case val != uint64(expected[key]):
		t.Errorf("got %v, want %v", val, expected[key])
	}
	if err := vd.FinishValue(); err != nil {
		t.Fatalf("error in FinishValue: %v", err)
	}

	switch done, err := vd.NextEntry(); {
	case err != nil:
		t.Fatalf("error in call to NextEntry(): %v", err)
	case !done:
		t.Fatalf("expected end marker")
	}

	if _, err := vd.NextEntry(); err == nil {
		t.Errorf("expected error in final call to NextEntry()")
	}

	if err := vd.FinishValue(); err != nil {
		t.Fatalf("error in FinishValue: %v", err)
	}
}

type decoderTestStruct struct {
	A int32
	B []bool
	C string
}

func TestValueDecoderDecodeStruct(t *testing.T) { //nolint:gocyclo
	expected := decoderTestStruct{1, []bool{true, false}, "abc"}
	expectedType := TypeOf(expected)
	dec := ValueOf(expected).Decoder()
	if err := dec.StartValue(expectedType); err != nil {
		t.Errorf("error in StartValue: %v", err)
	}
	if got, want := dec.Type(), expectedType; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	var seen []string
loop:
	for {
		index, err := dec.NextField()
		switch {
		case err != nil:
			t.Fatalf("error in NextField: %v", err)
		case index == -1:
			sort.Strings(seen)
			if !reflect.DeepEqual(seen, []string{"A", "B", "C"}) {
				t.Errorf("unexpected field names received: %v", seen)
			}
			break loop
		}
		switch name := dec.Type().Field(index).Name; name {
		case "A":
			seen = append(seen, name)
			if err := dec.StartValue(Int32Type); err != nil {
				t.Errorf("error in StartValue: %v", err)
			}
			switch val, err := dec.DecodeInt(32); {
			case err != nil:
				t.Errorf("error during decode: %v", err)
			case val != 1:
				t.Errorf("got %v, want %v", val, 1)
			}
			if err := dec.FinishValue(); err != nil {
				t.Errorf("error in FinishValue: %v", err)
			}
		case "B":
			seen = append(seen, name)
			if err := dec.StartValue(expectedType.Field(1).Type); err != nil {
				t.Errorf("error in StartValue: %v", err)
			}

			switch done, err := dec.NextEntry(); {
			case err != nil:
				t.Errorf("error in NextEntry: %v", err)
			case done:
				t.Errorf("unexpected end marker")
			}
			if err := dec.SkipValue(); err != nil {
				t.Errorf("error in IgnoreValue: %v", err)
			}

			switch done, err := dec.NextEntry(); {
			case err != nil:
				t.Errorf("error in NextEntry: %v", err)
			case done:
				t.Errorf("unexpected end marker")
			}
			if err := dec.StartValue(BoolType); err != nil {
				t.Errorf("error in StartValue: %v", err)
			}
			switch val, err := dec.DecodeBool(); {
			case err != nil:
				t.Errorf("error during decode: %v", err)
			case val != false:
				t.Errorf("got %v, want %v", val, false)
			}
			if err := dec.FinishValue(); err != nil {
				t.Errorf("error in FinishValue: %v", err)
			}

			if err := dec.FinishValue(); err != nil {
				t.Errorf("error in FinishValue: %v", err)
			}
		case "C":
			seen = append(seen, name)
			if err := dec.StartValue(StringType); err != nil {
				t.Errorf("error in StartValue: %v", err)
			}
			switch val, err := dec.DecodeString(); {
			case err != nil:
				t.Errorf("error during decode: %v", err)
			case val != "abc":
				t.Errorf("got %v, want %v", val, "abc")
			}
			if err := dec.FinishValue(); err != nil {
				t.Errorf("error in FinishValue: %v", err)
			}
		default:
			t.Fatalf("received unknown field")
		}
	}
	if err := dec.FinishValue(); err != nil {
		t.Errorf("error in FinishValue: %v", err)
	}
}

func TestValueDecoderDecodeUnion(t *testing.T) {
	expectedType := UnionType(Field{"A", BoolType}, Field{"B", StringType})
	expected := ZeroValue(expectedType)
	dec := expected.Decoder()
	if err := dec.StartValue(expectedType); err != nil {
		t.Errorf("error in StartValue: %v", err)
	}
	if got, want := dec.Type(), expectedType; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	switch index, err := dec.NextField(); {
	case err != nil:
		t.Errorf("error in NextField(): %v", err)
	case index != 0:
		t.Errorf("unexpected field index: %v", index)
	}
	if err := dec.StartValue(BoolType); err != nil {
		t.Errorf("error in StartValue: %v", err)
	}
	switch val, err := dec.DecodeBool(); {
	case err != nil:
		t.Errorf("error during decode: %v", err)
	case val != false:
		t.Errorf("got %v, want %v", val, false)
	}
	if err := dec.FinishValue(); err != nil {
		t.Errorf("error in FinishValue: %v", err)
	}
	switch index, err := dec.NextField(); {
	case err != nil:
		t.Errorf("error in NextField(): %v", err)
	case index != -1:
		t.Errorf("unexpected field after end of fields: %v", index)
	}
	if err := dec.FinishValue(); err != nil {
		t.Errorf("error in FinishValue: %v", err)
	}
}
