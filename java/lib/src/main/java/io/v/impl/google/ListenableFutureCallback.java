// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google;

import com.google.common.util.concurrent.ListenableFuture;
import com.google.common.util.concurrent.SettableFuture;

import io.v.v23.VFutures;
import io.v.v23.context.VContext;
import io.v.v23.rpc.Callback;
import io.v.v23.verror.VException;

/**
 * A {@link Callback} that creates a {@link ListenableFuture} whose success/failure depends
 * on success/failure of the callback.
 */
public class ListenableFutureCallback<T> implements Callback<T> {
    private final SettableFuture<T> future = SettableFuture.create();

    /**
     * Returns a {@link ListenableFuture} whose success/failure depends on success/failure of this
     * {@link Callback}.
     * <p>
     * The returned future will perform the checks specified in {@link VFutures#withUserLandChecks}.
     */
    public ListenableFuture<T> getFuture(VContext context) {
        return VFutures.withUserLandChecks(context, future);
    }

    /**
     * Returns a {@link ListenableFuture} whose success/failure depends on success/failure of this
     * {@link Callback} and is executed on an {@link java.util.concurrent.Executor} specified in the
     * given {@code context}.
     */
    public ListenableFuture<T> getFutureOnExecutor(VContext context) {
        return VFutures.onExecutor(context, future);
    }

    /**
     * Returns a {@link ListenableFuture} whose success/failure depends on success/failure of this
     * {@link Callback}.
     */
    public ListenableFuture<T> getVanillaFuture() {
        return future;
    }

    @Override
    public void onSuccess(T result) {
        future.set(result);
    }
    @Override
    public void onFailure(VException error) {
        future.setException(error);
    }
}
