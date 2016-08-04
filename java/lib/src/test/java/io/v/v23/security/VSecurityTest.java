// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import junit.framework.TestCase;

import org.joda.time.DateTime;

import com.google.common.collect.ImmutableList;
import static com.google.common.truth.Truth.assertThat;

import io.v.v23.V;
import io.v.v23.context.VContext;
import io.v.v23.verror.VException;

import java.util.Arrays;
import java.security.interfaces.ECPublicKey;
import java.util.List;

/**
 * Tests for the {@link VSecurity} utility methods.
 */
public class VSecurityTest extends TestCase {
    public void testGetRemoteBlessingNames() throws Exception {
        VContext context = V.init();
        VPrincipal p1 = VSecurity.newPrincipal();
        VPrincipal p2 = VSecurity.newPrincipal();
        Blessings alice = p1.blessSelf("alice");
        VSecurity.addToRoots(p2, alice);

        Blessings aliceWorkFriend = p1.bless(p2.publicKey(),
                alice, "work:friend", VSecurity.newUnconstrainedUseCaveat());
        Call call = VSecurity.newCall(
                new CallParams().withRemoteBlessings(aliceWorkFriend).withLocalPrincipal(p2));
        String[] blessings = VSecurity.getRemoteBlessingNames(context, call);
        if (!Arrays.equals(new String[]{ "alice:work:friend" }, blessings)) {
            fail(String.format("Expected blessings [\"alice:work:friend\"], got %s",
                    Arrays.toString(blessings)));
        }
    }

    public void testGetLocalBlessingNames() throws Exception {
        VContext context = V.init();
        VPrincipal p1 = VSecurity.newPrincipal();
        VPrincipal p2 = VSecurity.newPrincipal();
        Blessings alice = p1.blessSelf("alice");
        VSecurity.addToRoots(p2, alice);

        Blessings aliceWorkFriend = p1.bless(p2.publicKey(),
                alice, "work:friend", VSecurity.newUnconstrainedUseCaveat());
        Call call = VSecurity.newCall(
                new CallParams().withLocalBlessings(aliceWorkFriend).withLocalPrincipal(p2));
        String[] blessings = VSecurity.getLocalBlessingNames(context, call);
        if (!Arrays.equals(new String[]{ "alice:work:friend" }, blessings)) {
            fail(String.format("Expected blessings [\"alice:work:friend\"], got %s",
                    Arrays.toString(blessings)));
        }
        blessings = VSecurity.getBlessingNames(p2, aliceWorkFriend);
        if (!Arrays.equals(new String[]{ "alice:work:friend" }, blessings)) {
            fail(String.format("Expected blessings [\"alice:work:friend\"], got %s",
                    Arrays.toString(blessings)));
        }
        blessings = VSecurity.getBlessingNames(p1, aliceWorkFriend);
        if (!Arrays.equals(new String[]{}, blessings)) {
            fail(String.format("Expected blessings [], got %s",
                    Arrays.toString(blessings)));
        }
    }

    public void testSigning() throws Exception {
        VSigner signer = VSecurity.newInMemorySigner();
        byte[] purpose = (new String("test")).getBytes();
        byte[] msg = (new String("this is a signing test message")).getBytes();
        VSignature signature = signer.sign(purpose, msg);
        try {
            VSecurity.verifySignature(signature, signer.publicKey(), msg);
        } catch (VException e) {
            fail(String.format("Couldn't verify signature: %s", e.getMessage()));
        }
    }

    public void testGetSigningBlessingNames() throws VException {
        VContext context = V.init();
        VPrincipal p = VSecurity.newPrincipal();
        ECPublicKey pk = p.publicKey();
        List<Caveat> passingCaveats = ImmutableList.of(
                VSecurity.newExpiryCaveat(DateTime.now().plusDays(1)),
                VSecurity.newExpiryCaveat(DateTime.now().plusYears(1)));
        List<Caveat> failingCaveats = ImmutableList.of(
                VSecurity.newMethodCaveat("MethodName"),
                VSecurity.newExpiryCaveat(DateTime.now()));

        Blessings b1 = p.blessSelf("alice");
        Blessings b2 = p.blessSelf("alice");
        Blessings passing = p.bless(pk, b1, "passing", passingCaveats.get(0),
            passingCaveats.subList(1, passingCaveats.size()).toArray(new Caveat[0]));
        Blessings failing = p.bless(pk, b2, "failing", failingCaveats.get(0),
            failingCaveats.subList(1, failingCaveats.size()).toArray(new Caveat[0]));
        Blessings union = VSecurity.unionOfBlessings(new Blessings[]{passing, failing});
        VSecurity.addToRoots(p, passing);

        String[] signingBlessingNames = VSecurity.getSigningBlessingNames(context, p, union);
        assertThat(Arrays.asList(signingBlessingNames)).containsExactly("alice:passing");
    }
}
