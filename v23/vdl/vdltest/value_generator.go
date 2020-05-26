// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdltest

import (
	"fmt"
	"math"
	"math/rand"
	"time"
	"unicode/utf8"

	"v.io/v23/vdl"
)

// ValueGenerator generates values.
type ValueGenerator struct {
	// Types contains the total set of types used for creating any and typeobject
	// values.  It's initialized by NewValueGenerator.
	Types []*vdl.Type
	// RandomZeroPercentage specifies the percentage from [0,100] that we will
	// return a zero value.
	RandomZeroPercentage int
	// MaxLen limits the maximum length of lists, sets and maps.
	MaxLen int
	// MaxCycleDepth limits the depth for values of cyclic types.
	MaxCycleDepth int

	rng *rand.Rand
}

// NewValueGenerator returns a new ValueGenerator, which uses a random number
// generator seeded to the current time.  The given types control what types are
// used to generate TypeObject and Any values.
func NewValueGenerator(types []*vdl.Type) *ValueGenerator {
	return &ValueGenerator{
		Types:                types,
		RandomZeroPercentage: 20,
		MaxLen:               3,
		MaxCycleDepth:        3,
		rng:                  rand.New(rand.NewSource(time.Now().Unix())),
	}
}

// RandSeed sets the seed for the random number generator used by g.
func (g *ValueGenerator) RandSeed(seed int64) {
	g.rng.Seed(seed)
}

// GenMode represents the mode used to generate values.
type GenMode int

const (
	// GenFull generates deterministic values that are recursively non-zero,
	// except for types that are part of a cycle, which must have a zero value in
	// order to terminate.
	GenFull GenMode = iota

	GenPosMax // Generate maximal positive numbers.
	GenPosMin // Generate minimal positive numbers.
	GenNegMax // Generate maximal negative numbers.
	GenNegMin // Generate minimal negative numbers.

	// GenRandom generates random values that are never zero for the top-level
	// type, but may be zero otherwise.
	GenRandom
)

func (m GenMode) String() string {
	switch m {
	case GenFull:
		return "Full"
	case GenPosMax:
		return "PosMax"
	case GenPosMin:
		return "PosMin"
	case GenNegMax:
		return "NegMax"
	case GenNegMin:
		return "NegMin"
	case GenRandom:
		return "Random"
	}
	panic(fmt.Errorf("vdltest: unhandled mode %d", m))
}

func needsGenPos(tt *vdl.Type) bool {
	return tt.ContainsKind(vdl.WalkAll, vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64, vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64, vdl.Float32, vdl.Float64)
}

func needsGenNeg(tt *vdl.Type) bool {
	return tt.ContainsKind(vdl.WalkAll, vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64, vdl.Float32, vdl.Float64)
}

func needsGenPosNeg(tt *vdl.Type, mode GenMode) bool {
	switch mode {
	case GenPosMax, GenPosMin:
		return needsGenPos(tt)
	case GenNegMax, GenNegMin:
		return needsGenNeg(tt)
	}
	panic(fmt.Errorf("vdltest: unhandled mode %v", mode))
}

// needsGenFull returns true iff we should generate full values of type tt.  We
// generate full values as long as there is more than one valid value of type
// tt; i.e. as long as the value can be non-zero.
func needsGenFull(tt *vdl.Type) bool {
	switch kind := tt.Kind(); kind {
	case vdl.Enum:
		return tt.NumEnumLabel() > 1
	case vdl.Array:
		return needsGenFull(tt.Elem())
	case vdl.Struct, vdl.Union:
		if kind == vdl.Union && tt.NumField() > 1 {
			// Any union with more than one field can be non-zero.
			return true
		}
		for ix := 0; ix < tt.NumField(); ix++ {
			if needsGenFull(tt.Field(ix).Type) {
				return true
			}
		}
		return false
	default:
		// Every other kind can be non-zero.  Since cyclic types must have an
		// Optional, List, Set or Map somewhere in the cycle, and those kinds are
		// all handled here, there's no need for a seen map to avoid infinite loops.
		return true
	}
}

