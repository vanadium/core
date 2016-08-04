// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google.namespace;

import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableMap;

import io.v.impl.google.services.mounttable.MountTableServer;
import io.v.v23.V;
import io.v.v23.V23TestUtil;
import io.v.v23.context.VContext;
import io.v.v23.namespace.Namespace;
import io.v.v23.rpc.ListenSpec;
import io.v.v23.security.BlessingPattern;
import io.v.v23.security.access.AccessList;
import io.v.v23.security.access.Constants;
import io.v.v23.security.access.Permissions;

/**
 * Various test utilities for namespaces.
 */
public class NamespaceTestUtil {
    /**
     * Starts a new test mount server and attaches it to a new context derived from the
     * provided context.
     */
    public static VContext withTestMountServer(VContext ctx) throws Exception {
        ctx = V.withListenSpec(ctx, V.getListenSpec(ctx).withAddress(
                new ListenSpec.Address("tcp", "localhost:0")));
        AccessList acl = new AccessList(
                ImmutableList.of(new BlessingPattern("...")), ImmutableList.<String>of());
        Permissions allowAll = new Permissions(ImmutableMap.of(
                Constants.READ.getValue(), acl,
                Constants.WRITE.getValue(), acl,
                Constants.ADMIN.getValue(), acl));
        ctx = MountTableServer.withNewServer(ctx, new MountTableServer.Params()
                .withPermissions(ImmutableMap.of("test", allowAll))
                .withStatsPrefix("test"));
        Namespace n = V.getNamespace(ctx);
        n.setRoots(ImmutableList.of(V23TestUtil.getServerEndpoint(ctx).name()));
        return ctx;
    }
}
