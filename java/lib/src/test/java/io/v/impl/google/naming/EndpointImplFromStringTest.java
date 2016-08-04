// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google.naming;

import com.google.common.collect.ImmutableList;

import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.Parameterized;

import java.util.Arrays;
import java.util.Collection;

import io.v.v23.naming.RoutingId;

import static com.google.common.truth.Truth.assertThat;

/**
 * Unit tests for {@link EndpointImpl#fromString}.
 */
@RunWith(Parameterized.class)
public class EndpointImplFromStringTest {
    private final EndpointImpl expectedEndpoint;
    private final String input;

    @Parameterized.Parameters
    public static Collection<Object[]> data() {
        return Arrays.asList(new Object[][]{
                {new EndpointImpl("", "batman.com:1234", ImmutableList.<String>of(),
                        RoutingId.NULL_ROUTING_ID, ImmutableList.<String>of(), true, false),
                        "batman.com:1234"},
                {new EndpointImpl("", "batman.com:1234", Arrays.asList("foo", "bar"),
                        RoutingId.fixedRoutingId(0xdabbad00L), ImmutableList.<String>of(),
                        true, false),
                        "@6@@batman.com:1234@foo,bar@000000000000000000000000dabbad00@m@@@"},
        });
    }

    public EndpointImplFromStringTest(EndpointImpl expectedEndpoint, String input) {
        this.expectedEndpoint = expectedEndpoint;
        this.input = input;
    }

    @Test
    public void testFromString() {
        assertThat(EndpointImpl.fromString(input)).isEqualTo(expectedEndpoint);
    }
}
