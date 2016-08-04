// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

import com.google.common.util.concurrent.ListenableFuture;

import javax.annotation.CheckReturnValue;

import io.v.v23.InputChannel;

/**
 * Represents the receiving side of the client bidirectional stream.
 *
 * @param <RecvT>   type of values that the client is receiving from the server
 * @param <FinishT> type of the final return value from the server
 */
public interface ClientRecvStream<RecvT, FinishT> extends InputChannel<RecvT> {
    /**
     * Returns a new {@link ListenableFuture} that waits until the server is done and then returns
     * the call return value.
     * <p>
     * Calling {@link #finish} is mandatory for releasing stream resources, unless the call context
     * has been canceled or any of the other methods threw an exception.
     * <p>
     * Must be called at most once.
     * <p>
     * The returned future is guaranteed to be executed on an {@link java.util.concurrent.Executor}
     * specified in the context used for creating this stream (see {@link io.v.v23.V#withExecutor}).
     * <p>
     * The returned future will fail with {@link java.util.concurrent.CancellationException} if the
     * context used for creating this stream has been canceled.
     *
     * @return a new {@link ListenableFuture} whose result is the call return value
     */
    @CheckReturnValue
    ListenableFuture<FinishT> finish();
}
