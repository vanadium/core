// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.util;

import io.v.v23.rpc.Callback;
import io.v.v23.verror.VException;

/**
 * A {@link Callback} that calls native functions to handle success/failures.
 */
class NativeCallback<T> implements Callback<T> {
    private long nativeSuccessRef;
    private long nativeFailureRef;

    private native void nativeOnSuccess(long nativeSuccessRef, T result);
    private native void nativeOnFailure(long nativeFailureRef, VException error);
    private native void nativeFinalize(long nativeSuccessRef, long nativeFailureRef);

    private NativeCallback(long nativeSuccessRef, long nativeFailureRef) {
        this.nativeSuccessRef = nativeSuccessRef;
        this.nativeFailureRef = nativeFailureRef;
    }
    @Override
    public void onSuccess(T result) {
        nativeOnSuccess(nativeSuccessRef, result);
    }
    @Override
    public void onFailure(VException error) {
        nativeOnFailure(nativeFailureRef, error);
    }
    @Override
    protected void finalize() {
        nativeFinalize(nativeSuccessRef, nativeFailureRef);
    }
}
