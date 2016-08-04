// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google.rpc;


import com.google.common.util.concurrent.ListenableFuture;

import io.v.v23.naming.Endpoint;
import io.v.v23.rpc.Server;
import io.v.v23.rpc.ServerCall;
import io.v.v23.rpc.Stream;
import io.v.v23.rpc.StreamServerCall;
import io.v.v23.security.Blessings;
import io.v.v23.security.Call;

import java.lang.reflect.Type;

public class StreamServerCallImpl implements StreamServerCall {
    private final long nativeRef;
    private final Stream stream;
    private final ServerCall serverCall;

    private native void nativeFinalize(long nativeRef);

    private StreamServerCallImpl(long nativeRef, Stream stream, ServerCall serverCall) {
        this.nativeRef = nativeRef;
        this.stream = stream;
        this.serverCall = serverCall;
    }
    // Implements io.v.v23.rpc.Stream.
    @Override
    public ListenableFuture<Void> send(Object item, Type type) {
        return stream.send(item, type);
    }
    @Override
    public ListenableFuture<Object> recv(Type type) {
        return stream.recv(type);
    }
    // Implements io.v.v23.rpc.ServerCall.
    @Override
    public Blessings grantedBlessings() {
        return serverCall.grantedBlessings();
    }

    @Override
    public Server server() {
        return serverCall.server();
    }

    // Implements io.v.v23.rpc.ServerCall.
    @Override
    public Call security() {
        return serverCall.security();
    }
    @Override
    public String suffix() {
        return this.serverCall.suffix();
    }
    @Override
    public Endpoint localEndpoint() {
        return this.serverCall.localEndpoint();
    }
    @Override
    public Endpoint remoteEndpoint() {
        return this.serverCall.remoteEndpoint();
    }
    // Implements java.lang.Object.
    @Override
    protected void finalize() {
        nativeFinalize(this.nativeRef);
    }
}
