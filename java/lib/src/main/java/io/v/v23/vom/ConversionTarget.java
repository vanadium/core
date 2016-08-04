// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vom;

import com.google.common.reflect.TypeToken;
import io.v.v23.vdl.Kind;
import io.v.v23.vdl.Types;
import io.v.v23.vdl.VdlAny;
import io.v.v23.vdl.VdlArray;
import io.v.v23.vdl.VdlBool;
import io.v.v23.vdl.VdlByte;
import io.v.v23.vdl.VdlEnum;
import io.v.v23.vdl.VdlFloat32;
import io.v.v23.vdl.VdlFloat64;
import io.v.v23.vdl.VdlInt16;
import io.v.v23.vdl.VdlInt32;
import io.v.v23.vdl.VdlInt64;
import io.v.v23.vdl.VdlInt8;
import io.v.v23.vdl.VdlList;
import io.v.v23.vdl.VdlMap;
import io.v.v23.vdl.VdlOptional;
import io.v.v23.vdl.VdlSet;
import io.v.v23.vdl.VdlString;
import io.v.v23.vdl.VdlStruct;
import io.v.v23.vdl.VdlType;
import io.v.v23.vdl.VdlTypeObject;
import io.v.v23.vdl.VdlUint16;
import io.v.v23.vdl.VdlUint32;
import io.v.v23.vdl.VdlUint64;
import io.v.v23.vdl.VdlUnion;
import io.v.v23.vdl.VdlValue;

import java.lang.reflect.Type;

/**
 * ConversionTarget keeps type information required for value conversion.
 */
public class ConversionTarget {
    private final Class<?> targetClass;
    private final Type targetType;
    private final VdlType vdlType;

    private static Type getTargetTypeForVdlType(VdlType vdlType) {
        switch (vdlType.getKind()) {
            case ANY: return VdlAny.class;
            case ARRAY: return new TypeToken<VdlArray<VdlValue>>(){}.getType();
            case BOOL: return VdlBool.class;
            case BYTE: return VdlByte.class;
            case ENUM: return VdlEnum.class;
            case FLOAT32: return VdlFloat32.class;
            case FLOAT64: return VdlFloat64.class;
            case INT8: return VdlInt8.class;
            case INT16: return VdlInt16.class;
            case INT32: return VdlInt32.class;
            case INT64: return VdlInt64.class;
            case LIST: return new TypeToken<VdlList<VdlValue>>(){}.getType();
            case MAP: return new TypeToken<VdlMap<VdlValue, VdlValue>>(){}.getType();
            case OPTIONAL: return new TypeToken<VdlOptional<VdlValue>>(){}.getType();
            case SET: return new TypeToken<VdlSet<VdlValue>>(){}.getType();
            case STRING: return VdlString.class;
            case STRUCT: return VdlStruct.class;
            case TYPEOBJECT: return VdlTypeObject.class;
            case UINT16: return VdlUint16.class;
            case UINT32: return VdlUint32.class;
            case UINT64: return VdlUint64.class;
            case UNION: return VdlUnion.class;
            default:
                throw new IllegalArgumentException("Unsupported VDL type " + vdlType);
        }
    }

    public ConversionTarget(Type targetType, VdlType vdlType) {
        this.targetType = targetType;
        this.targetClass = ReflectUtil.getRawClass(targetType);
        this.vdlType = vdlType;
    }

    public ConversionTarget(Type targetType) {
        this(targetType, Types.getVdlTypeFromReflect(targetType));
    }

    public ConversionTarget(VdlType targetVdlType) {
        this(getTargetTypeForVdlType(targetVdlType), targetVdlType);
    }

    public Class<?> getTargetClass() {
        return targetClass;
    }

    public Type getTargetType() {
        return targetType;
    }

    public VdlType getVdlType() {
        return vdlType;
    }

    public Kind getKind() {
        return vdlType.getKind();
    }
}
