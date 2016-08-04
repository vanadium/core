// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

/**
 * VdlInt64 is a representation of a VDL int64.
 */
public class VdlInt64 extends VdlValue {
    private static final long serialVersionUID = 1L;

    private final long value;

    public VdlInt64(VdlType type, long value) {
        super(type);
        assertKind(Kind.INT64);
        this.value = value;
    }

    public VdlInt64(long value) {
        this(Types.INT64, value);
    }

    public VdlInt64() {
        this(0);
    }

    protected VdlInt64(VdlType type) {
        this(type, 0);
    }

    public long getValue() {
        return this.value;
    }

    @Override
    public boolean equals(Object obj) {
        if (this == obj) return true;
        if (!(obj instanceof VdlInt64)) return false;
        VdlInt64 other = (VdlInt64) obj;
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
