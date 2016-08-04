// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import io.v.v23.verror.VException;
import io.v.v23.vom.VomUtil;

import java.security.interfaces.ECPublicKey;

class Util {
    // VOM-encodes the provided VSignature.
    static byte[] encodeSignature(VSignature signature) throws VException {
        return VomUtil.encode(signature, VSignature.class);
    }

    // VOM-decodes the VOM-encoded VSignature.
    static VSignature decodeSignature(byte[] encoded) throws VException {
        if (encoded == null || encoded.length == 0) {
            return null;
        }
        return (VSignature) VomUtil.decode(encoded, VSignature.class);
    }

    // DER-encodes the provided ECPublicKey.
    static byte[] encodePublicKey(ECPublicKey key) {
        if (key == null) {
            return new byte[0];
        }
        return key.getEncoded();
    }

    // Decodes the DER-encoded ECPublicKey.
    static ECPublicKey decodePublicKey(byte[] encoded) throws VException {
        if (encoded == null || encoded.length == 0) {
            return null;
        }
        return CryptoUtil.decodeECPublicKey(encoded);
    }
}
