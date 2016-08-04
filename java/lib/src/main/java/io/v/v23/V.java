// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23;

import com.google.common.io.ByteStreams;
import com.google.common.io.Resources;

import java.io.File;
import java.io.FileOutputStream;
import java.io.IOException;
import java.net.URL;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.Executor;
import java.util.concurrent.Executors;

import io.v.impl.google.rt.VRuntimeImpl;
import io.v.v23.context.VContext;
import io.v.v23.discovery.Discovery;
import io.v.v23.namespace.Namespace;
import io.v.v23.options.RpcServerOptions;
import io.v.v23.rpc.Client;
import io.v.v23.rpc.Dispatcher;
import io.v.v23.rpc.ListenSpec;
import io.v.v23.rpc.Server;
import io.v.v23.security.Authorizer;
import io.v.v23.security.CaveatRegistry;
import io.v.v23.security.ConstCaveatValidator;
import io.v.v23.security.Constants;
import io.v.v23.security.ExpiryCaveatValidator;
import io.v.v23.security.MethodCaveatValidator;
import io.v.v23.security.PublicKeyThirdPartyCaveatValidator;
import io.v.v23.security.VPrincipal;
import io.v.v23.verror.VException;

/**
 * The local environment allowing clients and servers to communicate with one another.  The expected
 * usage pattern of this class goes something like this:
 * <p><blockquote><pre>
 *     VContext ctx = V.init(opts);
 *     ...
 *     ctx = V.withNewServer(ctx, "server", obj, null);
 *     ...
 *     Client c = V.getClient(ctx);
 *     ...
 * </pre></blockquote><p>
 */
public class V {
    private static native void nativeInitGlobalShared() throws VException;
    private static native void nativeInitGlobalJava(Options opts) throws VException;

    private static volatile VRuntime runtime;
    private static volatile VContext globalContext;

    private static boolean isDarwin() {
        return System.getProperty("os.name").toLowerCase().contains("os x");
    }

    private static boolean isLinux() {
        return System.getProperty("os.name").toLowerCase().contains("linux");
    }

    // Initializes the Vanadium global state, i.e., state that needs to be cleaned up only
    // before the process is about to terminate.  This method is shared between Java and Android
    // implementations.
    protected static VContext initGlobalShared(Options opts) {
        System.err.println("java.library.path = " + System.getProperty("java.library.path"));
        List<Throwable> errors = new ArrayList<Throwable>();
        try {
            // First, attempt to find the library in java.library.path.
            System.loadLibrary("v23");
        } catch (UnsatisfiedLinkError ule) {
            // Thrown if the library does not exist. In this case, try to find it in our classpath.
            errors.add(new RuntimeException("loadLibrary attempt failed", ule));
            try {
                URL resource = null;
                File file = null;
                if (isLinux()) {
                    resource = Resources.getResource("libv23.so");
                    file = File.createTempFile("libv23-", ".so");
                } else if (isDarwin()) {
                    resource = Resources.getResource("libv23.dylib");
                    file = File.createTempFile("libv23-", ".dylib");
                } else {
                    String os = System.getProperty("os.name");
                    errors.add(new RuntimeException("unsupported OS: " + os));
                    throw new RuntimeException("Unsupported OS: " + os, new VLoaderException(errors));
                }
                file.deleteOnExit();
                ByteStreams.copy(resource.openStream(), new FileOutputStream(file));
                System.load(file.getAbsolutePath());
            } catch (IllegalArgumentException iae) {
                errors.add(new RuntimeException("couldn't locate libv23.so on the classpath", iae));
                throw new RuntimeException("Could not load v23 native library", new VLoaderException(errors));
            } catch (IOException e) {
                errors.add(new RuntimeException("error while reading libv23.so from the classpath", e));
                throw new RuntimeException("Could not load v23 native library", new VLoaderException(errors));
            } catch (UnsatisfiedLinkError e) {
                errors.add(new RuntimeException("error while reading libv23.so from the classpath", e));
                throw new RuntimeException("Could not load v23 native library", new VLoaderException(errors));
            }
        }
        try {
            nativeInitGlobalShared();
        } catch (VException e) {
            throw new RuntimeException("Could not initialize v23 native library", e);
        }
        try {
            CaveatRegistry.register(
                    io.v.v23.security.Constants.CONST_CAVEAT,
                    ConstCaveatValidator.INSTANCE);
            CaveatRegistry.register(
                    io.v.v23.security.Constants.EXPIRY_CAVEAT,
                    ExpiryCaveatValidator.INSTANCE);
            CaveatRegistry.register(io.v.v23.security.Constants.METHOD_CAVEAT,
                    MethodCaveatValidator.INSTANCE);
            CaveatRegistry.register(Constants.PUBLIC_KEY_THIRD_PARTY_CAVEAT,
                    PublicKeyThirdPartyCaveatValidator.INSTANCE);
        } catch (VException e) {
            throw new RuntimeException("Couldn't register caveat validators", e);
        }
        // See if a runtime was provided as an option.
        if (opts.get(OptionDefs.RUNTIME) != null) {
            runtime = opts.get(OptionDefs.RUNTIME, VRuntime.class);
        } else {
            // Use the default runtime implementation.
            try {
                runtime = VRuntimeImpl.create(opts);
            } catch (VException e) {
                throw new RuntimeException("Couldn't initialize Google Vanadium Runtime", e);
            }
        }
        VContext ctx = runtime.getContext();
        if (!ctx.isCancelable()) {
            throw new RuntimeException("Context returned by the runtime must be cancelable");
        }
        return ctx;
    }

