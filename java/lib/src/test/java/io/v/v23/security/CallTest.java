// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import junit.framework.TestCase;

import org.joda.time.DateTime;
import org.junit.Assert;

import io.v.v23.V;
import io.v.v23.context.VContext;
import io.v.v23.vdl.VdlString;
import io.v.v23.vdl.VdlUint32;
import io.v.v23.vdl.VdlValue;
import io.v.v23.verror.VException;

/**
 * Tests the VContext implementations.
 */
public class CallTest extends TestCase {
    public void testContextParams() throws VException {
        VContext ctx = V.init();
        DateTime timestamp = new DateTime();
        String method = "bono";
        VdlValue[] methodTags = { new VdlUint32(12), new VdlString("edge") };
        String suffix = "larry";
        VPrincipal principal = VSecurity.newPrincipal();
        Blessings localBlessings = principal.blessSelf("adam");
        Blessings remoteBlessings = principal.blessSelf("u2");
        String localEndpoint = "@3@tcp@10.0.0.0:1000@";
        String remoteEndpoint = "@3@tcp@10.1.1.1:1111@";
        CallParams params = new CallParams()
                .withTimestamp(timestamp)
                .withMethod(method)
                .withMethodTags(methodTags)
                .withSuffix(suffix)
                .withLocalEndpoint(localEndpoint)
                .withRemoteEndpoint(remoteEndpoint)
                .withLocalPrincipal(principal)
                .withLocalBlessings(localBlessings)
                .withRemoteBlessings(remoteBlessings)
                .withContext(ctx);
        Call call = VSecurity.newCall(params);
        assertEquals(timestamp, call.timestamp());
        assertEquals(method, call.method());
        Assert.assertArrayEquals(methodTags, call.methodTags());
        assertEquals(suffix, call.suffix());
        assertEquals(principal, call.localPrincipal());
        assertEquals(localBlessings, call.localBlessings());
        assertEquals(remoteBlessings, call.remoteBlessings());
        assertEquals(localEndpoint, call.localEndpoint());
        assertEquals(remoteEndpoint, call.remoteEndpoint());
    }
}
