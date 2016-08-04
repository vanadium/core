// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vdltest

import (
	"reflect"

	"v.io/v23/vdl"
)

// Entry represents a test entry, which contains a target and source value.
// Each test tries to convert the source value to the type of the target value.
// This conversion will either pass or fail; AllPass returns passing entries,
// while AllFail returns failing entries.
//
// An entry may either be canonical or not.  For a given canonical entry C,
// C.Target==C.Source always holds, and in addition, C.Source is the canonical
// representation of the value.
type Entry struct {
	IsCanonical bool
	Label       string        // Label describes the entry, e.g. Full, Random...
	TargetLabel string        // TargetLabel describes the Target value
	Target      reflect.Value // Target value for conversion test
	SourceLabel string        // SourceLabel describes the Source value
	Source      reflect.Value // Source value for conversion test
}

// Name returns the name of the entry, which combines the entry, target and
// source labels.
func (e Entry) Name() string {
	return e.Label + " Target(" + e.TargetLabel + ") Source(" + e.SourceLabel + ")"
}

// EntryValue is like Entry, but represents the target and source values as
// *vdl.Value, rather than interface{}.
type EntryValue struct {
	Label       string
	TargetLabel string
	Target      *vdl.Value
	SourceLabel string
	Source      *vdl.Value
}

// Name returns the name of the EntryValue.
func (e EntryValue) Name() string {
	tLabel := e.TargetLabel
	if tLabel == "" {
		tLabel = e.Target.String()
	}
	sLabel := e.SourceLabel
	if sLabel == "" {
		sLabel = e.Source.String()
	}
	return e.Label + " Target(" + tLabel + ") Source(" + sLabel + ")"
}

// ToEntryValue converts the Entry e into an EntryValue.
func ToEntryValue(e Entry) EntryValue {
	target, err := vdl.ValueFromReflect(e.Target)
	if err != nil {
		panic(err)
	}
	source, err := vdl.ValueFromReflect(e.Source)
	if err != nil {
		panic(err)
	}
	return EntryValue{
		Label:       e.Label,
		TargetLabel: e.TargetLabel,
		Target:      target,
		SourceLabel: e.SourceLabel,
		Source:      source,
	}
}

// ToEntryValues converts each Entry in entries into a corresponding EntryValue.
func ToEntryValues(entries []Entry) []EntryValue {
	var result []EntryValue
	for _, e := range entries {
		result = append(result, ToEntryValue(e))
	}
	return result
}