    // Initializes the Vanadium Java-specific global state.
    private static VContext initGlobalJava(VContext ctx, Options opts) {
        try {
            nativeInitGlobalJava(opts);
            ctx = V.withExecutor(ctx, Executors.newCachedThreadPool());
            return ctx;
        } catch (VException e) {
            throw new RuntimeException("Couldn't initialize Java", e);
        }
    }

    // Initializes the Vanadium global state
    private static VContext initGlobal(Options opts) {
        if (globalContext != null) {
            return globalContext;
        }
        synchronized (V.class) {
            if (globalContext != null) {
                return globalContext;
            }
            if (opts == null) opts = new Options();
            VContext ctx = initGlobalShared(opts);
            ctx = initGlobalJava(ctx, opts);
            // Set the VException component name to this binary name.
            ctx = VException.contextWithComponentName(ctx, System.getProperty("program.name", ""));
            globalContext = ctx;
            return ctx;
        }
    }

    /**
     * Initializes the Vanadium environment, returning the base context.  This method may be
     * called multiple times: in each invocation, a different base context will be returned.
     * <p>
     * {@link VContext#cancel() Canceling} the returned context will release all the Vanadium
     * resources and shut down all services associated with the context.  Forgetting to cancel
     * the context will therefore result in memory leaks of those resources.
     * <p>
     * A caller may pass the following option that specifies the runtime implementation to be used:
     * <p><ul>
     *     <li>{@link OptionDefs#RUNTIME}</li>
     * </ul><p>
     * If this option isn't provided, the default runtime implementation is used, which supports
     * the following options:
     * <p><ul>
     *     <li>(CURRENTLY NO OPTIONS ARE MANDATED)</li>
     * </ul></p>
     *
     * @param  opts options
     * @return      base context
     */
    private static synchronized VContext init(Options opts) {
        VContext ctx = initGlobal(opts);
        return ctx.withCancel();
    }

    /**
     * Initializes the Vanadium environment without options.  See {@link #init(Options)} for more
     * information.
     *
     * @return base context
     */
    public static VContext init() {
        return V.init(null);
    }

    /**
     * Creates a new client instance and attaches it to a new context (which is derived from the
     * provided context).
     *
     * @param  ctx             current context
     * @return                 child context to which the new client is attached
     * @throws VException      if a new client cannot be created
     */
    public static VContext withNewClient(VContext ctx) throws VException {
        return withNewClient(ctx, null);
    }

