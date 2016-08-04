// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google.namespace;

import com.google.common.base.Function;
import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableMap;
import com.google.common.collect.Iterables;
import com.google.common.collect.Lists;

import io.v.v23.InputChannel;
import io.v.v23.InputChannels;
import junit.framework.TestCase;

import org.joda.time.Duration;

import java.util.List;
import java.util.Map;

import io.v.v23.V;
import io.v.v23.V23TestUtil;
import io.v.v23.context.VContext;
import io.v.v23.namespace.Namespace;
import io.v.v23.naming.Endpoint;
import io.v.v23.naming.GlobReply;
import io.v.v23.naming.MountEntry;
import io.v.v23.security.BlessingPattern;
import io.v.v23.security.access.AccessList;
import io.v.v23.security.access.Permissions;
import io.v.v23.verror.CanceledException;
import io.v.v23.verror.VException;

import static com.google.common.truth.Truth.assertThat;
import static io.v.v23.VFutures.sync;

/**
 * Test the {@link NamespaceImpl} implementation.
 */
public class NamespaceTest extends TestCase {
    private VContext ctx;
    private VContext dummyServerCtx;
    private Endpoint mountTableEndpoint;
    private Endpoint dummyServerEndpoint;

    @Override
    protected void setUp() throws Exception {
        ctx = V.init();
        dummyServerCtx = V23TestUtil.withDummyServer(ctx);
        dummyServerEndpoint = V23TestUtil.getServerEndpoint(dummyServerCtx);
        ctx = NamespaceTestUtil.withTestMountServer(ctx);
        mountTableEndpoint = V23TestUtil.getServerEndpoint(ctx);
    }

    @Override
    protected void tearDown() throws Exception {
        ctx.cancel();
    }

    private Iterable<String> globNames(InputChannel<GlobReply> globReplies) {
        return Iterables.transform(InputChannels.asIterable(globReplies),
                new Function<GlobReply, String>() {
            @Override
            public String apply(GlobReply reply) {
                if (reply instanceof GlobReply.Entry) {
                    return ((GlobReply.Entry) reply).getElem().getName();
                } else {
                    return ((GlobReply.Error) reply).getElem().getError().getMessage();
                }
            }
        });
    }

    public void testMountAndUnmount() throws Exception {
        Namespace n = V.getNamespace(ctx);
        sync(n.mount(ctx, "test/test", dummyServerEndpoint.name(), Duration.standardDays(1)));
        assertThat(globNames(n.glob(ctx, "test/*"))).containsExactly("test/test");
        sync(n.unmount(ctx, "test/test", ""));
        assertThat(globNames(n.glob(ctx, "test/*"))).isEmpty();
    }

    public void testDelete() throws Exception {
        Namespace n = V.getNamespace(ctx);
        sync(n.mount(ctx, "test/test/test", dummyServerEndpoint.name(), Duration.standardDays(1)));
        sync(n.mount(ctx, "test/test/test2", dummyServerEndpoint.name(), Duration.standardDays(1)));
        assertThat(globNames(n.glob(ctx, "test/*/*"))).containsExactly(
                "test/test/test", "test/test/test2");
        // Shouldn't delete anything since the dir contains children.
        try {
            sync(n.delete(ctx, "test/test", false));
            fail("Namespace.delete() should have failed because a directory has children");
        } catch (VException e) {
            // OK
        }
        assertThat(globNames(n.glob(ctx, "test/*/*"))).containsExactly(
                "test/test/test", "test/test/test2");
        sync(n.delete(ctx, "test/test", true));
        assertThat(globNames(n.glob(ctx, "test/*"))).isEmpty();
    }

    public void testGlob() throws Exception {
        Namespace n = V.getNamespace(ctx);
        sync(n.mount(ctx, "test/test", dummyServerEndpoint.name(), Duration.standardDays(1)));
        List<GlobReply> result = Lists.newArrayList(
                InputChannels.asIterable(n.glob(ctx, "test/*")));
        assertThat(result).hasSize(1);
        assertThat(result.get(0).getElem()).isInstanceOf(MountEntry.class);
        assertThat(((MountEntry) (result.get(0).getElem())).getName()).isEqualTo("test/test");
    }

    public void testGlobWithContextCancel() throws Exception {
        Namespace n = V.getNamespace(ctx);
        sync(n.mount(ctx, "test/test", dummyServerEndpoint.name(), Duration.standardDays(1)));

        VContext ctxC = ctx.withCancel();
        InputChannel<GlobReply> channel = n.glob(ctxC, "test/*");
        ctxC.cancel();
        try {
            sync(channel.recv());
            fail("Expected a cancellation exception");
        } catch (CanceledException e) {
            // OK
        }
    }

    public void testResolve() throws Exception {
        Namespace n = V.getNamespace(ctx);
        sync(n.mount(ctx, "test/test", dummyServerEndpoint.name(), Duration.standardDays(1)));
        MountEntry entry = sync(n.resolve(ctx, "test/test"));
        assertThat(entry).isNotNull();
        assertThat(entry.getServers()).isNotNull();
        assertThat(entry.getServers()).hasSize(1);
        assertThat(entry.getServers().get(0).getServer()).isEqualTo(dummyServerEndpoint.name());
    }

    public void testResolveToMountTable() throws Exception {
        Namespace n = V.getNamespace(ctx);
        sync(n.mount(ctx, "test/test", dummyServerEndpoint.name(), Duration.standardDays(1)));
        MountEntry entry = sync(n.resolveToMountTable(ctx, "test/test"));
        assertThat(entry).isNotNull();
        assertThat(entry.getServers()).isNotNull();
        assertThat(entry.getServers()).hasSize(1);
        assertThat(entry.getServers().get(0).getServer()).isEqualTo(mountTableEndpoint.name());
    }

    public void testPermissions() throws Exception {
        AccessList acl = new AccessList(ImmutableList.of(new BlessingPattern("...")),
                ImmutableList.<String>of());
        Namespace n = V.getNamespace(ctx);
        sync(n.mount(ctx, "test/test", dummyServerEndpoint.name(), Duration.standardDays(1)));
        sync(n.setPermissions(ctx, "test/test", new Permissions(ImmutableMap.of("1", acl)), "1"));
        Map<String, Permissions> permissions = sync(n.getPermissions(ctx, "test/test"));
        assertThat(permissions).isNotNull();
        assertThat(permissions).hasSize(1);
        // TODO(sjr): figure out what is actually in this map
        assertThat(permissions).containsKey("2");
    }
}
