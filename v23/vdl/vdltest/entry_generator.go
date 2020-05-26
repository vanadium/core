// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdltest

import (
	"fmt"
	"hash"
	"hash/fnv"
	"math/rand"
	"strconv"
	"time"

	"v.io/v23/vdl"
)

// IsCanonical returns true iff e.Target == e.Source.
func (e EntryValue) IsCanonical() bool {
	return vdl.EqualValue(e.Target, e.Source)
}

// EntryGenerator generates test entries.
type EntryGenerator struct {
	// AllMaxMinTargets specifies that all max/min targets should be generated.
	// By default max/min targets are only generated for numbers.
	AllMaxMinTargets bool
	// RandomTargetLimit limits the number of random targets that are generated.
	RandomTargetLimit int
	// Each of the *Entries fields configures, for each unique target, the
	// number of non-canonical entries with the given label that are generated.
	ZeroEntryLimit   int
	MaxMinEntryLimit int
	FullEntryLimit   int
	RandomEntryLimit int

	valueGen *ValueGenerator
	hasher   hash.Hash64
	randSeed int64
	rng      *rand.Rand
	// Keep the total set of types separated by kind, for quick filtering.
	ttBool, ttStringEnum, ttNumber, ttArrayList, ttSet, ttMap, ttStruct, ttUnion []*vdl.Type
}

// NewEntryGenerator returns a new EntryGenerator, which uses a random number
// generator seeded to the current time.  The sourceTypes specify the types to
// consider when creating source values for each entry.
func NewEntryGenerator(sourceTypes []*vdl.Type) *EntryGenerator {
	now := time.Now().Unix()
	g := &EntryGenerator{
		RandomTargetLimit: 1,
		ZeroEntryLimit:    2,
		MaxMinEntryLimit:  2,
		FullEntryLimit:    1,
		RandomEntryLimit:  1,
		valueGen:          NewValueGenerator(sourceTypes),
		hasher:            fnv.New64a(),
		randSeed:          now,
		rng:               rand.New(rand.NewSource(now)),
	}
	for _, tt := range sourceTypes {
		kind := tt.NonOptional().Kind()
		if kind.IsNumber() {
			g.ttNumber = append(g.ttNumber, tt)
		}
		switch kind {
		case vdl.Bool:
			g.ttBool = append(g.ttBool, tt)
		case vdl.String, vdl.Enum:
			g.ttStringEnum = append(g.ttStringEnum, tt)
		case vdl.Array, vdl.List:
			g.ttArrayList = append(g.ttArrayList, tt)
		case vdl.Set:
			g.ttSet = append(g.ttSet, tt)
		case vdl.Map:
			g.ttMap = append(g.ttMap, tt)
		case vdl.Struct:
			g.ttStruct = append(g.ttStruct, tt)
		case vdl.Union:
			g.ttUnion = append(g.ttUnion, tt)
		}
	}
	return g
}

// RandSeed sets the seed for the random number generator used by g.
func (g *EntryGenerator) RandSeed(seed int64) {
	g.randSeed = seed
}

