// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import io.v.v23.context.VContext;
import io.v.v23.verror.VException;

/**
 * Validator for {@link Constants#PUBLIC_KEY_THIRD_PARTY_CAVEAT} caveat, which represents a caveat
 * that validates iff the remote party provided a discharge that corresponds to the key stored in
 * the caveat.
 */
public class PublicKeyThirdPartyCaveatValidator implements CaveatValidator {
    /**
     * A singleton instance of {@link PublicKeyThirdPartyCaveatValidator}.
     */
    public static final PublicKeyThirdPartyCaveatValidator INSTANCE =
            new PublicKeyThirdPartyCaveatValidator();

    private static native void nativeValidate(VContext context, Call call, Object param)
            throws VException;

    private PublicKeyThirdPartyCaveatValidator() {
    }

    @Override
    public void validate(VContext context, Call call, Object param) throws VException {
        nativeValidate(context, call, param);
    }
}
