// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.x.jni.test.fortune;

import com.google.common.base.Function;
import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableMap;
import com.google.common.util.concurrent.AsyncFunction;
import com.google.common.util.concurrent.FutureCallback;
import com.google.common.util.concurrent.Futures;
import com.google.common.util.concurrent.ListenableFuture;
import com.google.common.util.concurrent.SettableFuture;

import io.v.v23.InputChannelCallback;
import io.v.v23.InputChannels;
import io.v.v23.context.VContext;
import io.v.v23.naming.GlobError;
import io.v.v23.naming.GlobReply;
import io.v.v23.naming.MountEntry;
import io.v.v23.naming.MountedServer;
import io.v.v23.rpc.Globber;
import io.v.v23.rpc.ServerCall;
import io.v.v23.vdl.ServerSendStream;
import io.v.v23.vdl.ServerStream;
import io.v.v23.vdl.VdlUint32;
import io.v.v23.verror.VException;

import java.util.Map;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.atomic.AtomicInteger;

public class FortuneServerImpl implements FortuneServer, Globber {
    private static final ComplexErrorParam COMPLEX_PARAM = new ComplexErrorParam(
            "StrVal",
            11,
            ImmutableList.<VdlUint32>of(new VdlUint32(22), new VdlUint32(33)));

    public static final ComplexException COMPLEX_ERROR = new ComplexException(
            "en", "test", "test", COMPLEX_PARAM, "secondParam", 3);
    private final CountDownLatch clientLatch;
    private final CountDownLatch serverLatch;

    private String lastAddedFortune;

    public FortuneServerImpl() {
        this(null, null);
    }

    /**
     * Creates a new instance of this server with the provided latches.
     *
     * @param clientLatch   if not {@code null}, {@link FortuneServerImpl#get} method will block
     *                      until the latch is counted down
     * @param serverLatch   if not {@code null}, {@link FortuneServerImpl#get} method will count
     *                      down this latch upon its invocation
     */
    public FortuneServerImpl(CountDownLatch clientLatch, CountDownLatch serverLatch) {
        this.clientLatch = clientLatch;
        this.serverLatch = serverLatch;
    }

    @Override
    public ListenableFuture<String> get(final VContext context, ServerCall call) {
        if (serverLatch != null) {
            serverLatch.countDown();
        }
        if (clientLatch != null) {
            try {
                // Caution: this is not idiomatic for server methods: they must be non-blocking.
                // However, it helps us with LameDuck tests.
                clientLatch.await();
            } catch (InterruptedException e) {
                return Futures.immediateFailedFuture(new VException(e.getMessage()));
            }
        }
        if (lastAddedFortune == null) {
            return Futures.immediateFailedFuture(new NoFortunesException(context));
        }
        return Futures.immediateFuture(lastAddedFortune);
    }

    @Override
    public ListenableFuture<Map<String, String>> parameterizedGet(
            VContext context, ServerCall call) {
        if (lastAddedFortune == null) {
            return Futures.immediateFailedFuture(new NoFortunesException(context));
        }
        return Futures.immediateFuture(
                (Map<String, String>) ImmutableMap.of(lastAddedFortune, lastAddedFortune));
    }

    @Override
    public ListenableFuture<Void> add(VContext context, ServerCall call, String fortune) {
        lastAddedFortune = fortune;
        return Futures.immediateFuture(null);
    }

    @Override
    public ListenableFuture<Integer> streamingGet(
            final VContext context, ServerCall call, final ServerStream<String, Boolean> stream) {
        final SettableFuture<Integer> future = SettableFuture.create();
        final AtomicInteger numSent = new AtomicInteger(0);
        Futures.addCallback(InputChannels.withCallback(stream, new InputChannelCallback<Boolean>() {
            @Override
            public ListenableFuture<Void> onNext(Boolean result) {
                if (lastAddedFortune == null) {
                    return Futures.immediateFailedFuture(new NoFortunesException(context));
                }
                return Futures.transform(stream.send(lastAddedFortune),
                        new Function<Void, Void>() {
                            @Override
                            public Void apply(Void input) {
                                numSent.incrementAndGet();
                                return null;
                            }
                        });
            }
        }), new FutureCallback<Void>() {
            @Override
            public void onSuccess(Void result) {
                future.set(numSent.get());
            }
            @Override
            public void onFailure(Throwable t) {
                future.setException(t);
            }
        });
        return future;
    }

