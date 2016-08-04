// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

/**
 * VdlInt32 is a representation of a VDL int32.
 */
public class VdlInt32 extends VdlValue {
    private static final long serialVersionUID = 1L;

    private final int value;

    public VdlInt32(VdlType type, int value) {
        super(type);
        assertKind(Kind.INT32);
        this.value = value;
    }

    public VdlInt32(int value) {
        this(Types.INT32, value);
    }

    public VdlInt32() {
        this(0);
    }

    protected VdlInt32(VdlType type) {
        this(type, 0);
    }

    public int getValue() {
        return this.value;
    }

    @Override
    public boolean equals(Object obj) {
        if (this == obj) return true;
        if (!(obj instanceof VdlInt32)) return false;
        VdlInt32 other = (VdlInt32) obj;
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
