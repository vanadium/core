// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vom;

import io.v.v23.vdl.Kind;
import io.v.v23.vdl.VdlArray;
import io.v.v23.vdl.VdlByte;
import io.v.v23.vdl.VdlEnum;
import io.v.v23.vdl.VdlString;
import io.v.v23.vdl.VdlValue;

import java.lang.reflect.Array;
import java.lang.reflect.Type;
import java.util.ArrayList;
import java.util.List;

/**
 * ConvertUtil provides helpers to convert VDL values.
 */
final class ConvertUtil {

    // IEEE 754 represents float64 using 52 bits to represent the mantissa, with
    // an extra implied leading bit. That gives us 53 bits to store integers
    // without overflow - i.e. [0, (2^53)-1]. And since 2^53 is a small power of
    // two, it can also be stored without loss via mantissa=1 exponent=53. Thus
    // we have our max and min values. Ditto for float32, which uses 23 bits
    // with
    // an extra implied leading bit.
    private static long DOUBLE_MAX_LOSSLESS_INTEGER = (1L << 53);
    private static long DOUBLE_MIN_LOSSLESS_INTEGER = -(1L << 53);
    private static long FLOAT_MAX_LOSSLESS_INTEGER = (1L << 24);
    private static long FLOAT_MIN_LOSSLESS_INTEGER = -(1L << 24);

    static boolean hasOverflowUint(long x, long bitlen) {
        long shift = 64 - bitlen;
        return x != (x << shift) >>> shift;
    }

    static boolean hasOverflowInt(long x, long bitlen) {
        long shift = 64 - bitlen;
        return x != (x << shift) >> shift;
    }

    static boolean canConvertUintToInt(long x, long bitlen) {
        return x >= 0 && !hasOverflowInt(x, bitlen);
    }

    static boolean canConvertIntToUint(long x, long bitlen) {
        return x >= 0 && !hasOverflowUint(x, bitlen);
    }

    static boolean canConvertUintToFloat(long x, long bitlen) {
        if (x < 0)
            return false; // These values are too large for either type.
        switch ((int) bitlen) {
            case 32:
                return x <= FLOAT_MAX_LOSSLESS_INTEGER;
            default:
                return x <= DOUBLE_MAX_LOSSLESS_INTEGER;
        }
    }

    static boolean canConvertIntToFloat(long x, long bitlen) {
        switch ((int) bitlen) {
            case 32:
                return FLOAT_MIN_LOSSLESS_INTEGER <= x
                        && x <= FLOAT_MAX_LOSSLESS_INTEGER;
            default:
                return DOUBLE_MIN_LOSSLESS_INTEGER <= x
                        && x <= DOUBLE_MAX_LOSSLESS_INTEGER;
        }
    }

    static long explicitConvertFloatToUint(double x) {
        if (x < DOUBLE_MAX_LOSSLESS_INTEGER * 4) {
            return (long) x;
        } else {
            return ((long) (x / 2)) << 1;
        }
    }

    static boolean canConvertFloatToUint(double x, long bitlen) {
        if (x < 0) {
            return false;
        }
        if (x < DOUBLE_MAX_LOSSLESS_INTEGER * 4) {
            return canConvertFloatToInt(x, bitlen);
        } else {
            return canConvertFloatToInt(x / 2, bitlen);
        }
    }

    static boolean canConvertFloatToInt(double x, long bitlen) {
        long intPart = (long) x;
        double fracPart = x - intPart;
        return fracPart == 0 && x >= Long.MIN_VALUE
                && x <= Long.MAX_VALUE
                && !hasOverflowInt(intPart, bitlen);
    }

    /**
     * Converts uint values to uint, int or float values only.
     * This is used to check O(n) conversion rules instead of O(n^2) for n numeric types.
     */
    private static Object convertUint(long value, ConversionTarget target)
            throws ConversionException {
        switch (target.getKind()) {
            case BYTE:
                if (!hasOverflowUint(value, 8)) {
                    return ReflectUtil.createPrimitive(target, (byte) value, Byte.TYPE);
                }
                break;
            case UINT16:
                if (!hasOverflowUint(value, 16)) {
                    return ReflectUtil.createPrimitive(target, (short) value, Short.TYPE);
                }
                break;
            case UINT32:
                if (!hasOverflowUint(value, 32)) {
                    return ReflectUtil.createPrimitive(target, (int) value, Integer.TYPE);
                }
                break;
            case UINT64:
                return ReflectUtil.createPrimitive(target, value, Long.TYPE);
            default:
                if (ConvertUtil.canConvertUintToInt(value, 64)) {
                    return convertInt(value, target);
                }
        }
        throw new ConversionException("Can't convert " + value + " to " + target.getTargetType());
    }

