// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import static com.google.common.truth.Truth.assertThat;

import junit.framework.TestCase;

import org.joda.time.DateTime;

import io.v.v23.V;
import io.v.v23.context.VContext;
import io.v.v23.verror.VException;
import io.v.x.jni.test.security.TestCaveatValidator;

import java.util.Arrays;

/**
 * Tests the various caveat implementations.
 */
public class CaveatTest extends TestCase {
    public void testMethodCaveat() throws VException {
        VContext context = V.init();
        VPrincipal p1 = VSecurity.newPrincipal();
        Blessings alice = p1.blessSelf("alice", VSecurity.newMethodCaveat("Succeed"));
        VSecurity.addToRoots(p1, alice);
        {
            Call call = VSecurity.newCall(
                    new CallParams().withLocalPrincipal(p1).withRemoteBlessings(alice).withMethod("succeed"));
            String[] result = VSecurity.getRemoteBlessingNames(context, call);
            assertThat(Arrays.asList(result)).containsExactly("alice");
        }
        {
            Call call = VSecurity.newCall(
                    new CallParams().withLocalPrincipal(p1).withMethod("Fail"));
            String[] result = VSecurity.getRemoteBlessingNames(context, call);
            assertThat(result != null).isTrue();
            assertThat(Arrays.asList(result)).containsExactly();
        }
    }

    public void testExpiryCaveat() throws VException {
        VContext context = V.init();
        VPrincipal p1 = VSecurity.newPrincipal();
        Blessings alice = p1.blessSelf(
            "alice", VSecurity.newExpiryCaveat(DateTime.now().plusHours(1)));
        VSecurity.addToRoots(p1, alice);
        {
            Call call = VSecurity.newCall(new CallParams()
                    .withLocalPrincipal(p1)
                    .withRemoteBlessings(alice)
                    .withTimestamp(DateTime.now()));

            String[] result = VSecurity.getRemoteBlessingNames(context, call);
            assertThat(Arrays.asList(result)).containsExactly("alice");
        }
        {
            Call call = VSecurity.newCall(new CallParams()
                    .withLocalPrincipal(p1)
                    .withTimestamp(DateTime.now().plusHours(2)));
            String[] result = VSecurity.getRemoteBlessingNames(context, call);
            assertThat(result != null).isTrue();
            assertThat(Arrays.asList(result)).containsExactly();
        }
    }

    public void testCustomCaveat() throws VException {
        VContext context = V.init();
        CaveatRegistry.register(io.v.x.jni.test.security.Constants.TEST_CAVEAT,
                new TestCaveatValidator());
        VPrincipal p1 = VSecurity.newPrincipal();
        Blessings alice = p1.blessSelf("alice",
                VSecurity.newCaveat(io.v.x.jni.test.security.Constants.TEST_CAVEAT, "succeed"));
        VSecurity.addToRoots(p1, alice);
        {
            Call call = VSecurity.newCall(new CallParams()
                    .withLocalPrincipal(p1)
                    .withRemoteBlessings(alice)
                    .withSuffix("succeed"));
            String[] result = VSecurity.getRemoteBlessingNames(context, call);
            assertThat(Arrays.asList(result)).containsExactly("alice");
        }
        {
            Call call = VSecurity.newCall(new CallParams()
                    .withLocalPrincipal(p1)
                    .withRemoteBlessings(alice)
                    .withSuffix("fail"));
            String[] result = VSecurity.getRemoteBlessingNames(context, call);
            assertThat(result != null).isTrue();
            assertThat(Arrays.asList(result)).containsExactly();
        }
    }
}
