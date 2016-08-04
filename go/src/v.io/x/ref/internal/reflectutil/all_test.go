// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflectutil

import (
	"reflect"
	"testing"
	"unsafe"
)

type (
	Struct struct {
		A uint
		B string
	}

	Recurse struct {
		U uint
		R *Recurse
	}

	RecurseA struct {
		Ua uint
		B  *RecurseB
	}
	RecurseB struct {
		Ub uint
		A  *RecurseA
	}

	abIntPtr struct {
		A, B *int
	}
)

var (
	recurseCycle   *Recurse  = &Recurse{}
	recurseABCycle *RecurseA = &RecurseA{}

	intPtr1a *int = new(int)
	intPtr1b *int = new(int)

	iface interface{}
)

func init() {
	recurseCycle.U = 5
	recurseCycle.R = recurseCycle

	recurseABCycle.Ua = 5
	recurseABCycle.B = &RecurseB{6, recurseABCycle}

	*intPtr1a = 1
	*intPtr1b = 1
}

func TestDeepEqual(t *testing.T) {
	tests := []struct {
		a, b   interface{}
		expect bool
	}{
		{true, true, true},
		{1, 1, true},
		{-1, -1, true},
		{1.1, 1.1, true},
		{"abc", "abc", true},
		{1 + 1i, 1 + 1i, true},
		{[2]uint{1, 1}, [2]uint{1, 1}, true},
		{[]uint{1, 1}, []uint{1, 1}, true},
		{map[uint]string{1: "1", 2: "2"}, map[uint]string{1: "1", 2: "2"}, true},
		{Struct{1, "a"}, Struct{1, "a"}, true},
		{recurseCycle, recurseCycle, true},
		{recurseABCycle, recurseABCycle, true},
		{abIntPtr{intPtr1a, intPtr1a}, abIntPtr{intPtr1a, intPtr1a}, true},
		{abIntPtr{intPtr1a, intPtr1b}, abIntPtr{intPtr1a, intPtr1b}, true},

		{true, false, false},
		{1, 2, false},
		{-1, -2, false},
		{1.1, 2.2, false},
		{"abc", "def", false},
		{1 + 1i, 2 + 2i, false},
		{[2]uint{1, 1}, [2]uint{2, 2}, false},
		{[]uint{1, 1}, []uint{2, 2}, false},
		{map[uint]string{1: "1", 2: "2"}, map[uint]string{3: "3", 4: "4"}, false},
		{Struct{1, "a"}, Struct{1, "b"}, false},
		{recurseCycle, &Recurse{5, &Recurse{5, nil}}, false},
		{recurseABCycle, &RecurseA{5, &RecurseB{6, nil}}, false},
		{abIntPtr{intPtr1a, intPtr1a}, abIntPtr{intPtr1a, intPtr1b}, false},
		{abIntPtr{intPtr1a, intPtr1b}, abIntPtr{intPtr1a, intPtr1a}, false},
	}
	for _, test := range tests {
		actual := DeepEqual(test.a, test.b, &DeepEqualOpts{})
		if actual != test.expect {
			t.Errorf("DeepEqual(%#v, %#v) != %v", test.a, test.b, test.expect)
		}
	}
}

func TestAreComparable(t *testing.T) {
	tests := []struct {
		a, b   interface{}
		expect bool
	}{
		{true, true, true},
		{"", "", true},
		{uint(0), uint(0), true},
		{uint8(0), uint8(0), true},
		{uint16(0), uint16(0), true},
		{uint32(0), uint32(0), true},
		{uint64(0), uint64(0), true},
		{uintptr(0), uintptr(0), true},
		{int(0), int(0), true},
		{int8(0), int8(0), true},
		{int16(0), int16(0), true},
		{int32(0), int32(0), true},
		{int64(0), int64(0), true},
		{float32(0), float32(0), true},
		{float64(0), float64(0), true},
		{complex64(0), complex64(0), true},
		{complex128(0), complex128(0), true},
		{[2]uint{1, 1}, [2]uint{1, 1}, true},
		{[]uint{1, 1}, []uint{1, 1}, true},
		{Struct{1, "a"}, Struct{1, "a"}, true},
		{(*int)(nil), (*int)(nil), true},
		{recurseCycle, recurseCycle, true},
		{recurseABCycle, recurseABCycle, true},
		{abIntPtr{intPtr1a, intPtr1a}, abIntPtr{intPtr1a, intPtr1a}, true},
		{abIntPtr{intPtr1a, intPtr1b}, abIntPtr{intPtr1a, intPtr1b}, true},

		{map[uint]string{1: "1"}, map[uint]string{1: "1"}, false},
		{&iface, &iface, false},
		{make(chan int), make(chan int), false},
		{TestAreComparable, TestAreComparable, false},
		{unsafe.Pointer(nil), unsafe.Pointer(nil), false},
	}
	for _, test := range tests {
		actual := AreComparable(test.a, test.b)
		if actual != test.expect {
			t.Errorf("AreComparable(%#v, %#v) != %v", test.a, test.b, test.expect)
		}
	}
}

