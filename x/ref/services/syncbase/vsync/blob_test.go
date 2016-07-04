// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vsync

// Tests for Blob support in Syncbase.

import (
	"reflect"
	"testing"

	wire "v.io/v23/services/syncbase"
	"v.io/v23/vdl"
	"v.io/v23/vom"
	_ "v.io/x/ref/runtime/factories/generic"
	td "v.io/x/ref/services/syncbase/vsync/testdata"
)

// checkBlobRefs takes a data entry and a list of expected blob refs expected to
// be found in it.  First it VOM-encodes the data entry, then VOM-decodes it and
// extracts all blob refs in it.  Finally it compares the blob refs found to the
// ones expected (given in string format).
func checkBlobRefs(t *testing.T, msg string, data interface{}, exp ...string) {
	buf, err := vom.Encode(data)
	if err != nil {
		t.Errorf("vom encode failed: %s: %v", msg, err)
		return
	}

	var val *vdl.Value
	if err = vom.Decode(buf, &val); err != nil {
		t.Errorf("vom decode failed: %s: %v", msg, err)
		return
	}

	brs := extractBlobRefs(val)

	expBrs := make(map[wire.BlobRef]struct{})
	for _, e := range exp {
		expBrs[wire.BlobRef(e)] = struct{}{}
	}

	if !reflect.DeepEqual(brs, expBrs) {
		t.Errorf("wrong blob refs extracted: %s: got %v, want %v", msg, brs, expBrs)
	}
}

// TestExtractBlobRefs checks the extraction of blob refs from various datatypes
// with different kinds of nesting.
func TestExtractBlobRefs(t *testing.T) {
	checkBlobRefs(t, "nil data", nil)

	br := wire.BlobRef("123")
	checkBlobRefs(t, "simple BlobRef", br, "123")

	type test1 struct {
		A int64
		B string
		C wire.BlobRef
	}
	v1 := test1{10, "foo", wire.BlobRef("456")}
	checkBlobRefs(t, "struct with BlobRef", v1, "456")

	v2 := struct {
		A int64
		B string
		C test1
	}{11, "bar", v1}
	checkBlobRefs(t, "nested struct with BlobRef", v2, "456")

	v3 := struct {
		A string
		B []wire.BlobRef
	}{"hello", []wire.BlobRef{"111", "222", "333"}}
	checkBlobRefs(t, "struct with BlobRef array", v3, "111", "222", "333")

	v4 := struct {
		A string
		B map[wire.BlobRef]bool
	}{"hello", make(map[wire.BlobRef]bool)}
	for _, b := range []string{"999", "888", "777"} {
		v4.B[wire.BlobRef(b)] = true
	}
	checkBlobRefs(t, "struct with map of BlobRef keys", v4, "777", "888", "999")

	v5 := struct {
		A map[string]wire.BlobRef
		B int
	}{make(map[string]wire.BlobRef), 99}
	for _, b := range []string{"11", "22", "33"} {
		v5.A[b] = wire.BlobRef(b)
	}
	checkBlobRefs(t, "struct with map of BlobRef values", v5, "11", "22", "33")

	v6 := td.BlobUnionNum{45}
	checkBlobRefs(t, "union w/o blob ref", v6)

	v7 := td.BlobUnionBi{td.BlobInfo{"foobar", wire.BlobRef("xyz")}}
	checkBlobRefs(t, "union with blob ref", v7, "xyz")

	// Array of union entries, the 1st with no blob ref, the 2nd with a
	// blob ref to verify that the early-break optimization doesn't apply
	// when a union is used.
	v7b := struct {
		A string
		B []td.BlobUnion
	}{"hello", nil}
	v7b.B = append(v7b.B, v6)
	v7b.B = append(v7b.B, v7)
	checkBlobRefs(t, "struct with array of union", v7b, "xyz")

	v8 := td.BlobSet{"foobar", make(map[wire.BlobRef]struct{})}
	for _, b := range []string{"haha", "hoho", "hihi"} {
		v8.Bs[wire.BlobRef(b)] = struct{}{}
	}
	checkBlobRefs(t, "struct with blob set", v8, "haha", "hoho", "hihi")

	// Blob refs in an array of "any" entries with the first blob ref
	// appearing later in the array to verify that the early-break
	// optimization doesn't apply when "any" is used.
	v9 := td.BlobAny{"foobar", nil}
	v9.Baa = append(v9.Baa, vom.RawBytesOf("xxxxx"))
	v9.Baa = append(v9.Baa, vom.RawBytesOf(999999))
	v9.Baa = append(v9.Baa, vom.RawBytesOf(br))
	v9.Baa = append(v9.Baa, vom.RawBytesOf(v1))
	v9.Baa = append(v9.Baa, vom.RawBytesOf(v2))
	v9.Baa = append(v9.Baa, vom.RawBytesOf(v3))
	v9.Baa = append(v9.Baa, vom.RawBytesOf(v4))
	v9.Baa = append(v9.Baa, vom.RawBytesOf(v5))
	v9.Baa = append(v9.Baa, vom.RawBytesOf(v6))
	v9.Baa = append(v9.Baa, vom.RawBytesOf(v7))
	v9.Baa = append(v9.Baa, vom.RawBytesOf(v8))
	checkBlobRefs(t, "struct with array of any", v9, "456", "123", "111", "222", "333",
		"777", "888", "999", "11", "22", "33", "xyz", "haha", "hoho", "hihi")

	v10 := struct {
		A string
		B []int
	}{"hello", []int{111, 222, 333}}
	checkBlobRefs(t, "struct with array of non-blobs (early break)", v10)

	v11 := struct {
		A string
		B map[string]int
	}{"hello", map[string]int{"abc": 111, "def": 222, "ghi": 333}}
	checkBlobRefs(t, "struct with map of non-blobs (early break)", v11)

	v12 := td.NonBlobSet{"foobar", make(map[string]struct{})}
	for _, b := range []string{"haha", "hoho", "hihi"} {
		v12.S[b] = struct{}{}
	}
	checkBlobRefs(t, "struct with non-blob set (early break)", v12)

	v13 := struct {
		A map[wire.BlobRef]wire.BlobRef
		B int
	}{make(map[wire.BlobRef]wire.BlobRef), 99}
	for _, b := range []string{"11", "22"} {
		v13.A[wire.BlobRef("k-"+b)] = wire.BlobRef("v-" + b)
	}
	checkBlobRefs(t, "struct with map of BlobRef keys and values", v13, "k-11", "v-11", "k-22", "v-22")

	v14 := struct {
		A string
		B [3]int
	}{"hello", [3]int{111, 222, 333}}
	checkBlobRefs(t, "struct with fixed-array of non-blobs (early break)", v14)

	v15 := struct {
		A string
		B [3]wire.BlobRef
	}{"hello", [3]wire.BlobRef{"111", "222", "333"}}
	checkBlobRefs(t, "struct with fixed-array of blobs", v15, "111", "222", "333")

	v16 := td.BlobOpt{"foobar", nil}
	checkBlobRefs(t, "struct with nil optional blob", v16)

	v17 := td.BlobOpt{"foobar", &td.BlobInfo{"haha", wire.BlobRef("xyz")}}
	checkBlobRefs(t, "struct with non-nil optional blob", v17, "xyz")
}
