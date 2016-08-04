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
 * Unit tests for {@link NamingUtil#clean}.
 */
@RunWith(Parameterized.class)
public class NamingUtilCleanTest {
    private final String input;
    private final String expected;

    @Parameterized.Parameters
    public static Collection<Object[]> data() {
        return Arrays.asList(new Object[][]{
                {"", ""},
                {"//", "/"},
                {"/", "/"},
                {"/a//b", "/a/b"},
                {"/a//b/", "/a/b"},
                {"a//b", "a/b"},
                {"a//b/", "a/b"},
                {"///a//b/", "/a/b"},
                {"a////////b/", "a/b"},
        });
    }

    public NamingUtilCleanTest(String input, String expected) {
        this.input = input;
        this.expected = expected;
    }

    @Test
    public void testClean() {
        assertThat(NamingUtil.clean(input)).isEqualTo(expected);
    }
}