    @Override
    public ListenableFuture<MultipleGetOut> multipleGet(VContext context, ServerCall call) {
        if (lastAddedFortune == null) {
            return Futures.immediateFailedFuture(new NoFortunesException(context));
        }
        MultipleGetOut ret = new MultipleGetOut();
        ret.fortune = lastAddedFortune;
        ret.another = lastAddedFortune;
        return Futures.immediateFuture(ret);
    }

    @Override
    public ListenableFuture<MultipleStreamingGetOut> multipleStreamingGet(
            final VContext context, ServerCall call, final ServerStream<String, Boolean> stream) {
        final SettableFuture<MultipleStreamingGetOut> future = SettableFuture.create();
        final AtomicInteger numSent = new AtomicInteger(0);
        Futures.addCallback(InputChannels.withCallback(stream, new InputChannelCallback<Boolean>() {
            @Override
            public ListenableFuture<Void> onNext(Boolean result) {
                if (lastAddedFortune == null) {
                    return Futures.immediateFailedFuture(new NoFortunesException(context));
                }
                return Futures.transform(stream.send(lastAddedFortune),
                        new Function<Void, Void>() {
                            @Override
                            public Void apply(Void input) {
                                numSent.incrementAndGet();
                                return null;
                            }
                        });
            }
        }), new FutureCallback<Void>() {
            @Override
            public void onSuccess(Void result) {
                MultipleStreamingGetOut ret = new MultipleStreamingGetOut();
                ret.total = numSent.get();
                ret.another = numSent.get();
                future.set(ret);
            }
            @Override
            public void onFailure(Throwable t) {
                future.setException(t);
            }
        });
        return future;
    }

    @Override
    public ListenableFuture<Void> getComplexError(VContext context, ServerCall call) {
        return Futures.immediateFailedFuture(COMPLEX_ERROR);
    }

    @Override
    public ListenableFuture<Void> testServerCall(VContext context, ServerCall call) {
        if (call == null) {
            return Futures.immediateFailedFuture(new VException("ServerCall is null"));
        }
        if (call.suffix() == null) {
            return Futures.immediateFailedFuture(new VException("Suffix is null"));
        }
        if (call.localEndpoint() == null) {
            return Futures.immediateFailedFuture(new VException("Local endpoint is null"));
        }
        if (call.remoteEndpoint() == null) {
            return Futures.immediateFailedFuture(new VException("Remote endpoint is null"));
        }
        if (context == null) {
            return Futures.immediateFailedFuture(new VException("Vanadium context is null"));
        }
        return Futures.immediateFuture(null);
    }

    @Override
    public ListenableFuture<String> getServerThread(VContext context, ServerCall call) {
        return Futures.immediateFuture(Thread.currentThread().toString());
    }

    @Override
    public ListenableFuture<Void> noTags(VContext context, ServerCall call) {
        return Futures.immediateFuture(null);
    }

    @Override
    public ListenableFuture<Void> glob(VContext context, ServerCall call,
                                       String pattern, final ServerSendStream<GlobReply> stream) {
        final GlobReply.Entry entry = new GlobReply.Entry(
                new MountEntry("helloworld", ImmutableList.<MountedServer>of(), false, false));
        final GlobReply.Error error = new GlobReply.Error(
                new GlobError("Hello, world!", new VException("Some error")));
        return Futures.transform(stream.send(entry), new AsyncFunction<Void, Void>() {
                    @Override
                    public ListenableFuture<Void> apply(Void input) throws Exception {
                        return stream.send(error);
                    }
                });
    }
}
