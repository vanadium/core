// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.rpc;

import com.google.common.util.concurrent.ListenableFuture;

import io.v.v23.context.VContext;
import io.v.v23.vdl.VdlValue;
import io.v.v23.vdlroot.signature.Interface;
import io.v.v23.vdlroot.signature.Method;

import java.lang.reflect.Type;

import javax.annotation.CheckReturnValue;

/**
 * Interface used by the server for invoking methods on named objects. Typically,
 * {@link ReflectInvoker} is used, which makes all public methods on the given
 * object invokable.
 */
public interface Invoker extends Globber {
    /**
     * Returns a new {@link ListenableFuture} whose result is the result of invoking the given
     * method with the provided arguments.
     *
     * @param  context    server context
     * @param  call       call state
     * @param  method     name of the invoked method
     * @param  args       method arguments
     * @return            a new {@link ListenableFuture} whose result is the result of invoking the
     *                    given method with the provided arguments
     */
    @CheckReturnValue
    ListenableFuture<Object[]> invoke(
            VContext context, StreamServerCall call, String method, Object[] args);

    /**
     * Returns a new {@link ListenableFuture} whose result are the signatures of the interfaces
     * that the underlying object implements.
     *
     * @param  context    server context
     * @return            a new {@link ListenableFuture} whose result are the signatures of the
     *                    interfaces that the underlying object implements
     */
    @CheckReturnValue
    ListenableFuture<Interface[]> getSignature(VContext context);

    /**
     * Returns a new {@link ListenableFuture} whose result is the signature of the given method.
     *
     * @param  context    server context
     * @param  method     method name
     * @return            a new {@link ListenableFuture} whose result is the signature of the
     *                    given method
     */
    @CheckReturnValue
    ListenableFuture<Method> getMethodSignature(VContext context, String method);

    /**
     * Returns a new {@link ListenableFuture} whose result are the argument types for the given
     * method.
     *
     * @param  context    server context
     * @param  method     method name
     * @return            a new {@link ListenableFuture} whose result are the argument types for
     *                    the given method
     */
    @CheckReturnValue
    ListenableFuture<Type[]> getArgumentTypes(VContext context, String method);

    /**
     * Returns a new {@link ListenableFuture} whose result are the result types for the given
     * method.
     *
     * @param  context    server context
     * @param  method     method name
     * @return            a new {@link ListenableFuture} whose result are the result types for the
     *                    given method
     */
    @CheckReturnValue
    ListenableFuture<Type[]> getResultTypes(VContext context, String method);

    /**
     * Returns a new {@link ListenableFuture} whose result are all the tags associated with the
     * provided method, or an empty array if no tags have been associated with it.
     *
     * @param  context    server context
     * @param  method     method name
     * @return            a new {@link ListenableFuture} whose result are all the tags associated
     *                    with the provided method or an empty array if no tags have been associated
     *                    with it
     */
    @CheckReturnValue
    ListenableFuture<VdlValue[]> getMethodTags(VContext context, String method);
}
