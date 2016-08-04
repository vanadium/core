// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import io.v.v23.verror.VException;

import java.security.interfaces.ECPublicKey;

class VPrincipalImpl implements VPrincipal {
    private final long nativeRef;
    private final VSigner signer;
    private final BlessingStore store;
    private final BlessingRoots roots;

    private static native VPrincipalImpl nativeCreate() throws VException;
    private static native VPrincipalImpl nativeCreateForSigner(VSigner signer) throws VException;
    private static native VPrincipalImpl nativeCreateForAll(VSigner signer, BlessingStore store,
        BlessingRoots roots) throws VException;
    private static native VPrincipalImpl nativeCreatePersistent(String passphrase, String dir)
            throws VException;
    private static native VPrincipalImpl nativeCreatePersistentForSigner(VSigner signer, String dir)
            throws VException;

    static VPrincipalImpl create() throws VException {
        return nativeCreate();
    }
    static VPrincipalImpl create(VSigner signer) throws VException {
        return nativeCreateForSigner(signer);
    }
    static VPrincipalImpl create(VSigner signer, BlessingStore store, BlessingRoots roots)
        throws VException {
        return nativeCreateForAll(signer, store, roots);
    }
    static VPrincipalImpl createPersistent(String passphrase, String dir) throws VException {
        return nativeCreatePersistent(passphrase, dir);
    }
    static VPrincipalImpl createPersistent(VSigner signer, String dir) throws VException {
        return nativeCreatePersistentForSigner(signer, dir);
    }

    private native Blessings nativeBless(long nativeRef, ECPublicKey key, Blessings with,
        String extension, Caveat caveat, Caveat[] additionalCaveats) throws VException;
    private native Blessings nativeBlessSelf(long nativeRef, String name, Caveat[] caveats)
            throws VException;
    private native VSignature nativeSign(long nativeRef, byte[] message) throws VException;
    private native ECPublicKey nativePublicKey(long nativeRef) throws VException;
    private native BlessingStore nativeBlessingStore(long nativeRef) throws VException;
    private native BlessingRoots nativeRoots(long nativeRef) throws VException;
    private native void nativeFinalize(long nativeRef);

    private VPrincipalImpl(
            long nativeRef, VSigner signer, BlessingStore store, BlessingRoots roots) {
        this.nativeRef = nativeRef;
        this.signer = signer;
        this.store = store;
        this.roots = roots;
    }

    @Override
    public Blessings bless(ECPublicKey key, Blessings with, String extension, Caveat caveat,
        Caveat... additionalCaveats) throws VException {
        return nativeBless(nativeRef, key, with, extension, caveat, additionalCaveats);
    }
    @Override
    public Blessings blessSelf(String name, Caveat... caveats) throws VException {
        return nativeBlessSelf(nativeRef, name, caveats);
    }
    @Override
    public VSignature sign(byte[] message) throws VException {
        if (this.signer != null) {
            byte[] purpose = Constants.SIGNATURE_FOR_MESSAGE_SIGNING.getBytes();
            return this.signer.sign(purpose, message);
        }
        return nativeSign(nativeRef, message);
    }
    @Override
    public ECPublicKey publicKey() {
        if (this.signer != null) {
            return this.signer.publicKey();
        }
        try {
            return nativePublicKey(nativeRef);
        } catch (VException e) {
            throw new RuntimeException("Couldn't get public key", e);
        }
    }
    @Override
    public BlessingStore blessingStore() {
        if (this.store != null) {
            return this.store;
        }
        try {
            return nativeBlessingStore(nativeRef);
        } catch (VException e) {
            throw new RuntimeException("Couldn't get Blessing Store", e);
        }
    }
    @Override
    public BlessingRoots roots() {
        if (this.roots != null) {
            return this.roots;
        }
        try {
            return nativeRoots(nativeRef);
        } catch (VException e) {
            throw new RuntimeException("Couldn't get Blessing Store", e);
        }
    }
    private long nativeRef() { return nativeRef; }
    @Override
    public boolean equals(Object other) {
        if (this == other) return true;
        if (other == null) return false;
        if (this.getClass() != other.getClass()) return false;
        return nativeRef == ((VPrincipalImpl) other).nativeRef;
    }
    @Override
    public int hashCode() {
        return Long.valueOf(nativeRef).hashCode();
    }
    @Override
    protected void finalize() {
        nativeFinalize(nativeRef);
    }
}
