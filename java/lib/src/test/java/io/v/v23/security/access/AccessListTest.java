// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security.access;

import static com.google.common.truth.Truth.assertThat;

import com.google.common.collect.ImmutableList;

import junit.framework.TestCase;

import io.v.v23.V;
import io.v.v23.security.BlessingPattern;
import io.v.v23.security.Constants;

/**
 * Tests the default {@code AccessList} implementation.
 */
public class AccessListTest extends TestCase {
    public void testIncludes() throws Exception {
        V.init();
        {
            AccessList acl = new AccessList(ImmutableList.of(Constants.ALL_PRINCIPALS), null);
            assertThat(acl.includes("google")).isTrue();
        }
        {
            AccessList acl = new AccessList(
                    ImmutableList.of(
                            (new BlessingPattern("google")).makeNonExtendable(),
                            new BlessingPattern("google:alice")),
                    null);
            assertThat(acl.includes("google", "google:alice")).isTrue();
            assertThat(acl.includes("google", "google:bob")).isTrue();
            assertThat(acl.includes("google:bob")).isFalse();
            assertThat(acl.includes("batmap")).isFalse();
        }
        {
            AccessList acl = new AccessList(
                    ImmutableList.of(
                            (new BlessingPattern("google")).makeNonExtendable(),
                            new BlessingPattern("google:alice")),
                    ImmutableList.of("google:alice:home"));

            assertThat(acl.includes("google")).isTrue();
            assertThat(acl.includes("google:alice:work")).isTrue();
            assertThat(acl.includes("google:alice:home")).isFalse();
            assertThat(acl.includes("google:alice:home:wifi")).isFalse();
        }
    }
}
