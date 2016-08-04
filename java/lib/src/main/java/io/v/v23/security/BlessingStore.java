// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import java.security.interfaces.ECPublicKey;
import java.util.List;
import java.util.Map;

import io.v.v23.verror.VException;

/**
 * The interface for storing blessings bound to a principal and managing the subset of blessings to
 * be presented to particular peers.
 */
public interface BlessingStore {
    /**
     * Marks the set of blessings to be shared with peers.
     * <p>
     * {@code set(b, pattern)} marks the intention to reveal {@code b} to peers who
     * present blessings that match the given {@code pattern}.
     * <p>
     * If multiple calls to {@link #set set} are made with the same pattern, the last call prevails.
     * <p>
     * {@code set(null, pattern)} can be used to remove the blessings previously associated with
     * the pattern (by a prior call to {@link #set set}).
     * <p>
     * It is an error to call {@link #set set} with blessings whose public key does not match the
     * public key of the principal for which this store hosts blessings.
     *
     * @param  blessings       blessings to be revealed to the specified peers
     * @param  forPeers        a peer to whom the blessings should be revealed
     * @return                 blessings previously associated with the specified pattern
     * @throws VException      if there was an error making the association
     */
    Blessings set(Blessings blessings, BlessingPattern forPeers) throws VException;

    /**
     * Returns the set of blessings that have been previously added to the store with an intent of
     * being shared with peers that have at least one of the provided blessings.
     * <p>
     * If no peer blessings are provided then blessings marked for all peers (i.e., those added
     * with the {@link Constants#ALL_PRINCIPALS} pattern) is returned.
     * <p>
     * Returns {@code null} if there are no matching blessings in the store.
     *
     * @param  peerBlessings human-readable peer blessings we're retrieving blessings for
     * @return               the set of blessings that have been previously added to the store with
     *                       an intent of being shared with peers that have at least one of the
     *                       provided (human-readable) blessings
     */
    Blessings forPeer(String... peerBlessings);

    /**
     * Sets up the blessings made available on a subsequent call to
     * {@link #defaultBlessings defaultBlessing}.
     * <p>
     * It is an error to call {@link #setDefaultBlessings setDefaultBlessings} with blessings whose
     * public key does not match the public key of the principal for which this store hosts
     * blessings.
     *
     * @param  blessings       blessings made available on a subsequent call to
     *                         {@link #defaultBlessings defaultBlessings}
     * @throws VException      if there was an error setting the default blessings
     */
    void setDefaultBlessings(Blessings blessings) throws VException;

    /**
     * Returns the blessings to be shared with peers for which no other information is
     * available in order to select blessings from the store.
     * <p>
     * For example, {@link #defaultBlessings defaultBlessings} can be used by servers to identify
     * themselves to clients before the client has identified itself.
     * <p>
     * This method returns the blessings provided to the last call to
     * {@link #setDefaultBlessings setDefaultBlessings}, or if no such call was made it is
     * equivalent to {@link #forPeer forPeer} with no arguments.
     *
     * @return blessings to be shared with peers for which no other information is available
     */
    Blessings defaultBlessings();

    /**
     * Returns the public key of the principal for which this store hosts blessings.
     */
    ECPublicKey publicKey();

    /**
     * Returns all the blessings that the store currently holds for various peers.  Never returns
     * {@code null}.
     */
    Map<BlessingPattern, Blessings> peerBlessings();

    /**
     * Inserts the discharge for the provided impetus and caveat into the cache.
     */
    void cacheDischarge(Discharge discharge, Caveat caveat, DischargeImpetus impetus);

    /**
     * Clears the input discharges from the BlessingStore's discharge cache.
     */
    void clearDischarges(List<Discharge> discharges);

    /**
     * Takes a caveat and DischargeImpetus and returns a cached discharge, returning a {@link
     * WireDischarge#zeroValue zero value} if no corresponding cached discharge can be found.
     */
    Discharge discharge(Caveat caveat, DischargeImpetus impetus);

    /**
     * Return a human-readable string description of the store.  This description is detailed and
     * lists out the contents of the store.  Use {@link Object#toString} for a more succinct
     * description.
     */
    String debugString();
}