    /**
     * Creates a new client instance with the provided options and attaches it to a new context
     * (which is derived from the provided context).
     * <p></p>
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
    public static VContext withNewClient(VContext ctx, Options opts) throws VException {
        if (opts == null) opts = new Options();
        return getRuntime(ctx).withNewClient(ctx, opts);
    }

    /**
     * Returns the client attached to the given context, or {@code null} if no client is attached.
     * <p>
     * If the passed-in context is derived from the context returned by {@link #init}, the returned
     * client will never be {@code null}.
     */
    public static Client getClient(VContext ctx) {
        return getRuntime(ctx).getClient(ctx);
    }

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
     *
     * @param  ctx             current context
     * @param  name            name under which the supplied object should be published,
     *                         or the empty string if the object should not be published
     * @param  object          object to be published under the given name
     * @param  authorizer      authorizer that will control access to objects methods
     * @return                 a child context to which the new server is attached
     * @throws VException      if a new server cannot be created
     */
    public static VContext withNewServer(VContext ctx, String name, Object object,
                                         Authorizer authorizer) throws VException {
        return withNewServer(ctx, name, object, authorizer, (RpcServerOptions) null);
    }

    /**
     * Creates a new {@link Server} instance to serve a service object and attaches
     * it to a new context (which is derived from the provided context).
     *
     * Same as {@link #withNewServer(VContext, String, Object, Authorizer)}
     * but accepts implementation-specific server-creation options.
     * <p>
     * A particular runtime implementation chooses which options to support, but at the minimum it
     * must handle the following options:
     * <p><ul>
     *     <li>(CURRENTLY NO OPTIONS ARE MANDATED)</li>
     * </ul>
     *
     * @param  ctx             current context
     * @param  name            name under which the supplied object should be published,
     *                         or the empty string if the object should not be published
     * @param  object          object to be published under the given name
     * @param  authorizer            authorizer that will control access to objects methods
     * @param  opts            server options
     * @return                 a child context to which the new server is attached
     * @throws VException      if a new server cannot be created
     */
    public static VContext withNewServer(
            VContext ctx, String name, Object object, Authorizer authorizer, RpcServerOptions opts)
            throws VException {
        if (opts == null) {
            opts = new RpcServerOptions();
        }
        return getRuntime(ctx).withNewServer(ctx, name, object, authorizer, opts);
    }

    /**
     * @deprecated Use
     *  {@link #withNewServer(VContext, String, Object, Authorizer, RpcServerOptions)} instead.
     */
    public static VContext withNewServer(VContext ctx, String name, Object object,
                                         Authorizer authorizer, Options opts) throws VException {
        return withNewServer(ctx, name, object, authorizer, RpcServerOptions.migrateOptions(opts));
    }

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
     *
     * @param  ctx             current context
     * @param  name            name under which the supplied object should be published,
     *                         or the empty string if the object should not be published
     * @param  dispatcher      dispatcher to be published under the given name
     * @return                 a child context to which the new server is attached
     * @throws VException      if a new server cannot be created
     */
    public static VContext withNewServer(VContext ctx, String name,
                                         Dispatcher dispatcher) throws VException {
        return withNewServer(ctx, name, dispatcher, (RpcServerOptions) null);
    }

    /**
     * Creates a new {@link Server} instance to serve a dispatcher and attaches
     * it to a new context (which is derived from the provided context).
     *
     * Same as {@link #withNewServer(VContext,String,Dispatcher)} but accepts
     * implementation-specific server-creation options.
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
    public static VContext withNewServer(VContext ctx, String name, Dispatcher dispatcher,
                                         RpcServerOptions opts) throws VException {
        if (opts == null) {
            opts = new RpcServerOptions();
        }
        return getRuntime(ctx).withNewServer(ctx, name, dispatcher, opts);
    }

    /**
     * @deprecated Use {@link #withNewServer(VContext, String, Dispatcher, RpcServerOptions)}
     */
    public static VContext withNewServer(VContext ctx, String name, Dispatcher dispatcher,
                                         Options opts) throws VException {
        return withNewServer(ctx, name, dispatcher, RpcServerOptions.migrateOptions(opts));
    }

