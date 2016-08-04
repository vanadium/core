// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security.access;

import com.google.common.base.Function;
import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableMap;
import com.google.common.collect.ImmutableMultimap;
import com.google.common.collect.Lists;
import com.google.common.collect.Multimap;
import io.v.v23.V;
import io.v.v23.context.VContext;
import io.v.v23.security.BlessingPattern;
import io.v.v23.security.BlessingRoots;
import io.v.v23.security.Blessings;
import io.v.v23.security.Call;
import io.v.v23.security.CallParams;
import io.v.v23.security.Constants;
import io.v.v23.security.VPrincipal;
import io.v.v23.security.VSecurity;
import io.v.v23.security.VSigner;
import io.v.v23.security.access.internal.MyObjectServerWrapper;
import io.v.v23.security.access.internal.MyTag;
import io.v.v23.vdl.VdlValue;
import io.v.v23.verror.VException;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.Parameterized;

import java.security.interfaces.ECPublicKey;
import java.util.Arrays;
import java.util.Collection;
import java.util.List;

import static org.junit.Assert.fail;


/**
 * Tests the implementation of {@code PermissionsAuthorizer}.
 */
@RunWith(Parameterized.class)
public class PermissionsAuthorizerTest {
    private static final VContext CONTEXT;
    private static final PermissionsAuthorizer AUTHORIZER;
    private static final VPrincipal CLIENT_PRINCIPAL;
    private static final VPrincipal SERVER_PRINCIPAL;
    private static final Blessings SERVER_BLESSINGS;

    static {
        CONTEXT = V.init();
        Permissions perms = new Permissions(ImmutableMap.of(
                "R", new AccessList(
                        ImmutableList.of(Constants.ALL_PRINCIPALS),
                        null),
                "W", new AccessList(
                        ImmutableList.of(
                                new BlessingPattern("ali:family"),
                                new BlessingPattern("bob"),
                                new BlessingPattern("che:$")),
                        ImmutableList.of(
                                "bob:acquaintances")),
                "X", new AccessList(
                        ImmutableList.of(
                                new BlessingPattern("ali:family:boss:$"),
                                new BlessingPattern("superman:$")),
                        null)
        ));
        try {
            AUTHORIZER = PermissionsAuthorizer.create(perms, MyTag.class);
            CLIENT_PRINCIPAL = newPrincipal();
            SERVER_PRINCIPAL = newPrincipal();
            SERVER_BLESSINGS = SERVER_PRINCIPAL.blessSelf("server");
        } catch (VException e) {
            throw new RuntimeException(e);
        }
    }

    private static VPrincipal newPrincipal() throws VException {
        VSigner signer = VSecurity.newInMemorySigner();
        return VSecurity.newPrincipal(signer, null, new TrustAllRoots());
    }

    private static VdlValue[] getMethodTags(String method) throws VException {
        MyObjectServerWrapper s = new MyObjectServerWrapper(null);
        return s.getMethodTags(method);
    }

    private final String methodName;
    private final Blessings clientBlessings;
    private final boolean shouldAccept;
    private final VdlValue[] methodTags;

    @Parameterized.Parameters
    public static Collection<Object[]> data() {
        return Arrays.asList(new Object[][]{
                {"get", ImmutableList.of(), true},
                {"get", ImmutableList.of("ali"), true},
                {"get", ImmutableList.of("bob:friend", "che:enemy"), true},
                {"put", ImmutableList.of("ali:family:mom"), true},
                {"put", ImmutableList.of("bob:friends"), true},
                // granted because of "che"
                {"put", ImmutableList.of("bob:acquantainces:carol", "che"), true},
                {"resolve", ImmutableList.of("superman"), true},
                {"resolve", ImmutableList.of("ali:family:boss"), true},

                {"put", ImmutableList.of("ali", "bob:acquaintances", "bob:acquaintances:dave",
                        "che:friend", "dave"), false},
                {"resolve", ImmutableList.of("ali", "ali:friend", "ali:family", "ali:family:friend",
                        "alice:family:boss:friend", "superman:friend"), false},
                // Since there are no tags on the noTags method, it has an
                // empty ACL.  No client will have access.
                {"noTags", ImmutableList.of(
                        "ali", "ali:family:boss", "bob", "che", "superman"), false}
        });
    }

    public PermissionsAuthorizerTest(String methodName, List<String> clientBlessingNames,
                                     boolean isAccepted) throws VException {
        this.methodName = methodName;
        this.methodTags = new MyObjectServerWrapper(null).getMethodTags(methodName);
        this.clientBlessings = VSecurity.unionOfBlessings(Lists.transform(
                clientBlessingNames, new Function<String, Blessings>() {
                    @Override
                    public Blessings apply(String name) {
                        try {
                            return CLIENT_PRINCIPAL.blessSelf(name);
                        } catch (VException e) {
                            throw new RuntimeException(e);
                        }
                    }
                }).toArray(new Blessings[0]));
        this.shouldAccept = isAccepted;
    }

    @Test
    public void testAuthorize() {
        Call call = VSecurity.newCall(new CallParams()
                .withLocalPrincipal(SERVER_PRINCIPAL)
                .withLocalBlessings(SERVER_BLESSINGS)
                .withRemoteBlessings(clientBlessings)
                .withMethod(methodName)
                .withMethodTags(methodTags));
        try {
            AUTHORIZER.authorize(CONTEXT, call);
            if (!shouldAccept) {
                fail(String.format(
                        "Access granted for method %s to %s", methodName, clientBlessings));
            }
        } catch (VException e) {
            if (shouldAccept) {
                fail(String.format(
                        "Access denied for method %s to %s", methodName, clientBlessings));
            }
        }
    }

    private static class TrustAllRoots implements BlessingRoots {
        @Override
        public void add(ECPublicKey root, BlessingPattern pattern) throws VException {}
        @Override
        public void recognized(ECPublicKey root, String blessing) throws VException {}
        @Override
        public String debugString() { return TrustAllRoots.class.toString(); }
        @Override
        public Multimap<BlessingPattern, ECPublicKey> dump() {
            return ImmutableMultimap.of();
        }
    }
}