// needsGenRandom returns true iff we should generate random values of type tt.
// We don't generate random values if tt only has a small number of possible
// values.
func needsGenRandom(tt *vdl.Type) bool {
	// TODO(toddw): Handle cyclic types.
	switch kind := tt.Kind(); kind {
	case vdl.Bool, vdl.Enum:
		return false
	case vdl.Array, vdl.List, vdl.Optional:
		return needsGenRandom(tt.Elem())
	case vdl.Set:
		return needsGenRandom(tt.Key())
	case vdl.Map:
		return needsGenRandom(tt.Key()) || needsGenRandom(tt.Elem())
	case vdl.Struct, vdl.Union:
		for ix := 0; ix < tt.NumField(); ix++ {
			if needsGenRandom(tt.Field(ix).Type) {
				return true
			}
		}
		return false
	default:
		// Every other kind needs GenRandom.
		return true
	}
}

// Gen returns a value of type tt, unless tt is Any, in which case the type of
// the returned value is picked randomly from all types.  Use the mode to
// control properties of the returned value.
func (g *ValueGenerator) Gen(tt *vdl.Type, mode GenMode) *vdl.Value {
	// The loop is for a special-case.  If we're in GenRandom mode, and the value
	// we generated is zero, and we're supposed to generate random values for tt,
	// keep looping until we generate a non-zero value.
	for {
		value := g.gen(tt, mode, 0)
		if mode != GenRandom || !value.IsZero() || !needsGenRandom(tt) {
			return value
		}
	}
}

func (g *ValueGenerator) gen(tt *vdl.Type, mode GenMode, depth int) *vdl.Value {
	if tt == vdl.AnyType {
		if g.randomNil(tt, mode, depth) {
			return vdl.ZeroValue(vdl.AnyType)
		}
		switch {
		case mode == GenFull:
			tt = vdl.Int64Type
		default:
			tt = g.randomNonAnyType()
		}
	}
	if tt.Kind() == vdl.Optional {
		if g.randomNil(tt, mode, depth) {
			return vdl.ZeroValue(tt)
		}
	}
	value := g.genNonNilValue(tt.NonOptional(), mode, depth)
	if tt == vdl.ErrorType {
		// HACK: The codegen for the error "ParamList []any" field doesn't work.
		// We've hard-coded the vdl tool to represent any as interface{} in the
		// vdltest package, but vdl.WireError expects ParamList as []*vdl.Value.
		// This is hard to fix, so we just clear out the ParamList so that it
		// doesn't show up in generated code.
		//
		// TODO(toddw): Fix this.
		value.StructFieldByName("ParamList").AssignLen(0)
	}
	if tt.Kind() == vdl.Optional {
		value = vdl.OptionalValue(value)
	}
	return value
}

func (g *ValueGenerator) genNonNilValue(tt *vdl.Type, mode GenMode, depth int) *vdl.Value { //nolint:gocyclo
	if mode == GenRandom && depth > 0 && g.randomZero() {
		return vdl.ZeroValue(tt)
	}
	kind := tt.Kind()
	bitlen := uint(kind.BitLen())
	switch kind {
	case vdl.Bool:
		switch mode {
		case GenPosMax, GenPosMin, GenNegMax, GenNegMin:
			return vdl.ZeroValue(tt)
		case GenFull:
			return vdl.BoolValue(tt, true)
		default:
			return vdl.BoolValue(tt, g.randomPercentage(50))
		}
	case vdl.String:
		switch mode {
		case GenPosMax, GenPosMin, GenNegMax, GenNegMin:
			return vdl.ZeroValue(tt)
		case GenFull:
			return vdl.StringValue(tt, fullString)
		default:
			return vdl.StringValue(tt, g.randomString())
		}
	case vdl.Enum:
		switch mode {
		case GenPosMax, GenPosMin, GenNegMax, GenNegMin:
			return vdl.ZeroValue(tt)
		case GenFull:
			return vdl.EnumValue(tt, tt.NumEnumLabel()-1)
		default:
			return vdl.EnumValue(tt, g.rng.Intn(tt.NumEnumLabel()))
		}
	case vdl.TypeObject:
		switch mode {
		case GenPosMax, GenPosMin, GenNegMax, GenNegMin:
			return vdl.ZeroValue(tt)
		case GenFull:
			return vdl.TypeObjectValue(vdl.Int64Type)
		default:
			return vdl.TypeObjectValue(g.randomType())
		}
	case vdl.Byte, vdl.Uint16, vdl.Uint32, vdl.Uint64:
		switch mode {
		case GenNegMax, GenNegMin:
			return vdl.ZeroValue(tt)
		case GenPosMax:
			return vdl.UintValue(tt, (uint64(1)<<bitlen)-1)
		case GenPosMin:
			return vdl.UintValue(tt, 1)
		case GenFull:
			return vdl.UintValue(tt, 123)
		default:
			return vdl.UintValue(tt, g.randomUint(bitlen))
		}
	case vdl.Int8, vdl.Int16, vdl.Int32, vdl.Int64:
		switch mode {
		case GenPosMax:
			return vdl.IntValue(tt, (int64(1)<<(bitlen-1))-1)
		case GenNegMax:
			return vdl.IntValue(tt, int64(-1)<<(bitlen-1))
		case GenPosMin:
			return vdl.IntValue(tt, 1)
		case GenNegMin:
			return vdl.IntValue(tt, -1)
		case GenFull:
			return vdl.IntValue(tt, -123)
		default:
			return vdl.IntValue(tt, g.randomInt(bitlen))
		}
	case vdl.Float32, vdl.Float64:
		switch mode {
		case GenPosMax:
			return vdl.FloatValue(tt, g.maxFloat(kind))
		case GenNegMax:
			return vdl.FloatValue(tt, -g.maxFloat(kind))
		case GenPosMin:
			return vdl.FloatValue(tt, g.smallFloat(kind))
		case GenNegMin:
			return vdl.FloatValue(tt, -g.smallFloat(kind))
		case GenFull:
			return vdl.FloatValue(tt, 1.5)
		default:
			return vdl.FloatValue(tt, g.randomFloat())
		}
	case vdl.Array, vdl.List:
		return g.genArrayList(tt, mode, depth)
	case vdl.Set:
		return g.genSet(tt, mode, depth)
	case vdl.Map:
		return g.genMap(tt, mode, depth)
	case vdl.Struct:
		return g.genStruct(tt, mode, depth)
	case vdl.Union:
		return g.genUnion(tt, mode, depth)
	}
	panic(fmt.Errorf("genNonNilValue unhandled type %v", tt))
}

