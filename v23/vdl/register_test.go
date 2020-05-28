// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdl

import (
	"reflect"
	"sync"
	"testing"
)

var NUnionWant = []*reflectInfo{{
	Type: reflect.TypeOf((*NUnionABC)(nil)).Elem(),
	Name: "v.io/v23/vdl.NUnionABC",
	UnionFields: []reflectField{
		{"A", reflect.TypeOf(false), reflect.TypeOf(NUnionABCA{})},
		{"B", reflect.TypeOf(string("")), reflect.TypeOf(NUnionABCB{})},
		{"C", reflect.TypeOf(NStructInt64{}), reflect.TypeOf(NUnionABCC{})},
	},
}}

var reflectInfoTests = []struct {
	rt reflect.Type
	ri []*reflectInfo
}{
	{reflect.TypeOf(int64(0)), []*reflectInfo{{Type: reflect.TypeOf(int64(0))}}},
	{reflect.TypeOf(string("")), []*reflectInfo{{Type: reflect.TypeOf(string(""))}}},
	{reflect.TypeOf([]byte{}), []*reflectInfo{{Type: reflect.TypeOf([]byte{})}}},
	{
		reflect.TypeOf(NEnumA),
		[]*reflectInfo{{
			Type:       reflect.TypeOf(NEnumA),
			Name:       "v.io/v23/vdl.NEnum",
			EnumLabels: []string{"A", "B", "C", "ABC"},
		}},
	},
	{reflect.TypeOf((*NUnionABC)(nil)).Elem(), NUnionWant},
	{reflect.TypeOf(NUnionABCA{}), NUnionWant},
	{reflect.TypeOf(NUnionABCB{}), NUnionWant},
	{reflect.TypeOf(NUnionABCC{}), NUnionWant},
	{
		reflect.TypeOf(NRecurseSelf{}),
		[]*reflectInfo{{
			Type: reflect.TypeOf(NRecurseSelf{}),
			Name: "v.io/v23/vdl.NRecurseSelf",
		}},
	},
	{
		reflect.TypeOf(NRecurseA{}),
		[]*reflectInfo{
			{
				Type: reflect.TypeOf(NRecurseA{}),
				Name: "v.io/v23/vdl.NRecurseA",
			},
			{
				Type: reflect.TypeOf(NRecurseB{}),
				Name: "v.io/v23/vdl.NRecurseB",
			},
		},
	},
	{
		reflect.TypeOf(NRecurseB{}),
		[]*reflectInfo{
			{
				Type: reflect.TypeOf(NRecurseB{}),
				Name: "v.io/v23/vdl.NRecurseB",
			},
			{
				Type: reflect.TypeOf(NRecurseA{}),
				Name: "v.io/v23/vdl.NRecurseA",
			},
		},
	},
}

// Test deriveReflectInfo success.
func TestDeriveReflectInfo(t *testing.T) {
	for _, test := range reflectInfoTests {
		ri, _, err := deriveReflectInfo(test.rt)
		if ri == nil || err != nil {
			t.Errorf("%s deriveReflectInfo failed: (%v, %v)", test.rt, ri, err)
			continue
		}
		if got, want := ri, test.ri[0]; !reflect.DeepEqual(got, want) {
			t.Errorf("%s got %v, want %v", test.rt, got, want)
		}
	}
}

// Test Register called by multiple goroutines concurrently on the same types,
// to expose locking issues in the registry.
func TestRegister(t *testing.T) {
	var done sync.WaitGroup
	for i := 0; i < 3; i++ {
		done.Add(1)
		go func() {
			testRegister(t)
			done.Done()
		}()
	}
	done.Wait()
}

func testRegister(t *testing.T) {
	for _, test := range reflectInfoTests {
		Register(reflect.New(test.rt).Interface())
		for _, testri := range test.ri {
			if testri.Name != "" {
				ri := reflectInfoFromName(testri.Name)
				if got, want := ri, testri; !reflect.DeepEqual(got, want) {
					t.Errorf("%s reflectInfoFromName got %v, want %v", test.rt, got, want)
				}
			}
		}
	}
}

