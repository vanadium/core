// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vdltest provides a variety of VDL types and values for testing.
package vdltest

import (
	"flag"
	"reflect"
	"strings"

	"v.io/v23/vdl"
)

// The following causes data files to be generated when "go generate" is run.
//go:generate ./gen.sh

var flagVDLTest string

func init() {
	flag.StringVar(&flagVDLTest, "vdltest", "", `Filter vdltest.All to only return entries that contain the given substring.  If the value starts with "!", only returns entries that don't contain the given substring.`)
}

func filter() (bool, string) {
	if strings.HasPrefix(flagVDLTest, "!") {
		return false, strings.TrimPrefix(flagVDLTest, "!")
	}
	return true, flagVDLTest
}

// The following vars are defined in generated files:
//   var vAllPass, vAllFail, xAllPass, xAllFail []vdlEntry

// AllPass returns all entries where the source value, when converted to the
// type of the target value, results in exactly the target value.
//
// The -vdltest flag may be used to filter the returned entries.
func AllPass() []Entry {
	var result []Entry
	for _, e := range fromVDLEntries(vAllPass, xAllPass) {
		if want, substr := filter(); strings.Contains(e.Name(), substr) == want {
			result = append(result, e)
		}
	}
	return result
}

// AllPassFunc returns the entries in AllPass where fn(e) returns true for each
// returned entry.
func AllPassFunc(fn func(e Entry) bool) []Entry {
	var result []Entry
	for _, e := range AllPass() {
		if fn(e) {
			result = append(result, e)
		}
	}
	return result
}

// AllFail returns all entries where the source value, when converted to the
// type of the target value, results in a conversion error.
//
// E.g. the types of the source and target may be incompatible; trying to
// convert a source bool to a target struct returns an error.  Or the values may
// be inconvertible; trying to convert a source int32(-1) to a target uint32
// returns an error.
//
// The -vdltest flag may be used to filter the returned entries.
func AllFail() []Entry {
	var result []Entry
	for _, e := range fromVDLEntries(vAllFail, xAllFail) {
		if want, substr := filter(); strings.Contains(e.Name(), substr) == want {
			result = append(result, e)
		}
	}
	return result
}

// AllFailFunc returns the entries in AllFail where fn(e) returns true for each
// returned entry.
func AllFailFunc(fn func(e Entry) bool) []Entry {
	var result []Entry
	for _, e := range AllFail() {
		if fn(e) {
			result = append(result, e)
		}
	}
	return result
}

func fromVDLEntries(groups ...[]vdlEntry) []Entry {
	var result []Entry
	for _, entries := range groups {
		for _, e := range entries {
			result = append(result, fromVDLEntry(e))
			result = append(result, genExtraEntries(e)...)
		}
	}
	return result
}

func fromVDLEntry(e vdlEntry) Entry {
	return Entry{
		IsCanonical: e.IsCanonical,
		Label:       e.Label,
		TargetLabel: e.TargetLabel,
		Target:      rvSafeValueOf(e.Target),
		SourceLabel: e.SourceLabel,
		Source:      rvSafeValueOf(e.Source),
	}
}

func rvSafeValueOf(v interface{}) reflect.Value {
	// reflect.ValueOf(nil) returns an invalid reflect.Value, but we want a valid
	// reflect.Value representing nil.
	if v == nil {
		return reflect.ValueOf(&v).Elem()
	}
	return reflect.ValueOf(v)
}

// genExtraEntries generates extra entries based on e.  None of these are
// canonical; they all deal with quirks in the Go representation of VDL values.
func genExtraEntries(e vdlEntry) []Entry {
	var extra []Entry
	switch ttTarget := vdl.TypeOf(e.Target); {
	case ttTarget.Kind() == vdl.Union:
		// Add entry for top-level union interface.
		entry := fromVDLEntry(e)
		entry.IsCanonical = false
		entry.Label += " [union interface]"
		if target := vdl.WrapInUnionInterface(entry.Target); target.IsValid() {
			entry.Target = target
		} else {
			// If the target is a native type, with a union wire type of union, it
			// won't have a union interface.  There's no extra entry to generate.
			break
		}
		entry.TargetLabel += " [interface]"
		if vdl.TypeOf(e.Source).Kind() == vdl.Union {
			if source := vdl.WrapInUnionInterface(entry.Source); source.IsValid() {
				entry.Source = source
			}
			entry.SourceLabel += " [interface]"
		}
		extra = append(extra, entry)
	case ttTarget == vdl.ErrorType:
		// Add entry for top-level error interface.
		entry := fromVDLEntry(e)
		entry.IsCanonical = false
		entry.Label += " [error interface]"
		entry.Target = rvWrapInErrorInterface(entry.Target)
		entry.TargetLabel += " [interface]"
		if vdl.TypeOf(e.Source) == vdl.ErrorType {
			entry.Source = rvWrapInErrorInterface(entry.Source)
			entry.SourceLabel += " [interface]"
		}
		extra = append(extra, entry)
	}
	// Add entries with up to 2 additional pointers on the target and source.
	const maxPtrs = 2
	for t := 0; t < maxPtrs; t++ {
		for s := 0; s < maxPtrs; s++ {
			if t == 0 && s == 0 {
				continue // skip the 0,0 case, which is already represented by e
			}
			entry := fromVDLEntry(e)
			entry.IsCanonical = false
			entry.Label += " [pointers]"
			entry.Target, entry.TargetLabel = rvAddPointers(t, entry.Target, entry.TargetLabel)
			entry.Source, entry.SourceLabel = rvAddPointers(s, entry.Source, entry.SourceLabel)
			extra = append(extra, entry)
		}
	}
	return extra
}

var rtError = reflect.TypeOf((*error)(nil)).Elem()

// rvWrapInErrorInterface returns a reflect.Value whose type is the Go error
// interface, which is set to rv.
//
// REQUIRES: rv must implement the Go error interface
func rvWrapInErrorInterface(rv reflect.Value) reflect.Value {
	rvError := reflect.New(rtError).Elem()
	if rv.Kind() != reflect.Ptr || !rv.IsNil() {
		// Only set the value for non-nil pointers.  If rv is (*verror.E)(nil), we
		// want to end up with Go error(nil).
		rvError.Set(rv)
	}
	return rvError
}

// rvAddPointers adds num pointers to rv.
func rvAddPointers(num int, rv reflect.Value, label string) (reflect.Value, string) {
	for i := 0; i < num; i++ {
		rvPtr := reflect.New(rv.Type())
		rvPtr.Elem().Set(rv)
		rv = rvPtr
		label = "*" + label
	}
	return rv, label
}