// candidateTypes returns the types whose values might be convertible to values
// of the tt type.  In theory we could run a compatibility check here for fewer
// false positives, but we'll need to check compatibility again when generating
// values anyways, to handle inner types of nested any, so we don't bother.
func (g *EntryGenerator) candidateTypes(tt *vdl.Type, mode sourceMode) []*vdl.Type { //nolint:gocyclo
	var candidates []*vdl.Type
	kind := tt.NonOptional().Kind()
	if kind.IsNumber() {
		candidates = g.ttNumber
	}
	switch kind {
	case vdl.Bool:
		candidates = g.ttBool
	case vdl.String, vdl.Enum:
		candidates = g.ttStringEnum
	case vdl.Array, vdl.List:
		candidates = g.ttArrayList
	case vdl.Set:
		candidates = g.ttSet
	case vdl.Map:
		candidates = g.ttMap
	case vdl.Struct:
		candidates = g.ttStruct
	case vdl.Union:
		candidates = g.ttUnion
	case vdl.TypeObject:
		candidates = []*vdl.Type{vdl.TypeObjectType}
	}
	if mode == sourceAll {
		return candidates
	}
	var filtered []*vdl.Type
	for _, c := range candidates {
		if mode.Unnamed() && c.Name() == "" || mode.Named() && c.Name() != "" {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

// GenAllPass generates a list of passing entries for all targetTypes.
func (g *EntryGenerator) GenAllPass(targetTypes []*vdl.Type) []EntryValue {
	var entries []EntryValue
	for _, tt := range targetTypes {
		entries = append(entries, g.GenPass(tt)...)
	}
	return entries
}

// GenPass generates a list of passing entries for the tt type.  Each entry has
// a target value of type tt.  The source value of each entry is created with
// the property that, if the source value is converted to the target type, the
// result is exactly the target value.
func (g *EntryGenerator) GenPass(tt *vdl.Type) []EntryValue {
	// Add entries for zero values.
	ev := g.genPass("Zero", vdl.ZeroValue(tt), g.ZeroEntryLimit, sourceAll)
	if tt.Kind() == vdl.Optional {
		// Add entry to convert from any(nil) to optional(nil).
		ev = append(ev, EntryValue{
			Label:  "NilAny",
			Target: vdl.ZeroValue(tt),
			Source: vdl.ZeroValue(vdl.AnyType),
		})
	}
	// Handle some special-cases.
	switch {
	case tt == vdl.AnyType:
		// Don't create non-nil any values.
		return ev
	case tt.Kind() == vdl.Enum:
		// Test all enum values exhaustively.
		for ix := 1; ix < tt.NumEnumLabel(); ix++ {
			ev = append(ev, g.genPass("Full", vdl.EnumValue(tt, ix), -1, sourceAll)...)
		}
		return ev
	}
	// Add entries for max/min number testing.
	if needsGenPos(tt) && (g.AllMaxMinTargets || tt.Kind().IsNumber()) {
		max := g.makeValue(tt, "+Max", GenPosMax)
		min := g.makeValue(tt, "+Min", GenPosMin)
		ev = append(ev, g.genPassMaxMin("+Max", max)...)
		ev = append(ev, g.genPassMaxMin("+Min", min)...)
	}
	if needsGenNeg(tt) && (g.AllMaxMinTargets || tt.Kind().IsNumber()) {
		max := g.makeValue(tt, "-Max", GenNegMax)
		min := g.makeValue(tt, "-Min", GenNegMin)
		ev = append(ev, g.genPassMaxMin("-Max", max)...)
		ev = append(ev, g.genPassMaxMin("-Min", min)...)
	}
	// Add full entries, which are deterministic and recursively non-zero.  As a
	// special-case, for types that are part of a cycle, the values are still
	// deterministic but will contain zero items.
	if needsGenFull(tt) {
		full := g.makeValue(tt, "Full", GenFull)
		ev = append(ev, g.genPass("Full", full, g.FullEntryLimit, sourceAll)...)
	}
	// Add some random entries.
	if needsGenRandom(tt) {
		for ix := 0; ix < g.RandomTargetLimit; ix++ {
			label := "Random" + strconv.Itoa(ix)
			random := g.makeValue(tt, label, GenRandom)
			ev = append(ev, g.genPass(label, random, g.RandomEntryLimit, sourceAll)...)
		}
	}
	return ev
}

func (g *EntryGenerator) genPassMaxMin(label string, target *vdl.Value) []EntryValue {
	var ev []EntryValue
	switch ttTarget := target.Type(); {
	case ttTarget.Kind().IsNumber():
		// We test the boundaries of all unnamed (i.e. built-in) numbers with other
		// unnamed numbers, and test the boundaries of all named numbers with other
		// named numbers.  We limit the entries otherwise.
		if ttTarget.Name() == "" {
			ev = append(ev, g.genPass(label, target, -1, sourceUnnamed)...)
			ev = append(ev, g.genPass(label, target, g.MaxMinEntryLimit, sourceNamedNoCanonical)...)
		} else {
			ev = append(ev, g.genPass(label, target, -1, sourceNamed)...)
			ev = append(ev, g.genPass(label, target, g.MaxMinEntryLimit, sourceUnnamedNoCanonical)...)
		}
	default:
		ev = append(ev, g.genPass(label, target, g.MaxMinEntryLimit, sourceAll)...)
	}
	return ev
}

type sourceMode int

const (
	sourceAll                sourceMode = iota // Consider all source types.
	sourceUnnamed                              // Only unnamed types.
	sourceUnnamedNoCanonical                   // Only unnamed types, no canonical.
	sourceNamed                                // Only named types.
	sourceNamedNoCanonical                     // Only named types, no canonical.
)

func (m sourceMode) String() string {
	switch m {
	case sourceAll:
		return "All"
	case sourceUnnamed:
		return "Unnamed"
	case sourceUnnamedNoCanonical:
		return "UnnamedNoCanonical"
	case sourceNamed:
		return "Named"
	case sourceNamedNoCanonical:
		return "NamedNoCanonical"
	}
	panic(fmt.Errorf("vdltest: unhandled mode %d", m))
}

func (m sourceMode) Canonical() bool {
	return m == sourceAll || m == sourceUnnamed || m == sourceNamed
}

func (m sourceMode) Unnamed() bool {
	return m == sourceAll || m == sourceUnnamed || m == sourceUnnamedNoCanonical
}

func (m sourceMode) Named() bool {
	return m == sourceAll || m == sourceNamed || m == sourceNamedNoCanonical
}

// genPass generates a list of passing entries, where each entry has the given
// target value.  The given limit restricts the number of returned entries to
// that value; -1 returns all entries.
func (g *EntryGenerator) genPass(label string, target *vdl.Value, limit int, mode sourceMode) []EntryValue {
	var ev []EntryValue
	if mode.Canonical() {
		// Add the canonical identity conversion for each target value.
		ev = append(ev, EntryValue{
			Label:  label,
			Target: target,
			Source: target,
		})
	}
	// Add up to limit conversion entries.  The strategy is to add an entry for
	// each source type where we can create a value that can convert to the
	// target.  We filter out all types that cannot possibly be convertible, and
	// are left with candidates.  The candidates still might not be convertible,
	// so we try to mimic values for each type, and add the entry if it succeeds.
	candidates := g.candidateTypes(target.Type(), mode)
	switch {
	case limit == 0:
		candidates = nil
	case limit != -1:
		// Randomly permute the candidates if we're returning a limited number of
		// entries, to cover more cases.
		shuffled := make([]*vdl.Type, len(candidates))
		for i, p := range g.perm(len(candidates), target.Type(), label, mode) {
			shuffled[i] = candidates[p]
		}
		candidates = shuffled
	}
	num := 0
	for _, ttSource := range candidates {
		if ttSource == target.Type() {
			continue // Skip the canonical case, which was handled above.
		}
		if source := MimicValue(ttSource, target); source != nil {
			if limit > -1 && num >= limit {
				break
			}
			num++
			ev = append(ev, EntryValue{
				Label:  label,
				Target: target,
				Source: source,
			})
		}
	}
	return ev
}

func (g *EntryGenerator) makeValue(tt *vdl.Type, label string, mode GenMode) *vdl.Value {
	// ValueGenerator creates random values for us, but we'd like to ensure that
	// the values don't change spuriously.  I.e. adding new types or generating
	// more values shouldn't change any existing values.  To this end, we seed the
	// random source with a hash of the unique type string, gen mode and iteration
	// counter i.
	g.hasher.Reset()
	g.hasher.Write([]byte(tt.Unique()))   //nolint:errcheck
	g.hasher.Write([]byte(label))         //nolint:errcheck
	g.hasher.Write([]byte(mode.String())) //nolint:errcheck
	g.valueGen.RandSeed(g.randSeed + int64(g.hasher.Sum64()))
	return g.valueGen.Gen(tt, mode)
}

func (g *EntryGenerator) perm(n int, tt *vdl.Type, label string, mode sourceMode) []int {
	// Similar to makeValue, we'd like to ensure that our choice of random
	// candidate permutations don't change our test values spuriously.
	g.hasher.Reset()
	g.hasher.Write([]byte(tt.Unique()))   //nolint:errcheck
	g.hasher.Write([]byte(label))         //nolint:errcheck
	g.hasher.Write([]byte(mode.String())) //nolint:errcheck
	g.rng.Seed(g.randSeed + int64(g.hasher.Sum64()))
	return g.rng.Perm(n)
}

// GenAllFail generates a list of failing entries for all targetTypes.
func (g *EntryGenerator) GenAllFail(targetTypes []*vdl.Type) []EntryValue {
	var entries []EntryValue
	for _, tt := range targetTypes {
		entries = append(entries, g.GenFail(tt)...)
	}
	return entries
}

// GenFail generates a list of failing entries for the tt type.  Each entry has
// a target value of type tt.  The source value of each entry is created with
// the property that, if the source value is converted to the target type, the
// conversion fails.
func (g *EntryGenerator) GenFail(tt *vdl.Type) []EntryValue {
	// TODO(toddw): Implement this!
	return nil
}
