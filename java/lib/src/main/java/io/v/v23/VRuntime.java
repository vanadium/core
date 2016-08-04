// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23;

import io.v.v23.context.VContext;
import io.v.v23.discovery.Discovery;
import io.v.v23.namespace.Namespace;
import io.v.v23.options.RpcServerOptions;
import io.v.v23.rpc.Client;
import io.v.v23.rpc.Dispatcher;
import io.v.v23.rpc.ListenSpec;
import io.v.v23.rpc.Server;
import io.v.v23.security.Authorizer;
import io.v.v23.security.VPrincipal;
import io.v.v23.verror.VException;

/**
 * The base interface for all runtime implementations.
 *
 * Methods in this class aren't meant to be invoked directly; instead, {@link V}
 * should be initialized with an instance of this class, passed in through the
 * {@link OptionDefs#RUNTIME} option.
 */
public interface VRuntime {
    /**
     * Creates a new client instance with the provided options and attaches it to a new context
     * (which is derived from the given context).
     * <p>
     * A particular runtime implementation chooses which options to support, but at the minimum must
     * handle the following options:
     * <p><ul>
     *     <li>(CURRENTLY NO OPTIONS ARE MANDATED)</li>
     * </ul>
     *
     * @param  ctx             current context
     * @param  opts            client options
     * @return                 child context to which the new client is attached
     * @throws VException      if a new client cannot be created
     */
    VContext withNewClient(VContext ctx, Options opts) throws VException;

    /**
     * Returns the client attached to the given context, or {@code null} if no client is attached.
     * <p>
     * If the passed-in context is derived from the context returned by {@link #getContext},
     * the returned client will never be {@code null}.
     */
    Client getClient(VContext ctx);

    /**
     * Creates a new {@link Server} instance to serve a service object and attaches
     * it to a new context (which is derived from the provided context).
     *
     * The server will listen for network connections as specified by the {@link ListenSpec}
     * attached to the context. Depending on your runtime, 'roaming' support may be enabled.
     * In this mode the server will adapt to changes in the network configuration
     * and re-publish the current set of endpoints to the mount table accordingly.
     * <p>
     * This call associates object with name by publishing the address of this server with the
     * mount table under the supplied name and using the given authorizer to authorize access to it.
     * RPCs invoked on the supplied name will be delivered to methods implemented by the supplied
     * object.
     * <p>
     * Reflection is used to match requests to the object's method set.  As a special-case, if the
     * object implements the {@link io.v.v23.rpc.Invoker} interface, the invoker is used to invoke
     * methods directly, without reflection.
     * <p>
     * If name is an empty string, no attempt will made to publish that name to a mount table.
     * <p>
     * If the passed-in authorizer is {@code null}, the default authorizer will be used.
     * (The default authorizer uses the blessing chain derivation to determine if the client is
     * authorized to access the object's methods.)
     * <p>
     * A particular runtime implementation chooses which options to support,
     * but at the minimum it must handle the following options:
     * <p><ul>
     *     <li>(CURRENTLY NO OPTIONS ARE MANDATED)</li>
     * </ul>
     *
     * @param  ctx             current context
     * @param  name            name under which the supplied object should be published,
     *                         or the empty string if the object should not be published
     * @param  object          object to be published under the given name
     * @param  authorizer      authorizer that will control access to objects methods
     * @param  opts            server options
     * @return                 a child context to which the new server is attached
     * @throws VException      if a new server cannot be created
     */
    VContext withNewServer(VContext ctx, String name, Object object, Authorizer authorizer,
                           RpcServerOptions opts) throws VException;

    /**
     * @deprecated Use
     *  {@link #withNewServer(VContext, String, Object, Authorizer, RpcServerOptions)}.
     */
    VContext withNewServer(VContext ctx, String name, Object object, Authorizer authorizer,
                           Options opts) throws VException;

