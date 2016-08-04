// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

/**
 * VdlUint16 is a representation of a VDL uint16.
 */
public class VdlUint16 extends VdlValue {
    private static final long serialVersionUID = 1L;

    private final short value;

    public VdlUint16(VdlType type, short value) {
        super(type);
        assertKind(Kind.UINT16);
        this.value = value;
    }

    public VdlUint16(short value) {
        this(Types.UINT16, value);
    }

    public VdlUint16() {
        this((short) 0);
    }

    protected VdlUint16(VdlType type) {
        this(type, (short) 0);
    }

    public short getValue() {
        return this.value;
    }

    @Override
    public boolean equals(Object obj) {
        if (this == obj) return true;
        if (!(obj instanceof VdlUint16)) return false;
        VdlUint16 other = (VdlUint16) obj;
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
