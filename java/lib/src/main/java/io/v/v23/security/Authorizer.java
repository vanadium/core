// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import io.v.v23.context.VContext;
import io.v.v23.verror.VException;

/**
 * The interface for performing authorization checks.
 */
public interface Authorizer {
    /**
     * Performs authorization checks on the provided context, throwing a {@link VException}
     * iff the checks fail.
     *
     * @param  ctx             vanadium context
     * @param  call            security-related state associated with the call
     * @throws VException      iff the call isn't authorized
     */
    void authorize(VContext ctx, Call call) throws VException;
}
