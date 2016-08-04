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
 * Unit tests for {@link NamingUtil#trimSuffix}.
 */
@RunWith(Parameterized.class)
public class NamingUtilTrimSuffixTest {
    private final String name;
    private final String suffix;
    private final String expected;

    @Parameterized.Parameters
    public static Collection<Object[]> data() {
        return Arrays.asList(new Object[][]{
                {"", "", ""},
                {"a", "", "a"},
                {"a", "a", ""},
                {"/a", "a", "/a"},
                {"a/b", "b", "a"},
                {"a/b", "/b", "a/b"},
                {"a/b/", "b/", "a"},
                {"/a/b", "b", "/a"},
                {"/a/b/c", "c", "/a/b"},
                {"/a/b/c/d", "c/d", "/a/b"},
                {"/a/b//c/d", "c/d", "/a/b"},
                {"/a/b//c/d", "/c/d", "/a/b/c/d"},
                {"/a/b//c/d", "//c/d", "/a/b/c/d"},
                {"//a/b", "//a/b", ""},
                {"/a/b", "/a/b", ""},
                {"//a", "a", "/a"},
        });
    }

    public NamingUtilTrimSuffixTest(String name, String suffix, String expected) {
        this.name = name;
        this.suffix = suffix;
        this.expected = expected;
    }

    @Test
    public void testTrimSuffix() {
        assertThat(NamingUtil.trimSuffix(this.name, this.suffix)).isEqualTo(this.expected);
    }
}
