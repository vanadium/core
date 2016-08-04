// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google.lib.discovery;

import com.google.common.util.concurrent.ListenableFuture;
import com.google.common.util.concurrent.FutureFallback;
import com.google.common.util.concurrent.Futures;

import java.util.List;
import java.util.concurrent.CancellationException;

import io.v.v23.InputChannel;
import io.v.v23.context.VContext;
import io.v.v23.discovery.Advertisement;
import io.v.v23.discovery.Discovery;
import io.v.v23.discovery.Update;
import io.v.v23.security.BlessingPattern;
import io.v.v23.verror.VException;

import io.v.impl.google.ListenableFutureCallback;

class DiscoveryImpl implements Discovery {
    private final long nativeRef;

    private native void nativeAdvertise(
            long nativeRef,
            VContext ctx,
            Advertisement ad,
            List<BlessingPattern> visibility,
            ListenableFutureCallback<Void> cb)
            throws VException;

    private native InputChannel<Update> nativeScan(long nativeRef, VContext ctx, String query)
            throws VException;

    private native void nativeFinalize(long nativeRef);

    private DiscoveryImpl(long nativeRef) {
        this.nativeRef = nativeRef;
    }

    @Override
    public ListenableFuture<Void> advertise(
            VContext ctx, Advertisement ad, List<BlessingPattern> visibility) throws VException {
        ListenableFutureCallback<Void> cb = new ListenableFutureCallback<>();
        nativeAdvertise(nativeRef, ctx, ad, visibility, cb);
        return Futures.withFallback(
                cb.getFuture(ctx),
                new FutureFallback<Void>() {
                    public ListenableFuture<Void> create(Throwable t) {
                        if (t instanceof CancellationException) {
                            return Futures.immediateFuture(null);
                        }
                        return Futures.immediateFailedFuture(t);
                    }
                });
    }

    @Override
    public InputChannel<Update> scan(VContext ctx, String query) throws VException {
        return nativeScan(nativeRef, ctx, query);
    }

    @Override
    protected void finalize() throws Throwable {
        super.finalize();
        nativeFinalize(nativeRef);
    }
}
