// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vom;

import io.v.v23.vdl.Kind;
import io.v.v23.vdl.NativeTypes;
import io.v.v23.vdl.Types;
import io.v.v23.vdl.VdlAny;
import io.v.v23.vdl.VdlArray;
import io.v.v23.vdl.VdlBool;
import io.v.v23.vdl.VdlByte;
import io.v.v23.vdl.VdlEnum;
import io.v.v23.vdl.VdlField;
import io.v.v23.vdl.VdlFloat32;
import io.v.v23.vdl.VdlFloat64;
import io.v.v23.vdl.VdlInt16;
import io.v.v23.vdl.VdlInt32;
import io.v.v23.vdl.VdlInt64;
import io.v.v23.vdl.VdlInt8;
import io.v.v23.vdl.VdlList;
import io.v.v23.vdl.VdlOptional;
import io.v.v23.vdl.VdlString;
import io.v.v23.vdl.VdlStruct;
import io.v.v23.vdl.VdlType;
import io.v.v23.vdl.VdlTypeObject;
import io.v.v23.vdl.VdlUint16;
import io.v.v23.vdl.VdlUint32;
import io.v.v23.vdl.VdlUint64;
import io.v.v23.vdl.VdlUnion;
import io.v.v23.vdl.VdlValue;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.OutputStream;
import java.lang.reflect.Array;
import java.lang.reflect.Field;
import java.lang.reflect.Type;
import java.util.ArrayList;
import java.util.Collection;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * BinaryEncoder writes VDL values to {@code OutputStream} in binary VOM format.
 */
public class BinaryEncoder {
    private final EncodingStream valueBuffer;
    private final EncodingStream typeBuffer;
    private final OutputStream out;
    private final Map<VdlType, TypeId> visitedTypes;
    private TypeId nextTypeId;
    private boolean binaryMagicByteWritten;
    private Version version;
    private List<Long> typeIds;
    private List<Long> anyLens;

    public BinaryEncoder(OutputStream out) {
        this(out, Constants.DEFAULT_VERSION);
    }

    public BinaryEncoder(OutputStream out, Version version) {
        this.valueBuffer = new EncodingStream();
        this.typeBuffer = new EncodingStream();
        this.out = out;
        this.visitedTypes = new HashMap<VdlType, TypeId>();
        this.nextTypeId = Constants.WIRE_ID_FIRST_USER_TYPE;
        this.binaryMagicByteWritten = false;
        this.version = version;
    }

    /**
     * Encodes a value into binary VOM format.
     *
     * @param type runtime VDL type of the value
     * @param value the value to encode
     * @throws IOException
     */
    public void encodeValue(VdlType type, Object value) throws IOException {
        if (!binaryMagicByteWritten) {
            binaryMagicByteWritten = true;
            out.write(version.getValue());
        }
        valueBuffer.reset();
        typeIds = new ArrayList<Long>();
        anyLens = new ArrayList<Long>();
        TypeId typeId = getType(type);
        writeValue(valueBuffer, value, type);
        writeMessage(valueBuffer, BinaryUtil.hasAny(type), BinaryUtil.hasTypeObject(type),
                false, typeId.getValue(), BinaryUtil.hasBinaryMsgLen(type));
    }

    /**
     * Encodes a value into binary VOM format.
     *
     * @param type runtime {@code  java.lang.reflectType} of the value
     * @param value the value to encode
     * @throws IOException
     */
    public void encodeValue(Type type, Object value) throws IOException {
        encodeValue(Types.getVdlTypeFromReflect(type), value);
    }

    /**
     * Encodes a VDL value into binary VOM format.
     *
     * @param value the value to encode
     * @throws IOException
     */
    public void encodeValue(VdlValue value) throws IOException {
        encodeValue(value.vdlType(), value);
    }

    private void writeMessage(ByteArrayOutputStream buffer, boolean hasAny, boolean hasTypeObject,
                              boolean typeIncomplete, long messageId, boolean encodeLength)
            throws IOException {
        if (version != Constants.VERSION_80 && typeIncomplete) {
            out.write(Constants.WIRE_CTRL_TYPE_INCOMPLETE);
        }
        BinaryUtil.encodeInt(out, messageId);
        if (version != Constants.VERSION_80 && (hasAny || hasTypeObject) && messageId > 0) {
            BinaryUtil.encodeUint(out, typeIds.size());
            for (Long id : typeIds) {
                BinaryUtil.encodeUint(out, id);
            }
            typeIds = null;
        }
        if (version != Constants.VERSION_80 && hasAny && messageId > 0) {
            BinaryUtil.encodeUint(out, anyLens.size());
            for (Long len : anyLens) {
                BinaryUtil.encodeUint(out, len);
            }
            anyLens = null;
        }
        if (encodeLength) {
            BinaryUtil.encodeUint(out, buffer.size());
        }
        buffer.writeTo(out);
    }

