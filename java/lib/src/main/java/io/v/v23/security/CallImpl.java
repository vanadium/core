// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import org.joda.time.DateTime;

import java.util.Map;

import io.v.v23.vdl.VdlValue;
import io.v.v23.verror.VException;

class CallImpl implements Call {
    private final long nativeRef;

    private native DateTime nativeTimestamp(long nativeRef) throws VException;
    private native String nativeMethod(long nativeRef);
    private native VdlValue[] nativeMethodTags(long nativeRef) throws VException;
    private native String nativeSuffix(long nativeRef);
    private native Map<String, Discharge> nativeLocalDischarges(long nativeRef);
    private native Map<String, Discharge> nativeRemoteDischarges(long nativeRef);
    private native String nativeLocalEndpoint(long nativeRef);
    private native String nativeRemoteEndpoint(long nativeRef);
    private native VPrincipal nativeLocalPrincipal(long nativeRef) throws VException;
    private native Blessings nativeLocalBlessings(long nativeRef) throws VException;
    private native Blessings nativeRemoteBlessings(long nativeRef) throws VException;
    private native void nativeFinalize(long nativeRef);

    CallImpl(long nativeRef) {
        this.nativeRef = nativeRef;
    }
    // Implements io.v.v23.security.VContext.
    @Override
    public DateTime timestamp() {
        try {
            return nativeTimestamp(nativeRef);
        } catch (VException e) {
            throw new RuntimeException("Couldn't get timestamp", e);
        }
    }
    @Override
    public String method() {
        return nativeMethod(nativeRef);
    }
    @Override
    public VdlValue[] methodTags() {
        try {
            VdlValue[] tags = nativeMethodTags(nativeRef);
            return tags != null ? tags : new VdlValue[0];
        } catch (VException e) {
            throw new RuntimeException("Couldn't get method tags", e);
        }
    }
    @Override
    public String suffix() {
        return nativeSuffix(nativeRef);
    }
    @Override
    public Map<String, Discharge> localDischarges() {
        return nativeLocalDischarges(nativeRef);
    }
    @Override
    public Map<String, Discharge> remoteDischarges() {
        return nativeRemoteDischarges(nativeRef);
    }
    @Override
    public String localEndpoint() {
        return nativeLocalEndpoint(nativeRef);
    }
    @Override
    public String remoteEndpoint() {
        return nativeRemoteEndpoint(nativeRef);
    }
    @Override
    public VPrincipal localPrincipal() {
        try {
            return nativeLocalPrincipal(nativeRef);
        } catch (VException e) {
            throw new RuntimeException("Couldn't get local principal", e);
        }
    }
    @Override
    public Blessings localBlessings() {
        try {
            return nativeLocalBlessings(nativeRef);
        } catch (VException e) {
            throw new RuntimeException("Couldn't get local blessings", e);
        }
    }
    @Override
    public Blessings remoteBlessings() {
        try {
            return nativeRemoteBlessings(nativeRef);
        } catch (VException e) {
            throw new RuntimeException("Couldn't get remote blessings", e);
        }
    }
    // Implements java.lang.Object.
    @Override
    protected void finalize() {
        nativeFinalize(nativeRef);
    }

    private long nativeRef() {
        return nativeRef;
    }
}
