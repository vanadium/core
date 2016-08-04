// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

import com.google.common.util.concurrent.ListenableFuture;

import javax.annotation.CheckReturnValue;

/**
 * Represents the send side of the server bidirectional stream.
 *
 * @param <SendT>   type of values that the server is sending to the client
 */
public interface ServerSendStream<SendT> {
    /**
     * Writes the given value to the stream.
     * <p>
     * The returned future is guaranteed to be executed on an {@link java.util.concurrent.Executor}
     * specified in the context used for creating this channel
     * (see {@link io.v.v23.V#withExecutor}).
     * <p>
     * The returned future will fail with {@link java.util.concurrent.CancellationException} if the
     * context used for creating this channel has been canceled.
     *
     * @param item        an item to be sent
     */
    @CheckReturnValue
    ListenableFuture<Void> send(SendT item);
}
