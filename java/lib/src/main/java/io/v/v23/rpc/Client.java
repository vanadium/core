// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.rpc;

import com.google.common.util.concurrent.ListenableFuture;

import java.lang.reflect.Type;

import javax.annotation.CheckReturnValue;

import io.v.v23.Options;
import io.v.v23.context.VContext;
import io.v.v23.options.RpcOptions;

/**
 * The interface for making RPC calls.  There may be multiple outstanding calls associated with a
 * single client, and a client may be used by multiple threads concurrently.
 */
public interface Client {
    /**
     * @deprecated Use {@link #startCall(VContext, String, String, Object[], Type[], RpcOptions)}
     *  instead, which uses a strongly-typed {@link RpcOptions} object that supports more features.
     */
    @CheckReturnValue
    ListenableFuture<ClientCall> startCall(VContext context, String name, String method,
                                           Object[] args, Type[] argTypes, Options opts);

    /**
     * Starts an asynchronous call of the method on the server instance identified by name with the
     * given input args (of any arity) and provided options.
     * <p>
     * A particular implementation of this interface chooses which options to support,
     * but at the minimum it must handle the following pre-defined options:
     * <ul>
     *     <li>{@link io.v.v23.OptionDefs#SKIP_SERVER_ENDPOINT_AUTHORIZATION}</li>
     * </ul>
     * <p>
     * The returned future is guaranteed to be executed on an {@link java.util.concurrent.Executor}
     * specified in {@code context} (see {@link io.v.v23.V#withExecutor}).
     * <p>
     * The returned future will fail with {@link java.util.concurrent.CancellationException} if
     * {@code context} gets canceled.
     *
     * @param  context   client context
     * @param  name      name of the server
     * @param  method    name of the server's method to be invoked
     * @param  args      arguments to the server method.
     * @param  argTypes  types of the provided arguments
     * @param  opts      call options
     * @return           a new {@link ListenableFuture} whose result is the call object that
     *                   manages streaming args and results
     */
    @CheckReturnValue
    ListenableFuture<ClientCall> startCall(VContext context, String name, String method,
                                           Object[] args, Type[] argTypes, RpcOptions opts);

    /**
     * A shortcut for {@link #startCall(VContext, String, String, Object[], Type[], Options)} with
     * a {@code null} options parameter.
     */
    @CheckReturnValue
    ListenableFuture<ClientCall> startCall(VContext context, String name, String method,
                                           Object[] args, Type[] argTypes);

    /**
     * Discards the state associated with this client.  In-flight calls may be terminated with
     * an error.
     * <p>
     * This is a non-blocking method.
     */
    void close();
}
