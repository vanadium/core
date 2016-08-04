// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import io.v.v23.context.VContext;
import io.v.v23.verror.VException;

/**
 * Validator for {@link Constants#CONST_CAVEAT} caveat, which represents a caveat that either
 * always validates or never validates.
 */
public class ConstCaveatValidator implements CaveatValidator {
    /**
     * A singleton instance of {@link ConstCaveatValidator}.
     */
    public static final ConstCaveatValidator INSTANCE = new ConstCaveatValidator();

    @Override
    public void validate(VContext context, Call call, Object param) throws VException {
        if (param == null) param = Boolean.valueOf(false);
        if (!(param instanceof Boolean)) {
            throw new VException(String.format(
                    "Caveat param %s of wrong type: want %s", param, Boolean.class));
        }
        if (!(Boolean)param) {
            throw new VException(String.format("ConstCaveat(%s) failed validation.", param));
        }
    }

    private ConstCaveatValidator() {}
}