func (g *ValueGenerator) genArrayList(tt *vdl.Type, mode GenMode, depth int) *vdl.Value {
	value := vdl.ZeroValue(tt)
	if tt.Kind() == vdl.List {
		value.AssignLen(g.randomLen(tt, mode, depth))
	}
	for i := 0; i < value.Len(); i++ {
		value.AssignIndex(i, g.gen(tt.Elem(), mode, depth+1))
	}
	return value
}

func (g *ValueGenerator) genSet(tt *vdl.Type, mode GenMode, depth int) *vdl.Value {
	value := vdl.ZeroValue(tt)
	for i, len := 0, g.randomLen(tt, mode, depth); i < len; i++ {
		value.AssignSetKey(g.gen(tt.Key(), mode, depth+1))
	}
	return value
}

func (g *ValueGenerator) genMap(tt *vdl.Type, mode GenMode, depth int) *vdl.Value {
	value := vdl.ZeroValue(tt)
	for i, len := 0, g.randomLen(tt, mode, depth); i < len; i++ {
		value.AssignMapIndex(g.gen(tt.Key(), mode, depth+1), g.gen(tt.Elem(), mode, depth+1))
	}
	return value
}

func (g *ValueGenerator) genStruct(tt *vdl.Type, mode GenMode, depth int) *vdl.Value {
	value := vdl.ZeroValue(tt)
	for i := 0; i < tt.NumField(); i++ {
		value.AssignField(i, g.gen(tt.Field(i).Type, mode, depth+1))
	}
	return value
}

func (g *ValueGenerator) genUnion(tt *vdl.Type, mode GenMode, depth int) *vdl.Value {
	var index int
	switch mode {
	case GenFull:
		index = tt.NumField() - 1
	case GenPosMax, GenPosMin, GenNegMax, GenNegMin:
		if !needsGenPosNeg(tt, mode) {
			return vdl.ZeroValue(tt)
		}
		index = findUnionPosNegField(tt, mode)
	default:
		index = g.rng.Intn(tt.NumField())
	}
	return vdl.UnionValue(tt, index, g.gen(tt.Field(index).Type, mode, depth+1))
}

// findUnionPosNegField returns the first field in tt that satisfies the mode.
//
// REQUIRES: tt must be a union, and must have a field that satisfies the mode.
// The mode must be one of GenPosMax, GenPosMin, GenNegMax, GenNegMin.
func findUnionPosNegField(tt *vdl.Type, mode GenMode) int {
	for index := 0; index < tt.NumField(); index++ {
		if needsGenPosNeg(tt.Field(index).Type, mode) {
			return index
		}
	}
	panic(fmt.Errorf("vdltest: no field in %v satisfies %v", tt, mode))
}

