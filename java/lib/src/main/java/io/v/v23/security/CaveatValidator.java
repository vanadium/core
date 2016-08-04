// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import io.v.v23.context.VContext;
import io.v.v23.verror.VException;

/**
 * The interface for validating the restrictions specified in a {@link Caveat}.
 */
public interface CaveatValidator {
    /**
     * Throws an exception iff the restriction encapsulated in the corresponding caveat parameter
     * hasn't been satisfied given the context.
     *
     * @param  context         a vanadium context
     * @param  param           the (sole) caveat parameter
     * @throws VException      if the caveat couldn't be validated
     */
    void validate(VContext context, Call call, Object param) throws VException;
}
