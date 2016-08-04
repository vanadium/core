// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import junit.framework.TestCase;

import io.v.v23.V;

import static com.google.common.truth.Truth.assertThat;

/**
 * Tests the default {@code BlessingPattern} implementation.
 */
public class BlessingPatternTest extends TestCase {
    @Override
    protected void setUp() throws Exception {
        super.setUp();
        V.init();
    }

    public void testIsValid() throws Exception {
        assertThat((new BlessingPattern("google")).isValid()).isTrue();
        assertThat((new BlessingPattern("google:alice")).isValid()).isTrue();
        assertThat((new BlessingPattern("google::alice")).isValid()).isFalse();
    }

    public void testIsMatchedBy() throws Exception {
        BlessingPattern pattern = new BlessingPattern("google:alice");
        assertThat(pattern.isMatchedBy("google:alice")).isTrue();
        assertThat(pattern.isMatchedBy("google:alice:friends")).isTrue();
        assertThat(pattern.isMatchedBy("google:bob", "google:alice:coworkers")).isTrue();

        assertThat(pattern.isMatchedBy("google")).isFalse();
        assertThat(pattern.isMatchedBy("google:bob")).isFalse();
    }

    public void testMakeNonExtendable() throws Exception {
        BlessingPattern pattern = (new BlessingPattern("google:alice")).makeNonExtendable();
        assertThat(pattern.isMatchedBy("google:alice")).isTrue();
        assertThat(pattern.isMatchedBy("google")).isFalse();
        assertThat(pattern.isMatchedBy("google:alice:friends")).isFalse();
    }
}
