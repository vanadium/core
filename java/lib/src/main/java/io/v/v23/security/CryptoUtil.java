// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import io.v.v23.verror.VException;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.math.BigInteger;
import java.security.InvalidAlgorithmParameterException;
import java.security.KeyFactory;
import java.security.KeyPairGenerator;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.security.interfaces.ECPublicKey;
import java.security.spec.ECGenParameterSpec;
import java.security.spec.ECParameterSpec;
import java.security.spec.ECPoint;
import java.security.spec.EllipticCurve;
import java.security.spec.InvalidKeySpecException;
import java.security.spec.X509EncodedKeySpec;
import java.util.Arrays;

/**
 * Various cryptographic utilities.
 */
public class CryptoUtil {
    private static final String PK_ALGORITHM = "EC";

    @SuppressWarnings("unused")
    private static final ECParameterSpec EC_P256_PARAMS = getParameterSpec("secp256r1"); // NIST-256
    @SuppressWarnings("unused")
    private static final ECParameterSpec EC_P384_PARAMS = getParameterSpec("secp384r1"); // NIST-384
    @SuppressWarnings("unused")
    private static final ECParameterSpec EC_P521_PARAMS = getParameterSpec("secp521r1"); // NIST-521

    private static ECParameterSpec getParameterSpec(String algorithm) {
        try {
            KeyPairGenerator gen = KeyPairGenerator.getInstance("EC");
            ECGenParameterSpec spec = new ECGenParameterSpec(algorithm);
            gen.initialize(spec);
            return ((ECPublicKey)gen.generateKeyPair().getPublic()).getParams();
        } catch (NoSuchAlgorithmException e) {
            throw new RuntimeException("EC crypto not supported!", e);
        } catch (InvalidAlgorithmParameterException e) {
            throw new RuntimeException("EC algorithm " + algorithm + " not supported!", e);
        }
    }

    /**
     * Decodes the provided DER-encoded ECDSA public key.
     *
     * @param  encodedKey      DER-encoded ECDSA public key
     * @return                 ECDSA public key
     * @throws VException      if the public key could not be decoded
     */
    public static ECPublicKey decodeECPublicKey(byte[] encodedKey) throws VException {
        try {
            X509EncodedKeySpec spec = new X509EncodedKeySpec(encodedKey);
            KeyFactory factory = KeyFactory.getInstance(PK_ALGORITHM);
            return (ECPublicKey)factory.generatePublic(spec);
        } catch (NoSuchAlgorithmException e) {
            throw new VException(
                "Java runtime doesn't support " + PK_ALGORITHM + " algorithm: " + e.getMessage());
        } catch (InvalidKeySpecException e) {
            throw new VException("Encoded key is incompatible with " + PK_ALGORITHM +
                " algorithm: " + e.getMessage());
        }
    }

    /**
     * Encodes the EC point into the uncompressed ANSI X9.62 format.
     *
     * @param curve            EC curve
     * @param point            EC point
     * @return                 ANSI X9.62-encoded EC point
     * @throws VException      if the curve and the point are incompatible
     */
    public static byte[] encodeECPoint(EllipticCurve curve, ECPoint point) throws VException {
        int byteLen = (curve.getField().getFieldSize() + 7)  >> 3;
        byte[] x = point.getAffineX().toByteArray();
        byte[] y = point.getAffineY().toByteArray();
        if (x.length != byteLen) {
            throw new VException(String.format(
                    "Illegal length for X axis of EC point, want %d have %d", byteLen, x.length));
        }
        if (y.length != byteLen) {
            throw new VException(String.format(
                    "Illegal length for Y axis of EC point, want %d have %d", byteLen, y.length));
        }
        byte[] xy = new byte[1 + 2 * byteLen];
        xy[0] = 4;
        System.arraycopy(x, 0, xy, 1, byteLen);
        System.arraycopy(y, 0, xy, 1 + byteLen, byteLen);
        return xy;
    }