    private TypeId getType(VdlType type) throws IOException {
        return getTypeInternal(type, new HashSet<VdlType>());
    }

    private TypeId getTypeInternal(VdlType type, Set<VdlType> pending) throws IOException {
        TypeId typeId = BootstrapType.getBootstrapTypeId(type);
        if (typeId != null) {
            return typeId;
        } else if (visitedTypes.containsKey(type)) {
            return visitedTypes.get(type);
        } else {
            return encodeType(type, pending);
        }
    }

    private TypeId encodeType(VdlType type, Set<VdlType> pending) throws IOException {
        pending.add(type);
        TypeId typeId = nextTypeId;
        nextTypeId = new TypeId(nextTypeId.getValue() + 1);
        visitedTypes.put(type, typeId);

        WireType wireType = convertToWireType(type, pending);
        pending.remove(type);
        boolean incomplete = typeIncomplete(type, pending, new HashSet<VdlType>());
        typeBuffer.reset();
        writeValue(typeBuffer, wireType, wireType.vdlType());
        writeMessage(typeBuffer, BinaryUtil.hasAny(type), BinaryUtil.hasTypeObject(type),
                incomplete, -typeId.getValue(), true);
        return typeId;
    }

    private WireType convertToWireType(VdlType type, Set<VdlType> pending) throws IOException {
        switch (type.getKind()) {
            case INT8:
                if (version == Constants.VERSION_80) {
                    throw new RuntimeException("int8 not supported in VOM version 80");
                }
                // fallthrough
            case BOOL:
            case BYTE:
            case UINT16:
            case UINT32:
            case UINT64:
            case INT16:
            case INT32:
            case INT64:
            case FLOAT32:
            case FLOAT64:
            case STRING:
                return new WireType.NamedT(new WireNamed(
                        type.getName(), getTypeInternal(Types.primitiveTypeFromKind(type.getKind()), pending)));
            case ARRAY:
                return new WireType.ArrayT(new WireArray(
                        type.getName(), getTypeInternal(type.getElem(), pending), new VdlUint64(type.getLength())));
            case ENUM:
                return new WireType.EnumT(new WireEnum(type.getName(), type.getLabels()));
            case LIST:
                return new WireType.ListT(new WireList(type.getName(), getTypeInternal(type.getElem(), pending)));
            case MAP:
                return new WireType.MapT(new WireMap(
                        type.getName(), getTypeInternal(type.getKey(),pending),
                        getTypeInternal(type.getElem(), pending)));
            case STRUCT:
            case UNION:
                List<WireField> wireFields = new ArrayList<WireField>();
                for (VdlField field : type.getFields()) {
                    wireFields.add(new WireField(field.getName(), getTypeInternal(field.getType(), pending)));
                }
                if (type.getKind() == Kind.UNION) {
                    return new WireType.UnionT(new WireUnion(type.getName(), wireFields));
                } else {
                    return new WireType.StructT(new WireStruct(type.getName(), wireFields));
                }
            case SET:
                return new WireType.SetT(new WireSet(type.getName(), getTypeInternal(type.getKey(), pending)));
            case OPTIONAL:
                return new WireType.OptionalT(new WireOptional(
                        type.getName(), getTypeInternal(type.getElem(), pending)));
            default:
                throw new RuntimeException("Unknown wiretype for kind: " + type.getKind());
        }
    }

    private boolean typeIncomplete(VdlType type, Set<VdlType> pending, Set<VdlType> seen) {
        if (seen.contains(type)) {
            return false;
        }
        seen.add(type);
        if (pending.contains(type)) {
            return true;
        }
        switch (type.getKind()) {
            case OPTIONAL:
            case ARRAY:
            case LIST:
                return typeIncomplete(type.getElem(), pending, seen);
            case SET:
                return typeIncomplete(type.getKey(), pending, seen);
            case MAP:
                return typeIncomplete(type.getKey(), pending, seen) ||
                        typeIncomplete(type.getElem(), pending, seen);
            case STRUCT:
            case UNION:
                for (VdlField field : type.getFields()) {
                    if (typeIncomplete(field.getType(), pending, seen)) {
                        return true;
                    }
                }
                return false;
            default:
                return false;
        }
    }