type (
	nBadDescribe1 struct{}
	nBadDescribe2 struct{}
	nBadDescribe3 struct{}

	nBadEnumNoLabels int
	nBadEnumString1  int
	nBadEnumString2  int
	nBadEnumString3  int
	nBadEnumSet1     int
	nBadEnumSet2     int
	nBadEnumSet3     int
	nBadEnumSet4     int

	nBadUnionNoFields struct{}
	nBadUnionUnexp    struct{}
	nBadUnionField1   struct{}
	nBadUnionField2   struct{}
	nBadUnionField3   struct{}
	nBadUnionName1    struct{ Value bool }
	nBadUnionName2    struct{ Value bool }
	nBadUnionIndex1   struct{ Value bool }
	nBadUnionIndex2   struct{ Value bool }
)

// No description
func (nBadDescribe1) VDLReflect() { panic("X") }

// In-arg isn't a struct
func (nBadDescribe2) VDLReflect(int) { panic("X") }

// Can't have out-arg
func (nBadDescribe3) VDLReflect(struct{}) error { panic("X") }

// No enum labels
func (nBadEnumNoLabels) VDLReflect(struct{ Enum struct{} }) { panic("X") }

// No String method
func (nBadEnumString1) VDLReflect(struct{ Enum struct{ A string } }) { panic("X") }

// String method isn't String() string
func (nBadEnumString2) VDLReflect(struct{ Enum struct{ A string } }) { panic("X") }
func (nBadEnumString2) String()                                      { panic("X") }

// String method isn't String() string
func (nBadEnumString3) VDLReflect(struct{ Enum struct{ A string } }) { panic("X") }
func (nBadEnumString3) String() bool                                 { panic("X") }

// No Set method
func (nBadEnumSet1) VDLReflect(struct{ Enum struct{ A string } }) { panic("X") }
func (nBadEnumSet1) String() string                               { panic("X") }

// Set method isn't Set(string) error
func (nBadEnumSet2) VDLReflect(struct{ Enum struct{ A string } }) { panic("X") }
func (nBadEnumSet2) String() string                               { panic("X") }
func (nBadEnumSet2) Set()                                         { panic("X") }

// Set method isn't Set(string) error
func (nBadEnumSet3) VDLReflect(struct{ Enum struct{ A string } }) { panic("X") }
func (nBadEnumSet3) String() string                               { panic("X") }
func (nBadEnumSet3) Set(bool) error                               { panic("X") }

// Set method receiver isn't a pointer
func (nBadEnumSet4) VDLReflect(struct{ Enum struct{ A string } }) { panic("X") }
func (nBadEnumSet4) String() string                               { panic("X") }
func (nBadEnumSet4) Set(string) error                             { panic("X") }

// No union fields
func (nBadUnionNoFields) VDLReflect(struct {
	Type  NUnionABC
	Union struct{}
}) {
	panic("X")
}

// Field name isn't exported
func (nBadUnionUnexp) VDLReflect(struct {
	Type  NUnionABC
	Union struct{ a NUnionABCA }
}) {
	panic("X")
}

// Field type isn't struct
func (nBadUnionField1) VDLReflect(struct {
	Type  NUnionABC
	Union struct{ A bool }
}) {
	panic("X")
}

// Field type has no field
func (nBadUnionField2) VDLReflect(struct {
	Type  NUnionABC
	Union struct{ A struct{} }
}) {
	panic("X")
}

// Field type name isn't "Value"
func (nBadUnionField3) VDLReflect(struct {
	Type  NUnionABC
	Union struct{ A struct{ value bool } }
}) {
	panic("X")
}

// Name method isn't Name() string
func (nBadUnionName1) Name() { panic("X") }
func (nBadUnionName1) VDLReflect(struct {
	Type  NUnionABC
	Union struct{ A nBadUnionName1 }
}) {
	panic("X")
}

// Name method isn't Name() string
func (nBadUnionName2) Name() bool { panic("X") }
func (nBadUnionName2) VDLReflect(struct {
	Type  NUnionABC
	Union struct{ A nBadUnionName2 }
}) {
	panic("X")
}

