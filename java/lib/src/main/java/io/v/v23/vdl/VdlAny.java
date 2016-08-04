// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

import java.lang.reflect.Type;
import java.lang.reflect.Array;
import java.util.Arrays;
import java.util.Objects;

/**
* VdlAny is a representation of a VDL any.
*/
public final class VdlAny extends VdlValue {
  private static final long serialVersionUID = 1L;

  /**
  * Vdl type for {@link VdlAny}.
  */
  public static final io.v.v23.vdl.VdlType VDL_TYPE = io.v.v23.vdl.Types.ANY;

  private final Object elem;
  private final VdlType elemType;

  public VdlAny(VdlType vdlType, Object value) {
    super(Types.ANY);
    elem = value;
    elemType = vdlType;
  }

  public VdlAny(Type type, Object value) {
    this(Types.getVdlTypeFromReflect(type), value);
  }

  public VdlAny(VdlValue value) {
    this(value.vdlType(), value);
  }

  public VdlAny() {
    this((VdlType) null, null);
  }

  public Object getElem() {
    return elem;
  }

  public VdlType getElemType() {
    return elemType;
  }

  @Override
  public boolean equals(Object obj) {
    if (this == obj) return true;
    if (obj == null) return false;
    if (this.getClass() != obj.getClass()) return false;
    VdlAny other = (VdlAny) obj;

    if (elem == null) {
      return other.elem == null;
    }
    if (other.elem == null) {
      return false;
    }
    if (elemType != other.elemType) {
      return false;
    }

    if (elem.getClass().isArray() != other.elem.getClass().isArray()) {
      return false;
    }
    if (elem.getClass().isArray()) {
      if (Array.getLength(elem) != Array.getLength(other.elem)) {
        return false;
      }
      for (int i = 0; i < Array.getLength(elem); i++) {
        Object obj1 = Array.get(elem, i);
        Object obj2 = Array.get(other.elem, i);
        if (!Objects.equals(obj1, obj2)) {
          return false;
        }
      }
      return true;
    }
    return elem == null ? other.elem == null : elem.equals(other.elem);
  }

  @Override
  public int hashCode() {
    if (elem != null && elem.getClass().isArray()) {
      Object[] arr = new Object[Array.getLength(elem)];
      for (int i = 0; i < Array.getLength(elem); i++) {
        arr[i] = Array.get(elem, i);
      }
      return Objects.hash(elemType, Arrays.hashCode(arr));
    } else {
      return Objects.hash(elemType, elem);
    }
  }

  @Override
  public String toString() {
    if (elem == null) {
      return "any(null)";
    }
    String typeString = elemType.toString();
    if (elem.getClass().isArray()) {
      StringBuilder sb = new StringBuilder();
      sb.append(typeString);
      sb.append("{");
      for (int i = 0; i < Array.getLength(elem); i++) {
        if (i > 0) {
          sb.append(",");
        }
        sb.append(Array.get(elem, i).toString());
      }
      sb.append("}");
      return sb.toString();
    }
    return typeString+elem.toString();
  }
}
