// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.naming;

import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.Parameterized;

import java.util.Arrays;
import java.util.Collection;

import static com.google.common.truth.Truth.assertThat;

/**
 * Unit tests for {@link RoutingId#fromString} and {@link RoutingId#toString}.
 */
@RunWith(Parameterized.class)
public class RoutingIdTest {
    private final String input;

    @Parameterized.Parameters
    public static Collection<String> data() {
        return Arrays.asList(new String[]{
                "00000000000000000000000000000000",
                "0f000f20a010d07f20f0df70cf7fff0f",
        });
    }

    public RoutingIdTest(String input) {
        this.input = input;
    }

    @Test
    public void testString() {
        assertThat(RoutingId.fromString(input).toString()).isEqualTo(input);
    }
}
