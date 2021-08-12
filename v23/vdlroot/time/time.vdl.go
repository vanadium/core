// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: time

// Package time defines standard representations of absolute and relative times.
//
// The representations described below are required to provide wire
// compatibility between different programming environments.  Generated code for
// different environments typically provide automatic conversions into native
// representations, for simpler idiomatic usage.
//nolint:revive
package time

import (
	"time"

	"v.io/v23/vdl"
)

var _ = initializeVDL() // Must be first; see initializeVDL comments for details.

// Type definitions
// ================

// Duration represents the elapsed duration between two points in time, with
// up to nanosecond precision.
type Duration struct {
	// Seconds represents the seconds in the duration.  The range is roughly
	// +/-290 billion years, larger than the estimated age of the universe.
	Seconds int64
	// Nanos represents the fractions of a second at nanosecond resolution.  Must
	// be in the inclusive range between +/-999,999,999.
	//
	// In normalized form, durations less than one second are represented with 0
	// Seconds and +/-Nanos.  For durations one second or more, the sign of Nanos
	// must match Seconds, or be 0.
	Nanos int32
}

func (Duration) VDLReflect(struct {
	Name string `vdl:"time.Duration"`
}) {
}

func (x Duration) VDLIsZero() bool { //nolint:gocyclo
	return x == Duration{}
}

func (x Duration) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.StartValue(vdlTypeStruct1); err != nil {
		return err
	}
	if x.Seconds != 0 {
		if err := enc.NextFieldValueInt(0, vdl.Int64Type, x.Seconds); err != nil {
			return err
		}
	}
	if x.Nanos != 0 {
		if err := enc.NextFieldValueInt(1, vdl.Int32Type, int64(x.Nanos)); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *Duration) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	*x = Duration{}
	if err := dec.StartValue(vdlTypeStruct1); err != nil {
		return err
	}
	decType := dec.Type()
	for {
		index, err := dec.NextField()
		switch {
		case err != nil:
			return err
		case index == -1:
			return dec.FinishValue()
		}
		if decType != vdlTypeStruct1 {
			index = vdlTypeStruct1.FieldIndexByName(decType.Field(index).Name)
			if index == -1 {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}
		switch index {
		case 0:
			switch value, err := dec.ReadValueInt(64); {
			case err != nil:
				return err
			default:
				x.Seconds = value
			}
		case 1:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.Nanos = int32(value)
			}
		}
	}
}

// Time represents an absolute point in time with up to nanosecond precision.
//
// Time is represented as the duration before or after a fixed epoch.  The zero
// Time represents the epoch 0001-01-01T00:00:00.000000000Z.  This uses the
// proleptic Gregorian calendar; the calendar runs on an exact 400 year cycle.
// Leap seconds are "smeared", ensuring that no leap second table is necessary
// for interpretation.
//
// This is similar to Go time.Time, but always in the UTC location.
// http://golang.org/pkg/time/#Time
//
// This is similar to conventional "unix time", but with the epoch defined at
// year 1 rather than year 1970.  This allows the zero Time to be used as a
// natural sentry, since it isn't a valid time for many practical applications.
// http://en.wikipedia.org/wiki/Unix_time
type Time struct {
	Seconds int64
	Nanos   int32
}

func (Time) VDLReflect(struct {
	Name string `vdl:"time.Time"`
}) {
}

func (x Time) VDLIsZero() bool { //nolint:gocyclo
	return x == Time{}
}

func (x Time) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.StartValue(vdlTypeStruct2); err != nil {
		return err
	}
	if x.Seconds != 0 {
		if err := enc.NextFieldValueInt(0, vdl.Int64Type, x.Seconds); err != nil {
			return err
		}
	}
	if x.Nanos != 0 {
		if err := enc.NextFieldValueInt(1, vdl.Int32Type, int64(x.Nanos)); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *Time) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	*x = Time{}
	if err := dec.StartValue(vdlTypeStruct2); err != nil {
		return err
	}
	decType := dec.Type()
	for {
		index, err := dec.NextField()
		switch {
		case err != nil:
			return err
		case index == -1:
			return dec.FinishValue()
		}
		if decType != vdlTypeStruct2 {
			index = vdlTypeStruct2.FieldIndexByName(decType.Field(index).Name)
			if index == -1 {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}
		switch index {
		case 0:
			switch value, err := dec.ReadValueInt(64); {
			case err != nil:
				return err
			default:
				x.Seconds = value
			}
		case 1:
			switch value, err := dec.ReadValueInt(32); {
			case err != nil:
				return err
			default:
				x.Nanos = int32(value)
			}
		}
	}
}