// Index method isn't Index() int
func (nBadUnionIndex1) Name() string { panic("X") }
func (nBadUnionIndex1) Index()       { panic("X") }
func (nBadUnionIndex1) VDLReflect(struct {
	Type  NUnionABC
	Union struct{ A nBadUnionIndex1 }
}) {
	panic("X")
}

// Index method isn't Index() int
func (nBadUnionIndex2) Name() string { panic("X") }
func (nBadUnionIndex2) Index() bool  { panic("X") }
func (nBadUnionIndex2) VDLReflect(struct {
	Type  NUnionABC
	Union struct{ A nBadUnionIndex2 }
}) {
	panic("X")
}

// rtErrorTest describes a test case with rt as input, and errstr as output.
type rtErrorTest struct {
	rt     reflect.Type
	errstr string
}

const (
	badDescribe   = `invalid VDLReflect (want VDLReflect(struct{...}))`
	badEnumString = `must have method String() string`
	badEnumSet    = `must have pointer method Set(string) error`
	badUnionField = `bad concrete field type`
	badUnionName  = `must have method Name() string`
	badUnionIndex = `must have method Index() int`
)

var reflectInfoErrorTests = []rtErrorTest{
	{reflect.TypeOf(nBadDescribe1{}), badDescribe},
	{reflect.TypeOf(nBadDescribe2{}), badDescribe},
	{reflect.TypeOf(nBadDescribe3{}), badDescribe},
	{reflect.TypeOf(nBadEnumNoLabels(0)), `no labels`},
	{reflect.TypeOf(nBadEnumString1(0)), badEnumString},
	{reflect.TypeOf(nBadEnumString2(0)), badEnumString},
	{reflect.TypeOf(nBadEnumString3(0)), badEnumString},
	{reflect.TypeOf(nBadEnumSet1(0)), badEnumSet},
	{reflect.TypeOf(nBadEnumSet2(0)), badEnumSet},
	{reflect.TypeOf(nBadEnumSet3(0)), badEnumSet},
	{reflect.TypeOf(nBadEnumSet4(0)), badEnumSet},
	{reflect.TypeOf(nBadUnionNoFields{}), `no fields`},
	{reflect.TypeOf(nBadUnionUnexp{}), `must be exported`},
	{reflect.TypeOf(nBadUnionField1{}), badUnionField},
	{reflect.TypeOf(nBadUnionField2{}), badUnionField},
	{reflect.TypeOf(nBadUnionField3{}), badUnionField},
	{reflect.TypeOf(nBadUnionName1{}), badUnionName},
	{reflect.TypeOf(nBadUnionName2{}), badUnionName},
	{reflect.TypeOf(nBadUnionIndex1{}), badUnionIndex},
	{reflect.TypeOf(nBadUnionIndex2{}), badUnionIndex},
}

// Test deriveReflectInfo errors.
func TestDeriveReflectInfoError(t *testing.T) {
	for _, test := range reflectInfoErrorTests {
		got, _, err := deriveReflectInfo(test.rt)
		ExpectErr(t, err, test.errstr, "deriveReflectInfo(%v)", test.rt)
		if got != nil {
			t.Errorf("deriveReflectInfo(%v) got %v, want nil", test.rt, got)
		}
	}
}

type (
	NameConflictType interface {
		Index() int
		Interface() interface{}
		Name() string
		VDLReflect(privateNameConflictReflect)
	}
	privateNameConflictReflect struct {
		Name  string `vdl:"v.io/v23/vdl.OtherNameConflictType"`
		Type  NameConflictType
		Union struct{ A NameConflictUnionField }
	}
	NameConflictUnionField struct{ Value int64 }
	OtherNameConflictType  struct{}
)

func (x NameConflictUnionField) Index() int                            { return 0 }
func (x NameConflictUnionField) Interface() interface{}                { return x.Value }
func (x NameConflictUnionField) Name() string                          { return "A" }
func (x NameConflictUnionField) VDLReflect(privateNameConflictReflect) {}

func TestReflectNameConflicts(t *testing.T) {
	Register(NameConflictUnionField{})
	ExpectPanic(t, func() {
		Register(OtherNameConflictType{})
	}, "duplicate name", "")
}
