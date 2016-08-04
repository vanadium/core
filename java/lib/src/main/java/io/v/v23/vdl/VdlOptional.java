// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

import java.lang.reflect.Type;

/**
 * VdlOptional is a representation of a VDL optional.
 *
 * @param <T> The type of the element.
 */
public class VdlOptional<T> extends VdlValue {
    private static final long serialVersionUID = 1L;

    private final T elem;

    /**
     * Creates a VdlOptional of provided VDL type that wraps a provided element.
     *
     * @param vdlType the runtime VDL type of VdlOptional
     * @param element the wrapped element
     */
    public VdlOptional(VdlType vdlType, T element) {
        super(vdlType);
        assertKind(Kind.OPTIONAL);
        this.elem = element;
    }

    /**
     * Creates a VdlOptional of provided VDL type that wraps null.
     *
     * @param vdlType the runtime VDL type of VdlOptional
     */
    public VdlOptional(VdlType vdlType) {
        this(vdlType, null);
    }

    /**
     * Creates a VdlOptional of provided type that wraps null.
     *
     * @param type the runtime type of VdlOptional
     */
    public VdlOptional(Type type) {
        this(Types.getVdlTypeFromReflect(type));
    }

    /**
     * Creates a VdlOptional that wraps a provided non-null VDL value.
     *
     * @param element the wrapped element
     * @throws NullPointerException is the element is null
     */
    public static <T extends VdlValue> VdlOptional<T> of(T element) {
        return new VdlOptional<T>(Types.optionalOf(element.vdlType()), element);
    }

    public boolean isNull() {
        return elem == null;
    }

    public T getElem() {
        return elem;
    }

    @Override
    public boolean equals(Object obj) {
        if (this == obj) return true;
        if (obj == null) return false;
        if (!(obj instanceof VdlOptional)) return false;
        VdlOptional<?> other = (VdlOptional<?>) obj;
        return elem == null ? other.elem == null : elem.equals(other.elem);
    }

    @Override
    public int hashCode() {
        return elem == null ? 0 : elem.hashCode();
    }

    @Override
    public String toString() {
        return elem == null ? null : elem.toString();
    }
}
