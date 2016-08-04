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
 * Unit tests for {@link NamingUtil#escape}, {@link NamingUtil#unescape},
 * {@link NamingUtil#encodeAsNameElement} and {@link NamingUtil#decodeFromNameElement(String)}
 */
@RunWith(Parameterized.class)
public class NamingUtilEscapingTest {
    private final String input;
    private final String encodedInput;

    @Parameterized.Parameters
    public static Collection<Object[]> data() {
        return Arrays.asList(new Object[][]{
            {"", ""},
            {"/", "%2F"},
            {"%", "%25"},
            {"/The % rain in /% Spain", "%2FThe %25 rain in %2F%25 Spain"},
            {"/%/%", "%2F%25%2F%25"},
            {"ᚸӲ읔+קAل", "ᚸӲ읔+קAل"},
            {"ᚸ/Ӳ%읔/ק%A+ل", "ᚸ%2FӲ%25읔%2Fק%25A+ل"},
        });
    }

    public NamingUtilEscapingTest(String input, String encodedInput) {
        this.input = input;
        this.encodedInput = encodedInput;
    }

    @Test
    public void testEncodingNameElements() {
        assertThat(NamingUtil.encodeAsNameElement(input)).isEqualTo(encodedInput);
    }

    @Test
    public void testDecodingNameElements() {
        assertThat(NamingUtil.decodeFromNameElement(encodedInput)).isEqualTo(input);
    }

    @Test
    public void testEscapingMultipleCharacters() {
        String input = "ᚸ/Ӳ%읔/ק%A+ل";
        // Escape multiple characters, also % in special should be ignored since
        // % is always escaped regardless.
        String escaped = NamingUtil.escape(input, new char[] {'/','읔','%'});
        assertThat(escaped).doesNotContain("읔");
        assertThat(escaped).contains("Ӳ");
        assertThat(NamingUtil.unescape(escaped)).isEqualTo(input);
    }
}
