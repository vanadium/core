// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import junit.framework.TestCase;

import org.joda.time.DateTime;

import com.google.common.collect.ImmutableList;
import static com.google.common.truth.Truth.assertThat;

import io.v.v23.V;
import io.v.v23.vom.VomUtil;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.ObjectInputStream;
import java.io.ObjectOutputStream;
import java.util.Arrays;
import java.security.interfaces.ECPublicKey;
import java.util.List;

/**
 * Tests the default Blessings implementation.
 */
public class BlessingsTest extends TestCase {
    public void testPublicKey() throws Exception {
        V.init();
        VPrincipal p1 = VSecurity.newPrincipal();
        VPrincipal p2 = VSecurity.newPrincipal();
        Blessings alice = p1.blessSelf("alice");
        assertTrue(Arrays.equals(p1.publicKey().getEncoded(), alice.publicKey().getEncoded()));
        VSecurity.addToRoots(p2, alice);

        Blessings aliceWorkFriend = p1.bless(p2.publicKey(),
                alice, "work:friend", VSecurity.newUnconstrainedUseCaveat());
        if (!Arrays.equals(
                aliceWorkFriend.publicKey().getEncoded(), p2.publicKey().getEncoded())) {
            fail(String.format("Expected public key: %s, got %s",
                    aliceWorkFriend.publicKey().getEncoded(), p2.publicKey().getEncoded()));
        }
    }

    public void testVomEncodeDecode() throws Exception {
        V.init();
        VPrincipal p = VSecurity.newPrincipal();
        Blessings alice = p.blessSelf("alice");
        byte[] data = VomUtil.encode(alice, Blessings.class);
        Blessings aliceCopy = (Blessings) VomUtil.decode(data, Blessings.class);
        if (!alice.equals(aliceCopy)) {
            fail(String.format("Blessings don't match, want %s, got %s", alice, aliceCopy));
        }
    }

    public void testSerialization() throws Exception {
        V.init();
        VPrincipal p = VSecurity.newPrincipal();
        Blessings blessings = p.blessSelf("alice");

        // Write
        ByteArrayOutputStream data = new ByteArrayOutputStream();
        ObjectOutputStream out = new ObjectOutputStream(data);
        out.writeObject(blessings);
        out.close();

        // Read
        final ObjectInputStream in =
                new ObjectInputStream(new ByteArrayInputStream(data.toByteArray()));

        // Verify
        final Object copy = in.readObject();
        assertThat(copy).isEqualTo(blessings);
        assertThat(copy.hashCode()).isEqualTo(copy.hashCode());
    }

    public void testSigningBlessings() throws Exception {
        V.init();
        VPrincipal p = VSecurity.newPrincipal();
        ECPublicKey pk = p.publicKey();
        List<Caveat> signingCaveats = ImmutableList.of(VSecurity.newExpiryCaveat(DateTime.now()),
                VSecurity.newExpiryCaveat(DateTime.now()));
        List<Caveat> nonSigningCaveats = ImmutableList.of(VSecurity.newMethodCaveat("MethodName"),
                VSecurity.newExpiryCaveat(DateTime.now()));

        Blessings b1 = p.blessSelf("alice");
        Blessings b2 = p.blessSelf("alice");
        Blessings signing = p.bless(pk, b1, "signing", signingCaveats.get(0),
            signingCaveats.subList(1, signingCaveats.size()).toArray(new Caveat[0]));
        Blessings notSigning = p.bless(pk, b2, "notSigning", nonSigningCaveats.get(0),
            nonSigningCaveats.subList(1, nonSigningCaveats.size()).toArray(new Caveat[0]));
        Blessings union = VSecurity.unionOfBlessings(new Blessings[]{signing, notSigning});

        assertThat(union.signingBlessings().getCertificateChains().size()).isEqualTo(1);
    }
}