    /**
     * Decodes ANSI X9.62-encoded (uncompressed) EC point.
     *
     * @param  xy              ANSI X9.62-encoded EC point
     * @return                 EC point
     * @throws VException      if the EC point couldn't be decoded
     */
    public static ECPoint decodeECPoint(EllipticCurve curve, byte[] xy) throws VException {
        int byteLen = (curve.getField().getFieldSize() + 7)  >> 3;
        if (xy.length != (1 + 2 * byteLen)) {
            throw new VException(String.format(
                    "Data length mismatch: want %d have %d", (1 + 2 * byteLen), xy.length));
        }
        if (xy[0] != 4) { // compressed form
            throw new VException("Compressed curve formats not supported");
        }
        BigInteger x = new BigInteger(Arrays.copyOfRange(xy, 1, 1 + byteLen));
        BigInteger y = new BigInteger(Arrays.copyOfRange(xy, 1 + byteLen, xy.length));
        return new ECPoint(x, y);
    }

    /**
     * Applies the specified cryptographic hash function on the provided message, using the
     * provided hashing algorithm.
     *
     * @param  vHashAlgorithm  name of the Vanadium hash algorithm to use
     * @param  message         message to apply the hash function on
     * @return                 hashed message
     * @throws VException      if the message couldn't be hashed
     */
    public static byte[] hash(String vHashAlgorithm, byte[] message) throws VException {
        try {
            String hashAlgorithm = javaHashAlgorithm(vHashAlgorithm);
            MessageDigest md = MessageDigest.getInstance(hashAlgorithm);
            md.update(message);
            byte[] ret = md.digest();
            if (ret == null || ret.length == 0) {
                throw new VException("Got empty message after a hash using " + hashAlgorithm);
            }
            return ret;
        } catch (NoSuchAlgorithmException e) {
            throw new VException("Hashing algorithm " + vHashAlgorithm + " not " +
                "supported by the runtime: " + e.getMessage());
        }
    }

    /**
     * Returns a digest of the provided (message, purpose) pair which is meant
     * to be signed by the private counterpart of key.
     *
     * @param  vHashAlgorithm  name of the Vanadium hash algorithm to use
     * @param  message         message that is part of the digest
     * @param  purpose         purpose that is part of the digest
     * @param  key             public key of the signer of the returned digest
     * @return                 digest for the specified message and digest
     * @throws VException      if there was an error creating a digest
     */
    static byte[] messageDigest(String vHashAlgorithm, byte[] message, byte[] purpose, ECPublicKey key)
            throws VException {
        if (message == null) {
            throw new VException("Empty message.");
        }
        if (purpose == null) {
            throw new VException("Empty purpose.");
        }
        if (key == null) {
            throw new VException("PublicKey of signer not provided.");
        }
        // TODO(ashankar): Remove this if once https://github.com/vanadium/issues/issues/619 is resolved.
        String purposeStr = purpose.toString();
        byte[] keyBytes = hash(vHashAlgorithm, key.getEncoded());
        message = hash(vHashAlgorithm, message);
        purpose = hash(vHashAlgorithm, purpose);
        return join(join(keyBytes, message), purpose);
    }

    private static byte[] join(byte[] a, byte[] b) {
        if (a == null || a.length == 0) return b;
        if (b == null || b.length == 0) return a;
        byte[] c = new byte[a.length + b.length];
        System.arraycopy(a, 0, c, 0, a.length);
        System.arraycopy(b, 0, c, a.length, b.length);
        return c;
    }

    /**
     * Converts the provided Vanadium signature into the ASN.1 format (used by Java).
     *
     * @param  sig             signature in Vanadium format
     * @return                 signature in ASN.1 format
     * @throws VException      if the signature couldn't be converted
     */
    public static byte[] javaSignature(VSignature sig) throws VException {
        // The ASN.1 format of the signature should be:
        //    Signature ::= SEQUENCE {
        //       r   INTEGER,
        //       s   INTEGER
        //    }
        // When encoded, this translates into the following byte sequence:
        //    0x30 len 0x02 rlen [r bytes] 0x02 slen [s bytes].
        //
        // Note that we could have used BouncyCastle or an ASN1-decoding package to decode
        // the byte sequence, but the encoding is simple enough that we can do it by hand.
        byte[] r = sig.getR();
        byte[] s = sig.getS();
        if (r == null || r.length == 0) {
            throw new VException("Empty R component of signature.");
        }
        if (s == null || s.length == 0) {
            throw new VException("Empty S component of signature.");
        }
        ByteArrayOutputStream out = new ByteArrayOutputStream();
        out.write(0x30);
        out.write(4 + r.length + s.length);
        out.write(0x02);
        out.write(r.length);
        out.write(r, 0, r.length);
        out.write(0x02);
        out.write(s.length);
        out.write(s, 0, s.length);
        return out.toByteArray();
    }

