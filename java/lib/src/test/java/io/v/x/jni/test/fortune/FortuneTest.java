// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.x.jni.test.fortune;

import com.google.common.reflect.TypeToken;
import com.google.common.util.concurrent.FutureCallback;
import com.google.common.util.concurrent.Futures;
import com.google.common.util.concurrent.ListenableFuture;
import com.google.common.util.concurrent.SettableFuture;
import com.google.common.util.concurrent.Uninterruptibles;

import io.v.v23.InputChannels;
import io.v.v23.V;
import io.v.v23.V23TestUtil;
import io.v.v23.context.VContext;
import io.v.v23.naming.GlobReply;
import io.v.v23.rpc.Client;
import io.v.v23.rpc.ClientCall;
import io.v.v23.rpc.Dispatcher;
import io.v.v23.rpc.Invoker;
import io.v.v23.rpc.ListenSpec;
import io.v.v23.rpc.Server;
import io.v.v23.rpc.ServerStatus;
import io.v.v23.rpc.ServerState;
import io.v.v23.rpc.ServerCall;
import io.v.v23.rpc.ServiceObjectWithAuthorizer;
import io.v.v23.rpc.StreamServerCall;
import io.v.v23.vdl.ClientStream;
import io.v.v23.vdl.ServerSendStream;
import io.v.v23.vdl.VdlValue;
import io.v.v23.vdlroot.signature.Interface;
import io.v.v23.vdlroot.signature.Method;
import io.v.v23.verror.CanceledException;
import io.v.v23.verror.VException;
import junit.framework.TestCase;
import org.joda.time.Duration;

import java.lang.reflect.Type;
import java.util.List;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.Executor;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;

import static com.google.common.truth.Truth.assertThat;
import static io.v.v23.VFutures.sync;

public class FortuneTest extends TestCase {
    private static final String TEST_INVOKER_FORTUNE = "Test invoker fortune";

    private VContext ctx;

    private static String name(VContext ctx) {
        Server s = V.getServer(ctx);
        if (s == null) {
            return "";
        }
        return "/" + s.getStatus().getEndpoints()[0];
    }

    @Override
    protected void setUp() throws Exception {
        ctx = V.init();
        ListenSpec.Address addr = new ListenSpec.Address("tcp", "127.0.0.1:0");
        ctx = V.withListenSpec(ctx, V.getListenSpec(ctx).withAddress(addr));
    }

    @Override
    protected void tearDown() throws Exception {
        ctx.cancel();
    }

    public void testFortune() throws Exception {
        FortuneServer server = new FortuneServerImpl();
        ctx = V.withNewServer(ctx, "", server, null);
        FortuneClient client = FortuneClientFactory.getFortuneClient(name(ctx));
        VContext ctxT = ctx.withTimeout(new Duration(20000)); // 20s
        try {
            sync(client.get(ctxT));
            fail("Expected exception during call to get() before call to add()");
        } catch (NoFortunesException e) {
            // OK
        } catch (VException e) {
            fail("Expected NoFortuneException, got: " + e);
        }
        String firstMessage = "First fortune";
        sync(client.add(ctxT, firstMessage));
        assertThat(sync(client.get(ctxT))).isEqualTo(firstMessage);
    }

    public void testFortuneWithCancel() throws Exception {
        CountDownLatch callLatch = new CountDownLatch(1);
        FortuneServer server = new FortuneServerImpl(callLatch, null);
        ctx = V.withNewServer(ctx, "", server, null);
        VContext ctxC = ctx.withCancel();

        FortuneClient client = FortuneClientFactory.getFortuneClient(name(ctx));
        VContext ctxT = ctx.withTimeout(new Duration(20000)); // 20s
        sync(client.add(ctxT, "Hello world"));
        ListenableFuture<String> result = client.get(ctxC);
        // Cancel the RPC.
        ctxC.cancel();
        // Allow the server RPC impl to finish.
        callLatch.countDown();
        // The call should have failed, it was canceled before it completed.
        try {
            sync(result);
            fail("get() should have failed");
        } catch (CanceledException e) {
            // OK
        }
    }

