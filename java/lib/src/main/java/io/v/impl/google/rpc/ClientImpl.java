// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google.rpc;

import com.google.common.util.concurrent.ListenableFuture;

import io.v.impl.google.ListenableFutureCallback;
import io.v.v23.Options;
import io.v.v23.context.VContext;
import io.v.v23.options.RpcOptions;
import io.v.v23.rpc.Callback;
import io.v.v23.rpc.Client;
import io.v.v23.rpc.ClientCall;
import io.v.v23.verror.VException;
import io.v.v23.vom.VomUtil;

import java.lang.reflect.Type;

/**
 * An implementation of the {@link Client} interface that calls to native code for most of its
 * functionalities.
 */
public class ClientImpl implements Client {
    private final long nativeRef;

    private native void nativeStartCall(long nativeRef, VContext context,
                                        String name, String method, byte[][] vomArgs,
                                        RpcOptions opts,
                                        Callback<ClientCall> callback);
    private native void nativeClose(long nativeRef);
    private native void nativeFinalize(long nativeRef);

    private ClientImpl(long nativeRef) {
        this.nativeRef = nativeRef;
    }

    // Implement io.v.v23.rpc.Client.
    @Override
    public ListenableFuture<ClientCall> startCall(
            VContext ctx, String name, String method, Object[] args, Type[] argTypes) {
        return startCall(ctx, name, method, args, argTypes, (RpcOptions)null);
    }

    @Deprecated
    @Override
    public ListenableFuture<ClientCall> startCall(VContext ctx, String name, String method,
                                                  Object[] args, Type[] argTypes, Options opts) {
        if (opts == null) {
            opts = new Options();
        }

        return startCall(ctx, name, method, args, argTypes, RpcOptions.migrateOptions(opts));
    }

    @Override
    public ListenableFuture<ClientCall> startCall(VContext ctx, String name, String method,
                                                  Object[] args, Type[] argTypes,
                                                  RpcOptions opts) {
        ListenableFutureCallback<ClientCall> callback = new ListenableFutureCallback<>();
        if (opts == null) {
            opts = new RpcOptions();
        }
        try {
            checkStartCallArgs(name, method, args, argTypes);
            nativeStartCall(nativeRef, ctx, name, getMethodName(method),
                    getEncodedVomArgs(args, argTypes), opts, callback);
        } catch (VException e) {
            callback.onFailure(e);
        }
        return callback.getFuture(ctx);
    }

    private String getMethodName(String method) {
        return Character.toUpperCase(method.charAt(0)) + method.substring(1);
    }

    /**
     * Asserts that the given parameters are valid for {@code startCall} and throws {@link
     * VException} if they are not.
     */
    private void checkStartCallArgs(String name, String method, Object[] args, Type[] argTypes)
            throws VException {
        if ("".equals(method)) {
            throw new VException(String.format("Empty method name invoked on object %s", name));
        }
        if (args.length != argTypes.length) {
            throw new VException(String.format(
                    "Argument count (%d) doesn't match type count (%d) for method %s of object %s",
                    args.length, argTypes.length, name, method));
        }
    }

    private byte[][] getEncodedVomArgs(Object[] args, Type[] argTypes) throws VException {
        // VOM-encode all input arguments, individually.
        byte[][] vomArgs = new byte[args.length][];
        for (int i = 0; i < args.length; ++i) {
            vomArgs[i] = VomUtil.encode(args[i], argTypes[i]);
        }
        return vomArgs;
    }

    @Override
    public void close() {
        nativeClose(nativeRef);
    }
    // Implement java.lang.Object.
    @Override
    public boolean equals(Object other) {
        if (this == other) return true;
        if (other == null) return false;
        if (getClass() != other.getClass()) return false;
        return nativeRef == ((ClientImpl) other).nativeRef;
    }
    @Override
    public int hashCode() {
        return Long.valueOf(nativeRef).hashCode();
    }
    @Override
    protected void finalize() {
        nativeFinalize(nativeRef);
    }
}
