// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom_test

import (
	"bytes"
	"io"
	"reflect"
	"testing"

	"v.io/v23/vdl"
	"v.io/v23/vom"
	"v.io/v23/vom/vomtest"
)

type testUint64 uint64

type structTypeObject struct {
	T *vdl.Type
}

type structAny struct {
	X *vom.RawBytes
}

type structAnyAndTypes struct {
	A *vdl.Type
	B *vom.RawBytes
	C *vdl.Type
	D *vom.RawBytes
}

// Test various combinations of having/not having any and type object.
var rawBytesTestCases = []struct {
	name     string
	goValue  interface{}
	rawBytes vom.RawBytes
}{
	{
		name:    "testUint64(99)",
		goValue: testUint64(99),
		rawBytes: vom.RawBytes{
			Version: vom.DefaultVersion,
			Type:    vdl.TypeOf(testUint64(0)),
			Data:    []byte{0x63},
		},
	},
	{
		name:    "typeobject(int32)",
		goValue: vdl.Int32Type,
		rawBytes: vom.RawBytes{
			Version:  vom.DefaultVersion,
			Type:     vdl.TypeOf(vdl.Int32Type),
			RefTypes: []*vdl.Type{vdl.Int32Type},
			Data:     []byte{0x00},
		},
	},
	{
		name:    "structTypeObject{typeobject(int32)}",
		goValue: structTypeObject{vdl.Int32Type},
		rawBytes: vom.RawBytes{
			Version:  vom.DefaultVersion,
			Type:     vdl.TypeOf(structTypeObject{}),
			RefTypes: []*vdl.Type{vdl.Int32Type},
			Data:     []byte{0x00, 0x00, vom.WireCtrlEnd},
		},
	},
	{
		name: `structAnyAndTypes{typeobject(int32), true, typeobject(bool), "abc"}`,
		goValue: structAnyAndTypes{
			vdl.Int32Type,
			&vom.RawBytes{
				Version: vom.DefaultVersion,
				Type:    vdl.BoolType,
				Data:    []byte{0x01},
			},
			vdl.BoolType,
			&vom.RawBytes{
				Version: vom.DefaultVersion,
				Type:    vdl.TypeOf(""),
				Data:    []byte{0x03, 0x61, 0x62, 0x63},
			},
		},
		rawBytes: vom.RawBytes{
			Version:    vom.DefaultVersion,
			Type:       vdl.TypeOf(structAnyAndTypes{}),
			RefTypes:   []*vdl.Type{vdl.Int32Type, vdl.BoolType, vdl.StringType},
			AnyLengths: []int{1, 4},
			Data: []byte{
				0x00, 0x00, // A
				0x01, 0x01, 0x00, 0x01, // B
				0x02, 0x01, // C
				0x03, 0x02, 0x01, 0x03, 0x61, 0x62, 0x63, // D
				vom.WireCtrlEnd,
			},
		},
	},
	{
		name:    "large message", // to test that multibyte length is encoded properly
		goValue: makeLargeBytes(1000),
		rawBytes: vom.RawBytes{
			Version: vom.DefaultVersion,
			Type:    vdl.ListType(vdl.ByteType),
			Data:    append([]byte{0xfe, 0x03, 0xe8}, makeLargeBytes(1000)...),
		},
	},
	{
		name:    "*vdl.Value",
		goValue: vdl.ValueOf(uint16(5)),
		rawBytes: vom.RawBytes{
			Version: vom.DefaultVersion,
			Type:    vdl.Uint16Type,
			Data:    []byte{0x05},
		},
	},
	{
		name:    "*vdl.Value - top level any",
		goValue: vdl.ValueOf([]interface{}{uint16(5)}).Index(0),
		rawBytes: vom.RawBytes{
			Version: vom.DefaultVersion,
			Type:    vdl.Uint16Type,
			Data:    []byte{0x05},
		},
	},
	{
		name: "any(nil)",
		goValue: &vom.RawBytes{
			Version: vom.DefaultVersion,
			Type:    vdl.AnyType,
			Data:    []byte{vom.WireCtrlNil},
		},
		rawBytes: vom.RawBytes{
			Version: vom.DefaultVersion,
			Type:    vdl.AnyType,
			Data:    []byte{vom.WireCtrlNil},
		},
	},
}

func makeLargeBytes(size int) []byte {
	b := make([]byte, size)
	for i := range b {
		b[i] = byte(i % 10)
	}
	return b
}