    public void testFortuneClientWithExecutor() throws Exception {
        Executor executor = Executors.newSingleThreadExecutor();
        Thread executorThread = getThread(executor);
        ctx = V.withExecutor(ctx, new DelayedExecutor(executor, 100));
        FortuneServer server = new FortuneServerImpl();
        ctx = V.withNewServer(ctx, "", server, null);
        FortuneClient client = FortuneClientFactory.getFortuneClient(name(ctx));
        VContext ctxT = ctx.withTimeout(new Duration(20000)); // 20s
        String firstMessage = "First fortune";
        V23TestUtil.assertRunsOnThread(client.add(ctxT, firstMessage), executorThread);
        V23TestUtil.assertRunsOnThread(client.get(ctxT), executorThread);
    }

    public void testFortuneServerWithExecutor() throws Exception {
        Executor executor = Executors.newSingleThreadExecutor();
        Thread executorThread = getThread(executor);
        ctx = V.withExecutor(ctx, new DelayedExecutor(executor, 100));
        FortuneServer server = new FortuneServerImpl();
        ctx = V.withNewServer(ctx, "", server, null);
        FortuneClient client = FortuneClientFactory.getFortuneClient(name(ctx));
        VContext ctxT = ctx.withTimeout(new Duration(20000)); // 20s
        String serverThreadName = sync(client.getServerThread(ctxT));
        assertThat(serverThreadName).isEqualTo(executorThread.toString());
    }

// TODO(spetrovic): disabled due to flakiness: fix and re-enable.
//    public void testLameDuck() throws Exception {
//        CountDownLatch clientLatch = new CountDownLatch(1);
//        CountDownLatch serverLatch = new CountDownLatch(1);
//        FortuneServer server = new FortuneServerImpl(clientLatch, serverLatch);
//        Options opts = new Options();
//        opts.set(OptionDefs.SERVER_LAME_DUCK_TIMEOUT, Duration.standardSeconds(20));
//        VContext serverCtx = ctx.withCancel();
//        // Set a single-thread executor so that only one get() request is processed at-a-time.
//        serverCtx = V.withExecutor(serverCtx, Executors.newSingleThreadExecutor());
//        serverCtx = V.withNewServer(serverCtx, "", server, null, opts);
//        FortuneClient client = FortuneClientFactory.getFortuneClient(name(serverCtx));
//        VContext ctxT = ctx.withTimeout(new Duration(20000)); // 20s
//        String firstMessage = "The only fortune";
//        sync(client.add(ctxT, firstMessage));
//        final SettableFuture<String> future1 = SettableFuture.create();
//        final SettableFuture<String> future2 = SettableFuture.create();
//        Futures.addCallback(client.get(ctxT), new FutureCallback<String>() {
//            @Override
//            public void onSuccess(String fortune) {
//                future1.set(fortune);
//            }
//            @Override
//            public void onFailure(Throwable t) {
//                future1.setException(t);
//            }
//        });
//        Futures.addCallback(client.get(ctxT), new FutureCallback<String>() {
//            @Override
//            public void onSuccess(String fortune) {
//                future2.set(fortune);
//            }
//            @Override
//            public void onFailure(Throwable t) {
//                future2.setException(t);
//            }
//        });
//        if (!serverLatch.await(20, TimeUnit.SECONDS)) {
//            fail("server get() method never invoked");
//        }
//        serverCtx.cancel();
//        clientLatch.countDown();
//        assertThat(sync(future1)).isEqualTo(firstMessage);
//        assertThat(sync(future2)).isEqualTo(firstMessage);
//        try {
//            ctxT = ctx.withTimeout(new Duration(1000));  // 1s
//            sync(client.get(ctxT));
//            fail("get() that starts after server context cancel should fail");
//        } catch (CanceledException e) {
//            // OK
//        }
//    }

    public void testNoLameDuck() throws Exception {
        CountDownLatch clientLatch = new CountDownLatch(1);
        CountDownLatch serverLatch = new CountDownLatch(1);
        FortuneServer server = new FortuneServerImpl(clientLatch, serverLatch);
        VContext serverCtx = ctx.withCancel();
        // Set a single-thread executor so that only one get() request is processed at-a-time.
        serverCtx = V.withExecutor(serverCtx, Executors.newSingleThreadExecutor());
        serverCtx = V.withNewServer(serverCtx, "", server, null);
        FortuneClient client = FortuneClientFactory.getFortuneClient(name(serverCtx));
        VContext ctxT = ctx.withTimeout(new Duration(20000)); // 20s
        String firstMessage = "The only fortune";
        sync(client.add(ctxT, firstMessage));
        final SettableFuture<String> future = SettableFuture.create();
        Futures.addCallback(client.get(ctxT), new FutureCallback<String>() {
            @Override
            public void onSuccess(String fortune) {
                future.set(fortune);
            }
            @Override
            public void onFailure(Throwable t) {
                future.setException(t);
            }
        });
        if (!serverLatch.await(20, TimeUnit.SECONDS)) {
            fail("server get() method never invoked");
        }
        serverCtx.cancel();
        clientLatch.countDown();
        try {
            sync(future);
            fail("get() in progress during server context cancel should fail");
        } catch (VException e) {
            // OK
        }
    }

