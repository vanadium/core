// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

/**
 * VdlBool is a representation of a VDL bool.
 */
public class VdlBool extends VdlValue {
    private static final long serialVersionUID = 1L;

    private final boolean value;

    public VdlBool(VdlType type, boolean value) {
        super(type);
        assertKind(Kind.BOOL);
        this.value = value;
    }

    public VdlBool(boolean value) {
        this(Types.BOOL, value);
    }

    public VdlBool() {
        this(false);
    }

    protected VdlBool(VdlType type) {
        this(type, false);
    }

    public boolean getValue() {
        return this.value;
    }

    @Override
    public boolean equals(Object obj) {
        if (this == obj) return true;
        if (!(obj instanceof VdlBool)) return false;
        VdlBool other = (VdlBool) obj;
        return value == other.value;
    }

    @Override
    public int hashCode() {
        return Boolean.valueOf(value).hashCode();
    }

    @Override
    public String toString() {
        return Boolean.toString(value);
    }
}
