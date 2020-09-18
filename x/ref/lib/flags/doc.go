// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package flags implements utilities to augment the standard Go flag package.
// It defines commonly used Vanadium flags, and implementations of the
// flag.Value interface for those flags to ensure that only valid values of
// those flags are supplied.  Some of these flags may also be specified using
// environment variables directly and are documented accordingly; in these cases
// the command line value takes precedence over the environment variable.
//
// Flags are defined as 'groups' of related flags so that the caller may choose
// which ones to use without having to be burdened with the full set.  The
// groups may be used directly or via the Flags type that aggregates multiple
// groups.  In all cases, the flags are registered with a supplied flag.FlagSet
// and hence are not forced onto the command line unless the caller passes in
// flag.CommandLine as the flag.FlagSet to use.
//
// In general, this package will be used by vanadium profiles and the runtime
// implementations, but can also be used by any application that wants access to
// the flags and environment variables it supports.
//
// Default values are provided for all flags that can be overridden either
// by calling functions in this package, or globally, by providing an alternative
// implementation of the ./flags/sitedefaults package. sitedefaults is sub-module
// that can be overridden using a 'replace' statement in go.mod to provide
// site specific defaults.
package flags
