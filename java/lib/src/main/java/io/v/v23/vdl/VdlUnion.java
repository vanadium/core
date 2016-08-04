// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

/**
 * VdlUnion is a representation of a VDL union.
 */
public class VdlUnion extends VdlValue {
    private static final long serialVersionUID = 1L;

    private Object elem;
    private int index;

    protected VdlUnion(VdlType type, int index, Object elem) {
        super(type);
        assertKind(Kind.UNION);
        if (index < 0 || index > type.getFields().size()) {
            throw new IndexOutOfBoundsException("Union index " + index + " is out of range " + 0 +
                    "..." + (type.getFields().size() - 1));
        }
        this.index = index;
        this.elem = elem;
    }

    public VdlUnion(VdlType type, int index, VdlType elemType, Object elem) {
        this(type, index, elem);
        if (!vdlType().getFields().get(index).getType().equals(elemType)) {
            throw new IllegalArgumentException("Illegal type " + elemType + " of elem: it should"
                    + "be " + vdlType().getFields().get(index).getType());
        }
    }

    public Object getElem() {
        return elem;
    }

    public int getIndex() {
        return index;
    }

    public String getName() {
        return vdlType().getFields().get(index).getName();
    }

    @Override
    public boolean equals(Object obj) {
        if (this == obj) return true;
        if (!(obj instanceof VdlUnion)) return false;
        VdlUnion other = (VdlUnion) obj;
        return getElem().equals(other.getElem());
    }

    @Override
    public int hashCode() {
        return elem == null ? 0 : elem.hashCode();
    }

    @Override
    public String toString() {
        return elem.toString();
    }
}