func (g *ValueGenerator) randomPercentage(p int) bool {
	return g.rng.Intn(100) < p
}

func (g *ValueGenerator) randomZero() bool {
	return g.randomPercentage(g.RandomZeroPercentage)
}

func (g *ValueGenerator) randomUint(bitlen uint) uint64 {
	u := uint64(g.rng.Uint32())<<32 + uint64(g.rng.Uint32())
	shift := 64 - bitlen
	return (u << shift) >> shift
}

func (g *ValueGenerator) randomInt(bitlen uint) int64 {
	neg := int64(-1)
	if g.randomPercentage(50) {
		neg = 1
	}
	u := uint64(g.rng.Uint32())<<32 + uint64(g.rng.Uint32())
	// The shift uses 65 since the topmost bit is the sign bit.  I.e. 32 bit
	// numbers should be shifted by 33 rather than 32.
	shift := 65 - bitlen
	return (int64(u<<shift) >> shift) * neg
}

func (g *ValueGenerator) maxFloat(kind vdl.Kind) float64 {
	// TODO(toddw): Fix floating-point issues.  Currently the MaxFloat* values
	// don't convert correctly.  There are Go bugs related to this:
	// https://github.com/golang/go/issues/14651
	if kind == vdl.Float32 {
		return math.MaxFloat32 / 2
	}
	return math.MaxFloat64 / 2
}

func (g *ValueGenerator) smallFloat(kind vdl.Kind) float64 {
	// TODO(toddw): Fix floating-point issues.  Currently the Smallest* values
	// don't convert correctly.  There are Go bugs related to this:
	// https://github.com/golang/go/issues/14651
	if kind == vdl.Float32 {
		return math.SmallestNonzeroFloat32 * 10
	}
	return math.SmallestNonzeroFloat64 * 10
}

func (g *ValueGenerator) randomFloat() float64 {
	// Float64 returns in the range [0.0,1.0), so we adjust to a larger range.
	neg := float64(-1)
	if g.randomPercentage(50) {
		neg = 1
	}
	return g.rng.Float64() * float64(g.rng.Uint32()) * neg
}

const fullString = "abcdeΔΘΠΣΦ王普澤世界"

func (g *ValueGenerator) randomString() string {
	rLen := utf8.RuneCountInString(fullString)
	r0, r1 := g.rng.Intn(rLen), g.rng.Intn(rLen)
	if r0 > r1 {
		r0, r1 = r1, r0
	}
	if r0 == r1 {
		r1 = rLen
	}
	// Translate the startR and endR rune counts into byte indices.
	rCount, start, end := 0, 0, len(fullString)
	for i := range fullString {
		if rCount == r0 {
			start = i
		}
		if rCount == r1 {
			end = i
		}
		rCount++
	}
	return fullString[start:end]
}

func (g *ValueGenerator) randomNil(tt *vdl.Type, mode GenMode, depth int) bool {
	if tt.IsPartOfCycle() && depth > g.MaxCycleDepth {
		return true
	}
	switch mode {
	case GenFull:
		return false
	case GenPosMax, GenPosMin, GenNegMax, GenNegMin:
		return !needsGenPosNeg(tt, mode)
	}
	return g.randomZero()
}

func (g *ValueGenerator) randomLen(tt *vdl.Type, mode GenMode, depth int) int {
	if g.MaxLen == 0 {
		return 0
	}
	if tt.IsPartOfCycle() && depth > g.MaxCycleDepth {
		return 0
	}
	switch mode {
	case GenFull:
		return 1
	case GenPosMax, GenPosMin, GenNegMax, GenNegMin:
		if !needsGenPosNeg(tt, mode) {
			return 0
		}
		return 1
	}
	return 1 + g.rng.Intn(g.MaxLen)
}

func (g *ValueGenerator) randomType() *vdl.Type {
	if g.randomZero() {
		return vdl.AnyType
	}
	return g.randomNonAnyType()
}

func (g *ValueGenerator) randomNonAnyType() *vdl.Type {
	for {
		var tt *vdl.Type
		if len(g.Types) > 0 {
			tt = g.Types[g.rng.Intn(len(g.Types))]
		} else {
			tt = ttBuiltIn[g.rng.Intn(len(ttBuiltIn))]
		}
		if tt != vdl.AnyType {
			return tt
		}
	}
}
