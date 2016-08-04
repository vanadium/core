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
 * Unit tests for {@link NamingUtil#join}.
 */
@RunWith(Parameterized.class)
public class NamingUtilJoinTest {
    private final String[] names;
    private final String expected;

    @Parameterized.Parameters
    public static Collection<Object[]> data() {
        return Arrays.asList(new Object[][]{
                {new String[]{}, ""},
                {new String[]{""}, ""},
                {new String[]{"", ""}, ""},
                {new String[]{"a"}, "a"},
                {new String[]{"a", ""}, "a"},
                {new String[]{"a/"}, "a"},
                {new String[]{"a/", ""}, "a"},
                {new String[]{"a", "/"}, "a"},
                {new String[]{"", "a"}, "a"},
                {new String[]{"", "/a"}, "/a"},
                {new String[]{"a", "b"}, "a/b"},
                {new String[]{"a/", "b/"}, "a/b"},
                {new String[]{"a/", "/b"}, "a/b"},
                {new String[]{"/a", "b"}, "/a/b"},
                {new String[]{"a", "/", "b"}, "a/b"},
                {new String[]{"a", "/", "/b"}, "a/b"},
                {new String[]{"a/", "/", "/b"}, "a/b"},
                {new String[]{"/a/b", "c"}, "/a/b/c"},
                {new String[]{"/a", "b", "c"}, "/a/b/c"},
                {new String[]{"/a/", "/b/", "/c/"}, "/a/b/c"},
                {new String[]{"a", "b", "c"}, "a/b/c"},
                {new String[]{"a", "", "c"}, "a/c"},
                {new String[]{"a", "", "", "c"}, "a/c"},
                {new String[]{"/a/b", "c/d"}, "/a/b/c/d"},
                {new String[]{"/a/b", "/c/d"}, "/a/b/c/d"},
                {new String[]{"/a/b", "//c/d"}, "/a/b/c/d"},
                {new String[]{"/a//", "c"}, "/a/c"},
                {new String[]{"/a", "//"}, "/a"},
                {new String[]{"", "//a/b"}, "/a/b"},
                {new String[]{"a", "b//"}, "a/b"},
                {new String[]{"a", "//", "b"}, "a/b"},
                {new String[]{"a", "//", "/b"}, "a/b"},
                {new String[]{"a", "//", "//b"}, "a/b"},
                {new String[]{"a/", "//", "b"}, "a/b"},
                {new String[]{"a//", "//", "b"}, "a/b"},
                {new String[]{"a//", "//", "//b"}, "a/b"},
                {new String[]{"a", "/", "/", "b"}, "a/b"},
                {new String[]{"a/", "/", "/", "/b"}, "a/b"},
                {new String[]{"a", "//", "//", "b"}, "a/b"},
                {new String[]{"a//", "//", "//", "//b"}, "a/b"},
                {new String[]{"a//", "//b//", "//c//"}, "a/b/c"},
                {new String[]{"a//", "", "//c//"}, "a/c"},
                {new String[]{"a///", "////b"}, "a/b"},
                {new String[]{"////a", "b"}, "/a/b"},
                {new String[]{"a", "b////"}, "a/b"},
                {new String[]{"/ep//", ""}, "/ep"},
                {new String[]{"/ep//", "a"}, "/ep/a"},
                {new String[]{"/ep//", "//a"}, "/ep/a"},
        });
    }

    public NamingUtilJoinTest(String[] names, String expected) {
        this.names = names;
        this.expected = expected;
    }

    @Test
    public void testJoin() {
        assertThat(NamingUtil.join(this.names)).isEqualTo(this.expected);
    }
}
