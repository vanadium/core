// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

import java.lang.reflect.Type;

/**
 * VdlTypeObject is a representation of a VDL typeObject.
 */
public final class VdlTypeObject extends VdlValue {
    private static final long serialVersionUID = 1L;

    private final VdlType typeObject;

    public VdlTypeObject(VdlType typeObject) {
        super(Types.TYPEOBJECT);
        this.typeObject = typeObject;
    }

    public VdlTypeObject(Type type) {
        this(Types.getVdlTypeFromReflect(type));
    }

    public VdlTypeObject() {
        this(Types.ANY);
    }

    public VdlType getTypeObject() {
        return typeObject;
    }

    @Override
    public boolean equals(Object obj) {
        if (this == obj) return true;
        if (obj == null) return false;
        if (this.getClass() != obj.getClass()) return false;
        VdlTypeObject other = (VdlTypeObject) obj;
        return typeObject.equals(other.typeObject);
    }

    @Override
    public int hashCode() {
        return typeObject.hashCode();
    }

    @Override
    public String toString() {
        return typeObject.toString();
    }
}
