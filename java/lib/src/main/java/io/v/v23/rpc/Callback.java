// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.rpc;

import io.v.v23.verror.VException;

/**
 * Instances of this interface are called when an asynchronous RPC succeeds or fails.
 */
public interface Callback<T> {
    /**
     * This method will be called when an asynchronous RPC succeeds. Implementations
     * should not perform expensive operations on this thread.
     *
     * @param result the result of the RPC
     */
    void onSuccess(T result);

    /**
     * This method will be called when an asynchronous RPC fails. Implementations
     * should not perform expensive operations on this thread.
     *
     * @param error the {@link VException} that caused the RPC to fail
     */
    void onFailure(VException error);
}
