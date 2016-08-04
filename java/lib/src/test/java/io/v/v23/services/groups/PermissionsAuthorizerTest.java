// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.services.groups;

import com.google.common.base.Function;
import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableMap;
import com.google.common.collect.ImmutableMultimap;
import com.google.common.collect.Lists;
import com.google.common.collect.Multimap;
import io.v.impl.google.naming.NamingUtil;
import io.v.impl.google.services.groups.GroupServer;
import io.v.v23.V;
import io.v.v23.VFutures;
import io.v.v23.context.VContext;
import io.v.v23.naming.Endpoint;
import io.v.v23.rpc.ListenSpec;
import io.v.v23.rpc.Server;
import io.v.v23.security.BlessingPattern;
import io.v.v23.security.BlessingRoots;
import io.v.v23.security.Blessings;
import io.v.v23.security.Call;
import io.v.v23.security.CallParams;
import io.v.v23.security.VPrincipal;
import io.v.v23.security.VSecurity;
import io.v.v23.security.VSigner;
import io.v.v23.security.access.Access;
import io.v.v23.security.access.AccessList;
import io.v.v23.security.access.Permissions;
import io.v.v23.vdl.VdlValue;
import io.v.v23.verror.VException;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.Parameterized;

import java.security.interfaces.ECPublicKey;
import java.util.Arrays;
import java.util.Collection;
import java.util.List;

import static com.google.common.truth.Truth.assertThat;
import static org.junit.Assert.fail;

/**
 * Tests the implementation of {@code PermissionsAuthorizer}.
 */
@RunWith(Parameterized.class)
public class PermissionsAuthorizerTest {
    private static final VContext CONTEXT;
    private static final Blessings SERVER_BLESSINGS;
    private static final VPrincipal SERVER_PRINCIPAL;
    private static final VPrincipal CLIENT_PRINCIPAL;
    private static final PermissionsAuthorizer AUTHORIZER;

    private static final Server GROUP_SERVER;

    static {
        try {
            CONTEXT = V.withListenSpec(V.init(), V.getListenSpec(V.init()).withAddress(
                    new ListenSpec.Address("tcp", "localhost:0")));
            AccessList acl = new AccessList(
                    ImmutableList.of(new BlessingPattern("...")), ImmutableList.<String>of());
            Permissions allowAll = new Permissions(ImmutableMap.of(
                    io.v.v23.security.access.Constants.RESOLVE.getValue(), acl,
                    io.v.v23.security.access.Constants.READ.getValue(), acl,
                    io.v.v23.security.access.Constants.WRITE.getValue(), acl,
                    io.v.v23.security.access.Constants.ADMIN.getValue(), acl));
            CLIENT_PRINCIPAL = newPrincipal();
            SERVER_PRINCIPAL = newPrincipal();
            SERVER_BLESSINGS = SERVER_PRINCIPAL.blessSelf("server");

            // Start group server.
            GROUP_SERVER = V.getServer(
                    GroupServer.withNewServer(CONTEXT, new GroupServer.Params()
                            .withStorageEngine(GroupServer.StorageEngine.MEMSTORE)));
            assertThat(GROUP_SERVER).isNotNull();
            assertThat(GROUP_SERVER.getStatus().getEndpoints()).isNotEmpty();
            Endpoint groupServerEndpoint = GROUP_SERVER.getStatus().getEndpoints()[0];
            String groupNameReaders = NamingUtil.join(groupServerEndpoint.name(), "readers");
            String groupNameWriters = NamingUtil.join(groupServerEndpoint.name(), "writers");

            // Populate the group server.
            {
                GroupClient client = GroupClientFactory.getGroupClient(groupNameReaders);
                VFutures.sync(client.create(CONTEXT, allowAll, ImmutableList.of(
                        new BlessingPatternChunk("root:alice"),
                        new BlessingPatternChunk("root:bob"))));
            }
            {
                GroupClient client = GroupClientFactory.getGroupClient(groupNameWriters);
                VFutures.sync(client.create(CONTEXT, allowAll, ImmutableList.of(
                        new BlessingPatternChunk("root:alice"))));
            }

            AUTHORIZER = PermissionsAuthorizer.create(new Permissions(ImmutableMap.of(
                    io.v.v23.security.access.Constants.READ.getValue(), new AccessList(
                            ImmutableList.of(new BlessingPattern("<grp:" + groupNameReaders + ">")),
                            null),
                    io.v.v23.security.access.Constants.WRITE.getValue(), new AccessList(
                            ImmutableList.of(new BlessingPattern("<grp:" + groupNameWriters + ">")),
                            null))), Access.typicalTagType());
        } catch (VException e) {
            throw new RuntimeException(e);
        } catch (GroupServer.StartException e) {
            throw new RuntimeException(e);
        }
    }

    private static VPrincipal newPrincipal() throws VException {
        VSigner signer = VSecurity.newInMemorySigner();
        return VSecurity.newPrincipal(signer, null, new TrustAllRoots());
    }

    private final String methodName;
    private final Blessings clientBlessings;
    private final boolean shouldAccept;
    private final VdlValue[] methodTags;

    @Parameterized.Parameters
    public static Collection<Object[]> data() {
        return Arrays.asList(new Object[][]{
                {"get", io.v.v23.security.access.Constants.READ,
                        ImmutableList.of("root:alice"), true},
                {"put", io.v.v23.security.access.Constants.WRITE,
                        ImmutableList.of("root:alice"), true},
                {"get", io.v.v23.security.access.Constants.READ,
                        ImmutableList.of("root:bob"), true},
                {"put", io.v.v23.security.access.Constants.WRITE,
                        ImmutableList.of("root:bob"), false}
        });
    }

    public PermissionsAuthorizerTest(String methodName, VdlValue methodTag,
                                     List<String> clientBlessingNames,
                                     boolean isAccepted) throws VException {
        this.methodName = methodName;
        this.methodTags = new VdlValue[]{ methodTag };
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
    public void testAuthorize() throws VException {
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
                throw e;
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
