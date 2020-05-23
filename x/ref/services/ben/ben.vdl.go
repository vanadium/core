// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated by the vanadium vdl tool.
// Package: ben

// Package ben defines datastructures to archive microbenchmark results.
//
// These are the data structures common to tools described in
// https://docs.google.com/document/d/1v-iKwej3eYT_RNhPwQ81A9fa8H15Q6RzNyv2rrAeAUc/edit?usp=sharing
package ben

import (
	"v.io/v23/vdl"
)

var _ = initializeVDL() // Must be first; see initializeVDL comments for details.

//////////////////////////////////////////////////
// Type definitions

// Cpu describes the CPU of the machine on which the microbenchmarks were run.
type Cpu struct {
	Architecture  string // Architecture of the CPU, e.g. "amd64", "386" etc.
	Description   string // A detailed description of the CPU, e.g., "Intel(R) Core(TM) i7-5557U CPU @ 3.10GHz"
	ClockSpeedMhz uint32 // Clock speed of the CPU in MHz
}

func (Cpu) VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/ben.Cpu"`
}) {
}

func (x Cpu) VDLIsZero() bool { //nolint:gocyclo
	return x == Cpu{}
}

func (x Cpu) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.StartValue(vdlTypeStruct1); err != nil {
		return err
	}
	if x.Architecture != "" {
		if err := enc.NextFieldValueString(0, vdl.StringType, x.Architecture); err != nil {
			return err
		}
	}
	if x.Description != "" {
		if err := enc.NextFieldValueString(1, vdl.StringType, x.Description); err != nil {
			return err
		}
	}
	if x.ClockSpeedMhz != 0 {
		if err := enc.NextFieldValueUint(2, vdl.Uint32Type, uint64(x.ClockSpeedMhz)); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *Cpu) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	*x = Cpu{}
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
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.Architecture = value
			}
		case 1:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.Description = value
			}
		case 2:
			switch value, err := dec.ReadValueUint(32); {
			case err != nil:
				return err
			default:
				x.ClockSpeedMhz = uint32(value)
			}
		}
	}
}

// Os describes the Operating System on which the microbenchmarks were run.
type Os struct {
	Name    string // Short name of the operating system: linux, darwin, android etc.
	Version string // Details of the distribution/version, e.g., "Ubuntu 14.04", "Mac OS X 10.11.2 15C50" etc.
}

func (Os) VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/ben.Os"`
}) {
}

func (x Os) VDLIsZero() bool { //nolint:gocyclo
	return x == Os{}
}

func (x Os) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.StartValue(vdlTypeStruct2); err != nil {
		return err
	}
	if x.Name != "" {
		if err := enc.NextFieldValueString(0, vdl.StringType, x.Name); err != nil {
			return err
		}
	}
	if x.Version != "" {
		if err := enc.NextFieldValueString(1, vdl.StringType, x.Version); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *Os) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	*x = Os{}
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
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.Name = value
			}
		case 1:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.Version = value
			}
		}
	}
}

// Scenario encapsulates the conditions on the machine on which the microbenchmarks were run.
type Scenario struct {
	Cpu   Cpu
	Os    Os
	Label string // Arbitrary string label assigned by the uploader.
}

func (Scenario) VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/ben.Scenario"`
}) {
}

func (x Scenario) VDLIsZero() bool { //nolint:gocyclo
	return x == Scenario{}
}

func (x Scenario) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.StartValue(vdlTypeStruct3); err != nil {
		return err
	}
	if x.Cpu != (Cpu{}) {
		if err := enc.NextField(0); err != nil {
			return err
		}
		if err := x.Cpu.VDLWrite(enc); err != nil {
			return err
		}
	}
	if x.Os != (Os{}) {
		if err := enc.NextField(1); err != nil {
			return err
		}
		if err := x.Os.VDLWrite(enc); err != nil {
			return err
		}
	}
	if x.Label != "" {
		if err := enc.NextFieldValueString(2, vdl.StringType, x.Label); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *Scenario) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	*x = Scenario{}
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
		switch index {
		case 0:
			if err := x.Cpu.VDLRead(dec); err != nil {
				return err
			}
		case 1:
			if err := x.Os.VDLRead(dec); err != nil {
				return err
			}
		case 2:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.Label = value
			}
		}
	}
}

// SourceCode represents the state of the source code used to build the
// microbenchmarks.
//
// Typically it would be the commit hash of a git repository or the contents of
// a manifest of a jiri (https://github.com/vanadium/go.jiri) project and not
// the complete source code itself.
type SourceCode string

func (SourceCode) VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/ben.SourceCode"`
}) {
}

func (x SourceCode) VDLIsZero() bool { //nolint:gocyclo
	return x == ""
}

func (x SourceCode) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.WriteValueString(vdlTypeString4, string(x)); err != nil {
		return err
	}
	return nil
}

func (x *SourceCode) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	switch value, err := dec.ReadValueString(); {
	case err != nil:
		return err
	default:
		*x = SourceCode(value)
	}
	return nil
}

