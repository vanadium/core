// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google.naming;

import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.Parameterized;

import java.util.Arrays;
import java.util.Collection;

import static com.google.common.truth.Truth.assertThat;

/**
 * Unit tests for {@link NamingUtil#joinAddressName}.
 */
@RunWith(Parameterized.class)
public class NamingUtilJoinAddressNameTest {
    private final String address;
    private final String name;
    private final String expected;

    @Parameterized.Parameters
    public static Collection<Object[]> data() {
        return Arrays.asList(new Object[][]{
                {"", "", ""},
                {"", "a", "a"},
                {"", "/a", "/a"},
                {"", "a", "a"},
                {"", "///a", "/a"},
                {"/", "", ""},
                {"//", "", ""},
                {"/a", "", "/a"},
                {"//a", "", "/a"},
                {"aaa", "", "/aaa"},
                {"/aaa", "aa", "/aaa/aa"},
                {"ab", "/cd", "/ab/cd"},
                {"/ab", "/cd", "/ab/cd"},
                {"ab", "//cd", "/ab/cd"},
        });
    }

    public NamingUtilJoinAddressNameTest(String address, String name, String expected) {
        this.address = address;
        this.name = name;
        this.expected = expected;
    }

    @Test
    public void testJoinAddressName() {
        assertThat(NamingUtil.joinAddressName(address, name)).isEqualTo(expected);
    }
}
