// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

/**
 * VdlFloat64 is a representation of a VDL float64.
 */
public class VdlFloat64 extends VdlValue {
    private static final long serialVersionUID = 1L;

    private final double value;

    public VdlFloat64(VdlType type, double value) {
        super(type);
        assertKind(Kind.FLOAT64);
        this.value = value;
    }

    public VdlFloat64(double value) {
        this(Types.FLOAT64, value);
    }

    public VdlFloat64() {
        this(0);
    }

    protected VdlFloat64(VdlType type) {
        this(type, 0);
    }

    public double getValue() {
        return this.value;
    }

    @Override
    public boolean equals(Object obj) {
        if (this == obj) return true;
        if (!(obj instanceof VdlFloat64)) return false;
        VdlFloat64 other = (VdlFloat64) obj;
        return value == other.value;
    }

    @Override
    public int hashCode() {
        return Double.valueOf(value).hashCode();
    }

    @Override
    public String toString() {
        return Double.toString(value);
    }
}