    /**
     * Returns the server attached to the given context, or {@code null} if no server is attached.
     */
    public static Server getServer(VContext ctx) {
        return getRuntime(ctx).getServer(ctx);
    }

    /**
     * Attaches the given principal to a new context (which is derived from the provided context).
     *
     * @param  ctx             current context
     * @param  principal       principal to be attached
     * @return                 child context to which the principal is attached
     * @throws VException      if the principal couldn't be attached
     */
    public static VContext withPrincipal(VContext ctx, VPrincipal principal) throws VException {
        return getRuntime(ctx).withPrincipal(ctx, principal);
    }

    /**
     * Returns the principal attached to the given context or {@code null} if no principal is
     * attached.
     * <p>
     * If the passed-in context is derived from the context returned by {@link #init}, the returned
     * principal will never be {@code null}.
     */
    public static VPrincipal getPrincipal(VContext ctx) {
        return getRuntime(ctx).getPrincipal(ctx);
    }

    /**
     * Creates a new namespace instance and attaches it to a new context (which is derived from the
     * given context).
     *
     * @param  ctx             current context
     * @param  roots           roots of the namespace
     * @return                 child context to which the principal is attached
     * @throws VException      if the namespace couldn't be created
     */
    public static VContext withNewNamespace(VContext ctx, String... roots) throws VException {
        return getRuntime(ctx).withNewNamespace(ctx, roots);
    }

    /**
     * Returns the namespace attached to the given context, or {@code null} if no namespace is
     * attached.
     * <p>
     * If the passed-in context is derived from the context returned by {@link #init}, the returned
     * namespace will never be {@code null}.
     */
    public static Namespace getNamespace(VContext ctx) {
        return getRuntime(ctx).getNamespace(ctx);
    }

    /**
     * Attaches the given {@link ListenSpec} to a new context (which is derived from the given
     * context).
     *
     * @param ctx        current context
     * @param spec       the {@link ListenSpec} to attach
     * @return           child context to which the {@link ListenSpec} is attached.
     */
    public static VContext withListenSpec(VContext ctx, ListenSpec spec) throws VException {
        return getRuntime(ctx).withListenSpec(ctx, spec);
    }

    /**
     * Returns the {@link ListenSpec} attached to the given context, or {@code null} if no spec
     * is attached.
     * <p>
     * If the passed-in context is derived from the context returned by {@link #init}, the returned
     * spec will never be {@code null}.
     */
    public static ListenSpec getListenSpec(VContext ctx) {
        return getRuntime(ctx).getListenSpec(ctx);
    }

    /**
     * Returns a new {@link Discovery} instance.
     *
     * @param  ctx             current context
     * @throws VException      if a new discovery instance cannot be created
     */
    public static Discovery newDiscovery(VContext ctx) throws VException {
        return getRuntime(ctx).newDiscovery(ctx);
    }

    /**
     * Attaches the given {@link Executor} to a new context (which is derived from the given
     * context).  This executor will be used for all callbacks into the user code.
     * <p>
     * If this method is never invoked, a default executor for the platform will be used.
     */
    public static VContext withExecutor(VContext ctx, Executor executor) {
        return ctx.withValue(new ExecutorKey(), executor);
    }

    /**
     * Returns the {@link Executor} attached to the given context, or {@code null} if no
     * {@link Executor} is attached.
     * <p>
     * If the passed-in context is derived from the context returned by {@link #init}, the returned
     * instance will never be {@code null}.
     */
    public static Executor getExecutor(VContext ctx) {
        return (Executor) ctx.value(new ExecutorKey());
    }

    private static VRuntime getRuntime(VContext ctx) {
        if (runtime == null) {
            throw new RuntimeException("Vanadium runtime is null: did you call V.init()?");
        }
        return runtime;
    }

    private static class ExecutorKey {
        @Override
        public int hashCode() {
            return 0;
        }
    }

    protected V() {}
}
