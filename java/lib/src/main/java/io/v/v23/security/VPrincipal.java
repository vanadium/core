// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import io.v.v23.verror.VException;

import java.security.interfaces.ECPublicKey;

/**
 * An entity capable of making or receiving RPCs.  {@link VPrincipal}s have a unique
 * (public, private) key pair, have blessings bound to them, and can bless other principals.
 * <p>
 * Multiple goroutines may invoke methods on a {@link VPrincipal} simultaneously.
 * <p>
 * See also: <a href="https://vanadium.github.io/glossary.html#principal">https://vanadium.github.io/glossary.html#principal</a>.
 */
public interface VPrincipal {
    /**
     * Binds extensions of blessings held by this principal to another principal (represented by
     * its public key).
     * <p>
     * For example, a principal with the blessings {@code "google:alice"} and
     * {@code "vanadium:alice"} can bind the blessings {@code "google:alice:friend"} and
     * {@code "vanadium:alice:friend"} to another principal using:
     * <p><blockquote><pre>
     *   {@literal bless(<other principal>, <google:alice, vanadium:alice>, "friend", ...)}
     * </pre></blockquote><p>
     * To discourage unconstrained delegation of authority, the interface requires at least one
     * caveat to be provided. If unconstrained delegation is desired, the
     * {@link VSecurity#newUnconstrainedUseCaveat} method can be used to produce this argument.
     * <p>
     * {@code with.publicKey()} must be the same as the principal's public key.
     *
     * @param  key               public key representing the principal being blessed
     * @param  with              blessings of the current principal (i.e., the one doing the
     *                           blessing) that should be used for the blessing
     * @param  extension         extension that the blessee should be blessed with
     * @param  caveat            caveat on the blessing
     * @param  additionalCaveats addional caveats on the blessing
     * @return                   the resulting blessings
     * @throws VException        if the blessee couldn't be blessed
     */
    Blessings bless(ECPublicKey key, Blessings with, String extension, Caveat caveat,
        Caveat... additionalCaveats) throws VException;

    /**
     * Creates a blessing with the provided name for this principal.
     *
     * @param  name            the name to bless self with
     * @param  caveats         caveats on the blessings
     * @return                 the resulting blessings
     * @throws VException      if there was an error blessing self
     */
    Blessings blessSelf(String name, Caveat... caveats) throws VException;

    /**
     * Uses the private key of the principal to sign a message.
     *
     * @param  message         the message to be signed
     * @return                 signature of the message
     * @throws VException      if the message couldn't be signed
     */
    VSignature sign(byte[] message) throws VException;

    /**
     * Returns the public key counterpart of the private key held by the principal.
     */
    ECPublicKey publicKey();

    /**
     * Provides access to the {@link BlessingStore} containing blessings that have been granted to
     * this principal.  The returned {@link BlessingStore} is never {@code null}.
     *
     * @return a {@link BlessingStore} containing blessings that have been granted to this principal
     */
    BlessingStore blessingStore();

    /**
     * Returns the set of recognized authorities (identified by their public keys) on blessings that
     * match specific patterns.  Never returns {@code null}.
     */
    BlessingRoots roots();
}