func TestDecodeToRawBytes(t *testing.T) {
	for _, test := range rawBytesTestCases {
		bytes, err := vom.Encode(test.goValue)
		if err != nil {
			t.Errorf("%s: Encode failed: %v", test.name, err)
			continue
		}
		var rb vom.RawBytes
		if err := vom.Decode(bytes, &rb); err != nil {
			t.Errorf("%s: Decode failed: %v", test.name, err)
			continue
		}
		if got, want := rb, test.rawBytes; !reflect.DeepEqual(got, want) {
			t.Errorf("%s\nGOT  %v\nWANT %v", test.name, got, want)
		}
	}
}

func TestEncodeFromRawBytes(t *testing.T) {
	for _, test := range rawBytesTestCases {
		fullBytes, err := vom.Encode(test.goValue)
		if err != nil {
			t.Errorf("%s: Encode goValue failed: %v", test.name, err)
			continue
		}
		fullBytesFromRaw, err := vom.Encode(&test.rawBytes)
		if err != nil {
			t.Errorf("%s: Encode RawBytes failed: %v", test.name, err)
			continue
		}
		if got, want := fullBytesFromRaw, fullBytes; !bytes.Equal(got, want) {
			t.Errorf("%s\nGOT  %x\nWANT %x", test.name, got, want)
		}
	}
}

// Same as rawBytesTestCases, but wrapped within structAny
var rawBytesWrappedTestCases = []struct {
	name     string
	goValue  interface{}
	rawBytes vom.RawBytes
}{
	{
		name:    "testUint64(99)",
		goValue: testUint64(99),
		rawBytes: vom.RawBytes{
			Version:    vom.DefaultVersion,
			Type:       vdl.TypeOf(testUint64(0)),
			RefTypes:   []*vdl.Type{vdl.TypeOf(testUint64(0))},
			AnyLengths: []int{1},
			Data:       []byte{0x63},
		},
	},
	{
		name:    "typeobject(int32)",
		goValue: vdl.Int32Type,
		rawBytes: vom.RawBytes{
			Version:    vom.DefaultVersion,
			Type:       vdl.TypeOf(vdl.Int32Type),
			RefTypes:   []*vdl.Type{vdl.TypeObjectType, vdl.Int32Type},
			AnyLengths: []int{1},
			Data:       []byte{0x01},
		},
	},
	{
		name:    "structTypeObject{typeobject(int32)}",
		goValue: structTypeObject{vdl.Int32Type},
		rawBytes: vom.RawBytes{
			Version:    vom.DefaultVersion,
			Type:       vdl.TypeOf(structTypeObject{}),
			RefTypes:   []*vdl.Type{vdl.TypeOf(structTypeObject{}), vdl.Int32Type},
			AnyLengths: []int{3},
			Data:       []byte{0x00, 0x01, 0xe1},
		},
	},
	{
		name: `structAnyAndTypes{typeobject(int32), true, typeobject(bool), "abc"}`,
		goValue: structAnyAndTypes{
			vdl.Int32Type,
			&vom.RawBytes{
				Version: vom.DefaultVersion,
				Type:    vdl.BoolType,
				Data:    []byte{0x01},
			},
			vdl.BoolType,
			&vom.RawBytes{
				Version: vom.DefaultVersion,
				Type:    vdl.TypeOf(""),
				Data:    []byte{0x03, 0x61, 0x62, 0x63},
			},
		},
		rawBytes: vom.RawBytes{
			Version:    vom.DefaultVersion,
			Type:       vdl.TypeOf(structAnyAndTypes{}),
			RefTypes:   []*vdl.Type{vdl.TypeOf(structAnyAndTypes{}), vdl.Int32Type, vdl.BoolType, vdl.StringType},
			AnyLengths: []int{16, 1, 4},
			Data: []byte{
				0x00, 0x01, // A
				0x01, 0x02, 0x01, 0x01, // B
				0x02, 0x02, // C
				0x03, 0x03, 0x02, 0x03, 0x61, 0x62, 0x63, // D
				0xe1,
			},
		},
	},
	{
		name:    "large message", // to test that multibyte length is encoded properly
		goValue: makeLargeBytes(1000),
		rawBytes: vom.RawBytes{
			Version:    vom.DefaultVersion,
			Type:       vdl.ListType(vdl.ByteType),
			RefTypes:   []*vdl.Type{vdl.ListType(vdl.ByteType)},
			AnyLengths: []int{0x3eb},
			Data:       append([]byte{0xfe, 0x03, 0xe8}, makeLargeBytes(1000)...),
		},
	},
	{
		name:    "*vdl.Value",
		goValue: vdl.ValueOf(uint16(5)),
		rawBytes: vom.RawBytes{
			Version:    vom.DefaultVersion,
			Type:       vdl.Uint16Type,
			RefTypes:   []*vdl.Type{vdl.Uint16Type},
			AnyLengths: []int{1},
			Data:       []byte{0x05},
		},
	},
	{
		name:    "*vdl.Value - top level any",
		goValue: vdl.ValueOf([]interface{}{uint16(5)}).Index(0),
		rawBytes: vom.RawBytes{
			Version:    vom.DefaultVersion,
			Type:       vdl.Uint16Type,
			RefTypes:   []*vdl.Type{vdl.Uint16Type},
			AnyLengths: []int{1},
			Data:       []byte{0x05},
		},
	},
}