    /**
     * Writes a value to output stream and returns true iff the value is non-zero.
     * The returned value can be used skip encoding of zero fields in structs.
     */
    private boolean writeValue(EncodingStream out, Object value, VdlType type) throws IOException {
        if (value == null) {
            value = VdlValue.zeroValue(type);
        }

        // Convert native value.
        NativeTypes.Converter converter = Types.getNativeTypeConverter(value.getClass());
        if (converter != null) {
            VdlValue vdlValue = converter.vdlValueFromNative(value);
            return writeValue(out, vdlValue, type);
        }
        switch (type.getKind()) {
            case ANY:
                return writeVdlAny(out, value);
            case ARRAY:
                if (type.getElem().getKind() == Kind.BYTE) {
                    return writeVdlBytes(out, value, type);
                } else {
                    return writeVdlArray(out, value);
                }
            case BOOL:
                return writeVdlBool(out, value);
            case ENUM:
                return writeVdlEnum(out, value);
            case FLOAT32:
            case FLOAT64:
                return writeVdlFloat(out, value);
            case INT8:
            case INT16:
            case INT32:
            case INT64:
                if (type.getKind() == Kind.INT8 && version == Constants.VERSION_80) {
                    throw new RuntimeException("int8 not supported in VOM version 80");
                }
                return writeVdlInt(out, value);
            case LIST:
                if (type.getElem().getKind() == Kind.BYTE) {
                    return writeVdlBytes(out, value, type);
                } else {
                    return writeVdlList(out, value, type);
                }
            case MAP:
                return writeVdlMap(out, value, type);
            case UNION:
                return writeVdlUnion(out, value);
            case OPTIONAL:
                return writeVdlOptional(out, value);
            case SET:
                return writeVdlSet(out, value, type);
            case STRING:
                return writeVdlString(out, value);
            case STRUCT:
                return writeVdlStruct(out, value, type);
            case BYTE:
            case UINT16:
            case UINT32:
            case UINT64:
                return writeVdlUint(out, value);
            case TYPEOBJECT:
                return writeVdlTypeObject(out, value);
            default:
                throw new RuntimeException("Unknown kind: " + type.getKind());
        }
    }

    /**
     * Writes a VDL any to output stream and returns true iff the value is non-zero.
     */
    private boolean writeVdlAny(EncodingStream out, Object value) throws IOException {
        expectClass(Kind.ANY, value, VdlAny.class);
        VdlAny anyValue = (VdlAny) value;
        Object elem = anyValue.getElem();

        if (elem != null) {
            long id = getType(anyValue.getElemType()).getValue();
            writeTypeId(out, id);
            int anyLenIndex = -1;
            long startPos = -1;
            if (version != Constants.VERSION_80) {
                anyLenIndex = anyLens.size();
                BinaryUtil.encodeUint(out, anyLenIndex);
                anyLens.add(0L);
                startPos = out.getCount();
            }
            writeValue(out, elem, anyValue.getElemType());
            if (version != Constants.VERSION_80) {
                long endPos = out.getCount();
                anyLens.set(anyLenIndex, endPos - startPos);
            }
            return true;
        } else {
            writeVdlControlByte(out, Constants.WIRE_CTRL_NIL);
            return false;
        }
    }

    /**
     * Writes the type id in a format suitable for the current VOM version.
     */
    public void writeTypeId(OutputStream out, long id) throws IOException {
        if (version == Constants.VERSION_80) {
            BinaryUtil.encodeUint(out, id);
        } else {
            int index = typeIds.indexOf(id);
            if (index == -1) {
                index = typeIds.size();
                typeIds.add(id);
            }
            BinaryUtil.encodeUint(out, index);
        }
    }

    /**
     * Writes a VDL array to output stream and returns true iff the value is non-zero.
     */
    private boolean writeVdlArray(EncodingStream out, Object value) throws IOException {
        expectClass(Kind.ARRAY, value, VdlArray.class);
        VdlArray<?> arrayValue = (VdlArray<?>) value;
        BinaryUtil.encodeUint(out, 0);
        boolean isNonzero = false;
        for (Object elem : arrayValue) {
            isNonzero = writeValue(out, elem, arrayValue.vdlType().getElem()) || isNonzero;
        }
        return isNonzero;
    }

