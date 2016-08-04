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
import io.v.v23.rpc.Stream;
import io.v.v23.verror.VException;
import io.v.v23.vom.VomUtil;

import java.lang.reflect.Type;

public class StreamImpl implements Stream {
    private final VContext ctx;
    private final long nativeRef;

    private native void nativeSend(long nativeRef, byte[] vomItem, Callback<Void> callback);
    private native void nativeRecv(long nativeRef, Callback<byte[]> callback);
    private native void nativeFinalize(long nativeRef);

    private StreamImpl(VContext ctx, long nativeRef) {
        this.ctx = ctx;
        this.nativeRef = nativeRef;
    }

    @Override
    public ListenableFuture<Void> send(Object item, Type type) {
        ListenableFutureCallback<Void> callback = new ListenableFutureCallback<>();
        try {
            byte[] vomItem = VomUtil.encode(item, type);
            nativeSend(nativeRef, vomItem, callback);
        } catch (VException e) {
            callback.onFailure(e);
        }
        return callback.getFuture(ctx);
    }
    @Override
    public ListenableFuture<Object> recv(final Type type) {
        ListenableFutureCallback<byte[]> callback = new ListenableFutureCallback<>();
        nativeRecv(nativeRef, callback);
        return VFutures.withUserLandChecks(ctx,
                Futures.transform(callback.getVanillaFuture(),
                        new AsyncFunction<byte[], Object>() {
                    @Override
                    public ListenableFuture<Object> apply(byte[] result) throws Exception {
                        return Futures.immediateFuture(VomUtil.decode(result, type));
                    }
                }));
    }
    @Override
    protected void finalize() {
        nativeFinalize(this.nativeRef);
    }
}