    /**
     * Creates a new {@link Server} instance to serve a dispatcher and attaches
     * it to a new context (which is derived from the provided context).
     *
     * The server will listen for network connections as specified by the {@link ListenSpec}
     * attached to the context. Depending on your runtime, 'roaming' support may be enabled.
     * In this mode the server will adapt to changes in the network configuration
     * and re-publish the current set of endpoints to the mount table accordingly.
     * <p>
     * Associates dispatcher with the portion of the mount table's name space for which
     * {@code name} is a prefix, by publishing the address of this dispatcher with the mount
     * table under the supplied name.
     * <p>
     * RPCs invoked on the supplied name will be delivered to the supplied {@link Dispatcher}'s
     * {@link Dispatcher#lookup lookup} method which will in turn return the object and
     * {@link Authorizer} used to serve the actual RPC call.
     * <p>
     * If name is an empty string, no attempt will made to publish that name to a mount table.
     * <p>
     * A particular runtime implementation chooses which options to support,
     * but at the minimum it must handle the following options:
     * <p><ul>
     *     <li>(CURRENTLY NO OPTIONS ARE MANDATED)</li>
     * </ul>
     *
     * @param  ctx             current context
     * @param  name            name under which the supplied object should be published,
     *                         or the empty string if the object should not be published
     * @param  dispatcher      dispatcher to be published under the given name
     * @param  opts            server options
     * @return                 a child context to which the new server is attached
     * @throws VException      if a new server cannot be created
     */
    VContext withNewServer(VContext ctx, String name, Dispatcher dispatcher,
                           RpcServerOptions opts) throws VException;

    /**
     * @deprecated Use {@link #withNewServer(VContext, String, Dispatcher, RpcServerOptions)}
     */
    VContext withNewServer(VContext ctx, String name, Dispatcher dispatcher,
                           Options opts) throws VException;

    /**
     * Returns the server attached to the given context, or {@code null} if no server is attached.
     */
    Server getServer(VContext ctx);

    /**
     * Attaches the given principal to a new context (which is derived from the given context).
     *
     * @param  ctx             current context
     * @param  principal       principal to be attached
     * @return                 child context to which the principal is attached
     * @throws VException      if the principal couldn't be attached
     */
    VContext withPrincipal(VContext ctx, VPrincipal principal) throws VException;

    /**
     * Returns the principal attached to the given context, or {@code null} if no principal
     * is attached.
     * <p>
     * If the passed-in context is derived from the context returned by {@link #getContext},
     * the returned principal will never be {@code null}.
     */
    VPrincipal getPrincipal(VContext ctx);

    /**
     * Creates a new namespace instance and attaches it to a new context (which is derived from the
     * provided context).
     *
     * @param  ctx             current context
     * @param  roots           roots of the new namespace
     * @return                 child context to which the principal is attached
     * @throws VException      if the namespace couldn't be created
     */
    VContext withNewNamespace(VContext ctx, String... roots) throws VException;

    /**
     * Returns the namespace attached to the given context, or {@code null} if no namespace
     * is attached.
     * <p>
     * If the passed-in context is derived from the context returned by {@link #getContext},
     * the returned namespace will never be {@code null}.
     */
    Namespace getNamespace(VContext ctx);

    /**
     * Attaches the given {@code ListenSpec} to a new context (which is derived from the provided
     * context).
     *
     * @param ctx        current context
     * @param spec       the {@code ListenSpec} to attach
     * @return           child context to which the {@code ListenSpec} is attached.
     */
    VContext withListenSpec(VContext ctx, ListenSpec spec) throws VException;

    /**
     * Returns the {@code ListenSpec} attached to the given context, or {@code null} if no spec
     * is attached.
     * <p>
     * If the passed-in context is derived from the context returned by {@link #getContext},
     * the returned spec will never be {@code null}.
     */
    ListenSpec getListenSpec(VContext ctx);

    /**
     * Returns the base context associated with the runtime.
     * <p>
     * {@link VContext#cancel() Canceling} this context will shut down the runtime,
     * allowing it to to release resources, shutdown services, and the like.
     */
    VContext getContext();

    /**
     * Returns a new {@code Discovery} instance.
     *
     * @param  ctx             current context
     * @throws VException      if a new discovery instance cannot be created
     */
    Discovery newDiscovery(VContext ctx) throws VException;
}