    /**
     * Writes a VDL bool to output stream and returns true iff the value is non-zero.
     */
    private boolean writeVdlBool(OutputStream out, Object value) throws IOException {
        boolean val;
        if (value instanceof VdlBool) {
            val = ((VdlBool) value).getValue();
        } else if (value instanceof Boolean) {
            val = (Boolean) value;
        } else {
            throw new IOException("Unsupported VDL bool value (type " + value.getClass()
                    + ", value " + value + ")");
        }
        int boolAsInt = 0;
        if (val) {
            boolAsInt = 1;
        }
        if (version == Constants.VERSION_80) {
            out.write(boolAsInt);
        } else {
            BinaryUtil.encodeUint(out, boolAsInt);
        }
        return val;
    }

    /**
     * Writes a VDL byte to output stream and returns true iff the value is non-zero.
     * In version 0x80, bytes are written directly as bytes to the stream.
     * In later versions, they are written as vdl uints.
     */
    private boolean writeVdlByte(EncodingStream out, Object value) throws IOException {
        byte byteValue;
        if (value instanceof VdlByte) {
            byteValue = ((VdlByte) value).getValue();
        } else if (value instanceof Byte) {
            byteValue = (Byte) value;
        } else {
            throw new IOException("Unsupported VDL byte value (type " + value.getClass()
                    + ", value " + value + ")");
        }
        if (version == Constants.VERSION_80) {
            out.write(byteValue);
        } else {
            int fullValue = byteValue;
            if (fullValue < 0) {
                fullValue += 0x100;
            }
            BinaryUtil.encodeUint(out, fullValue);
        }
        return byteValue != 0;
    }

    /**
     * Writes a VDL byte to output stream and returns true iff the value is non-zero.
     * Control bytes are always written to the wire directly as bytes.
     */
    private boolean writeVdlControlByte(EncodingStream out, Object value) throws IOException {
        byte byteValue;
        if (value instanceof VdlByte) {
            byteValue = ((VdlByte) value).getValue();
            out.write(byteValue);
        } else if (value instanceof Byte) {
            byteValue = (Byte) value;
            out.write(byteValue);
        } else {
            throw new IOException("Unsupported VDL control byte value (type " + value.getClass()
                    + ", value " + value + ")");
        }
        return byteValue != 0;
    }

    /**
     * Writes a VDL enum to output stream and returns true iff the value is non-zero.
     */
    private boolean writeVdlEnum(EncodingStream out, Object value) throws IOException {
        expectClass(Kind.ENUM, value, VdlEnum.class);
        int ordinal = ((VdlEnum) value).ordinal();
        BinaryUtil.encodeUint(out, ordinal);
        return ordinal != 0;
    }

    /**
     * Writes a VDL float to output stream and returns true iff the value is non-zero.
     */
    private boolean writeVdlFloat(EncodingStream out, Object value) throws IOException {
        if (value instanceof VdlFloat32) {
            return BinaryUtil.encodeDouble(out, ((VdlFloat32) value).getValue());
        } else if (value instanceof VdlFloat64) {
            return BinaryUtil.encodeDouble(out, ((VdlFloat64) value).getValue());
        } else if (value instanceof Float) {
            return BinaryUtil.encodeDouble(out, (Float) value);
        } else if (value instanceof Double){
            return BinaryUtil.encodeDouble(out, (Double) value);
        } else {
            throw new IOException("Unsupported VDL float value (type " + value.getClass()
                    + ", value " + value + ")");
        }
    }

    /**
     * Writes a VDL int to output stream and returns true iff the value is non-zero.
     */
    private boolean writeVdlInt(EncodingStream out, Object value) throws IOException {
        if (value instanceof VdlInt8) {
            return BinaryUtil.encodeInt(out, ((VdlInt8) value).getValue());
        } else if (value instanceof VdlInt16) {
            return BinaryUtil.encodeInt(out, ((VdlInt16) value).getValue());
        } else if (value instanceof VdlInt32) {
            return BinaryUtil.encodeInt(out, ((VdlInt32) value).getValue());
        } else if (value instanceof VdlInt64) {
            return BinaryUtil.encodeInt(out, ((VdlInt64) value).getValue());
        } else if (value instanceof Short){
            return BinaryUtil.encodeInt(out, (Short) value);
        } else if (value instanceof Integer) {
            return BinaryUtil.encodeInt(out, (Integer) value);
        } else if (value instanceof Long) {
            return BinaryUtil.encodeInt(out, (Long) value);
        } else {
            throw new IOException("Unsupported VDL int value (type " + value.getClass()
                    + ", value " + value + ")");
        }
    }

