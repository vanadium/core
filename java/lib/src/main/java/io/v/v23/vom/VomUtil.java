// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vom;

import io.v.v23.vdl.VdlType;
import io.v.v23.vdl.VdlValue;
import io.v.v23.verror.VException;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.lang.reflect.Type;

/**
 * VomUtil contains convenience methods for VOM-encoding/decoding.
 */
public class VomUtil {
    /**
     * VOM-encodes the provided value using a new VOM-encoder.
     *
     * @param  value           value to be encoded
     * @param  type            type of the provided value
     * @return                 VOM-encoded value as a byte array
     * @throws VException      if there was an error encoding the value
     */
    public static byte[] encode(Object value, Type type) throws VException {
        ByteArrayOutputStream out = new ByteArrayOutputStream();
        BinaryEncoder encoder = new BinaryEncoder(out);
        try {
            encoder.encodeValue(type, value);
        } catch (IOException e) {
            VException ve = new VException(e.getMessage());
            ve.initCause(e);
            throw ve;
        }
        return out.toByteArray();
    }

    /**
     * VOM-encodes the provided value using a new VOM-encoder.
     *
     * @param  value           value to be encoded
     * @param  type            type of the provided value
     * @return                 VOM-encoded value as a byte array
     * @throws VException      if there was an error encoding the value
     */
    public static byte[] encode(Object value, VdlType type) throws VException {
        ByteArrayOutputStream out = new ByteArrayOutputStream();
        BinaryEncoder encoder = new BinaryEncoder(out);
        try {
            encoder.encodeValue(type, value);
        } catch (IOException e) {
            VException ve = new VException(e.getMessage());
            ve.initCause(e);
            throw ve;
        }
        return out.toByteArray();
    }

    /**
     * VOM-encodes the provided value using a new VOM-encoder, returning a hex-encoded string.
     *
     * @param  value           value to be encoded
     * @param  type            type of the provided value
     * @return                 VOM-encoded value in a hex-string format
     * @throws VException      if there was an error encoding the value
     */
    public static String encodeToString(Object value, Type type) throws VException {
        byte[] data = encode(value, type);
        return bytesToHexString(data);
    }

    /**
     * VOM-encodes the provided VDL value using a new VOM-encoder.
     *
     * @param  value           VDL value to be encoded
     * @return                 VOM-encoded value as a byte array
     * @throws VException      if there was an error encoding the value
     */
    public static byte[] encode(VdlValue value) throws VException {
        ByteArrayOutputStream out = new ByteArrayOutputStream();
        BinaryEncoder encoder = new BinaryEncoder(out);
        try {
            encoder.encodeValue(value);
        } catch (IOException e) {
            VException ve = new VException(e.getMessage());
            ve.initCause(e);
            throw ve;
        }
        return out.toByteArray();
    }

    /**
     * VOM-decodes the provided data using a new VOM-decoder.
     *
     * @param  data            VOM-encoded data
     * @param  type            type of the object that the data should be decoded into
     * @return                 VOM-decoded object
     * @throws VException      if there was an error decoding the data
     */
    public static Object decode(byte[] data, Type type) throws VException {
        BinaryDecoder decoder = new BinaryDecoder(new ByteArrayInputStream(data));
        try {
            return decoder.decodeValue(type);
        } catch (IOException e) {
            VException ve = new VException(e.getMessage());
            ve.initCause(e);
            throw ve;
        } catch (ConversionException e) {
            VException ve = new VException(e.getMessage());
            ve.initCause(e);
            throw ve;
        }
    }

    /**
     * VOM-decodes the provided data using a new VOM-decoder.  A best effort is made to deduce the
     * type of the encoded data.
     *
     * @param  data            VOM-encoded data
     * @return                 VOM-decoded object
     * @throws VException      if there was an error decoding the data
     */
    public static Object decode(byte[] data) throws VException {
        BinaryDecoder decoder = new BinaryDecoder(new ByteArrayInputStream(data));
        try {
            return decoder.decodeValue();
        } catch (IOException e) {
            VException ve = new VException(e.getMessage());
            ve.initCause(e);
            throw ve;
        } catch (ConversionException e) {
            VException ve = new VException(e.getMessage());
            ve.initCause(e);
            throw ve;
        }
    }

    /**
     * VOM-decodes the provided data (stored as a hex string) using a new VOM-decoder.
     *
     * @param  hex             VOM-encoded data, stored as a hex string
     * @param  type            type of the object that the data should be decoded into
     * @return                 VOM-decoded object
     * @throws VException      if there was an error decoding the data
     */
    public static Object decodeFromString(String hex, Type type) throws VException {
        byte[] data = hexStringToBytes(hex);
        return decode(data, type);
    }

    /**
     * Returns a {@link VdlValue} corresponding to the given Java object of the provided type.
     *
     * @param obj            a java object
     * @param type           type of the provided object
     * @return               {@link VdlValue} corresponding to the given Java object
     * @throws VException    if the {@link VdlValue} couldn't be computed.
     */
    public static VdlValue valueOf(Object obj, Type type) throws VException {
        byte[] data = VomUtil.encode(obj, type);
        return (VdlValue) VomUtil.decode(data, VdlValue.class);
    }

    /**
     * Returns a {@link VdlValue} corresponding to the given Java object of the provided type.
     *
     * @param obj            a java object
     * @param type           type of the provided object
     * @return               {@link VdlValue} corresponding to the given Java object
     * @throws VException    if the {@link VdlValue} couldn't be computed.
     */
    public static VdlValue valueOf(Object obj, VdlType type) throws VException {
        byte[] data = VomUtil.encode(obj, type);
        return (VdlValue) VomUtil.decode(data, VdlValue.class);
    }

    /**
     * Encodes the provided byte array of data as a hex string.
     *
     * @param  data            arbitrary byte array
     * @return                 hex string representation of the byte array
     */
    public static String bytesToHexString(byte[] data) {
        StringBuilder builder = new StringBuilder();
        for (byte b : data) {
            builder.append(String.format("%02x", b));
        }
        return builder.toString();
    }

    /**
     * Decodes the provided data (stored as a hex string) into a byte array.
     *
     * @param  hex             hex string encoding of a byte array 
     *                         (encoded using {@link #bytesToHexString})
     * @return                 decoded byte array
     * @throws VException      if there was an error decoding the hex string
     */
    public static byte[] hexStringToBytes(String hex) throws VException {
        if (hex.length() % 2 != 0) {
            throw new VException("Hex strings must be multiples of 2 in length");
        }
        int outLen = hex.length() / 2;
        byte[] dat = new byte[outLen];
        for (int i = 0; i < outLen; i++) {
            dat[i] = (byte) Integer.parseInt(hex.substring(2 * i, 2 * i + 2), 16);
        }
        return dat;
    }
}
