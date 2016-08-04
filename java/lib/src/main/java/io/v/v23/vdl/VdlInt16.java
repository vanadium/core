// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

/**
 * VdlInt16 is a representation of a VDL int16.
 */
public class VdlInt16 extends VdlValue {
    private static final long serialVersionUID = 1L;

    private final short value;

    public VdlInt16(VdlType type, short value) {
        super(type);
        assertKind(Kind.INT16);
        this.value = value;
    }

    public VdlInt16(short value) {
        this(Types.INT16, value);
    }

    public VdlInt16() {
        this((short) 0);
    }

    protected VdlInt16(VdlType type) {
        this(type, (short) 0);
    }

    public short getValue() {
        return this.value;
    }

    @Override
    public boolean equals(Object obj) {
        if (this == obj) return true;
        if (!(obj instanceof VdlInt16)) return false;
        VdlInt16 other = (VdlInt16) obj;
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
