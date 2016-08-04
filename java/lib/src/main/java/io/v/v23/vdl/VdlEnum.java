// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

/**
 * VdlEnum is a representation of a VDL enum.
 */
public class VdlEnum extends VdlValue {
    private static final long serialVersionUID = 1L;

    private final String name;
    private final int ordinal;

    public VdlEnum(VdlType type, String name) {
        super(type);
        assertKind(Kind.ENUM);
        this.ordinal = type.getLabels().indexOf(name);
        if (this.ordinal == -1) {
            throw new IllegalArgumentException("Undeclared enum label " + name);
        }
        this.name = name;
    }

    public String name() {
        return name;
    }

    public int ordinal() {
        return ordinal;
    }

    @Override
    public boolean equals(Object obj) {
        if (this == obj) return true;
        if (!(obj instanceof VdlEnum)) return false;
        VdlEnum other = (VdlEnum) obj;
        return name.equals(other.name);
    }

    @Override
    public int hashCode() {
        return name.hashCode();
    }

    @Override
    public String toString() {
        return name;
    }
}
