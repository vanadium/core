// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google.naming;

import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.Parameterized;

import java.util.Arrays;
import java.util.Collection;
import java.util.List;

import static com.google.common.truth.Truth.assertThat;

/**
 * Unit tests for {@link NamingUtil#splitAddressName}.
 */
@RunWith(Parameterized.class)
public class NamingUtilSplitAddressNameTest {
    private final String name;
    private final String expectedAddress;
    private final String expectedRelativeName;

    @Parameterized.Parameters
    public static Collection<Object[]> data() {
        return Arrays.asList(new Object[][]{
                {"", "", ""},
                {"/", "", ""},
                {"//", "", ""},
                {"//abc@@host/foo", "abc@@host", "foo"},
                {"a", "", "a"},
                {"/a", "a", ""},
                {"/a/", "a", ""},
                {"a/b", "", "a/b"},
                {"/a/b", "a", "b"},
                {"abc@@/foo", "", "abc@@/foo"},
                {"/abc@@host/foo", "abc@@host", "foo"},
                {"/abc/foo", "abc", "foo"},
                {"/abc/foo//x", "abc", "foo/x"},
                {"/abc:20/foo", "abc:20", "foo"},
                {"/abc//foo/bar", "abc", "foo/bar"},
                {"/0abc:20/foo", "0abc:20", "foo"},
                {"/abc1.2:20/foo", "abc1.2:20", "foo"},
                {"/abc:xx/foo", "abc:xx", "foo"},
                {"/-abc/foo", "-abc", "foo"},
                {"/a.-abc/foo", "a.-abc", "foo"},
                {"/[01:02::]:444", "[01:02::]:444", ""},
                {"/[01:02::]:444/foo", "[01:02::]:444", "foo"},
                {"/12.3.4.5:444", "12.3.4.5:444", ""},
                {"/12.3.4.5:444/foo", "12.3.4.5:444", "foo"},
                {"/12.3.4.5", "12.3.4.5", ""},
                {"/12.3.4.5/foo", "12.3.4.5", "foo"},
                {"/12.3.4.5//foo", "12.3.4.5", "foo"},
                {"/12.3.4.5/foo//bar", "12.3.4.5", "foo/bar"},
                {"/user@domain.com@host:1234/foo/bar", "user@domain.com@host:1234", "foo/bar"},
                {"/(dev.v.io:services:mounttabled)@host:1234/foo/bar",
                        "(dev.v.io:services:mounttabled)@host:1234", "foo/bar"},
                {"/(dev.v.io:services:mounttabled)@host:1234/", "(dev.v.io:services:mounttabled)@host:1234", ""},
                {"/(dev.v.io:services:mounttabled)@host:1234", "(dev.v.io:services:mounttabled)@host:1234", ""},
                // malformed endpoint, doesn't end in a @@
                {"/@4@tcp@127.0.0.1:22@@@@s@dev.v.io", "@4@tcp@127.0.0.1:22@@@@s@dev.v.io", ""},
                {"/@4@tcp@127.0.0.1:22@@@@s@dev.v.io:services:mounttabled@@/foo/bar",
                        "@4@tcp@127.0.0.1:22@@@@s@dev.v.io:services:mounttabled@@", "foo/bar"},
                {"/@4@tcp@127.0.0.1:22@@@@s@dev.v.io:services:mounttabled,staging.v.io:services:" +
                        "nsroot@@/foo/bar", "@4@tcp@127.0.0.1:22@@@@s@dev.v.io:services:" +
                        "mounttabled,staging.v.io:services:nsroot@@", "foo/bar"},
                {"/@@@127.0.0.1:22@@@@/foo/bar", "@@@127.0.0.1:22@@@@", "foo/bar"},
                {"/@4@tcp@127.0.0.1:22@@@@s@dev.v.io:services:mounttabled,staging.v.io:services:" +
                        "nsroot@@", "@4@tcp@127.0.0.1:22@@@@s@dev.v.io:services:mounttabled" +
                        ",staging.v.io:services:nsroot@@", ""},
        });
    }

    public NamingUtilSplitAddressNameTest(String name, String addr, String relative) {
        this.name = name;
        this.expectedAddress = addr;
        this.expectedRelativeName = relative;
    }

    @Test
    public void testSplitAddressName() {
        List<String> result = NamingUtil.splitAddressName(this.name);
        assertThat(result).containsExactly(this.expectedAddress, this.expectedRelativeName);
    }
}
