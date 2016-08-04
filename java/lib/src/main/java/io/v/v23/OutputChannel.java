// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23;

import com.google.common.util.concurrent.ListenableFuture;

import javax.annotation.CheckReturnValue;

/**
 * The write-end of a channel of {@code T}.
 */
public interface OutputChannel<T> {
    /**
     * Writes the given value to the channel.
     * <p>
     * The returned future is guaranteed to be executed on an {@link java.util.concurrent.Executor}
     * specified in the context used for creating this channel (see {@link V#withExecutor}).
     * <p>
     * The returned future will fail with {@link java.util.concurrent.CancellationException} if the
     * context used for creating this channel has been canceled.
     *
     * @param item        an item to be sent
     */
    @CheckReturnValue
    ListenableFuture<Void> send(T item);

    /**
     * Indicates to the receiver that no more items will be sent.
     * <p>
     * This is an optional call intended to signal the receiver that no more items will be sent.
     * <p>
     * The returned future is guaranteed to be executed on an {@link java.util.concurrent.Executor}
     * specified in the context used for creating this channel (see {@link V#withExecutor}).
     * <p>
     * The returned future will fail with {@link java.util.concurrent.CancellationException} if the
     * context used for creating this channel has been canceled.
     */
    @CheckReturnValue
    ListenableFuture<Void> close();
}
