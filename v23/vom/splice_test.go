// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vom_test

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"v.io/v23/vom"
)

type m struct {
	M string
}

//nolint:errcheck
func ExampleMergeEncodedBytes() {

	abuf := &bytes.Buffer{}
	encA := vom.NewEncoder(abuf)
	encA.Encode(&m{"stream-a"}) //nolint:errcheck

	bbuf := &bytes.Buffer{}
	encB := vom.NewEncoder(bbuf)
	encB.Encode(m{"stream-b"})
	encB.Encode(vom.MergeEncodedBytes{Data: abuf.Bytes()})

	dec := vom.NewDecoder(bbuf)
	va, vb := m{}, m{}
	dec.Decode(&vb)
	dec.Decode(&va)
	fmt.Println(va.M)
	fmt.Println(vb.M)
	// Output:
	// stream-a
	// stream-b
}

//nolint:errcheck
func ExampleExtractEncodedBytes() {
	abuf := &bytes.Buffer{}
	encA := vom.NewEncoder(abuf)
	encA.Encode("first")
	encA.Encode(&m{"stream-a"})
	encA.Encode(int64(33))
	encA.Encode("last")

	var first, last string
	var vm m
	var vi int
	dec := vom.NewDecoder(abuf)
	dec.Decode(&first)
	var extractor vom.ExtractEncodedBytes
	dec.Decode(&extractor)
	edec := vom.NewDecoder(bytes.NewBuffer(extractor.Data))
	edec.Decode(&vm)
	edec.Decode(&vi)
	edec.Decode(&last)
	fmt.Println(extractor.Decoded)
	fmt.Println(first)
	fmt.Println(vm.M)
	fmt.Println(vi)
	fmt.Println(last)
	// Output:
	// 3
	// first
	// stream-a
	// 33
	// last
}

type tt1 struct {
	Dict map[string]bool
}
type tt2 struct {
	Name string
}
type tt3 int

func TestMerge(t *testing.T) {
	v0 := true
	v1 := tt1{map[string]bool{"t": true, "f": false}}
	v2 := tt2{"simple-test"}
	v3 := tt3(42)

	buf := &bytes.Buffer{}
	enc := vom.NewEncoder(buf)
	for _, v := range []interface{}{v1, v2, v3} {
		if err := enc.Encode(v); err != nil {
			t.Fatal(err)
		}
	}

	// Re-encode & write out the pre-encoded bytes from above, the result
	// should be the same byte stream.
	tbuf := &bytes.Buffer{}
	enc = vom.NewEncoder(tbuf)
	if err := enc.Encode(v0); err != nil {
		t.Fatal(err)
	}

	if err := enc.Encode(vom.MergeEncodedBytes{Data: buf.Bytes()}); err != nil {
		t.Fatal(err)
	}

	var (
		r0 bool
		r1 tt1
		r2 tt2
		r3 tt3
	)

	dec := vom.NewDecoder(tbuf)
	if err := dec.Decode(&r0); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&r1); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&r2); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&r3); err != nil {
		t.Fatal(err)
	}
	if got, want := r0, v0; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := r1, v1; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := r2, v2; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := r3, v3; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestExtract(t *testing.T) { //nolint:gocyclo
	v0 := true
	v1 := tt1{map[string]bool{"t": true, "f": false}}
	v2 := tt2{"simple-test"}
	v3 := tt3(42)

	buf := &bytes.Buffer{}
	enc := vom.NewEncoder(buf)
	for _, v := range []interface{}{v0, v1, v2, v3} {
		if err := enc.Encode(v); err != nil {
			t.Fatal(err)
		}
	}
	encoded := buf.Bytes()

	var (
		r0, r10 bool
		r1, r11 tt1
		r2, r12 tt2
		r3, r13 tt3
	)

	dec := vom.NewDecoder(bytes.NewBuffer(encoded))
	if err := dec.Decode(&r0); err != nil {
		t.Fatal(err)
	}

	extractor := vom.ExtractEncodedBytes{NumToDecode: 2}
	if err := dec.Decode(&extractor); err != nil {
		t.Fatal(err)
	}

	edec := vom.NewDecoder(bytes.NewBuffer(extractor.Data))
	if err := edec.Decode(&r1); err != nil {
		t.Fatal(err)
	}
	if err := edec.Decode(&r2); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&r3); err != nil {
		t.Fatal(err)
	}

	dec = vom.NewDecoder(bytes.NewBuffer(encoded))
	if err := dec.Decode(&r10); err != nil {
		t.Fatal(err)
	}
	extractor = vom.ExtractEncodedBytes{}
	if err := dec.Decode(&extractor); err != nil {
		t.Fatal(err)
	}
	edec = vom.NewDecoder(bytes.NewBuffer(extractor.Data))
	if err := edec.Decode(&r11); err != nil {
		t.Fatal(err)
	}
	if err := edec.Decode(&r12); err != nil {
		t.Fatal(err)
	}
	if err := edec.Decode(&r13); err != nil {
		t.Fatal(err)
	}

	if got, want := r0, v0; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := r1, v1; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := r2, v2; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := r3, v3; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := r10, v0; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := r11, v1; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := r12, v2; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := r13, v3; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
