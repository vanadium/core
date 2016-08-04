// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

/**
 * VdlInt8 is a representation of a VDL int8.
 */
public class VdlInt8 extends VdlValue {
    private static final long serialVersionUID = 1L;

    private final byte value;

    public VdlInt8(VdlType type, byte value) {
        super(type);
        assertKind(Kind.INT8);
        this.value = value;
    }

    public VdlInt8(byte value) {
        this(Types.INT8, value);
    }

    public VdlInt8() {
        this((byte) 0);
    }

    protected VdlInt8(VdlType type) {
        this(type, (byte) 0);
    }

    public byte getValue() {
        return this.value;
    }

    @Override
    public boolean equals(Object obj) {
        if (this == obj) return true;
        if (!(obj instanceof VdlInt8)) return false;
        VdlInt8 other = (VdlInt8) obj;
        return value == other.value;
    }

    @Override
    public int hashCode() {
        return value;
    }

    @Override
    public String toString() {
        return Short.toString(value);
    }
}
