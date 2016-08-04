// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vom;

import io.v.v23.vdl.Kind;
import io.v.v23.vdl.VdlField;
import io.v.v23.vdl.VdlType;

import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.nio.charset.Charset;
import java.util.HashSet;

/**
 * Binary encoding and decoding routines.
 */
public final class BinaryUtil {
    static final Charset UTF8_CHARSET = Charset.forName("UTF-8");
    private static final String END_OF_STREAM_MESSAGE = "End of stream prematurely reached.";

    /**
     * Unsigned integers are the basis for all other primitive values. This is a two-state encoding.
     * If the number is less than 128 (0 through 0x7f), its value is written directly. Otherwise the
     * value is written in big-endian byte order preceded by the negated byte length.
     * Returns true iff the value is non-zero.
     */
    public static boolean encodeUint(OutputStream out, final long value) throws IOException {
        if ((value & 0x7f) == value) {
            out.write((byte) value);
            return value != 0;
        }
        int len = 0;
        while (((value >>> (len * 8)) | 0xff) != 0xff) {
            len++;
        }
        len++;
        out.write(-len);
        while (len > 0) {
            len--;
            out.write((byte) (value >>> (len * 8)));
        }
        return true;
    }

    public static boolean encodeUint(OutputStream out, final int value) throws IOException {
        return encodeUint(out, value & 0xffffffffL);
    }

    public static boolean encodeUint(OutputStream out, final short value) throws IOException {
        return encodeUint(out, value & 0xffffL);
    }

    public static long decodeUint(InputStream in) throws IOException {
        int firstByte = in.read();
        if (firstByte == -1) {
            // EOF.
            throw new CorruptVomStreamException(END_OF_STREAM_MESSAGE);
        }
        if ((firstByte & 0x7f) == firstByte) {
            return firstByte;
        }
        int len = -(byte) firstByte;
        if (len > 8) {
            throw new CorruptVomStreamException("Invalid long byte length " + len);
        }
        long value = 0;
        while (len > 0) {
            len--;
            int nextByte = in.read();
            if (nextByte == -1) {
                throw new CorruptVomStreamException(END_OF_STREAM_MESSAGE);
            }
            value = (value << 8) | nextByte;
        }
        return value;
    }

    /**
     * Signed integers are encoded as unsigned integers, where the low bit says whether to
     * complement the other bits to recover the int.
     * Returns true iff the value is non-zero.
     */
    public static boolean encodeInt(OutputStream out, final long value) throws IOException {
        if (value < 0) {
            return encodeUint(out, ((~value) << 1) | 1);
        } else {
            return encodeUint(out, value << 1);
        }
    }

    public static long decodeInt(InputStream in) throws IOException {
        long uint = decodeUint(in);
        if ((uint & 1) == 1) {
            return ~(uint >>> 1);
        } else {
            return uint >>> 1;
        }
    }

    /**
     * Floating point numbers are encoded as byte-reversed ieee754.
     * Returns true iff the value is non-zero;
     */
    public static boolean encodeDouble(OutputStream out, final double value) throws IOException {
        return encodeUint(out, Long.reverseBytes(Double.doubleToLongBits(value)));
    }

    public static double decodeDouble(InputStream in) throws IOException {
        return Double.longBitsToDouble(Long.reverseBytes(decodeUint(in)));
    }

    public static boolean decodeBoolean(InputStream in) throws IOException {
        int nextByte = in.read();
        if (nextByte == -1) {
            throw new CorruptVomStreamException(END_OF_STREAM_MESSAGE);
        }
        return nextByte != 0;
    }

    /**
     * Encodes an array of bytes as byte count followed by byte values.
     */
    public static void encodeBytes(OutputStream out, byte[] data) throws IOException {
        encodeUint(out, data.length);
        out.write(data);
    }

    public static byte[] decodeBytes(InputStream in, int len) throws IOException {
        if (len == 0) {
            return new byte[0];
        }
        byte[] bytes = new byte[len];
        if (in.read(bytes) != bytes.length) {
            throw new CorruptVomStreamException(END_OF_STREAM_MESSAGE);
        }
        return bytes;
    }

    /**
     * Returns true iff the kind of type is []byte or [N]byte.
     */
    public static boolean isBytes(VdlType type) {
        return (type.getKind() == Kind.ARRAY || type.getKind() == Kind.LIST)
                && type.getElem().getKind() == Kind.BYTE;
    }

    /**
     * Returns true iff the type is encoded with a top-level message length.
     */
    public static boolean hasBinaryMsgLen(VdlType type) {
        if (isBytes(type)) {
            return false;
        }

        switch (type.getKind()) {
            case ANY:
            case ARRAY:
            case LIST:
            case MAP:
            case OPTIONAL:
            case SET:
            case STRUCT:
            case UNION:
                return true;
            default:
                return false;
        }
    }

    static boolean hasTypeObject(VdlType t) {
        return hasTypeObjectInternal(t, new HashSet<VdlType>());
    }

    private static boolean hasTypeObjectInternal(VdlType t, HashSet<VdlType> seen) {
        if (seen.contains(t)) {
            return false;
        }
        seen.add(t);

        switch (t.getKind()) {
            case TYPEOBJECT:
                return true;
            case OPTIONAL:
            case LIST:
            case ARRAY:
                return hasTypeObjectInternal(t.getElem(), seen);
            case SET:
                return hasTypeObjectInternal(t.getKey(), seen);
            case MAP:
                return hasTypeObjectInternal(t.getKey(), seen) ||
                        hasTypeObjectInternal(t.getElem(), seen);
            case STRUCT:
            case UNION:
                for (VdlField fld : t.getFields()) {
                    if (hasTypeObjectInternal(fld.getType(), seen)) {
                        return true;
                    }
                }
                return false;
            default:
                return false;
        }
    }

    static boolean hasAny(VdlType t) {
        return hasAnyInternal(t, new HashSet<VdlType>());
    }

    private static boolean hasAnyInternal(VdlType t, HashSet<VdlType> seen) {
        if (seen.contains(t)) {
            return false;
        }
        seen.add(t);

        switch (t.getKind()) {
            case ANY:
                return true;
            case OPTIONAL:
            case LIST:
            case ARRAY:
                return hasAnyInternal(t.getElem(), seen);
            case SET:
                return hasAnyInternal(t.getKey(), seen);
            case MAP:
                return hasAnyInternal(t.getKey(), seen) ||
                        hasAnyInternal(t.getElem(), seen);
            case STRUCT:
            case UNION:
                for (VdlField fld : t.getFields()) {
                    if (hasAnyInternal(fld.getType(), seen)) {
                        return true;
                    }
                }
                return false;
            default:
                return false;
        }
    }

    /**
     * Converts a string to byte array, null string is treated as empty string.
     */
    public static byte[] getBytes(String value) {
        if (value == null) {
            value = "";
        }
        return value.getBytes(UTF8_CHARSET);
    }

    /**
     * Upper-cases the first character of a given string.
     */
    public static String firstCharToUpper(String str) {
        return Character.toUpperCase(str.charAt(0)) + str.substring(1);
    }

    /**
     * Lower-cases the first character in a string.
     */
    public static String firstCharToLower(String str) {
        return Character.toLowerCase(str.charAt(0)) + str.substring(1);
    }
}