func TestLess(t *testing.T) {
	for _, test := range compareTests {
		actual := Less(test.a, test.b)
		expect := false
		if test.expect == -1 {
			expect = true // For eq and gt we expect Less to return false.
		}
		if actual != expect {
			t.Errorf("Less(%#v, %#v) != %v", test.a, test.b, expect)
		}
	}
}

func TestCompare(t *testing.T) {
	for _, test := range compareTests {
		actual := Compare(test.a, test.b)
		if actual != test.expect {
			t.Errorf("Compare(%#v, %#v) != %v", test.a, test.b, test.expect)
		}
	}
}

var compareTests = []struct {
	a, b   interface{}
	expect int
}{
	{false, true, -1},
	{false, false, 0},
	{true, false, +1},
	{true, true, 0},

	{"", "aa", -1},
	{"a", "aa", -1},
	{"aa", "ab", -1},
	{"aa", "b", -1},
	{"", "", 0},
	{"aa", "", +1},
	{"aa", "a", +1},
	{"ab", "aa", +1},
	{"b", "aa", +1},

	{uint(0), uint(1), -1},
	{uint(0), uint(0), 0},
	{uint(1), uint(0), +1},
	{uint(1), uint(1), 0},

	{int(-1), int(+1), -1},
	{int(-1), int(-1), 0},
	{int(+1), int(-1), +1},
	{int(+1), int(+1), 0},

	{float32(-1.1), float32(+1.1), -1},
	{float32(-1.1), float32(-1.1), 0},
	{float32(+1.1), float32(-1.1), +1},
	{float32(+1.1), float32(+1.1), 0},

	{complex64(1 + 1i), complex64(1 + 2i), -1},
	{complex64(1 + 2i), complex64(2 + 1i), -1},
	{complex64(1 + 2i), complex64(2 + 2i), -1},
	{complex64(1 + 2i), complex64(2 + 3i), -1},
	{complex64(1 + 1i), complex64(1 + 1i), 0},
	{complex64(1 + 2i), complex64(1 + 1i), +1},
	{complex64(2 + 1i), complex64(1 + 2i), +1},
	{complex64(2 + 2i), complex64(1 + 2i), +1},
	{complex64(2 + 3i), complex64(1 + 2i), +1},

	{[2]int{1, 1}, [2]int{1, 2}, -1},
	{[2]int{1, 2}, [2]int{2, 1}, -1},
	{[2]int{1, 2}, [2]int{2, 2}, -1},
	{[2]int{1, 2}, [2]int{2, 3}, -1},
	{[2]int{1, 1}, [2]int{1, 1}, 0},
	{[2]int{1, 2}, [2]int{1, 1}, +1},
	{[2]int{2, 1}, [2]int{1, 2}, +1},
	{[2]int{2, 2}, [2]int{1, 2}, +1},
	{[2]int{2, 3}, [2]int{1, 2}, +1},

	{[]int{}, []int{1, 1}, -1},
	{[]int{1}, []int{1, 1}, -1},
	{[]int{1, 1}, []int{}, +1},
	{[]int{1, 1}, []int{1}, +1},
	{[]int{1, 1}, []int{1, 2}, -1},
	{[]int{1, 2}, []int{2, 1}, -1},
	{[]int{1, 2}, []int{2, 2}, -1},
	{[]int{1, 2}, []int{2, 3}, -1},
	{[]int{1, 1}, []int{1, 1}, 0},
	{[]int{1, 2}, []int{1, 1}, +1},
	{[]int{2, 1}, []int{1, 2}, +1},
	{[]int{2, 2}, []int{1, 2}, +1},
	{[]int{2, 3}, []int{1, 2}, +1},

	{Struct{1, "a"}, Struct{1, "b"}, -1},
	{Struct{1, "b"}, Struct{2, "a"}, -1},
	{Struct{1, "b"}, Struct{2, "b"}, -1},
	{Struct{1, "b"}, Struct{2, "c"}, -1},
	{Struct{1, "a"}, Struct{1, "a"}, 0},
	{Struct{1, "b"}, Struct{1, "a"}, +1},
	{Struct{2, "a"}, Struct{1, "b"}, +1},
	{Struct{2, "b"}, Struct{1, "b"}, +1},
	{Struct{2, "c"}, Struct{1, "b"}, +1},

	{(*Struct)(nil), &Struct{1, "a"}, -1},
	{&Struct{1, "a"}, &Struct{1, "b"}, -1},
	{&Struct{1, "b"}, &Struct{2, "a"}, -1},
	{&Struct{1, "b"}, &Struct{2, "b"}, -1},
	{&Struct{1, "b"}, &Struct{2, "c"}, -1},
	{(*Struct)(nil), (*Struct)(nil), 0},
	{&Struct{1, "a"}, (*Struct)(nil), +1},
	{&Struct{1, "a"}, &Struct{1, "a"}, 0},
	{&Struct{1, "b"}, &Struct{1, "a"}, +1},
	{&Struct{2, "a"}, &Struct{1, "b"}, +1},
	{&Struct{2, "b"}, &Struct{1, "b"}, +1},
	{&Struct{2, "c"}, &Struct{1, "b"}, +1},
}

