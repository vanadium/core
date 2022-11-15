// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"reflect"
	"time"

	"v.io/v23/vom"
)

// Discharge represents a "proof" required for satisfying a ThirdPartyCaveat.
//
// A discharge may have caveats of its own (including ThirdPartyCaveats) that
// restrict the context in which the discharge is usable.
//
// Discharge objects are immutable and multiple goroutines may invoke methods
// on a Discharge simultaneously.
//
// See also: https://vanadium.github.io/glossary.html#discharge
type Discharge struct {
	wire WireDischarge
}

// Discharges represents a set of Discharges.
type Discharges []Discharge

// Find returns the Discharge with the requested id.
func (dl Discharges) Find(id string) (Discharge, bool) {
	for _, d := range dl {
		if d.ID() == id {
			return d, true
		}
	}
	return Discharge{}, false
}

// Equivalent returns true if discharges contains an equivalent discharge
// to that requested, that is, one with the same ID and that satifies
// discharge.Equivalent.
func (dl Discharges) Equivalent(discharge Discharge) bool {
	id := discharge.ID()
	for _, d := range dl {
		if d.ID() == id {
			return discharge.Equivalent(d)
		}
	}
	return false
}

// Copy returns a copy of the Discharges.
func (dl Discharges) Copy() Discharges {
	// avoid unnecessary allocation.
	if len(dl) == 0 {
		return nil
	}
	ret := make([]Discharge, len(dl))
	copy(ret, dl)
	return ret
}

// ID returns the identifier for the third-party caveat that d is a discharge
// for.
func (d Discharge) ID() string {
	switch v := d.wire.(type) {
	case WireDischargePublicKey:
		return v.Value.ThirdPartyCaveatId
	default:
		return ""
	}
}

// ThirdPartyCaveats returns the set of third-party caveats on the scope of the
// discharge.
func (d Discharge) ThirdPartyCaveats() []ThirdPartyCaveat {
	var ret []ThirdPartyCaveat
	if v, ok := d.wire.(WireDischargePublicKey); ok {
		for _, cav := range v.Value.Caveats {
			if tp := cav.ThirdPartyDetails(); tp != nil {
				ret = append(ret, tp)
			}
		}
	}
	return ret
}

// Expiry returns the time at which d will no longer be valid, or the zero
// value of time.Time if the discharge does not expire.
func (d Discharge) Expiry() time.Time {
	var min time.Time
	if v, ok := d.wire.(WireDischargePublicKey); ok {
		for _, cav := range v.Value.Caveats {
			if t := expiryTime(cav); !t.IsZero() && (min.IsZero() || t.Before(min)) {
				min = t
			}
		}
	}
	return min
}

// Equivalent returns true if 'd' and 'discharge' can be used interchangeably,
// i.e. any authorizations that are enabled by 'd' will be enabled by
// 'discharge' and vice versa.
func (d Discharge) Equivalent(discharge Discharge) bool {
	return reflect.DeepEqual(d, discharge)
}

// VDLIsZero implements the vdl.IsZeroer interface, and returns true if d
// represents an empty discharge.
func (d Discharge) VDLIsZero() bool {
	return d.wire == nil || d.wire.VDLIsZero()
}

func WireDischargeToNative(wire WireDischarge, native *Discharge) error {
	native.wire = wire
	return nil
}

func WireDischargeFromNative(wire *WireDischarge, native Discharge) error {
	*wire = native.wire
	return nil
}

func expiryTime(cav Caveat) time.Time {
	if cav.Id == ExpiryCaveat.Id {
		var t time.Time
		if err := vom.Decode(cav.ParamVom, &t); err != nil {
			// TODO(jsimsa): Decide what (if any) logging mechanism to use.
			// vlog.Errorf("Failed to decode ParamVOM for cav(%v): %v", cav, err)
			return time.Time{}
		}
		return t
	}
	return time.Time{}
}