    /**
     * Write a byte as a byte (as opposed to as a uint, as is done in version >= 0x81).
     * Returns true iff the value is non-zero.
     */
    private boolean writeVdlRawByte(EncodingStream out, Object value) {
        byte b;
        if (value instanceof VdlByte) {
            b = ((VdlByte)value).getValue();
        } else if (value instanceof Byte) {
            b = (Byte)value;
        } else {
            throw new RuntimeException("unknown raw byte value " + value);
        }
        out.write(b);
        return b != 0;
    }

    /**
     * Writes a VDL byte array to output stream and returns true iff the value is non-zero.
     */
    private boolean writeVdlBytes(EncodingStream out, Object value, VdlType type)
            throws IOException {
        if (value.getClass().isArray()) {
            Object arrayValue = value;
            int len = Array.getLength(arrayValue);
            BinaryUtil.encodeUint(out, len);
            boolean nonZero = false;
            for (int i = 0; i < len; i++) {
                nonZero = writeVdlRawByte(out, Array.getByte(arrayValue, i)) || nonZero;
            }
            return nonZero;
        }

        Collection<?> collection;
        int size;
        if (value instanceof VdlArray) {
            collection = (VdlArray<?>) value;
            size = 0;
        } else if (value instanceof VdlList) {
            VdlList<?> listValue = (VdlList<?>) value;
            collection = listValue;
            size = listValue.size();
        } else {
            throw new IOException("Unsupported VDL list value (type " + value.getClass()
                    + ", value " + value + ")");
        }

        BinaryUtil.encodeUint(out, size);
        boolean nonZero = false;
        for (Object elem : collection) {
            nonZero = writeVdlRawByte(out, elem) || nonZero;
        }
        return nonZero;
    }

    /**
     * Writes a VDL list to output stream and returns true iff the value is non-zero.
     */
    private boolean writeVdlList(EncodingStream out, Object value, VdlType type)
            throws IOException {
        if (value instanceof List) {
            List<?> listValue = (List<?>) value;
            BinaryUtil.encodeUint(out, listValue.size());
            for (Object elem : listValue) {
                writeValue(out, elem, type.getElem());
            }
            return listValue.size() != 0;
        } else if (value.getClass().isArray()) {
            Object arrayValue = value;
            int len = Array.getLength(arrayValue);
            BinaryUtil.encodeUint(out, len);
            for (int i = 0; i < len; i++) {
                writeValue(out, Array.get(arrayValue, i), type.getElem());
            }
            return len != 0;
        } else {
            throw new IOException("Unsupported VDL list value (type " + value.getClass()
                    + ", value " + value + ")");
        }
    }

    /**
     * Writes a VDL map to output stream and returns true iff the value is non-zero.
     */
    private boolean writeVdlMap(EncodingStream out, Object value, VdlType type) throws IOException {
        expectClass(Kind.MAP, value, Map.class);
        Map<?, ?> mapValue = (Map<?, ?>) value;
        BinaryUtil.encodeUint(out, mapValue.size());
        for (Map.Entry<?, ?> entry : mapValue.entrySet()) {
            writeValue(out, entry.getKey(), type.getKey());
            writeValue(out, entry.getValue(), type.getElem());
        }
        return mapValue.size() != 0;
    }

    /**
     * Writes a VDL union to output stream and returns true iff the value is non-zero.
     */
    private boolean writeVdlUnion(EncodingStream out, Object value) throws IOException {
        expectClass(Kind.UNION, value, VdlUnion.class);
        VdlUnion unionValue = (VdlUnion) value;
        Object elem = unionValue.getElem();
        int index = unionValue.getIndex();
        VdlType elemType = unionValue.vdlType().getFields().get(index).getType();
        BinaryUtil.encodeUint(out, index);
        boolean isNonZero = index != 0;
        isNonZero |= writeValue(out, elem, elemType);
        return isNonZero;
    }

