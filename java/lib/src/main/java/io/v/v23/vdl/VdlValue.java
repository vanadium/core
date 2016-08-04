// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

import io.v.v23.vom.BinaryDecoder;
import io.v.v23.vom.BinaryEncoder;
import io.v.v23.vom.ConversionException;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.Serializable;
import java.lang.reflect.Type;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.HashSet;

/**
 * Value is the generic representation of any value expressible in Vanadium.  All values are typed.
 */
public abstract class VdlValue implements Serializable {
    private static final long serialVersionUID = 1L;

    private final VdlType type;

    protected VdlValue(VdlType type) {
        this.type = type;
    }

    protected void assertKind(Kind kind) {
        if (type.getKind() != kind) {
            throw new IllegalArgumentException("Kind of VDL type should be " + kind);
        }
    }

    /**
     * Returns the runtime VDL type of this value.
     *
     * @return The {@code Type} object that represents the runtime
     *         VDL type of this VDL value.
     */
    public VdlType vdlType() {
        return type;
    }

    /**
     * Returns the {@code VdlValue} corresponding to the provided Java value of a given
     * {@code VdlType}.
     *
     * @param  value                    Java object to be converted to {@code VdlValue}
     * @param  type                     type of the value
     * @return                          {@code VdlValue} corresponding to the above value
     * @throws IllegalArgumentException if the provided value cannot be converted
     */
    public static VdlValue valueOf(Object value, VdlType type) {
        try {
            // VOM-Encode.
            ByteArrayOutputStream out = new ByteArrayOutputStream();
            BinaryEncoder encoder = new BinaryEncoder(out);
            encoder.encodeValue(type, value);

            // VOM-Decode to VdlValue.
            BinaryDecoder decoder = new BinaryDecoder(new ByteArrayInputStream(out.toByteArray()));
            return (VdlValue) decoder.decodeValue(VdlValue.class);
        } catch (IOException e) {
            throw new IllegalArgumentException(String.format(
                "IO error while converting value %s of type %s into VdlValue",
                value, type), e);
        } catch (ConversionException e) {
            throw new IllegalArgumentException(String.format(
                "Couldn't convert value %s of type %s into VdlValue",
                value, type), e);
        }
    }

    /**
     * Returns the {@code VdlValue} corresponding to the provided Java value of a given
     * reflection type.
     *
     * @param  value                    Java object to be converted to {@code VdlValue}
     * @param  type                     type of the value
     * @return                          {@code VdlValue} corresponding to the above value
     * @throws IllegalArgumentException if the provided value cannot be converted
     */
    public static VdlValue valueOf(Object value, Type type) {
        return valueOf(value, Types.getVdlTypeFromReflect(type));
    }

    /**
     * Returns the zero representation for each kind of VDL type.
     *
     * @throws IllegalArgumentException if the zero value couldn't be constructed
     */
    public static VdlValue zeroValue(VdlType type) {
        if (type == null) {
            throw new IllegalArgumentException("Trying to construct a zero value using null type");
        }
        switch (type.getKind()) {
            case ANY:
                return new VdlAny();
            case ARRAY:
                VdlValue[] backingArray = new VdlValue[type.getLength()];
                VdlValue elemValue = zeroValue(type.getElem());
                for (int i = 0; i < type.getLength(); i++) {
                    backingArray[i] = elemValue;
                }
                return new VdlArray<VdlValue>(type, backingArray);
            case BOOL:
                return new VdlBool();
            case BYTE:
                return new VdlByte();
            case ENUM:
                return new VdlEnum(type, type.getLabels().get(0));
            case FLOAT32:
                return new VdlFloat32();
            case FLOAT64:
                return new VdlFloat64();
            case INT16:
                return new VdlInt16();
            case INT32:
                return new VdlInt32();
            case INT64:
                return new VdlInt64();
            case LIST:
                return new VdlList<VdlValue>(type, new ArrayList<VdlValue>());
            case MAP:
                return new VdlMap<VdlValue, VdlValue>(type, new HashMap<VdlValue, VdlValue>());
            case UNION:
                VdlField zeroField = type.getFields().get(0);
                return new VdlUnion(type, 0, zeroField.getType(), zeroValue(zeroField.getType()));
            case OPTIONAL:
                return new VdlOptional<VdlValue>(type);
            case SET:
                return new VdlSet<VdlValue>(type, new HashSet<VdlValue>());
            case STRING:
                return new VdlString();
            case STRUCT:
                VdlStruct struct = new VdlStruct(type);
                for (VdlField field : type.getFields()) {
                    struct.assignField(field.getName(), zeroValue(field.getType()));
                }
                return struct;
            case TYPEOBJECT:
                return new VdlTypeObject(Types.ANY);
            case UINT16:
                return new VdlUint16();
            case UINT32:
                return new VdlUint32();
            case UINT64:
                return new VdlUint64();
            default:
                throw new IllegalArgumentException("Unhandled kind " + type.getKind());
        }
    }

    /**
     * Same as {@code zeroValue} except if type is {@code OPTIONAL} it returns a value representing
     * the zero value of the element type.
     *
     * @throws IllegalArgumentException if the non-null zero value couldn't be constructed
     */
    public static VdlValue nonNullZeroValue(VdlType type) {
        if (type == null) {
            throw new IllegalArgumentException(
                "Trying to construct a non-null zero value using null type");
        }
        switch (type.getKind()) {
            case ANY:
                throw new IllegalArgumentException("Type ANY doesn't have a non-null zero value");
            case OPTIONAL:
                return VdlOptional.of(zeroValue(type.getElem()));
            default:
                return zeroValue(type);
        }
    }
}
