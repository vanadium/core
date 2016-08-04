// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.context;

import junit.framework.TestCase;

import org.joda.time.DateTime;
import org.joda.time.Duration;

import io.v.v23.V;

import static com.google.common.truth.Truth.assertThat;
import static io.v.v23.VFutures.sync;

/**
 * Tests the VContext implementation.
 */
public class VContextTest extends TestCase {
    private VContext ctx;

    @Override
    protected void setUp() throws Exception {
        ctx = V.init();
    }

    @Override
    protected void tearDown() throws Exception {
        ctx.cancel();
    }

    public void testWithValue() {
        assertEquals(null, ctx.value("A"));
        VContext ctxA = ctx.withValue("A", 1);
        assertEquals(null, ctx.value("A"));
        assertEquals(1, ctxA.value("A"));
        assertEquals(null, ctx.value("B"));
        assertEquals(null, ctxA.value("B"));
        VContext ctxAB = ctxA.withValue("B", 2);
        assertEquals(null, ctx.value("A"));
        assertEquals(1, ctxA.value("A"));
        assertEquals(null, ctx.value("B"));
        assertEquals(null, ctxA.value("B"));
        assertEquals(1, ctxAB.value("A"));
        assertEquals(2, ctxAB.value("B"));
        VContext ctxNull = ctxAB.withValue("C", null);
        assertEquals(null, ctxNull.value("C"));
        assertEquals(1, ctxAB.value("A"));
        assertEquals(2, ctxAB.value("B"));
    }

    public void testWithCancel() throws Exception {
        VContext ctxC = ctx.withCancel();
        assertThat(ctxC.isCanceled()).isFalse();
        ctxC.cancel();
        assertThat(ctxC.isCanceled()).isTrue();
        sync(ctxC.onDone());
        assertThat(ctxC.isCanceled()).isTrue();
    }

    public void testWithDeadline() throws Exception {
        {
            VContext ctxD = ctx.withDeadline(DateTime.now().plus(500));
            assertThat(ctxD.isCanceled()).isFalse();
            sync(ctxD.onDone());
            assertThat(ctxD.isCanceled()).isTrue();
        }
        {
            VContext ctxD = ctx.withDeadline(DateTime.now().plus(100000));
            assertThat(ctxD.isCanceled()).isFalse();
            ctxD.cancel();
            assertThat(ctxD.isCanceled()).isTrue();
            sync(ctxD.onDone());
            assertThat(ctxD.isCanceled()).isTrue();
        }
    }

    public void testWithTimeout() throws Exception {
        {
            VContext ctxT = ctx.withTimeout(new Duration(500));
            assertThat(ctxT.isCanceled()).isFalse();
            sync(ctxT.onDone());
            assertThat(ctxT.isCanceled()).isTrue();
        }
        {
            VContext ctxT = ctx.withTimeout(new Duration(100000));
            assertThat(ctxT.isCanceled()).isFalse();
            ctxT.cancel();
            assertThat(ctxT.isCanceled()).isTrue();
            sync(ctxT.onDone());
            assertThat(ctxT.isCanceled()).isTrue();
        }
    }
}
