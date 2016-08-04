// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

/**
 * VdlUint32 is a representation of a VDL uint32.
 */
public class VdlUint32 extends VdlValue {
    private static final long serialVersionUID = 1L;

    private final int value;

    public VdlUint32(VdlType type, int value) {
        super(type);
        assertKind(Kind.UINT32);
        this.value = value;
    }

    public VdlUint32(int value) {
        this(Types.UINT32, value);
    }

    public VdlUint32() {
        this(0);
    }

    protected VdlUint32(VdlType type) {
        this(type, 0);
    }

    public int getValue() {
        return value;
    }

    @Override
    public boolean equals(Object obj) {
        if (this == obj) return true;
        if (!(obj instanceof VdlUint32)) return false;
        VdlUint32 other = (VdlUint32) obj;
        return value == other.value;
    }

    @Override
    public int hashCode() {
        return value;
    }

    @Override
    public String toString() {
        return Integer.toString(value);
    }
}
