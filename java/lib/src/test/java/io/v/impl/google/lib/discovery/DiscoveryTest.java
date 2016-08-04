// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google.lib.discovery;

import com.google.common.collect.ImmutableMap;
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
import io.v.v23.discovery.Attributes;
import io.v.v23.discovery.Attachments;
import io.v.v23.discovery.Update;
import io.v.v23.verror.VException;

import static com.google.common.truth.Truth.assertThat;
import static io.v.impl.google.lib.discovery.DiscoveryTestUtil.assertThat;

/**
 * Tests for {@link Discovery} implementation.
 */
public class DiscoveryTest extends TestCase {
    public void testBasicTest() throws VException {
        VContext ctx = V.init();
        FactoryUtil.injectMockPlugin(ctx);

        Discovery d1 = V.newDiscovery(ctx);

        Advertisement ad = new Advertisement();
        ad.setInterfaceName("v.io/v23/a");
        ad.setAddresses(Arrays.asList("/h1:123/x"));
        ad.setAttributes(new Attributes(ImmutableMap.of("a", "v")));
        ad.setAttachments(new Attachments(ImmutableMap.of("a", new byte[] {1, 2, 3})));

        VContext advCtx = ctx.withCancel();
        ListenableFuture<Void> advFuture = d1.advertise(advCtx, ad, null);

        Discovery d2 = V.newDiscovery(ctx);
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
        assertThat(update.getAdvertisement()).isEqualTo(ad);

        scanCtx.cancel();
        assertThat(it.hasNext()).isFalse();
    }
}