    public void testStreaming() throws Exception {
        FortuneServer server = new FortuneServerImpl();
        ctx = V.withNewServer(ctx, "", server, null);
        FortuneClient client = FortuneClientFactory.getFortuneClient(name(ctx));
        VContext ctxT = ctx.withTimeout(new Duration(20000));  // 20s
        ClientStream<Boolean, String, Integer> stream = client.streamingGet(ctxT);
        String msg = "The only fortune";
        sync(client.add(ctxT, msg));
        for (int i = 0; i < 5; ++i) {
            sync(stream.send(true));
        }
        sync(stream.close());
        assertThat(sync(InputChannels.asList(stream))).containsExactly(msg, msg, msg, msg, msg);
        int result = sync(stream.finish());
        assertEquals(5, result);
    }

    public void testStreamingWithExecutor() throws Exception {
        Executor executor = Executors.newSingleThreadExecutor();
        Thread executorThread = getThread(executor);
        ctx = V.withExecutor(ctx, new DelayedExecutor(executor, 100));
        FortuneServer server = new FortuneServerImpl();
        ctx = V.withNewServer(ctx, "", server, null);
        FortuneClient client = FortuneClientFactory.getFortuneClient(name(ctx));
        VContext ctxT = ctx.withTimeout(new Duration(20000));  // 20s
        ClientStream<Boolean, String, Integer> stream = client.streamingGet(ctxT);
        String msg = "The only fortune";
        sync(client.add(ctxT, msg));
        for (int i = 0; i < 5; ++i) {
            V23TestUtil.assertRunsOnThread(stream.send(true), executorThread);
        }
        V23TestUtil.assertRunsOnThread(stream.close(), executorThread);
        for (int i = 0; i < 5; ++i) {
            V23TestUtil.assertRunsOnThread(stream.recv(), executorThread);
        }
        V23TestUtil.assertRunsOnThread(stream.finish(), executorThread);
    }

    public void testStreamingWithCancel() throws Exception {
        FortuneServer server = new FortuneServerImpl();
        ctx = V.withNewServer(ctx, "", server, null);
        FortuneClient client = FortuneClientFactory.getFortuneClient(name(ctx));
        VContext ctxT = ctx.withTimeout(new Duration(20000));  // 20s
        ClientStream<Boolean, String, Integer> stream = client.streamingGet(ctxT);
        String msg = "The only fortune";
        sync(client.add(ctxT, msg));
        for (int i = 0; i < 5; ++i) {
            sync(stream.send(true));
        }
        sync(stream.close());
        for (int i = 0; i < 3; ++i) {
            assertThat(sync(stream.recv())).isEqualTo(msg);
        }
        ctxT.cancel();
        try {
            sync(stream.recv());
            fail("stream.recv() should fail after context has been canceled");
        } catch (CanceledException e) {
            // OK
        }
    }

    public void testMultiple() throws Exception {
        FortuneServer server = new FortuneServerImpl();
        ctx = V.withNewServer(ctx, "", server, null);

        FortuneClient client = FortuneClientFactory.getFortuneClient(name(ctx));
        VContext ctxT = ctx.withTimeout(new Duration(20000)); // 20s
        String firstMessage = "First fortune";
        sync(client.add(ctxT, firstMessage));

        FortuneClient.MultipleGetOut ret = sync(client.multipleGet(ctxT));
        assertEquals(firstMessage, ret.fortune);
        assertEquals(firstMessage, ret.another);
    }

