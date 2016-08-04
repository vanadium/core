// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import java.security.interfaces.ECPublicKey;
import java.util.List;
import java.util.Map;

import io.v.v23.verror.VException;

class BlessingStoreImpl implements BlessingStore {
    private final long nativeRef;

    private native Blessings nativeSet(
        long nativeRef, Blessings blessings, BlessingPattern forPeers) throws VException;
    private native Blessings nativeForPeer(long nativeRef, String[] peerBlessings)
            throws VException;
    private native void nativeSetDefaultBlessings(long nativeRef, Blessings blessings)
            throws VException;
    private native Blessings nativeDefaultBlessings(long nativeRef) throws VException;
    private native ECPublicKey nativePublicKey(long nativeRef) throws VException;
    private native Map<BlessingPattern, Blessings> nativePeerBlessings(long nativeRef)
            throws VException;
    private native void nativeCacheDischarge(long nativeRef, Discharge discharge, Caveat caveat,
                                             DischargeImpetus impetus);
    private native void nativeClearDischarges(long nativeRef, Object[] discharges);
    private native Discharge nativeDischarge(long nativeRef, Caveat caveat,
                                             DischargeImpetus impetus);
    private native String nativeDebugString(long nativeRef);
    private native String nativeToString(long nativeRef);
    private native void nativeFinalize(long nativeRef);

    private BlessingStoreImpl(long nativeRef) {
        this.nativeRef = nativeRef;
    }

    @Override
    public Blessings set(Blessings blessings, BlessingPattern forPeers) throws VException {
        return nativeSet(this.nativeRef, blessings, forPeers);
    }
    @Override
    public Blessings forPeer(String... peerBlessings) {
        try {
            return nativeForPeer(this.nativeRef, peerBlessings);
        } catch (VException e) {
            throw new RuntimeException("Couldn't get blessings for peers", e);
        }
    }
    @Override
    public void setDefaultBlessings(Blessings blessings) throws VException {
        nativeSetDefaultBlessings(this.nativeRef, blessings);
    }
    @Override
    public Blessings defaultBlessings() {
        try {
            return nativeDefaultBlessings(this.nativeRef);
        } catch (VException e) {
            throw new RuntimeException("Couldn't get default blessings", e);
        }
    }
    @Override
    public ECPublicKey publicKey() {
        try {
            return nativePublicKey(this.nativeRef);
        } catch (VException e) {
            throw new RuntimeException("Couldn't get public key", e);
        }
    }
    @Override
    public Map<BlessingPattern, Blessings> peerBlessings() {
        try {
            return nativePeerBlessings(this.nativeRef);
        } catch (VException e) {
            throw new RuntimeException("Couldn't get peer blessings", e);
        }
    }
    @Override
    public void cacheDischarge(Discharge discharge, Caveat caveat, DischargeImpetus impetus) {
        nativeCacheDischarge(nativeRef, discharge, caveat, impetus);
    }
    @Override
    public void clearDischarges(List<Discharge> discharges) {
        nativeClearDischarges(nativeRef, discharges.toArray());
    }
    @Override
    public Discharge discharge(Caveat caveat, DischargeImpetus impetus) {
        return nativeDischarge(nativeRef, caveat, impetus);
    }
    @Override
    public String debugString() {
        return nativeDebugString(this.nativeRef);
    }
    @Override
    public String toString() {
        return nativeToString(this.nativeRef);
    }
    @Override
    protected void finalize() {
        nativeFinalize(this.nativeRef);
    }
}
