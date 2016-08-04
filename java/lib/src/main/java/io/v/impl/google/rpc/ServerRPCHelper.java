// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google.rpc;

import com.google.common.util.concurrent.AsyncFunction;
import com.google.common.util.concurrent.Futures;
import com.google.common.util.concurrent.ListenableFuture;

import io.v.v23.OutputChannel;
import io.v.v23.context.VContext;
import io.v.v23.naming.GlobReply;
import io.v.v23.rpc.Dispatcher;
import io.v.v23.rpc.Invoker;
import io.v.v23.rpc.ReflectInvoker;
import io.v.v23.rpc.ServerCall;
import io.v.v23.rpc.ServiceObjectWithAuthorizer;
import io.v.v23.rpc.StreamServerCall;
import io.v.v23.security.Authorizer;
import io.v.v23.vdl.ServerSendStream;
import io.v.v23.vdl.VdlValue;
import io.v.v23.verror.VException;
import io.v.v23.vom.VomUtil;

import java.lang.reflect.Type;
import java.util.List;

/**
 * ServerRPCHelper provides a set of helper functions for RPC handling on the server side.
 */
class ServerRPCHelper {
    private static native long nativeGoInvoker(Object serviceObject) throws VException;
    private static native long nativeGoAuthorizer(Object authorizer) throws VException;

    // Helper function for getting tags from the provided invoker.
    static ListenableFuture<byte[][]> prepare(Invoker invoker, VContext ctx, String method) {
        return Futures.transform(invoker.getMethodTags(ctx, method),
                new AsyncFunction<VdlValue[], byte[][]>() {
                    @Override
                    public ListenableFuture<byte[][]> apply(VdlValue[] tags) throws Exception {
                        byte[][] vomTags = new byte[tags.length][];
                        for (int i = 0; i < tags.length; ++i) {
                            vomTags[i] = VomUtil.encode(tags[i], tags[i].vdlType());
                        }
                        return Futures.immediateFuture(vomTags);
                    }
                });
    }

    // Helper function for invoking a method on the provided invoker.
    static ListenableFuture<byte[][]> invoke(final Invoker invoker, final VContext ctx,
                                             final StreamServerCall call,
                                             final String method, final byte[][] vomArgs) {
        return Futures.transform(invoker.getArgumentTypes(ctx, method),
                new AsyncFunction<Type[], byte[][]>() {
                    @Override
                    public ListenableFuture<byte[][]> apply(Type[] argTypes) throws Exception {
                        if (argTypes.length != vomArgs.length) {
                            throw new VException(String.format(
                                    "Wrong number of args, want %d, got %d",
                                    argTypes.length, vomArgs.length));
                        }
                        final Object[] args = new Object[argTypes.length];
                        for (int i = 0; i < argTypes.length; ++i) {
                            args[i] = VomUtil.decode(vomArgs[i], argTypes[i]);
                        }
                        return Futures.transform(Futures.<Object>allAsList(
                                        invoker.getResultTypes(ctx, method),
                                        invoker.invoke(ctx, call, method, args)),
                                new AsyncFunction<List<Object>, byte[][]>() {
                                    @Override
                                    public ListenableFuture<byte[][]> apply(List<Object> input)
                                            throws Exception {
                                        Type[] resultTypes = (Type[]) input.get(0);
                                        Object[] results = (Object[]) input.get(1);
                                        if (resultTypes.length != results.length) {
                                            throw new VException(String.format(
                                                    "Wrong number of results, want %d, got %d",
                                                    resultTypes.length, results.length));
                                        }
                                        byte[][] vomResults = new byte[resultTypes.length][];
                                        for (int i = 0; i < resultTypes.length; ++i) {
                                            vomResults[i] =
                                                    VomUtil.encode(results[i], resultTypes[i]);
                                        }
                                        return Futures.immediateFuture(vomResults);
                                    }
                                });
                    }
                });
    }

    // Helper function for invoking a glob method on the provided invoker.
    static ListenableFuture<Void> glob(Invoker invoker, VContext ctx, ServerCall call,
                                       String pattern, final OutputChannel<GlobReply> channel) {
        return invoker.glob(ctx, call, pattern, new ServerSendStream<GlobReply>() {
            @Override
            public ListenableFuture<Void> send(GlobReply item) {
                return channel.send(item);
            }
        });
    }

    // Helper function for invoking a lookup method on the provided dispatcher.
    // The return value is:
    //    (1) null, if the dispatcher doesn't handle the object with the given suffix, or
    //    (2) an array containing:
    //        - pointer to the appropriate Go invoker,
    //        - pointer to the appropriate Go authorizer.
    static long[] lookup(Dispatcher d, String suffix) throws VException {
        ServiceObjectWithAuthorizer result = d.lookup(suffix);
        if (result == null) {  // object not handled
            return null;
        }
        Object obj = result.getServiceObject();
        if (obj == null) {
            throw new VException("Null service object returned by Java's dispatcher");
        }
        Invoker invoker = obj instanceof Invoker ? (Invoker) obj : new ReflectInvoker(obj);
        Authorizer auth = result.getAuthorizer();
        return new long[] { nativeGoInvoker(invoker), auth == null ? 0 : nativeGoAuthorizer(auth) };
    }

    private ServerRPCHelper() {}
}