    public void testMultipleStreaming() throws Exception {
        FortuneServer server = new FortuneServerImpl();
        ctx = V.withNewServer(ctx, "", server, null);

        FortuneClient client = FortuneClientFactory.getFortuneClient(name(ctx));
        VContext ctxT = ctx.withTimeout(new Duration(20000));  // 20s
        ClientStream<Boolean, String, FortuneClient.MultipleStreamingGetOut> stream =
                client.multipleStreamingGet(ctxT);
        String msg = "The only fortune";
        sync(client.add(ctxT, msg));
        for (int i = 0; i < 5; ++i) {
            sync(stream.send(true));
        }
        sync(stream.close());
        assertThat(sync(InputChannels.asList(stream))).containsExactly(msg, msg, msg, msg, msg);
        FortuneClient.MultipleStreamingGetOut result = sync(stream.finish());
        assertEquals(5, result.total);
        assertEquals(5, result.another);
    }

    public void testComplexError() throws Exception {
        FortuneServer server = new FortuneServerImpl();
        ctx = V.withNewServer(ctx, "", server, null);

        FortuneClient client = FortuneClientFactory.getFortuneClient(name(ctx));
        VContext ctxT = ctx.withTimeout(new Duration(20000)); // 20s
        try {
            sync(client.getComplexError(ctxT));
            fail("Expected exception during call to getComplexError()");
        } catch (ComplexException e) {
            if (!FortuneServerImpl.COMPLEX_ERROR.deepEquals(e)) {
                fail(String.format("Expected error %s, got %s",
                        FortuneServerImpl.COMPLEX_ERROR, e));
            }
        }
    }

    public void testContext() throws Exception {
        FortuneServer server = new FortuneServerImpl();
        ctx = V.withNewServer(ctx, "", server, null);

        FortuneClient client = FortuneClientFactory.getFortuneClient(name(ctx));
        VContext ctxT = ctx.withTimeout(new Duration(20000)); // 20s
        try {
            sync(client.testServerCall(ctxT));
        } catch (VException e) {
            fail("Context check failed: " + e.getMessage());
        }
    }

    public void testGetServer() throws Exception {
        FortuneServer server = new FortuneServerImpl();
        ctx = V.withNewServer(ctx, "", server, null);
        Server s = V.getServer(ctx);
        assertThat(s).isNotNull();
    }

    public void testGetSignature() throws Exception {
        FortuneServer server = new FortuneServerImpl();
        ctx = V.withNewServer(ctx, "", server, null);

        Client c = V.getClient(ctx);
        VContext ctxT = ctx.withTimeout(new Duration(20000)); // 20s
        ClientCall call = sync(
                c.startCall(ctxT, name(ctx), "__Signature", new Object[0], new Type[0]));
        Object[] results = sync(
                call.finish(new Type[]{new TypeToken<Interface[]>() {
                }.getType()}));
        assertThat(results.length == 1).isTrue();
        Interface[] signature = (Interface[]) results[0];
        assertThat(signature.length >= 1).isTrue();
        assertThat(signature[0].getMethods()).isNotEmpty();
    }

    public void testGlob() throws Exception {
        FortuneServer server = new FortuneServerImpl();
        ctx = V.withNewServer(ctx, "", server, null);

        List<GlobReply> globResult = sync(InputChannels.asList(
                V.getNamespace(ctx).glob(ctx, name(ctx) + "/*")));
        assertThat(globResult).hasSize(2);
        assertThat(globResult.get(0)).isInstanceOf(GlobReply.Entry.class);
        assertThat(((GlobReply.Entry) globResult.get(0)).getElem().getName())
                .isEqualTo(name(ctx) + "/helloworld");
        assertThat(globResult.get(1)).isInstanceOf(GlobReply.Error.class);
    }

    public void testCustomInvoker() throws Exception {
        ctx = V.withNewServer(ctx, "", new TestInvoker(), null);

        FortuneClient client = FortuneClientFactory.getFortuneClient(name(ctx));
        VContext ctxT = ctx.withTimeout(new Duration(20000)); // 20s
        assertThat(sync(client.get(ctxT))).isEqualTo(TEST_INVOKER_FORTUNE);
    }

    public void testCustomDispatcherReturningAServer() throws Exception {
        final FortuneServer server = new FortuneServerImpl();
        Dispatcher dispatcher = new Dispatcher() {
            @Override
            public ServiceObjectWithAuthorizer lookup(String suffix) throws VException {
                return new ServiceObjectWithAuthorizer(server, null);
            }
        };
        ctx = V.withNewServer(ctx, "", dispatcher);

        FortuneClient client = FortuneClientFactory.getFortuneClient(name(ctx));
        VContext ctxT = ctx.withTimeout(new Duration(20000)); // 20s
        String firstMessage = "First fortune";
        sync(client.add(ctxT, firstMessage));
        assertThat(sync(client.get(ctxT))).isEqualTo(firstMessage);
    }

