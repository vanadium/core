// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package signature

import (
	"reflect"
	"testing"

	"v.io/v23/vdl"
)

var (
	methodA1 = Method{Name: "A", Doc: "docA1"}
	methodB1 = Method{Name: "B", Doc: "docB1",
		InArgs:    []Arg{{"b1in0", "b1indoc0", vdl.BoolType}},
		OutArgs:   []Arg{{"b1out0", "b1outdoc0", vdl.StringType}},
		InStream:  &Arg{"b1ins", "b1insdoc", vdl.Int32Type},
		OutStream: &Arg{"b1outs", "b1outsdoc", vdl.Int64Type},
		Tags:      []*vdl.Value{vdl.BoolValue(nil, true), vdl.StringValue(nil, "abc"), vdl.IntValue(vdl.Int64Type, 123)},
	}
	methodB2 = Method{Name: "B", Doc: "docB2",
		InArgs:    []Arg{{"b2in0", "b2indoc0", vdl.AnyType}},
		OutArgs:   []Arg{{"b2out0", "b2outdoc0", vdl.ErrorType}},
		InStream:  &Arg{"b2ins", "b2insdoc", vdl.Uint32Type},
		OutStream: &Arg{"b2outs", "b2outsdoc", vdl.Uint64Type},
		Tags:      []*vdl.Value{vdl.StringValue(nil, "def")},
	}
	methodC1 = Method{Name: "C", Doc: "docC1"}
	methodC2 = Method{Name: "C", Doc: "docC2"}
	methodC3 = Method{Name: "C", Doc: "docC3"}

	ifaceABC = Interface{
		Name:    "ABC",
		PkgPath: "a/b/c",
		Doc:     "DocABC",
		Embeds:  []Embed{{"1", "1", "1"}},
		Methods: []Method{methodA1, methodB1, methodC1},
	}
	ifaceCBA = Interface{
		Name:    "CBA",
		PkgPath: "c/b/a",
		Doc:     "DocCBA",
		Embeds:  []Embed{{"2", "2", "2"}},
		Methods: []Method{methodC1, methodB1, methodA1},
	}
	ifaceC1 = Interface{
		Name:    "C1",
		PkgPath: "c/1",
		Doc:     "DocC1",
		Embeds:  []Embed{{"3", "3", "3"}},
		Methods: []Method{methodC1},
	}
	ifaceC2B1 = Interface{
		Name:    "C2B1",
		PkgPath: "c/2/b/1",
		Doc:     "DocC2B1",
		Embeds:  []Embed{{"4", "4", "4"}},
		Methods: []Method{methodC2, methodB1},
	}
	ifaceC3B2A1 = Interface{
		Name:    "C3B2A1",
		PkgPath: "c/3/b/2/a/1",
		Doc:     "DocC3B2A1",
		Embeds:  []Embed{{"5", "5", "5"}},
		Methods: []Method{methodC3, methodB2, methodA1},
	}
	sigABC   = []Interface{ifaceABC}
	sigCBA   = []Interface{ifaceCBA}
	sigMulti = []Interface{ifaceC1, ifaceC2B1, ifaceC3B2A1}
)

func TestFindMethod(t *testing.T) {
	tests := []struct {
		Iface Interface
		Name  string
		Want  *Method
	}{
		{ifaceABC, "A", &methodA1},
		{ifaceABC, "B", &methodB1},
		{ifaceABC, "C", &methodC1},
		{ifaceABC, "foo", nil},
		{ifaceCBA, "A", &methodA1},
		{ifaceCBA, "B", &methodB1},
		{ifaceCBA, "C", &methodC1},
		{ifaceCBA, "foo", nil},
	}
	for _, test := range tests {
		method, ok := test.Iface.FindMethod(test.Name)
		if got, want := ok, test.Want != nil; got != want {
			t.Errorf("%#v %q got ok %v, want %v", test.Iface, test.Name, got, want)
		}
		if test.Want != nil {
			if got, want := method, *test.Want; !reflect.DeepEqual(got, want) {
				t.Errorf("%#v %q got method %#v, want %#v", test.Iface, test.Name, got, want)
			}
		}
	}
}

func TestFirstMethod(t *testing.T) {
	tests := []struct {
		Sig  []Interface
		Name string
		Want *Method
	}{
		{sigABC, "A", &methodA1},
		{sigABC, "B", &methodB1},
		{sigABC, "C", &methodC1},
		{sigABC, "foo", nil},
		{sigCBA, "A", &methodA1},
		{sigCBA, "B", &methodB1},
		{sigCBA, "C", &methodC1},
		{sigCBA, "foo", nil},
		{sigMulti, "A", &methodA1},
		{sigMulti, "B", &methodB1},
		{sigMulti, "C", &methodC1},
		{sigMulti, "foo", nil},
	}
	for _, test := range tests {
		method, ok := FirstMethod(test.Sig, test.Name)
		if got, want := ok, test.Want != nil; got != want {
			t.Errorf("%#v %q got ok %v, want %v", test.Sig, test.Name, got, want)
		}
		if test.Want != nil {
			if got, want := method, *test.Want; !reflect.DeepEqual(got, want) {
				t.Errorf("%#v %q got method %#v, want %#v", test.Sig, test.Name, got, want)
			}
		}
	}
}

func TestCopyInterfaces(t *testing.T) {
	// Make sure the copy is equal to the original.
	cp := CopyInterfaces(sigMulti)
	if got, want := cp, sigMulti; !reflect.DeepEqual(got, want) {
		t.Errorf("CopyInterfaces got %#v, want %#v", got, want)
	}
	// Make sure the copy isn't equal to the original after mutation.
	cp[0].Methods[0].Name = "XYZ"
	if got, want := cp, sigMulti; reflect.DeepEqual(got, want) {
		t.Errorf("CopyInterfaces after mutation got %#v, want %#v", got, want)
	}
}

func TestMethodNames(t *testing.T) {
	tests := []struct {
		Sig  []Interface
		Want []string
	}{
		{sigABC, []string{"A", "B", "C"}},
		{sigCBA, []string{"A", "B", "C"}},
		{sigMulti, []string{"A", "B", "C"}},
	}
	for _, test := range tests {
		if got, want := MethodNames(test.Sig), test.Want; !reflect.DeepEqual(got, want) {
			t.Errorf("%#v got %v, want %v", test.Sig, got, want)
		}
	}
}

func TestCleanInterfaces(t *testing.T) {
	sigCBASorted := CopyInterfaces(sigCBA)
	sigCBASorted[0].Methods = []Method{methodA1, methodB1, methodC1}
	tests := []struct {
		Dirty, Clean []Interface
	}{
		{sigABC, sigABC},
		{sigCBA, sigCBASorted},
		{append(sigABC, sigABC...), sigABC},
		{
			[]Interface{{Methods: []Method{methodC1, methodB1, methodA1}}},
			[]Interface{{Methods: []Method{methodA1, methodB1, methodC1}}},
		},
	}
	for _, test := range tests {
		if got, want := CleanInterfaces(test.Dirty), test.Clean; !reflect.DeepEqual(got, want) {
			t.Errorf("%#v got %#v, want %#v", test.Dirty, got, want)
		}
	}
}
