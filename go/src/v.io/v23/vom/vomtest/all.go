// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vomtest provides protocol conformance tests for the Vanadium Object
// Marshaller (VOM).
package vomtest

import (
	"flag"
	"reflect"
	"strings"
)

// The following causes data files to be generated when "go generate" is run.
//go:generate ./gen.sh

var flagVOMTest string

// TODO(toddw): There seems to be a bug in the Go toolchain that prevents this
// init func from being run from the v.io/v23/vom tests, so the flag isn't
// registered.  Investigate this.
func init() {
	flag.StringVar(&flagVOMTest, "vomtest", "", `Filter vomtest.All to only return entries that contain the given substring.  If the value starts with "!", only returns entries that don't contain the given substring.`)
}

func filter() (bool, string) {
	if strings.HasPrefix(flagVOMTest, "!") {
		return false, strings.TrimPrefix(flagVOMTest, "!")
	}
	return true, flagVOMTest
}

// The following vars are defined in generated files:
//   var pass81 []Entry

// AllPass returns all entries that pass encoding and decoding tests.
//
// The -vomtest flag may be used to filter the returned entries.
func AllPass() []Entry {
	var result []Entry
	for _, e := range fromVDLEntries(pass81) {
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

// AllFail returns all entries that fail encoding and decoding tests.
//
// The -vomtest flag may be used to filter the returned entries.
func AllFail() []Entry {
	var result []Entry
	// TODO(toddw): Add failing tests.
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
		}
	}
	return result
}

func fromVDLEntry(e vdlEntry) Entry {
	return Entry{
		Label:      e.Label,
		ValueLabel: e.ValueLabel,
		Value:      rvSafeValueOf(e.Value),
		Version:    e.Version,
		HexType:    e.HexType,
		HexValue:   e.HexValue,
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