type v []interface{}

func toRVS(values v) (rvs []reflect.Value) {
	for _, val := range values {
		rvs = append(rvs, reflect.ValueOf(val))
	}
	return
}

func fromRVS(rvs []reflect.Value) (values v) {
	for _, rv := range rvs {
		values = append(values, rv.Interface())
	}
	return
}

func TestTrySortValues(t *testing.T) {
	tests := []struct {
		values v
		expect v
	}{
		{
			v{true, false},
			v{false, true},
		},
		{
			v{"c", "b", "a"},
			v{"a", "b", "c"},
		},
		{
			v{3, 1, 2},
			v{1, 2, 3},
		},
		{
			v{3.3, 1.1, 2.2},
			v{1.1, 2.2, 3.3},
		},
		{
			v{3 + 3i, 1 + 1i, 2 + 2i},
			v{1 + 1i, 2 + 2i, 3 + 3i},
		},
		{
			v{[1]int{3}, [1]int{1}, [1]int{2}},
			v{[1]int{1}, [1]int{2}, [1]int{3}},
		},
		{
			v{[]int{3}, []int{}, []int{2, 2}},
			v{[]int{}, []int{2, 2}, []int{3}},
		},
		{
			v{Struct{3, "c"}, Struct{1, "a"}, Struct{2, "b"}},
			v{Struct{1, "a"}, Struct{2, "b"}, Struct{3, "c"}},
		},
		{
			v{&Struct{3, "c"}, (*Struct)(nil), &Struct{2, "b"}},
			v{(*Struct)(nil), &Struct{2, "b"}, &Struct{3, "c"}},
		},
	}
	for _, test := range tests {
		actual := fromRVS(TrySortValues(toRVS(test.values)))
		if !reflect.DeepEqual(actual, test.expect) {
			t.Errorf("TrySortValues(%v) got %v, want %v", test.values, actual, test.expect)
		}
	}
}

func TestOptionSliceEqNilEmpty(t *testing.T) {
	tests := []struct {
		first               interface{}
		second              interface{}
		resultWithoutOption bool
		resultWithOption    bool
	}{
		{
			[]int{}, []int{}, true, true,
		},
		{
			[]int(nil), []int(nil), true, true,
		},
		{
			[]int{}, []int(nil), false, true,
		},
		{
			[]([]int){([]int)(nil)}, []([]int){[]int{}}, false, true,
		},
	}

	for _, nilEqOpt := range []bool{true, false} {
		for _, test := range tests {
			options := &DeepEqualOpts{
				SliceEqNilEmpty: nilEqOpt,
			}

			result := DeepEqual(test.first, test.second, options)

			if nilEqOpt {
				if result != test.resultWithOption {
					t.Errorf("Unexpected result with SliceEqNilEmpty option: inputs %#v and %#v. Got %v, expected: %v", test.first, test.second, result, test.resultWithOption)
				}
			} else {
				if result != test.resultWithoutOption {
					t.Errorf("Unexpected result without SliceEqNilEmpty option: inputs %#v and %#v. Got %v, expected: %v", test.first, test.second, result, test.resultWithoutOption)
				}
			}
		}
	}
}
