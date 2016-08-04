// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google.rpc;

import com.google.common.util.concurrent.AsyncFunction;
import com.google.common.util.concurrent.Futures;
import com.google.common.util.concurrent.ListenableFuture;

import io.v.impl.google.ListenableFutureCallback;
import io.v.v23.VFutures;
import io.v.v23.context.VContext;
import io.v.v23.rpc.Callback;
import io.v.v23.rpc.ClientCall;
import io.v.v23.rpc.Stream;
import io.v.v23.verror.VException;
import io.v.v23.vom.VomUtil;

import java.lang.reflect.Type;

public class ClientCallImpl implements ClientCall {
    private final VContext ctx;
    private final long nativeRef;
    private final Stream stream;

    private native void nativeCloseSend(long nativeRef, Callback<Void> callback);
    private native void nativeFinish(long nativeRef, int numResults, Callback<byte[][]> callback);
    private native void nativeFinalize(long nativeRef);

    private ClientCallImpl(VContext ctx, long nativeRef, Stream stream) {
        this.ctx = ctx;
        this.nativeRef = nativeRef;
        this.stream = stream;
    }

    @Override
    public ListenableFuture<Void> send(Object item, Type type) {
        return stream.send(item, type);
    }
    @Override
    public ListenableFuture<Object> recv(Type type) {
        return stream.recv(type);
    }
    @Override
    public ListenableFuture<Void> closeSend() {
        ListenableFutureCallback<Void> callback = new ListenableFutureCallback<>();
        nativeCloseSend(nativeRef, callback);
        return callback.getFuture(ctx);
    }
    @Override
    public ListenableFuture<Object[]> finish(final Type[] types) {
        ListenableFutureCallback<byte[][]> callback = new ListenableFutureCallback<>();
        nativeFinish(nativeRef, types.length, callback);
        return VFutures.withUserLandChecks(ctx,
                Futures.transform(callback.getVanillaFuture(),
                        new AsyncFunction<byte[][], Object[]>() {
                            @Override
                            public ListenableFuture<Object[]> apply(byte[][] vomResults) throws Exception {
                                if (vomResults.length != types.length) {
                                    throw new VException(String.format(
                                            "Mismatch in number of results, want %s, have %s",
                                            types.length, vomResults.length));
                                }
                                // VOM-decode results.
                                Object[] ret = new Object[types.length];
                                for (int i = 0; i < types.length; i++) {
                                    ret[i] = VomUtil.decode(vomResults[i], types[i]);
                                }
                                return Futures.immediateFuture(ret);
                            }
                        }));
    }
    @Override
    protected void finalize() {
        nativeFinalize(nativeRef);
    }
}
