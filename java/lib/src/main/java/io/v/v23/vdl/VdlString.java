// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

/**
 * VdlString is a representation of a VDL string.
 */
public class VdlString extends VdlValue {
    private static final long serialVersionUID = 1L;

    private final String value;

    public VdlString(VdlType type, String value) {
        super(type);
        assertKind(Kind.STRING);
        this.value = value;
    }

    public VdlString(String value) {
        this(Types.STRING, value);
    }

    public VdlString() {
        this("");
    }

    protected VdlString(VdlType type) {
        this(type, "");
    }

    public String getValue() {
        return value;
    }

    @Override
    public boolean equals(Object obj) {
        if (this == obj) return true;
        if (!(obj instanceof VdlString)) return false;
        final VdlString other = (VdlString) obj;
        return value.equals(other.value);
    }

    @Override
    public int hashCode() {
        return value.hashCode();
    }

    @Override
    public String toString() {
        return value;
    }
}
