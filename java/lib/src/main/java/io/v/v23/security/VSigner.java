// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import io.v.v23.verror.VException;

import java.security.interfaces.ECPublicKey;

/**
 * The interface for signing arbitrary length messages using ECDSA private keys.
 */
public interface VSigner {
    /**
     * Signs an arbitrary length message (often the hash of a larger message) using the private
     * key associated with this signer.
     * <p>
     * The provided {@code purpose} is appended to message before signing and is made available
     * (in clear text) with the signature.  Thus, a non-{@code null} {@code purpose} can be used to
     * avoid "type attacks", wherein an honest entity is cheated on interpreting a field in a message
     * as one with a type other than the intended one.
     *
     * @param  message         a message to be signed
     * @param  purpose         purpose of the message, used for preventing "type attacks"
     * @return                 the message signature
     * @throws VException      if the message cannot be signed
     */
    VSignature sign(byte[] purpose, byte[] message) throws VException;

    /**
     * Returns ECDSA public key corresponding to this signer's private key.
     */
    ECPublicKey publicKey();
}
