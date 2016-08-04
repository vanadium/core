// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.util;

import com.google.common.util.concurrent.FutureCallback;
import com.google.common.util.concurrent.Futures;
import com.google.common.util.concurrent.ListenableFuture;

import io.v.v23.rpc.Callback;
import io.v.v23.verror.VException;

class Util {
    // Helper class for transforming a future into a callback.
    static <T> void asCallback(ListenableFuture<T> future, final Callback<T> callback) {
        Futures.addCallback(future, new FutureCallback<T>() {
            @Override
            public void onSuccess(T result) {
                callback.onSuccess(result);
            }
            @Override
            public void onFailure(Throwable t) {
                callback.onFailure(
                        t instanceof VException ? (VException) t : new VException(t.getMessage()));
            }
        });
    }

    private Util() {}
}
