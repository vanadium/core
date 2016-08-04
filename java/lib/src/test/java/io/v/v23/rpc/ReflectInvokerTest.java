// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.rpc;

import static com.google.common.truth.Truth.assertThat;
import static com.google.common.truth.Truth.assert_;

import com.google.common.base.Function;
import com.google.common.collect.ImmutableMap;
import com.google.common.collect.Lists;

import junit.framework.TestCase;

import io.v.v23.V;
import io.v.v23.context.VContext;
import io.v.v23.security.access.Constants;
import io.v.v23.vdl.VdlValue;
import io.v.v23.vdlroot.signature.Interface;
import io.v.v23.vdlroot.signature.Method;
import io.v.v23.verror.VException;
import io.v.x.jni.test.fortune.FortuneServerImpl;

import java.util.Arrays;
import java.util.Map;

import static io.v.v23.VFutures.sync;

public class ReflectInvokerTest extends TestCase {
    private VContext ctx;

    @Override
    protected void setUp() throws Exception {
        ctx = V.init();
    }

    @Override
    protected void tearDown() throws Exception {
        ctx.cancel();
    }

    public void testInvoke() throws Exception {
        ReflectInvoker invoker = new ReflectInvoker(new FortuneServerImpl());
        StreamServerCall call = null;
        {
            Object[] results =
                    sync(invoker.invoke(ctx, call, "add", new Object[]{"test fortune"}));
            assertThat(Arrays.asList(results)).containsExactly();
        }
        {
            Object[] results = sync(invoker.invoke(ctx, call, "get", new Object[]{}));
            assertEquals(1, results.length);
            assertThat(Arrays.asList(results)).containsExactly("test fortune");
        }
        {
            Object[] results = sync(invoker.invoke(ctx, call, "multipleGet", new Object[]{}));
            assertThat(Arrays.asList(results)).containsExactly("test fortune", "test fortune");
        }
        {
            // Test error.
            try {
                sync(invoker.invoke(ctx, call, "getComplexError", new Object[]{}));
                fail("invocation of getComplexError() should raies an exception");
            } catch (VException e) {
                assert_().withFailureMessage(String.format(
                        "Want error %s, got %s", FortuneServerImpl.COMPLEX_ERROR, e))
                    .that(FortuneServerImpl.COMPLEX_ERROR.deepEquals(e)).isTrue();
            }
        }
    }

    public void testGetArgumentTypes() throws Exception {
        ReflectInvoker invoker = new ReflectInvoker(new FortuneServerImpl());
        assertThat(Arrays.asList(
                sync(invoker.getArgumentTypes(ctx, "add")))).containsExactly(String.class);
        assertThat(Arrays.asList(sync(invoker.getArgumentTypes(ctx, "get")))).containsExactly();
        try {
            sync(invoker.getArgumentTypes(ctx, "none"));
            fail("getArgumentTypes() call with a non-existent method should raise an exception.");
        } catch (VException e) {
            // OK
        }
    }

    public void testGetResultTypes() throws Exception {
        ReflectInvoker invoker = new ReflectInvoker(new FortuneServerImpl());
        assertThat(Arrays.asList(
                sync(invoker.getResultTypes(ctx, "get")))).containsExactly(String.class);
        assertThat(Arrays.asList(sync(invoker.getResultTypes(ctx, "add")))).containsExactly();
        try {
            sync(invoker.getResultTypes(ctx, "none"));
            fail("getResultTypes() call with a non-existent method should raise an exception.");
        } catch (VException e) {
            // OK
        }
    }

    public void testGetMethodTags() throws Exception {
        ReflectInvoker invoker = new ReflectInvoker(new FortuneServerImpl());
        Map<String, VdlValue[]> testCases = ImmutableMap.<String, VdlValue[]>builder()
                .put("add", new VdlValue[]{ Constants.WRITE })
                .put("get", new VdlValue[]{ Constants.READ })
                .put("streamingGet", new VdlValue[] { Constants.READ })
                .put("getComplexError", new VdlValue[] { Constants.READ })
                .put("noTags", new VdlValue[0])
                .build();
        for (Map.Entry<String, VdlValue[]> testCase : testCases.entrySet()) {
            String method = testCase.getKey();
            VdlValue[] expected = testCase.getValue();
            VdlValue[] actual = sync(invoker.getMethodTags(ctx, method));
            assertThat(Arrays.asList(actual)).containsExactlyElementsIn(Arrays.asList(expected));
        }
        try {
            sync(invoker.getMethodTags(ctx, "none"));
            fail("getMethodTags() call with a non-existent method should raise an exception.");
        } catch (VException e) {
            // OK
        }
    }

    public void testGetSignature() throws Exception {
        ReflectInvoker invoker = new ReflectInvoker(new FortuneServerImpl());
        Interface[] serverInterface = sync(invoker.getSignature(ctx));
        assertThat(serverInterface).hasLength(1);
        assertThat(serverInterface[0].getMethods()).hasSize(10);
        Function<Method, String> methodNameFunction = new Function<Method, String>() {
            @Override
            public String apply(Method input) {
                return input.getName();
            }
        };
        assertThat(Lists.transform(
                serverInterface[0].getMethods(), methodNameFunction)).containsAllOf(
                "get", "add", "parameterizedGet", "streamingGet", "multipleGet",
                "multipleStreamingGet", "getComplexError", "noTags", "testServerCall",
                "getServerThread");
        assertThat(serverInterface[0].getName()).isEqualTo("Fortune");
    }
}
