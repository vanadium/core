// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google.lib.discovery;

import com.google.common.util.concurrent.ListenableFuture;

import java.util.Arrays;
import java.util.Iterator;

import junit.framework.TestCase;

import io.v.v23.InputChannel;
import io.v.v23.InputChannels;
import io.v.v23.V;
import io.v.v23.VFutures;
import io.v.v23.context.VContext;
import io.v.v23.discovery.Discovery;
import io.v.v23.discovery.Advertisement;
import io.v.v23.discovery.Update;

import io.v.impl.google.namespace.NamespaceTestUtil;

import org.joda.time.Duration;

import static com.google.common.truth.Truth.assertThat;
import static io.v.impl.google.lib.discovery.DiscoveryTestUtil.assertThat;

/**
 * Tests for {@link GlobalDiscovery} implementation.
 */
public class GlobalDiscoveryTest extends TestCase {
    private static final String testPath = "a/b/c";

    public void testBasicTest() throws Exception {
        VContext ctx = V.init();
        ctx = NamespaceTestUtil.withTestMountServer(ctx);

        Discovery d1 = GlobalDiscovery.newDiscovery(ctx, testPath);

        Advertisement ad = new Advertisement();
        ad.setInterfaceName("foo/bar/baz");
        ad.setAddresses(Arrays.asList("/h1:123/x"));

        VContext advCtx = ctx.withCancel();
        ListenableFuture<Void> advFuture = d1.advertise(advCtx, ad, null);

        Discovery d2 =
                GlobalDiscovery.newDiscovery(ctx, testPath, Duration.ZERO, Duration.millis(1));
        VContext scanCtx = ctx.withCancel();

        InputChannel<Update> updateCh = d2.scan(scanCtx, "");
        Iterator<Update> it = InputChannels.asIterable(updateCh).iterator();

        assertThat(it.hasNext()).isTrue();
        Update update = it.next();
        assertThat(update.isLost()).isFalse();
        assertThat(update).isEqualTo(ctx, ad);

        advCtx.cancel();
        VFutures.sync(advFuture);

        assertThat(it.hasNext()).isTrue();
        update = it.next();
        assertThat(update.isLost()).isTrue();
        assertThat(update).isEqualTo(ctx, ad);

        scanCtx.cancel();
        assertThat(it.hasNext()).isFalse();
    }
}
