// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23;

import io.v.v23.verror.CanceledException;
import org.joda.time.Duration;
import org.junit.After;
import org.junit.Before;
import org.junit.Test;

import java.util.concurrent.CountDownLatch;

import io.v.v23.context.VContext;
import io.v.v23.verror.NoServersException;
import io.v.v23.verror.TimeoutException;
import io.v.x.jni.test.fortune.FortuneClient;
import io.v.x.jni.test.fortune.FortuneClientFactory;
import io.v.x.jni.test.fortune.FortuneServer;
import io.v.x.jni.test.fortune.FortuneServerImpl;

import static io.v.v23.VFutures.sync;

/**
 * The {@link VFutures} class is responsible for wrapping Vanadium asyncs in managed
 * {@link com.google.common.util.concurrent.ListenableFuture}s. These tests ensure that any errors
 * raised by the V23 Go layer are surfaced to the client as reasonable exceptions.
 */
public class VFuturesTest {
    private VContext ctx;

    @Before
    public void setup() throws Exception {
        ctx = V.init();
    }

    @After
    public void teardown() {
        ctx.cancel();
    }

    /**
     * When the context times out and the name was resolvable but not a Vanadium server, we expect a
     * {@link NoServersException}.
     */
    @Test(expected = NoServersException.class)
    public void testNoServers() throws Exception {
        final FortuneClient client = FortuneClientFactory.getFortuneClient("/127.0.0.1");
        final VContext timeout = ctx.withTimeout(Duration.millis(100));
        sync(client.get(timeout));
    }

    /**
     * When the context times out while a Vanadium server is handling the request, we expect either
     * a {@link TimeoutException} or a {@link CanceledException}.
     */
    @Test
    public void testTimeout() throws Exception {
        // Make the server hang forever; never count down
        final CountDownLatch callLatch = new CountDownLatch(1);
        final FortuneServer server = new FortuneServerImpl(callLatch, null);
        final VContext serverContext = V.withNewServer(ctx, "", server, null);

        final FortuneClient client = FortuneClientFactory.getFortuneClient(
                "/" + V23TestUtil.getServerEndpoint(serverContext));

        final VContext timeout = ctx.withTimeout(Duration.millis(100));

        try {
            sync(client.get(timeout));
        } catch (TimeoutException | CanceledException e) {
            // OK
        }
    }
}