    /**
     * Converts int values to int or float values only.
     * This is used to check O(n) conversion rules instead of O(n^2) for n numeric types.
     */
    private static Object convertInt(long value, ConversionTarget target)
            throws ConversionException {
        switch (target.getKind()) {
            case INT8:
                if (!hasOverflowInt(value, 8)) {
                    return ReflectUtil.createPrimitive(target, (byte) value, Byte.TYPE);
                }
                break;
            case INT16:
                if (!hasOverflowInt(value, 16)) {
                    return ReflectUtil.createPrimitive(target, (short) value, Short.TYPE);
                }
                break;
            case INT32:
                if (!hasOverflowInt(value, 32)) {
                    return ReflectUtil.createPrimitive(target, (int) value, Integer.TYPE);
                }
                break;
            case INT64:
                return ReflectUtil.createPrimitive(target, value, Long.TYPE);
            case FLOAT32:
                if (ConvertUtil.canConvertIntToFloat(value, 32)) {
                    return convertDouble(value, target);
                }
                break;
            default:
                if (ConvertUtil.canConvertIntToFloat(value, 64)) {
                    return convertDouble(value, target);
                }
        }
        throw new ConversionException("Can't convert " + value + " to " + target.getTargetType());
    }

    /**
     * Converts float values to float values only.
     * This is used to check O(n) conversion rules instead of O(n^2) for n numeric types.
     */
    private static Object convertDouble(double value, ConversionTarget target)
            throws ConversionException {
        switch (target.getKind()) {
            case FLOAT32:
                return ReflectUtil.createPrimitive(target, (float) value, Float.TYPE);
            case FLOAT64:
                return ReflectUtil.createPrimitive(target, value, Double.TYPE);
        }
        throw new ConversionException("Can't convert " + value + " to " + target.getTargetType());
    }

    /**
     * Converts from uint to number, one of uint, int or float.
     */
    static Object convertFromUint(long value, ConversionTarget target) throws ConversionException {
        return convertUint(value, target);
    }

    /**
     * Converts from byte to number, one of uint, int or float.
     */
    static Object convertFromByte(byte value, ConversionTarget target) throws ConversionException {
        return convertUint(value & 0xffL, target);
    }

    /**
     * Converts from int to number, one of uint, int or float.
     */
    static Object convertFromInt(long value, ConversionTarget target) throws ConversionException {
        if (canConvertIntToUint(value, 64)) {
            return convertFromUint(value, target);
        } else {
            return convertInt(value, target);
        }
    }

    /**
     * Converts from float to number, one of uint, int or float.
     */
    static Object convertFromDouble(double value, ConversionTarget target)
            throws ConversionException {
        switch (target.getKind()) {
            case UINT64:
                if (canConvertFloatToUint(value, 64)) {
                    return convertFromUint(explicitConvertFloatToUint(value), target);
                }
                break;
            case FLOAT32:
            case FLOAT64:
                break;
            default:
                if (canConvertFloatToInt(value, 64)) {
                    return convertFromInt((long) value, target);
                }
        }
        return convertDouble(value, target);
    }

    /**
     * Converts from []byte to []number, [N]number, string or enum.
     */
    static Object convertFromBytes(byte[] bytes, ConversionTarget target)
            throws ConversionException {
        Class<?> targetClass = target.getTargetClass();
        if (targetClass.equals(byte[].class)) {
            return bytes;
        }
        if (targetClass == String.class || VdlString.class.isAssignableFrom(targetClass)) {
            return ReflectUtil.createPrimitive(target, new String(bytes), String.class);
        }
        if (VdlEnum.class.isAssignableFrom(targetClass)) {
            return ReflectUtil.createEnum(target, new String(bytes));
        }
        int len = bytes.length;
        if (target.getKind() == Kind.ARRAY) {
            if (bytes.length > target.getVdlType().getLength()) {
                throw new ConversionException(bytes, target.getTargetType(),
                        "target array is too short");
            }
            len = target.getVdlType().getLength();
        }

        Type elemType = ReflectUtil.getElementType(target.getTargetType(), 0);
        if (elemType == VdlValue.class) {
            elemType = VdlByte.class;
        }
        ConversionTarget element = new ConversionTarget(elemType);
        if (targetClass.isArray() || VdlArray.class.isAssignableFrom(targetClass)) {
            Object data = Array.newInstance(element.getTargetClass(), len);
            for (int i = 0; i < bytes.length; i++) {
                ReflectUtil.setArrayValue(data, i, convertFromByte(bytes[i], element),
                        element.getTargetClass());
            }
            return ReflectUtil.createGeneric(target, data);
        } else {
            List<Object> list = new ArrayList<Object>();
            for (int i = 0; i < bytes.length; i++) {
                list.add(convertFromByte(bytes[i], element));
            }
            return ReflectUtil.createGeneric(target, list);
        }
    }
}
