// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import io.v.v23.context.VContext;
import io.v.v23.verror.VException;

import java.util.ArrayList;
import java.util.List;

/**
 * Validator for {@link Constants#METHOD_CAVEAT} caveat, which represents a caveat that validates
 * iff the method being invoked is included in the caveat-specified list.
 */
public class MethodCaveatValidator implements CaveatValidator {
    /**
     * A singleton instance of {@link MethodCaveatValidator}.
     */
    public static final MethodCaveatValidator INSTANCE = new MethodCaveatValidator();

    @Override
    public void validate(VContext context, Call call, Object param) throws VException {
        if (param == null) {
            param = new ArrayList<String>();
        }
        if (!(param instanceof List<?>)) {
            throw new VException(String.format(
                    "Caveat param %s of wrong type: want List<?>", param));
        }
        List<?> methods = (List<?>) param;
        if (call.method().isEmpty() && methods.size() == 0) {
            return;
        }
        for (Object method : methods) {
            if (!(method instanceof String)) {
                throw new VException(String.format(
                        "Caveat param %s element %s of wrong type: want String", param, method));
            }
            if (call.method().equals(method)) {
                return;
            }
        }
        throw new VException(String.format(
            "MethodCaveat(%s) failed validation for method %s", param, call.method()));
    }

    private MethodCaveatValidator() {}
}