func TestWrappedRawBytes(t *testing.T) {
	for i, test := range rawBytesWrappedTestCases {
		unwrapped := rawBytesTestCases[i]
		wrappedBytes, err := vom.Encode(structAny{&unwrapped.rawBytes})
		if err != nil {
			t.Errorf("%s: Encode failed: %v", test.name, err)
			continue
		}
		var any structAny
		if err := vom.Decode(wrappedBytes, &any); err != nil {
			t.Errorf("%s: Decode failed: %v", test.name, err)
			continue
		}
		if got, want := any.X, &test.rawBytes; !reflect.DeepEqual(got, want) {
			t.Errorf("%s\nGOT  %v\nWANT %v", test.name, got, want)
		}
	}
}

func TestEncodeNilRawBytes(t *testing.T) {
	// Top-level
	want, err := vom.Encode(vdl.ZeroValue(vdl.AnyType))
	if err != nil {
		t.Fatal(err)
	}
	got, err := vom.Encode((*vom.RawBytes)(nil))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("top-level\nGOT  %x\nWANT %x", got, want)
	}
	// Within an object.
	want, err = vom.Encode([]*vdl.Value{vdl.ZeroValue(vdl.AnyType)})
	if err != nil {
		t.Fatal(err)
	}
	got, err = vom.Encode([]*vom.RawBytes{nil})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("[]any\nGOT  %x\nWANT %x", got, want)
	}
}

func TestVdlTypeOfRawBytes(t *testing.T) {
	if got, want := vdl.TypeOf(&vom.RawBytes{}), vdl.AnyType; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestVdlValueOfRawBytes(t *testing.T) {
	for _, test := range rawBytesTestCases {
		want := vdl.ValueOf(test.goValue)
		got := vdl.ValueOf(test.rawBytes)
		if !vdl.EqualValue(got, want) {
			t.Errorf("vdl.ValueOf(RawBytes) %s: got %v, want %v", test.name, got, want)
		}
	}
}

func TestConvertRawBytes(t *testing.T) {
	for _, test := range rawBytesTestCases {
		var rb *vom.RawBytes
		if err := vdl.Convert(&rb, test.goValue); err != nil {
			t.Errorf("%s: Convert failed: %v", test.name, err)
		}
		if got, want := rb, &test.rawBytes; !reflect.DeepEqual(got, want) {
			t.Errorf("%s\nGOT  %v\nWANT %v", test.name, got, want)
		}
	}
}

type structAnyInterface struct {
	X interface{}
}

func TestConvertRawBytesWrapped(t *testing.T) {
	for _, test := range rawBytesTestCases {
		var any structAny
		src := structAnyInterface{test.goValue}
		if err := vdl.Convert(&any, src); err != nil {
			t.Errorf("%s: Convert failed: %v", test.name, err)
		}
		got, want := any, structAny{&test.rawBytes}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s\nGOT  %v\nWANT %v", test.name, got, want)
		}
	}
}

