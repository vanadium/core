// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.rpc;

import com.google.common.util.concurrent.ListenableFuture;

import java.lang.reflect.Type;

import javax.annotation.CheckReturnValue;

/**
 * The interface for each in-flight call on the {@link Client}.  Method {@link #finish finish}
 * must be called to finish the call; all other methods are optional.
 */
public interface ClientCall extends Stream {
    /**
     * Indicates to the server that no further items will be sent.
     * <p>
     * This method will cause the server's {@link StreamServerCall#recv recv} call to eventually
     * fail with {@link io.v.v23.verror.EndOfFileException} (i.e., after all sent items have been
     * received).
     * <p>
     * Completion of this method will cause all client's future {@link Stream#send send} calls
     * to fail.
     * <p>
     * The returned future is guaranteed to be executed on an {@link java.util.concurrent.Executor}
     * specified in the context used for creating this call (see {@link io.v.v23.V#withExecutor}).
     * <p>
     * The returned future will fail with {@link java.util.concurrent.CancellationException} if the
     * context used for creating this call has been canceled.
     */
    @CheckReturnValue
    ListenableFuture<Void> closeSend();

    /**
     * Returns a new {@link ListenableFuture} whose result are the positional output arguments
     * (of any arity) for the call.
     * <p>
     * The returned future is guaranteed to be executed on an {@link java.util.concurrent.Executor}
     * specified in the context used for creating this call (see {@link io.v.v23.V#withExecutor}).
     * <p>
     * The returned future will fail with {@link java.util.concurrent.CancellationException} if the
     * context used for creating this call has been canceled.
     *
     * @param  types types for all the output arguments
     */
    @CheckReturnValue
    ListenableFuture<Object[]> finish(Type[] types);
}
