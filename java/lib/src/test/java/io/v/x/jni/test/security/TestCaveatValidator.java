// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.x.jni.test.security;

import io.v.v23.context.VContext;
import io.v.v23.security.Call;
import io.v.v23.security.CaveatValidator;
import io.v.v23.verror.VException;

/**
 * Validator for {@code TestCaveat} that validates the context if its name suffix
 * matches the provided suffix.
 */
public class TestCaveatValidator implements CaveatValidator {
    @Override
    public void validate(VContext context, Call call, Object param) throws VException {
        if (param == null) param = "";
        if (!(param instanceof String)) {
            throw new VException(String.format(
                    "Caveat param %s of wrong type: want %s", param, String.class));
        }
        if (!param.equals(call.suffix())) {
            throw new VException(String.format("Got name %s, want %s", call.suffix(), param));
        }
    }
}
