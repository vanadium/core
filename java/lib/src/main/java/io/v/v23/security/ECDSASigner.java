// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import io.v.v23.verror.VException;

import java.security.InvalidKeyException;
import java.security.NoSuchAlgorithmException;
import java.security.PrivateKey;
import java.security.SignatureException;
import java.security.interfaces.ECPublicKey;
import java.util.Arrays;

class ECDSASigner implements VSigner {
    private static final String VANADIUM_HASH_ALGORITHM = "SHA256";

    private final PrivateKey privKey;
    private final ECPublicKey pubKey;

    ECDSASigner(PrivateKey privKey, ECPublicKey pubKey) {
        this.privKey = privKey;
        this.pubKey = pubKey;
    }

    @Override
    public VSignature sign(byte[] purpose, byte[] message) throws VException {
        String javaSignAlgorithm = CryptoUtil.javaSigningAlgorithm(VANADIUM_HASH_ALGORITHM);
        message = CryptoUtil.messageDigest(VANADIUM_HASH_ALGORITHM, message, purpose, this.pubKey);
        // Sign.  Note that the signer will first apply another hash on the message, resulting in:
        // ECDSA.Sign(Hash(Hash(publicKey) +Hash(message) + Hash(purpose))).
        try {
            java.security.Signature sig = java.security.Signature.getInstance(javaSignAlgorithm);
            sig.initSign(this.privKey);
            sig.update(message);
            byte[] asn1Sig = sig.sign();
            return CryptoUtil.vanadiumSignature(VANADIUM_HASH_ALGORITHM, purpose, asn1Sig);
        } catch (NoSuchAlgorithmException e) {
            throw new VException("Signing algorithm " + javaSignAlgorithm +
                " not supported by the runtime: " + e.getMessage());
        } catch (InvalidKeyException e) {
            throw new VException("Invalid private key: " + e.getMessage());
        } catch (SignatureException e) {
            throw new VException(
                "Invalid signing data [ " + Arrays.toString(message) + " ]: " + e.getMessage());
        }
    }

    @Override
    public ECPublicKey publicKey() {
        return this.pubKey;
    }
}