// Run encapsulates the results of a single microbenchmark run.
type Run struct {
	Name              string // Name of the microbenchmark. <package>.Benchmark<Name> in Go.
	Iterations        uint64
	NanoSecsPerOp     float64 // Nano-seconds per iteration.
	AllocsPerOp       uint64  // Memory allocations per iteration.
	AllocedBytesPerOp uint64  // Size of memory allocations per iteration.
	MegaBytesPerSec   float64 // Throughput in MB/s.
	Parallelism       uint32  // For Go, the GOMAXPROCS used during benchmark execution
}

func (Run) VDLReflect(struct {
	Name string `vdl:"v.io/x/ref/services/ben.Run"`
}) {
}

func (x Run) VDLIsZero() bool { //nolint:gocyclo
	return x == Run{}
}

func (x Run) VDLWrite(enc vdl.Encoder) error { //nolint:gocyclo
	if err := enc.StartValue(vdlTypeStruct5); err != nil {
		return err
	}
	if x.Name != "" {
		if err := enc.NextFieldValueString(0, vdl.StringType, x.Name); err != nil {
			return err
		}
	}
	if x.Iterations != 0 {
		if err := enc.NextFieldValueUint(1, vdl.Uint64Type, x.Iterations); err != nil {
			return err
		}
	}
	if x.NanoSecsPerOp != 0 {
		if err := enc.NextFieldValueFloat(2, vdl.Float64Type, x.NanoSecsPerOp); err != nil {
			return err
		}
	}
	if x.AllocsPerOp != 0 {
		if err := enc.NextFieldValueUint(3, vdl.Uint64Type, x.AllocsPerOp); err != nil {
			return err
		}
	}
	if x.AllocedBytesPerOp != 0 {
		if err := enc.NextFieldValueUint(4, vdl.Uint64Type, x.AllocedBytesPerOp); err != nil {
			return err
		}
	}
	if x.MegaBytesPerSec != 0 {
		if err := enc.NextFieldValueFloat(5, vdl.Float64Type, x.MegaBytesPerSec); err != nil {
			return err
		}
	}
	if x.Parallelism != 0 {
		if err := enc.NextFieldValueUint(6, vdl.Uint32Type, uint64(x.Parallelism)); err != nil {
			return err
		}
	}
	if err := enc.NextField(-1); err != nil {
		return err
	}
	return enc.FinishValue()
}

func (x *Run) VDLRead(dec vdl.Decoder) error { //nolint:gocyclo
	*x = Run{}
	if err := dec.StartValue(vdlTypeStruct5); err != nil {
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
		if decType != vdlTypeStruct5 {
			index = vdlTypeStruct5.FieldIndexByName(decType.Field(index).Name)
			if index == -1 {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}
		switch index {
		case 0:
			switch value, err := dec.ReadValueString(); {
			case err != nil:
				return err
			default:
				x.Name = value
			}
		case 1:
			switch value, err := dec.ReadValueUint(64); {
			case err != nil:
				return err
			default:
				x.Iterations = value
			}
		case 2:
			switch value, err := dec.ReadValueFloat(64); {
			case err != nil:
				return err
			default:
				x.NanoSecsPerOp = value
			}
		case 3:
			switch value, err := dec.ReadValueUint(64); {
			case err != nil:
				return err
			default:
				x.AllocsPerOp = value
			}
		case 4:
			switch value, err := dec.ReadValueUint(64); {
			case err != nil:
				return err
			default:
				x.AllocedBytesPerOp = value
			}
		case 5:
			switch value, err := dec.ReadValueFloat(64); {
			case err != nil:
				return err
			default:
				x.MegaBytesPerSec = value
			}
		case 6:
			switch value, err := dec.ReadValueUint(32); {
			case err != nil:
				return err
			default:
				x.Parallelism = uint32(value)
			}
		}
	}
}

// Hold type definitions in package-level variables, for better performance.
//nolint:unused
var (
	vdlTypeStruct1 *vdl.Type
	vdlTypeStruct2 *vdl.Type
	vdlTypeStruct3 *vdl.Type
	vdlTypeString4 *vdl.Type
	vdlTypeStruct5 *vdl.Type
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

	// Register types.
	vdl.Register((*Cpu)(nil))
	vdl.Register((*Os)(nil))
	vdl.Register((*Scenario)(nil))
	vdl.Register((*SourceCode)(nil))
	vdl.Register((*Run)(nil))

	// Initialize type definitions.
	vdlTypeStruct1 = vdl.TypeOf((*Cpu)(nil)).Elem()
	vdlTypeStruct2 = vdl.TypeOf((*Os)(nil)).Elem()
	vdlTypeStruct3 = vdl.TypeOf((*Scenario)(nil)).Elem()
	vdlTypeString4 = vdl.TypeOf((*SourceCode)(nil))
	vdlTypeStruct5 = vdl.TypeOf((*Run)(nil)).Elem()

	return struct{}{}
}
