// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23;

import com.google.common.util.concurrent.ListenableFuture;

/**
 * Instances of this interface are called for every successful asynchronous iteration
 * over the {@link InputChannel} using {@link InputChannels#withCallback}.
 */
public interface InputChannelCallback<T> {
    /**
     * This method invoked with the result of every successful asynchronous iteration
     * over the {@link InputChannel} using {@link InputChannels#withCallback}
     * <p>
     * All further iterations will be delayed until the returned future completes.  If {@code null}
     * is returned, it is treated as an immediate (successful) future.
     * <p>
     * If the returned future fails, the iteration will terminate early.
     */
    ListenableFuture<Void> onNext(T result);
}
