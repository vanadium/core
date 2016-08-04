// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import org.joda.time.DateTime;

import io.v.v23.context.VContext;
import io.v.v23.verror.VException;

/**
 * Validator for {@link Constants#EXPIRY_CAVEAT} caveat, which represents a caveat that validates
 * iff the current time is no later than the time specified in the caveat.
 */
public class ExpiryCaveatValidator implements CaveatValidator {
    /**
     * A singleton instance of {@link ExpiryCaveatValidator}.
     */
    public static final ExpiryCaveatValidator INSTANCE = new ExpiryCaveatValidator();

    @Override
    public void validate(VContext context, Call call, Object param) throws VException {
        if (param == null) param = new DateTime(0);
        if (!(param instanceof DateTime)) {
            throw new VException(String.format(
                    "Caveat param %s of wrong type: want %s", param, DateTime.class));
        }
        DateTime expiry = (DateTime) param;
        DateTime now = call.timestamp();
        if (now.isAfter(expiry)) {
            throw new VException(String.format(
                "ExpiryCaveat(%s) failed validation at %s", expiry, now));
        }
    }

    private ExpiryCaveatValidator() {}
}
