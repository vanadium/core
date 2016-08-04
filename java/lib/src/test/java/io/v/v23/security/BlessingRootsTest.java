// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import com.google.common.collect.ImmutableMap;

import com.google.common.collect.Multimap;
import junit.framework.TestCase;

import io.v.v23.V;
import io.v.v23.verror.VException;

import java.security.KeyPairGenerator;
import java.security.NoSuchAlgorithmException;
import java.security.interfaces.ECPublicKey;
import java.util.Map;

import static com.google.common.truth.Truth.assertThat;

/**
 * Tests the default {@code BlessingRoot} implementation.
 */
public class BlessingRootsTest extends TestCase {
    public void testRecognized() throws VException {
        V.init();
        VPrincipal principal = VSecurity.newPrincipal();
        BlessingRoots roots = principal.roots();
        ECPublicKey[] keys = { mintPublicKey(), mintPublicKey(), mintPublicKey() };
        roots.add(keys[0], new BlessingPattern("vanadium"));
        roots.add(keys[1], new BlessingPattern("google:foo"));
        roots.add(keys[0], new BlessingPattern("google:$"));

        Map<ECPublicKey, String[]> recognized =
                ImmutableMap.<ECPublicKey, String[]>builder()
                .put(keys[0], new String[]{
                        "vanadium", "vanadium:foo", "vanadium:foo:bar", "google" })
                .put(keys[1], new String[]{ "google:foo", "google:foo:bar" })
                .put(keys[2], new String[]{ })
                .build();
        Map<ECPublicKey, String[]> notRecognized =
                ImmutableMap.<ECPublicKey, String[]>builder()
                .put(keys[0], new String[]{ "google:foo", "foo", "foo:bar" })
                .put(keys[1], new String[]{
                        "google", "google:bar", "vanadium", "vanadium:foo", "foo", "foo:bar" })
                .put(keys[2], new String[] { "vanadium", "vanadium:foo", "vanadium:bar", "google",
                        "google:foo", "google:bar", "foo", "foo:bar" })
                .build();
        for (Map.Entry<ECPublicKey, String[]> entry : recognized.entrySet()) {
            ECPublicKey key = entry.getKey();
            for (String blessing : entry.getValue()) {
                try {
                    roots.recognized(key, blessing);
                } catch (VException e) {
                    fail("Didn't recognize root: " + entry.getKey() +
                            " as an authority for blessing: " + blessing);
                }
            }
        }
        for (Map.Entry<ECPublicKey, String[]> entry : notRecognized.entrySet()) {
            ECPublicKey key = entry.getKey();
            for (String blessing : entry.getValue()) {
                try {
                    roots.recognized(key, blessing);
                    fail("Shouldn't recognize root: " + entry.getKey() +
                            " as an authority for blessing: " + blessing);
                } catch (VException e) {
                    // OK
                }
            }
        }
    }

    public void testDump() throws VException {
        V.init();
        VPrincipal principal = VSecurity.newPrincipal();
        BlessingRoots roots = principal.roots();
        ECPublicKey[] keys = { mintPublicKey(), mintPublicKey(), mintPublicKey(), mintPublicKey() };
        roots.add(keys[0], new BlessingPattern("vanadium"));
        roots.add(keys[1], new BlessingPattern("google:foo"));
        roots.add(keys[0], new BlessingPattern("google:$"));
        roots.add(keys[3], new BlessingPattern("google:$"));

        Multimap<BlessingPattern, ECPublicKey> map = roots.dump();
        assertThat(map).hasSize(keys.length);
        assertThat((Iterable<ECPublicKey>) map.get(new BlessingPattern("google:$"))).containsAllOf(keys[0], keys[3]);
        assertThat((Iterable<ECPublicKey>) map.get(new BlessingPattern("google:foo"))).containsExactly(keys[1]);
    }

    private static ECPublicKey mintPublicKey() throws VException {
        try {
            KeyPairGenerator keyGen = KeyPairGenerator.getInstance("EC");
            keyGen.initialize(256);
            return (ECPublicKey) keyGen.generateKeyPair().getPublic();
        } catch (NoSuchAlgorithmException e) {
            throw new VException("Couldn't mint private key: " + e.getMessage());
        }
    }
}
