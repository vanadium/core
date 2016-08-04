// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google.channel;

import com.google.common.util.concurrent.ListenableFuture;

import io.v.impl.google.ListenableFutureCallback;
import io.v.v23.OutputChannel;
import io.v.v23.context.VContext;
import io.v.v23.rpc.Callback;

/**
 * An implementation of {@link OutputChannel} that sends data using Go send, convert, and close
 * functions.
 */
class OutputChannelImpl<T> implements OutputChannel<T> {
    private final VContext ctx;
    private final long nativeConvertRef;
    private final long nativeSendRef;
    private final long nativeCloseRef;

    private static native <T> void nativeSend(long nativeConvertRef, long nativeSendRef, T value,
                                              Callback<Void> callback);
    private static native void nativeClose(long nativeCloseRef, Callback<Void> callback);
    private static native void nativeFinalize(long nativeConvertRef, long nativeSendRef, long nativeCloseRef);

    private OutputChannelImpl(VContext ctx, long convertRef, long sendRef, long closeRef) {
        this.ctx = ctx;
        this.nativeConvertRef = convertRef;
        this.nativeSendRef = sendRef;
        this.nativeCloseRef = closeRef;
    }
    @Override
    public ListenableFuture<Void> send(T item) {
        ListenableFutureCallback<Void> callback = new ListenableFutureCallback<>();
        nativeSend(nativeConvertRef, nativeSendRef, item, callback);
        return callback.getFuture(ctx);
    }
    @Override
    public ListenableFuture<Void> close() {
        ListenableFutureCallback<Void> callback = new ListenableFutureCallback<>();
        nativeClose(nativeCloseRef, callback);
        return callback.getFuture(ctx);
    }
    @Override
    protected void finalize() throws Throwable {
        nativeFinalize(nativeConvertRef, nativeSendRef, nativeCloseRef);
    }
}
