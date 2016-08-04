// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google.rpc;

import io.v.v23.naming.Endpoint;
import io.v.v23.rpc.Server;
import io.v.v23.rpc.ServerCall;
import io.v.v23.security.Blessings;
import io.v.v23.security.Call;
import io.v.v23.verror.VException;

public class ServerCallImpl implements ServerCall {
    private final long nativeRef;

    private static native Call nativeSecurity(long nativeRef);
    private static native String nativeSuffix(long nativeRef);
    private static native Endpoint nativeLocalEndpoint(long nativeRef);
    private static native Endpoint nativeRemoteEndpoint(long nativeRef);
    private static native Blessings nativeGrantedBlessings(long nativeRef) throws VException;
    private static native Server nativeServer(long nativeRef) throws VException;
    private static native void nativeFinalize(long nativeRef);

    private ServerCallImpl(long nativeRef) {
        this.nativeRef = nativeRef;
    }

    @Override
    public Call security() {
        return nativeSecurity(nativeRef);
    }

    @Override
    public String suffix() {
        return nativeSuffix(nativeRef);
    }

    @Override
    public Endpoint localEndpoint() {
        return nativeLocalEndpoint(nativeRef);
    }

    @Override
    public Endpoint remoteEndpoint() {
        return nativeRemoteEndpoint(nativeRef);
    }

    @Override
    public Blessings grantedBlessings() {
        try {
            return nativeGrantedBlessings(nativeRef);
        } catch (VException e) {
            throw new RuntimeException("Couldn't get granted blessings: ", e);
        }
    }

    @Override
    public Server server() {
        try {
            return nativeServer(nativeRef);
        } catch (VException e) {
            throw new RuntimeException("Couldn't get server: ", e);
        }
    }

    @Override
    protected void finalize() throws Throwable {
        nativeFinalize(nativeRef);
    }
}