    /**
     * Writes a VDL optional to output stream and returns true iff the value is non-zero.
     */
    private boolean writeVdlOptional(EncodingStream out, Object value) throws IOException {
        expectClass(Kind.OPTIONAL, value, VdlOptional.class);
        VdlOptional<?> optionalValue = (VdlOptional<?>) value;
        if (optionalValue.isNull()) {
            writeVdlControlByte(out, Constants.WIRE_CTRL_NIL);
            return false;
        } else {
            writeValue(out, optionalValue.getElem(), optionalValue.vdlType().getElem());
            return true;
        }
    }

    /**
     * Writes a VDL set to output stream and returns true iff the value is non-zero.
     */
    private boolean writeVdlSet(EncodingStream out, Object value, VdlType type) throws IOException {
        expectClass(Kind.SET, value, Set.class);
        Set<?> setValue = (Set<?>) value;
        BinaryUtil.encodeUint(out, setValue.size());
        for (Object key : setValue) {
            writeValue(out, key, type.getKey());
        }
        return setValue.size() != 0;
    }

    /**
     * Writes a VDL string to output stream and returns true iff the value is non-zero.
     */
    private boolean writeVdlString(EncodingStream out, Object value) throws IOException {
        String stringValue;
        if (value instanceof VdlString) {
            stringValue = ((VdlString) value).getValue();
        } else if (value instanceof String ){
            stringValue = (String) value;
        } else {
            throw new IOException("Unsupported VDL string value (type " + value.getClass()
                    + ", value " + value + ")");
        }
        BinaryUtil.encodeBytes(out, BinaryUtil.getBytes(stringValue));
        return stringValue.length() != 0;
    }

    /**
     * Writes a VDL struct to output stream and returns true iff the value is non-zero.
     */
    private boolean writeVdlStruct(EncodingStream out, Object value, VdlType type) throws IOException {
        List<VdlField> fields = type.getFields();
        boolean hasNonZeroField = false;
        for (int i = 0; i < fields.size(); i++) {
            VdlField field = fields.get(i);
            Object fieldValue = null;
            if (value instanceof VdlStruct) {
                fieldValue = ((VdlStruct) value).getField(field.getName());
            } else {
                try {
                    Field f = value.getClass().getDeclaredField(
                            BinaryUtil.firstCharToLower(field.getName()));
                    f.setAccessible(true);
                    fieldValue = f.get(value);
                } catch (Exception e) {
                    throw new IOException("Unsupported VDL struct value (type " + value.getClass()
                            + ", value " + value + ")", e);
                }
            }
            int prevCount = out.getCount();
            int prevTypeIdCount = typeIds.size();
            BinaryUtil.encodeUint(out, i);
            if (writeValue(out, fieldValue, field.getType())) {
                hasNonZeroField = true;
            } else {
                // Roll back writing of a zero value.
                out.setCount(prevCount);
                for (;typeIds.size() > prevTypeIdCount;) {
                    typeIds.remove(typeIds.size() - 1);
                }
            }
        }
        writeVdlControlByte(out, Constants.WIRE_CTRL_END);
        return hasNonZeroField;
    }

    /**
     * Writes a VDL uint to output stream and returns true iff the value is non-zero.
     */
    private boolean writeVdlUint(EncodingStream out, Object value) throws IOException {
        if (value instanceof VdlByte || value instanceof Byte) {
            return writeVdlByte(out, value);
        } else if (value instanceof VdlUint16) {
            return BinaryUtil.encodeUint(out, ((VdlUint16) value).getValue());
        } else if (value instanceof VdlUint32) {
            return BinaryUtil.encodeUint(out, ((VdlUint32) value).getValue());
        } else if (value instanceof VdlUint64) {
            return BinaryUtil.encodeUint(out, ((VdlUint64) value).getValue());
        } else {
            throw new IOException("Unsupported VDL uint value (type " + value.getClass()
                    + ", value " + value + ")");
        }
    }

    /**
     * Writes a VDL typeObject to output stream and returns true iff the value is non-zero.
     */
    private boolean writeVdlTypeObject(EncodingStream out, Object object) throws IOException {
        expectClass(Kind.TYPEOBJECT, object, VdlTypeObject.class);
        VdlTypeObject value = (VdlTypeObject) object;
        long id = getType(value.getTypeObject()).getValue();
        writeTypeId(out, id);
        return value.getTypeObject() != Types.ANY;
    }

    private void expectClass(Kind kind, Object value, Class<?> klass) throws IOException {
        if (!klass.isAssignableFrom(value.getClass())) {
            throw new IOException("Unsupported VDL " + kind + " value (type " + value.getClass()
                    + ", value " + value + ")");
        }
    }
}
