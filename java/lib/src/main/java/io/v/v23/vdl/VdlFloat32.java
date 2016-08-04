// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

/**
 * VdlFloat32 is a representation of a VDL float32.
 */
public class VdlFloat32 extends VdlValue {
    private static final long serialVersionUID = 1L;

    private final float value;

    public VdlFloat32(VdlType type, float value) {
        super(type);
        assertKind(Kind.FLOAT32);
        this.value = value;
    }

    public VdlFloat32(float value) {
        this(Types.FLOAT32, value);
    }

    public VdlFloat32() {
        this(0);
    }

    protected VdlFloat32(VdlType type) {
        this(type, 0);
    }

    public float getValue() {
        return this.value;
    }

    @Override
    public boolean equals(Object obj) {
        if (this == obj) return true;
        if (!(obj instanceof VdlFloat32)) return false;
        VdlFloat32 other = (VdlFloat32) obj;
        return value == other.value;
    }

    @Override
    public int hashCode() {
        return Float.valueOf(value).hashCode();
    }

    @Override
    public String toString() {
        return Float.toString(value);
    }
}
