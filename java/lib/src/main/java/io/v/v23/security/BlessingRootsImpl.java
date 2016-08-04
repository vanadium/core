// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import com.google.common.collect.Multimap;
import io.v.v23.verror.VException;

import java.security.interfaces.ECPublicKey;

class BlessingRootsImpl implements BlessingRoots {
    private final long nativeRef;

    private native void nativeAdd(long nativeRef, ECPublicKey root, BlessingPattern pattern)
        throws VException;
    private native void nativeRecognized(long nativeRef, ECPublicKey root, String blessing)
        throws VException;
    private native String nativeDebugString(long nativeRef);
    private native String nativeToString(long nativeRef);
    private native Multimap<BlessingPattern, ECPublicKey> nativeDump(long nativeRef) throws VException;
    private native void nativeFinalize(long nativeRef);

    private BlessingRootsImpl(long nativeRef) {
        this.nativeRef = nativeRef;
    }

    @Override
    public void add(ECPublicKey root, BlessingPattern pattern) throws VException {
        nativeAdd(nativeRef, root, pattern);
    }
    @Override
    public void recognized(ECPublicKey root, String blessing) throws VException {
        nativeRecognized(nativeRef, root, blessing);
    }
    @Override
    public String debugString() {
        return nativeDebugString(nativeRef);
    }

    @Override
    public Multimap<BlessingPattern, ECPublicKey> dump() throws VException {
        return nativeDump(nativeRef);
    }
    private long nativeRef() {
        return nativeRef;
    }
    @Override
    public String toString() {
        return nativeToString(nativeRef);
    }
    @Override
    protected void finalize() {
        nativeFinalize(nativeRef);
    }
}