    /**
     * Converts the provided Java signature (ASN.1 format) into the Vanadium format.
     *
     * @param  vHashAlgorithm  Vanadium hash algorithm used when generating the signature
     * @param  purpose         purpose of the generated signature
     * @param  sig             signature in ASN.1 format
     * @return                 signature in Vanadium format
     * @throws VException      if the signature couldn't be converted
     */
    public static VSignature vanadiumSignature(String vHashAlgorithm, byte[] purpose, byte[] sig)
            throws VException {
        byte[] r, s;
        // The ASN.1 format of the signature should be:
        //    Signature ::= SEQUENCE {
        //       r   INTEGER,
        //       s   INTEGER
        //    }
        // When encoded, this translates into the following byte sequence:
        //    0x30 len 0x02 rlen [r bytes] 0x02 slen [s bytes].
        //
        // Note that we could have used BouncyCastle or an ASN1-decoding package to decode
        // the byte sequence, but the encoding is simple enough that we can do it by hand.
        ByteArrayInputStream in = new ByteArrayInputStream(sig);
        int b;
        if ((b = in.read()) != 0x30) {
            throw new VException(String.format("Invalid signature type, want SEQUENCE (0x30), got 0x%02X", b));
        }
        if ((b = in.read()) != in.available()) {
            throw new VException(String.format("Invalid signature length, want %d, got %d", in.available(), b));
        }
        if ((b = in.read()) != 0x02) {
            throw new VException(String.format("Invalid type for R, want INTEGER (0x02), got 0x%02X", b));
        }
        if ((b = in.read()) > in.available()) {
            throw new VException(String.format("Invalid length for R, want less than %d, got %d", in.available(), b));
        }
        r = new byte[b];
        if (in.read(r, 0, b) != b) {
            throw new VException(String.format("Error reading %d bytes of R from signature", b));
        }
        if ((b = in.read()) != 0x02) {
            throw new VException(String.format("Invalid type for S, want INTEGER (0x02), got 0x%02X", b));
        }
        if ((b = in.read()) > in.available()) {
            throw new VException(String.format("Invalid length for S, want less than %d, got %d", in.available(), b));
        }
        s = new byte[b];
        if (in.read(s, 0, b) != b) {
            throw new VException(String.format("Error reading %d bytes of S from signature", b));
        }
        return new VSignature(purpose, new Hash(vHashAlgorithm), r, s);
    }

    /**
     * Returns a Java hash algorithm corresponding to the given Vanadium hash algorithm.
     *
     * @param  vHashAlgorithm Vanadium hash algorithm
     * @throws VException     if the hash algorithm isn't supported
     */
    static String javaHashAlgorithm(String vHashAlgorithm) throws VException {
        if (vHashAlgorithm.equals("SHA1")) {
            return "SHA-1";
        }
        if (vHashAlgorithm.equals("SHA256")) {
            return "SHA-256";
        }
        if (vHashAlgorithm.equals("SHA384")) {
            return "SHA-384";
        }
        if (vHashAlgorithm.equals("SHA512")) {
            return "SHA-512";
        }
        throw new VException("Java currently doesn't support hashing algorithm: " + vHashAlgorithm);
    }

    /**
     * Returns a Java ECDSA signing algorithm corrsponding to the given Vanadium hash algorithm.
     *
     * @param  hashAlgorithm Vanadium hash algorithm
     * @throws VException    if the hash/signing algorithm isn't supported
     */
    static String javaSigningAlgorithm(String vHashAlgorithm) throws VException {
        if (vHashAlgorithm.equals("SHA1")) {
            return "SHA1withECDSA";
        }
        if (vHashAlgorithm.equals("SHA256")) {
            return "SHA256withECDSA";
        }
        if (vHashAlgorithm.equals("SHA384")) {
            return "SHA384withECDSA";
        }
        if (vHashAlgorithm.equals("SHA512")) {
            return "SHA512withECDSA";
        }
        throw new VException("Java Vanadium currently doesn't support hashing algorithm: " +
                vHashAlgorithm);
    }

    private CryptoUtil() {}
}
