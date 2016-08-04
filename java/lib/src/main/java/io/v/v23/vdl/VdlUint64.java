// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

/**
 * VdlUint64 is a representation of a VDL uint64.
 */
public class VdlUint64 extends VdlValue {
    private static final long serialVersionUID = 1L;

    private final long value;

    public VdlUint64(VdlType type, long value) {
        super(type);
        assertKind(Kind.UINT64);
        this.value = value;
    }

    public VdlUint64(long value) {
        this(Types.UINT64, value);
    }

    public VdlUint64() {
        this(0);
    }

    protected VdlUint64(VdlType type) {
        this(type, 0);
    }

    public long getValue() {
        return this.value;
    }

    @Override
    public boolean equals(Object obj) {
        if (this == obj) return true;
        if (!(obj instanceof VdlUint64)) return false;
        VdlUint64 other = (VdlUint64) obj;
        return value == other.value;
    }

    @Override
    public int hashCode() {
        return Long.valueOf(value).hashCode();
    }

    @Override
    public String toString() {
        return Long.toString(value);
    }
}