// WireDeadline represents the deadline for an operation, where the operation is
// expected to finish before the deadline.  The intended usage is for a client
// to set a deadline on an operation, say one minute from "now", and send the
// deadline to a server.  The server is expected to finish the operation before
// the deadline.
//
// On a single device, it is simplest to represent a deadline as an absolute
// time; when the time now reaches the deadline, the deadline has expired.
// However when sending a deadline between devices with potential clock skew, it
// is often more robust to represent the deadline as a duration from "now".  The
// sender computes the duration from its notion of "now", while the receiver
// computes the absolute deadline from its own notion of "now".
//
// This representation doesn't account for propagation delay, but does ensure
// that the deadline used by the receiver is no earlier than the deadline
// intended by the client.  In many common scenarios the propagation delay is
// small compared to the potential clock skew, making this a simple but
// effective approach.
//
// WireDeadline typically has a native representation called Deadline that is an
// absolute Time, which automatically performs the sender and receiver
// conversions from "now".
type WireDeadline struct {
	// FromNow represents the deadline as a duration from "now".  As a
	// special-case, the 0 duration indicates that there is no deadline; i.e. the
	// deadline is "infinite".
	FromNow time.Duration
}

func (WireDeadline) VDLReflect(struct {
	Name string `vdl:"time.WireDeadline"`
}) {
}

func (x WireDeadline) VDLIsZero() bool { //nolint:gocyclo
	return x == WireDeadline{}
}

func (x WireDeadline) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.StartValue(vdlTypeStruct3); err != nil {
		return err
	}
	if x.FromNow != 0 {
		if err := enc.NextField(0); err != nil {
			return err
		}
		var wire Duration
		if err := DurationFromNative(&wire, x.FromNow); err != nil {
			return err
		}
		if err := wire.VDLWrite(enc); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *WireDeadline) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	*x = WireDeadline{}
	if err := dec.StartValue(vdlTypeStruct3); err != nil {
		return err
	}
	decType := dec.Type()
	for {
		index, err := dec.NextField()
		switch {
		case err != nil:
			return err
		case index == -1:
			return dec.FinishValue()
		}
		if decType != vdlTypeStruct3 {
			index = vdlTypeStruct3.FieldIndexByName(decType.Field(index).Name)
			if index == -1 {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}
		if index == 0 {

			var wire Duration
			if err := wire.VDLRead(dec); err != nil {
				return err
			}
			if err := DurationToNative(wire, &x.FromNow); err != nil {
				return err
			}
		}
	}
}

// Type-check native conversion functions.
var (
	_ func(Duration, *time.Duration) error = DurationToNative
	_ func(*Duration, time.Duration) error = DurationFromNative
	_ func(Time, *time.Time) error         = TimeToNative
	_ func(*Time, time.Time) error         = TimeFromNative
	_ func(WireDeadline, *Deadline) error  = WireDeadlineToNative
	_ func(*WireDeadline, Deadline) error  = WireDeadlineFromNative
)

// Hold type definitions in package-level variables, for better performance.
//nolint:unused
var (
	vdlTypeStruct1 *vdl.Type
	vdlTypeStruct2 *vdl.Type
	vdlTypeStruct3 *vdl.Type
)

var initializeVDLCalled bool

// initializeVDL performs vdl initialization.  It is safe to call multiple times.
// If you have an init ordering issue, just insert the following line verbatim
// into your source files in this package, right after the "package foo" clause:
//
//    var _ = initializeVDL()
//
// The purpose of this function is to ensure that vdl initialization occurs in
// the right order, and very early in the init sequence.  In particular, vdl
// registration and package variable initialization needs to occur before
// functions like vdl.TypeOf will work properly.
//
// This function returns a dummy value, so that it can be used to initialize the
// first var in the file, to take advantage of Go's defined init order.
func initializeVDL() struct{} {
	if initializeVDLCalled {
		return struct{}{}
	}
	initializeVDLCalled = true

	// Register native type conversions first, so that vdl.TypeOf works.
	vdl.RegisterNative(DurationToNative, DurationFromNative)
	vdl.RegisterNative(TimeToNative, TimeFromNative)
	vdl.RegisterNative(WireDeadlineToNative, WireDeadlineFromNative)

	// Register types.
	vdl.Register((*Duration)(nil))
	vdl.Register((*Time)(nil))
	vdl.Register((*WireDeadline)(nil))

	// Initialize type definitions.
	vdlTypeStruct1 = vdl.TypeOf((*Duration)(nil)).Elem()
	vdlTypeStruct2 = vdl.TypeOf((*Time)(nil)).Elem()
	vdlTypeStruct3 = vdl.TypeOf((*WireDeadline)(nil)).Elem()

	return struct{}{}
}
