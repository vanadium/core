// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

/**
 * VdlByte is a representation of a VDL byte.
 */
public class VdlByte extends VdlValue {
    private static final long serialVersionUID = 1L;

    private final byte value;

    public VdlByte(VdlType type, byte value) {
        super(type);
        assertKind(Kind.BYTE);
        this.value = value;
    }

    public VdlByte(byte value) {
        this(Types.BYTE, value);
    }

    public byte getValue() {
        return this.value;
    }

    public VdlByte() {
        this((byte) 0);
    }

    protected VdlByte(VdlType type) {
        this(type, (byte) 0);
    }

    @Override
    public boolean equals(Object obj) {
        if (this == obj) return true;
        if (!(obj instanceof VdlByte)) return false;
        VdlByte other = (VdlByte) obj;
        return value == other.value;
    }

    @Override
    public int hashCode() {
        return value;
    }

    @Override
    public String toString() {
        return Byte.toString(value);
    }
}