// Ensure that the type id lists aren't corrupted when there
// are more ids in the encoder/decoder.
func TestReusedDecoderEncoderRawBytes(t *testing.T) {
	var buf bytes.Buffer
	enc := vom.NewEncoder(&buf)
	if err := enc.Encode(structAnyInterface{int64(4)}); err != nil {
		t.Fatalf("error on encode: %v", err)
	}
	if err := enc.Encode(structAnyInterface{"a"}); err != nil {
		t.Fatalf("error on encode: %v", err)
	}

	dec := vom.NewDecoder(bytes.NewReader(buf.Bytes()))
	var x structAny
	if err := dec.Decode(&x); err != nil {
		t.Fatalf("error on decode: %v", err)
	}
	var i int64
	if err := x.X.ToValue(&i); err != nil {
		t.Fatalf("error on value convert: %v", err)
	}
	if got, want := i, int64(4); got != want {
		t.Errorf("got %d, want %d", got, want)
	}
	var y structAny
	if err := dec.Decode(&y); err != nil {
		t.Fatalf("error on decode: %v", err)
	}
	var str string
	if err := y.X.ToValue(&str); err != nil {
		t.Fatalf("error on value convert: %v", err)
	}
	if got, want := str, "a"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestRawBytesString(t *testing.T) {
	tests := []struct {
		input    *vom.RawBytes
		expected string
	}{
		{
			input: &vom.RawBytes{
				vom.Version81,
				vdl.Int8Type,
				[]*vdl.Type{vdl.BoolType, vdl.StringType},
				[]int{4},
				[]byte{0xfa, 0x0e, 0x9d, 0xcc}},
			expected: "RawBytes{Version81, int8, RefTypes{bool, string}, AnyLengths{4}, fa0e9dcc}",
		},
		{
			input: &vom.RawBytes{
				vom.Version81,
				vdl.Int8Type,
				[]*vdl.Type{vdl.BoolType},
				[]int{},
				[]byte{0xfa, 0x0e, 0x9d, 0xcc}},
			expected: "RawBytes{Version81, int8, RefTypes{bool}, fa0e9dcc}",
		},
		{
			input: &vom.RawBytes{
				vom.Version81,
				vdl.Int8Type,
				nil,
				nil,
				[]byte{0xfa, 0x0e, 0x9d, 0xcc}},
			expected: "RawBytes{Version81, int8, fa0e9dcc}",
		},
	}
	for _, test := range tests {
		if got, want := test.input.String(), test.expected; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

func TestRawBytesVDLType(t *testing.T) {
	if got, want := vdl.TypeOf(vom.RawBytes{}), vdl.AnyType; got != want {
		t.Errorf("vom.RawBytes{} got %v, want %v", got, want)
	}
	if got, want := vdl.TypeOf((*vom.RawBytes)(nil)), vdl.AnyType; got != want {
		t.Errorf("vom.RawBytes{} got %v, want %v", got, want)
	}
}

type simpleStruct struct {
	X int16
}

// Ensure that RawBytes does not interpret the value portion of the vom message
// by ensuring that Decoding/Encoding to/from RawBytes doesn't fail when the
// value portion of the message is invalid vom.
func TestRawBytesNonVomPayload(t *testing.T) {
	// Compute expected type message bytes. A struct without any/typeobject is
	// used for this because it has a message length but is neither something that
	// could be special cased like a list of bytes nor has the complication
	// of dealing with the format for describing types with any in the
	// value message header.
	var typeBuf, buf bytes.Buffer
	typeEnc := vom.NewTypeEncoder(&typeBuf)
	if err := vom.NewEncoderWithTypeEncoder(&buf, typeEnc).Encode(simpleStruct{5}); err != nil {
		t.Fatalf("failure when preparing type message bytes: %v", t)
	}
	var inputMessage []byte = typeBuf.Bytes()
	inputMessage = append(inputMessage, byte(vom.WireIdFirstUserType*2)) // New value message tid
	inputMessage = append(inputMessage, 8)                               // Message length
	// non-vom bytes (invalid because there is no struct field at index 10)
	dataPortion := []byte{10, 20, 30, 40, 50, 60, 70, 80}
	inputMessage = append(inputMessage, dataPortion...)

	// Now ensure decoding into a RawBytes works correctly.
	var rb *vom.RawBytes
	if err := vom.Decode(inputMessage, &rb); err != nil {
		t.Fatalf("error decoding %x into a RawBytes: %v", inputMessage, err)
	}
	if got, want := rb.Type, vdl.TypeOf(simpleStruct{}); got != want {
		t.Errorf("Type: got %v, want %v", got, want)
	}
	if got, want := rb.Data, dataPortion; !bytes.Equal(got, want) {
		t.Errorf("Data: got %x, want %x", got, want)
	}

	// Now re-encode and ensure that we get the original bytes.
	encoded, err := vom.Encode(rb)
	if err != nil {
		t.Fatalf("error encoding RawBytes %v: %v", rb, err)
	}
	if got, want := encoded, inputMessage; !bytes.Equal(got, want) {
		t.Errorf("encoded RawBytes. got %v, want %v", got, want)
	}
}

func TestRawBytesDecodeEncode(t *testing.T) {
	for _, test := range vomtest.AllPass() {
		// Interleaved
		rb := vom.RawBytes{}
		interleavedReader := bytes.NewReader(test.Bytes())
		if err := vom.NewDecoder(interleavedReader).Decode(&rb); err != nil {
			t.Errorf("%s: decode failed: %v", test.Name(), err)
			continue
		}
		if _, err := interleavedReader.ReadByte(); err != io.EOF {
			t.Errorf("%s: expected EOF, but got %v", test.Name(), err)
			continue
		}

		var out bytes.Buffer
		enc := vom.NewVersionedEncoder(test.Version, &out)
		if err := enc.Encode(&rb); err != nil {
			t.Errorf("%s: encode failed: %v\nRawBytes: %v", test.Name(), err, rb)
			continue
		}
		if got, want := out.Bytes(), test.Bytes(); !bytes.Equal(got, want) {
			t.Errorf("%s\nGOT  %x\nWANT %x", test.Name(), got, want)
		}

		// Split type and value stream.
		rb = vom.RawBytes{}
		typeReader := bytes.NewReader(test.TypeBytes())
		typeDec := vom.NewTypeDecoder(typeReader)
		typeDec.Start()
		defer typeDec.Stop()
		valueReader := bytes.NewReader(test.ValueBytes())
		if err := vom.NewDecoderWithTypeDecoder(valueReader, typeDec).Decode(&rb); err != nil {
			t.Errorf("%s: decode failed: %v", test.Name(), err)
			continue
		}
		if _, err := typeReader.ReadByte(); err != io.EOF {
			t.Errorf("%s: type reader got %v, want EOF", test.Name(), err)
			continue
		}
		if _, err := valueReader.ReadByte(); err != io.EOF {
			t.Errorf("%s: value reader got %v, want EOF", test.Name(), err)
			continue
		}

		out.Reset()
		var typeOut bytes.Buffer
		typeEnc := vom.NewVersionedTypeEncoder(test.Version, &typeOut)
		enc = vom.NewVersionedEncoderWithTypeEncoder(test.Version, &out, typeEnc)
		if err := enc.Encode(&rb); err != nil {
			t.Errorf("%s: encode failed: %v\nRawBytes: %v", test.Name(), err, rb)
			continue
		}
		if got, want := typeOut.Bytes(), test.TypeBytes(); !bytes.Equal(got, want) {
			t.Errorf("%s: type bytes\nGOT  %x\nWANT %x", test.Name(), got, want)
		}
		if got, want := out.Bytes(), test.ValueBytes(); !bytes.Equal(got, want) {
			t.Errorf("%s: value bytes\nGOT  %x\nWANT %x", test.Name(), got, want)
		}
	}
}

func TestRawBytesToFromValue(t *testing.T) {
	for _, test := range vomtest.AllPass() {
		rb, err := vom.RawBytesFromValue(test.Value.Interface())
		if err != nil {
			t.Errorf("%v %s: RawBytesFromValue failed: %v", test.Version, test.Name(), err)
			continue
		}
		var vv *vdl.Value
		if err := rb.ToValue(&vv); err != nil {
			t.Errorf("%v %s: rb.ToValue failed: %v", test.Version, test.Name(), err)
			continue
		}
		if got, want := vv, vdl.ValueOf(test.Value.Interface()); !vdl.EqualValue(got, want) {
			t.Errorf("%v %s\nGOT  %v\nWANT %v", test.Version, test.Name(), got, want)
		}
	}
}

func TestRawBytesDecoder(t *testing.T) {
	for _, test := range vomtest.AllPass() {
		tt, err := vdl.TypeFromReflect(test.Value.Type())
		if err != nil {
			t.Errorf("%s: TypeFromReflect failed: %v", test.Name(), err)
			continue
		}
		in, err := vom.RawBytesFromValue(test.Value.Interface())
		if err != nil {
			t.Errorf("%s: RawBytesFromValue failed: %v", test.Name(), err)
			continue
		}
		out := vdl.ZeroValue(tt)
		if err := out.VDLRead(in.Decoder()); err != nil {
			t.Errorf("%s: VDLRead failed: %v", test.Name(), err)
			continue
		}
		if got, want := out, vdl.ValueOf(test.Value.Interface()); !vdl.EqualValue(got, want) {
			t.Errorf("%s\nGOT  %v\nWANT %v", test.Name(), got, want)
		}
	}
}

func TestRawBytesWriter(t *testing.T) {
	for _, test := range vomtest.AllPass() {
		var buf bytes.Buffer
		enc := vom.NewEncoder(&buf)
		rb, err := vom.RawBytesFromValue(test.Value.Interface())
		if err != nil {
			t.Errorf("%s: RawBytesFromValue failed: %v", test.Name(), err)
			continue
		}
		if err := rb.VDLWrite(enc.Encoder()); err != nil {
			t.Errorf("%s: VDLWrite failed: %v", test.Name(), err)
			continue
		}
		if got, want := buf.Bytes(), test.Bytes(); !bytes.Equal(got, want) {
			t.Errorf("%s\nGOT  %x\nWANT %x", test.Name(), got, want)
		}
	}
}