    public void testCustomDispatcherReturningAnInvoker() throws Exception {
        Dispatcher dispatcher = new Dispatcher() {
            @Override
            public ServiceObjectWithAuthorizer lookup(String suffix) throws VException {
                return new ServiceObjectWithAuthorizer(new TestInvoker(), null);
            }
        };
        ctx = V.withNewServer(ctx, "", dispatcher);

        FortuneClient client = FortuneClientFactory.getFortuneClient(name(ctx));
        VContext ctxT = ctx.withTimeout(new Duration(20000)); // 20s
        assertThat(sync(client.get(ctxT))).isEqualTo(TEST_INVOKER_FORTUNE);
    }

    public void testServerStatus() throws Exception {
        FortuneServer server = new FortuneServerImpl();
        ctx = V.withNewServer(ctx, "", server, null);
        Server s = V.getServer(ctx);
        ServerStatus status = s.getStatus();
        assertThat(status.getState()).isEqualTo(ServerState.SERVER_ACTIVE);
        assertThat(status.getEndpoints()).isNotEmpty();
        // We should have one successful entry in listenErrors.
        assertThat(status.getListenErrors()).isNotEmpty();
        assertThat(status.getProxyErrors()).isEmpty();
        // TODO(suharshs,sjr): Add a test for proxy errors, and listenErrors
        // that actually is populated with errors.
    }

    private static class TestInvoker implements Invoker {
        @Override
        public ListenableFuture<Object[]> invoke(
                VContext ctx, StreamServerCall call, String method, Object[] args) {
            if (call.security() == null) {
                return Futures.immediateFailedFuture(
                        new VException("Expected call.security() to return non-null"));
            }
            if (call.remoteEndpoint() == null) {
                return Futures.immediateFailedFuture(
                        new VException("Expected remoteEndpoint() to return non-null"));
            }
            if (method.equals("get")) {
                return Futures.immediateFuture(new Object[] { TEST_INVOKER_FORTUNE });
            }
            return Futures.immediateFailedFuture(new VException("Unsupported method: " + method));
        }
        @Override
        public ListenableFuture<Interface[]> getSignature(VContext ctx) {
            return Futures.immediateFailedFuture(new VException("getSignature() unimplemented"));
        }
        @Override
        public ListenableFuture<Method> getMethodSignature(VContext ctx, String method) {
            return Futures.immediateFailedFuture(
                    new VException("getMethodSignature() unimplemented"));
        }
        @Override
        public ListenableFuture<Type[]> getArgumentTypes(VContext ctx, String method) {
            if (method.equals("get")) {
                return Futures.immediateFuture(new Type[] {});
            }
            return Futures.immediateFailedFuture(new VException("Unsupported method: " + method));
        }
        @Override
        public ListenableFuture<Type[]> getResultTypes(VContext ctx, String method) {
            if (method.equals("get")) {
                return Futures.immediateFuture(new Type[] { String.class });
            }
            return Futures.immediateFailedFuture(new VException("Unsupported method: " + method));
        }
        @Override
        public ListenableFuture<VdlValue[]> getMethodTags(VContext ctx, String method) {
            return Futures.immediateFuture(new VdlValue[] {});
        }
        @Override
        public ListenableFuture<Void> glob(VContext ctx, ServerCall call, String pattern,
                                           ServerSendStream<GlobReply> responseChannel) {
            return Futures.immediateFuture(null);
        }
    }

    private static Thread getThread(Executor executor) throws Exception {
        final SettableFuture<Thread> threadFuture =
                com.google.common.util.concurrent.SettableFuture.create();
        executor.execute(new Runnable() {
            @Override
            public void run() {
                threadFuture.set(Thread.currentThread());
            }
        });
        return threadFuture.get();
    }

    private static class DelayedExecutor implements Executor {
        private final Executor executor;
        private final int delayMs;

        DelayedExecutor(Executor executor, int delayMs) {
            this.executor = executor;
            this.delayMs = delayMs;
        }
        @Override
        public void execute(final Runnable runnable) {
            executor.execute(new Runnable() {
                @Override
                public void run() {
                    Uninterruptibles.sleepUninterruptibly(delayMs, TimeUnit.MILLISECONDS);
                    runnable.run();
                }
            });
        }
    }
}
