// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.rpc;

import com.google.common.util.concurrent.ListenableFuture;

import java.lang.reflect.Type;

import javax.annotation.CheckReturnValue;

/**
 * The interface for a bidirectional FIFO stream of typed values.
 */
public interface Stream {
    /**
     * Places the item onto the output stream.
     * <p>
     * The returned future is guaranteed to be executed on an {@link java.util.concurrent.Executor}
     * specified in the context used for creating this stream (see {@link io.v.v23.V#withExecutor}).
     * <p>
     * The returned future will fail with {@link java.util.concurrent.CancellationException} if the
     * context used for creating this stream has been canceled.
     *
     * @param  item  an item to be sent
     * @param  type  type of the provided item
     */
    @CheckReturnValue
    ListenableFuture<Void> send(Object item, Type type);

    /**
     * Returns a new {@link ListenableFuture} whose result is the next item in the stream.
     * <p>
     * The returned {@link ListenableFuture} will fail if there was an error fetching the next
     * item;  {@link io.v.v23.verror.EndOfFileException} means that a graceful end of input has been
     * reached;  {@link java.util.concurrent.CancellationException} means that the context used
     * for creating this stream has been canceled.
     * <p>
     * The returned future is guaranteed to be executed on an {@link java.util.concurrent.Executor}
     * specified in the context used for creating this stream (see {@link io.v.v23.V#withExecutor}).
     *
     * @param  type type of the returned item
     */
    @CheckReturnValue
    ListenableFuture<Object> recv(Type type);
}
