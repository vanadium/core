// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.rpc;

import com.google.common.util.concurrent.AsyncFunction;
import com.google.common.util.concurrent.Futures;
import com.google.common.util.concurrent.ListenableFuture;
import com.google.common.util.concurrent.SettableFuture;

import io.v.v23.V;
import io.v.v23.context.VContext;
import io.v.v23.naming.GlobReply;
import io.v.v23.vdl.MultiReturn;
import io.v.v23.vdl.ServerSendStream;
import io.v.v23.vdl.VServer;
import io.v.v23.vdl.VdlValue;
import io.v.v23.vdlroot.signature.Interface;
import io.v.v23.verror.CanceledException;
import io.v.v23.verror.VException;

import java.lang.reflect.Constructor;
import java.lang.reflect.Field;
import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;
import java.lang.reflect.ParameterizedType;
import java.lang.reflect.Type;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Map.Entry;
import java.util.concurrent.Executor;

/**
 * An {@link Invoker} that uses reflection to make each compatible exported method in the provided
 * object available.
 *
 * The provided object must implement interface(s) whose methods satisfy the following constraints:
 * <p><ol>
 *     <li>The first in-arg must be a {@link VContext}.</li>
 *     <li>The second in-arg must be a {@link ServerCall}.</li>
 *     <li>For streaming methods, the third in-arg must be a {@link io.v.v23.vdl.ServerStream}.</li>
 *     <li>If the return value is a class annotated with
 *         {@link io.v.v23.vdl.MultiReturn @MultiReturn} annotation, the fields of that class are
 *         interpreted as multiple return values for that method; otherwise, return values are
 *         interpreted as-is.</li>
 *     <li>{@link VException} must be thrown on error.</li>
 * </ol>
 * <p>
 * In addition, the interface must have a corresponding wrapper object and point
 * to it via a {@link io.v.v23.vdl.VServer @VServer} annotation.  This wrapper object unifies the
 * streaming and non-streaming methods under the same set of constraints:
 * <p><ol>
 *     <li>The first in-arg must be {@link VContext}.
 *     <li>The second in-arg must be {@link StreamServerCall}.
 *     <li>{@link VException} is thrown on error.
 * </ol>
 * <p>
 * Each wrapper method should invoke the corresponding interface method.  In addition, a wrapper
 * must provide a constructor that takes the interface as an argument.
 * <p>
 * A wrapper may optionally implement the following methods:
 * <p><ul>
 *     <li>{@code signature()}, which returns the signatures of all server methods.</li>
 *     <li>{@code getMethodTags(method)}, which returns tags for the given method.</li>
 * </ul><p>
 * If a server implements {@link Globber} interface, its {@link Globber#glob glob} method will be
 * invoked on all {@link #glob glob} calls on the {@link Invoker}.
 * <p>
 * Here is an example implementation for the object, as well as the interface and the wrapper.
 * <p>
 * Object:
 * <p><blockquote><pre>
 * public class Server implements ServerInterface, Globber {
 *     public String notStreaming(VContext context, ServerCall call) throws VException { ... }
 *     public String streaming(VContext context, ServerCall call, ServerStream stream)
 *             throws VException { ... }
 *     public void glob(ServerCall call, String pattern, OutputChannel<GlobReply> response)
 *             throws VException { ... }
 * }</pre></blockquote><p>
 * Interface:
 * <p><blockquote><pre>
 * {@literal @}io.v.v23.vdl.VServer(
 *     serverWrapper = ServerWrapper.class
 * )
 * public interface ServerInterface {
 *     String notStreaming(VContext context, ServerCall call) throws VException;
 *     String streaming(VContext context, ServerCall call, ServerStream stream) throws VException;
 * }
 * </pre></blockquote><p>
 * Wrapper:
 * <p><blockquote><pre>
 * public class ServerWrapper {
 *     public ServerWrapper(ServerInterface server) { this.server = server; }
 *     public String notStreaming(VContext context, StreamServerCall call) throws VException {
 *         return this.server.notStreaming(context, call);
 *     }
 *     public String streaming(VContext context, StreamServerCall call) throws VException {
 *         // Generate vdl.ServerStream
 *         return this.server.streaming(context, call, stream);
 *
 *     public Interface signature() {
 *         // Generate signatures for methods streaming() and notStreaming().
 *         return signatures;
 *     }
 *     public VdlValue[] getMethodTags(String method) throws VException {
 *         if ("notStreaming".equals(method)) { return ... }
 *         if ("streaming".equals(method)) { return ... }
 *         throw new VException("Unrecognized method: " + method);
 *     }
 * }</pre></blockquote><p>
 * Typically, the interface and the wrapper will be provided by the vdl generator: users would
 * implement only the object above.
 */
public final class ReflectInvoker implements Invoker {
    // A cache of ClassInfo objects, aiming to reduce the cost of expensive
    // reflection operations.
    private static Map<Class<?>, ClassInfo> serverWrapperClasses =
        new HashMap<Class<?>, ClassInfo>();

    private static final class ServerMethod {
        private final Object wrappedServer;
        private final Method method;
        private final VdlValue[] tags;
        private final Type[] argTypes;
        private final Type[] resultTypes;
        private final Type returnType;

        ServerMethod(Object wrappedServer, Method method, VdlValue[] tags) throws VException {
            this.wrappedServer = wrappedServer;
            this.method = method;
            this.tags = tags != null ? Arrays.copyOf(tags, tags.length) : new VdlValue[0];
            Type[] args = method.getGenericParameterTypes();
            this.argTypes = Arrays.copyOfRange(args, 2, args.length);
            Type returnFutureType = method.getGenericReturnType();
            if (!(returnFutureType instanceof ParameterizedType)) {
                throw new VException(
                        "Couldn't get return parameter type for method: " + method.getName());
            }
            if (((ParameterizedType) returnFutureType).getRawType() != ListenableFuture.class) {
                throw new VException("Server wrapper method must return a ListenableFuture: " +
                        method.getName());
            }
            Type[] returnArgTypes = ((ParameterizedType) returnFutureType).getActualTypeArguments();
            if (returnArgTypes.length != 1) {
                throw new VException("Multiple return parameters for method: " + method.getName());
            }
            this.returnType = returnArgTypes[0];
            if (returnType == Void.class) {
                this.resultTypes = new Type[0];
            } else if (returnType instanceof Class &&
                    ((Class<?>) returnType).getAnnotation(MultiReturn.class) != null) {
                // Multiple return values.
                Field[] fields = ((Class<?>) returnType).getFields();
                this.resultTypes = new Type[fields.length];
                for (int i = 0; i < fields.length; ++i) {
                    this.resultTypes[i] = fields[i].getGenericType();
                }
            } else {
                this.resultTypes = new Type[] { returnType };
            }
        }
        public Method getReflectMethod() {
            return this.method;
        }
        public VdlValue[] getTags() {
            return Arrays.copyOf(this.tags, this.tags.length);
        }
        public Type[] getArgumentTypes() {
            return Arrays.copyOf(this.argTypes, this.argTypes.length);
        }
        public Type[] getResultTypes() {
            return Arrays.copyOf(this.resultTypes, this.resultTypes.length);
        }
        public Type getReturnType() {
            return returnType;
        }
        public Object invoke(Object... args) throws IllegalAccessException,
                IllegalArgumentException, InvocationTargetException {
            return method.invoke(wrappedServer, args);
        }
    }

    private final Map<String, ServerMethod> invokableMethods = new HashMap<String, ServerMethod>();

    private final Map<Object, Method> signatureMethods = new HashMap<Object, Method>();

    private final Object server;

    /**
     * Creates a new {@link ReflectInvoker} object.
     *
     * @param  obj        object whose methods will be invoked
     * @throws VException if the {@link ReflectInvoker} couldn't be created
     */
    public ReflectInvoker(Object obj) throws VException {
        if (obj == null) {
            throw new VException("Can't create ReflectInvoker with a null object.");
        }
        this.server = obj;
        List<Object> serverWrappers = wrapServer(obj);
        for (Object wrapper : serverWrappers) {
            Class<?> c = wrapper.getClass();
            ClassInfo cInfo;
            synchronized (ReflectInvoker.this) {
                cInfo = ReflectInvoker.serverWrapperClasses.get(c);
            }
            if (cInfo == null) {
                cInfo = new ClassInfo(c);

                // Note that multiple threads might decide to create a new
                // ClassInfo and insert it
                // into the cache, but that's just wasted work and not a race
                // condition.
                synchronized (ReflectInvoker.this) {
                    ReflectInvoker.serverWrapperClasses.put(c, cInfo);
                }
            }

            Map<String, Method> methods = cInfo.getMethods();
            Method tagGetter = methods.get("getMethodTags");
            Method signatureMethod = methods.get("signature");
            if (signatureMethod != null) {
                signatureMethods.put(wrapper, signatureMethod);
            }

            for (Entry<String, Method> m : methods.entrySet()) {
                // Make sure that the method signature is correct.
                Type[] argTypes = m.getValue().getGenericParameterTypes();
                if (argTypes.length < 2 ||
                        argTypes[0] != VContext.class || argTypes[1] != StreamServerCall.class) {
                    continue;
                }
                // Get the method tags.
                VdlValue[] tags = null;
                if (tagGetter != null) {
                    try {
                        tags = (VdlValue[])tagGetter.invoke(wrapper, m.getValue().getName());
                    } catch (IllegalAccessException e) {
                        // getMethodTags() not defined.
                    } catch (InvocationTargetException e) {
                        // getMethodTags() threw an exception.
                        throw new VException(String.format("Error getting tag for method %s: %s",
                            m.getKey(), e.getTargetException().getMessage()));
                    }
                }
                invokableMethods.put(m.getKey(), new ServerMethod(wrapper, m.getValue(), tags));
            }
        }
    }

    @Override
    public ListenableFuture<Object[]> invoke(
            final VContext ctx, final StreamServerCall call, final String method, final Object[] args) {
        Executor executor = V.getExecutor(ctx);
        if (executor == null) {
            return Futures.immediateFailedFuture(new VException("NULL executor in context: did " +
                    "you derive server context from the context returned by V.init()?"));
        }
        try {
            final ServerMethod m = findMethod(method);
            final SettableFuture<ListenableFuture<Object[]>> ret = SettableFuture.create();
            // Invoke the method.
            executor.execute(new Runnable() {
                @Override
                public void run() {
                    try {
                        if (ctx.isCanceled()) {
                            ret.setException(new CanceledException(ctx));
                            return;
                        }
                        // Invoke the method and process results.
                        Object[] allArgs = new Object[2 + args.length];
                        allArgs[0] = ctx;
                        allArgs[1] = call;
                        System.arraycopy(args, 0, allArgs, 2, args.length);
                        Object result = m.invoke(allArgs);
                        ret.set(prepareReply(m, result));
                    } catch (InvocationTargetException | IllegalAccessException e) {
                        ret.setException(new VException(String.format(
                                "Error invoking method %s: %s",
                                method, e.getCause().toString())));
                    }
                }
            });
            return Futures.dereference(ret);
        } catch (VException e) {
            return Futures.immediateFailedFuture(e);
        }
    }

    private static ListenableFuture<Object[]> prepareReply(
            final ServerMethod m, Object resultFuture) {
        if (resultFuture == null) {
            return Futures.immediateFailedFuture(new VException(String.format(
                    "Server method %s returned NULL ListenableFuture.",
                    m.getReflectMethod().getName())));
        }
        if (!(resultFuture instanceof ListenableFuture)) {
            return Futures.immediateFailedFuture(new VException(String.format(
                    "Server method %s didn't return a ListenableFuture.",
                    m.getReflectMethod().getName())));
        }
        return Futures.transform((ListenableFuture<?>) resultFuture,
                new AsyncFunction<Object, Object[]>() {
                    @Override
                    public ListenableFuture<Object[]> apply(Object result) throws Exception {
                        Type[] resultTypes = m.getResultTypes();
                        switch (resultTypes.length) {
                            case 0:
                                return Futures.immediateFuture(new Object[0]);  // Void
                            case 1:
                                return Futures.immediateFuture(new Object[]{result});
                            default: {  // Multiple return values.
                                Class<?> returnType = (Class<?>) m.getReturnType();
                                Field[] fields = returnType.getFields();
                                Object[] reply = new Object[fields.length];
                                for (int i = 0; i < fields.length; i++) {
                                    try {
                                        reply[i] = result != null ? fields[i].get(result) : null;
                                    } catch (IllegalAccessException e) {
                                        throw new VException(
                                                "Couldn't get field: " + e.getMessage());
                                    }
                                }
                                return Futures.immediateFuture(reply);
                            }
                        }
                    }
                });
    }

    @Override
    public ListenableFuture<Interface[]> getSignature(VContext ctx) {
        List<Interface> interfaces = new ArrayList<Interface>();
        for (Map.Entry<Object, Method> entry : signatureMethods.entrySet()) {
            try {
                interfaces.add((Interface) entry.getValue().invoke(entry.getKey()));
            } catch (IllegalAccessException e) {
                return Futures.immediateFailedFuture(new VException(String.format(
                        "Could not invoke signature method for server class %s: %s",
                        server.getClass().getName(), e.toString())));
            } catch (InvocationTargetException e) {
                e.printStackTrace();
                return Futures.immediateFailedFuture(new VException(String.format(
                        "Could not invoke signature method for server class %s: %s",
                        server.getClass().getName(), e.toString())));
            }
        }
        return Futures.immediateFuture(interfaces.toArray(new Interface[interfaces.size()]));
    }

    @Override
    public ListenableFuture<io.v.v23.vdlroot.signature.Method> getMethodSignature(
            VContext ctx, final String methodName) {
        return Futures.transform(getSignature(ctx),
                new AsyncFunction<Interface[], io.v.v23.vdlroot.signature.Method>() {
                    @Override
                    public ListenableFuture<io.v.v23.vdlroot.signature.Method> apply(
                            Interface[] interfaces) throws Exception {
                        for (Interface iface : interfaces) {
                            for (io.v.v23.vdlroot.signature.Method method : iface.getMethods()) {
                                if (method.getName().equals(methodName)) {
                                    return Futures.immediateFuture(method);
                                }
                            }
                        }
                        throw new VException(String.format("Could not find method %s", methodName));
                    }
                });
    }

    @Override
    public ListenableFuture<Type[]> getArgumentTypes(VContext ctx, String method) {
        try {
            return Futures.immediateFuture(findMethod(method).getArgumentTypes());
        } catch (VException e) {
            return Futures.immediateFailedFuture(e);
        }
    }

    @Override
    public ListenableFuture<Type[]> getResultTypes(VContext ctx, String method) {
        try {
            return Futures.immediateFuture(findMethod(method).getResultTypes());
        } catch (VException e) {
            return Futures.immediateFailedFuture(e);
        }
    }

    @Override
    public ListenableFuture<VdlValue[]> getMethodTags(VContext ctx, String method) {
        try {
            return Futures.immediateFuture(findMethod(method).getTags());
        } catch (VException e) {
            return Futures.immediateFailedFuture(e);
        }
    }

    @Override
    public ListenableFuture<Void> glob(VContext ctx, ServerCall call, String pattern,
                                       ServerSendStream<GlobReply> stream) {
        if (server instanceof Globber) {
            return ((Globber) server).glob(ctx, call, pattern, stream);
        }
        return Futures.immediateFuture(null);
    }

    private ServerMethod findMethod(String method) throws VException {
        ServerMethod m = this.invokableMethods.get(method);
        if (m == null) {
            throw new VException(String.format("Couldn't find method \"%s\" in class %s",
                    method, server.getClass().getCanonicalName()));
        }
        return m;
    }

    /**
     * Iterates through the Vanadium servers the object implements and generates
     * server wrappers for each.
     */
    private List<Object> wrapServer(Object srv) throws VException {
        List<Object> stubs = new ArrayList<Object>();
        for (Class<?> iface : srv.getClass().getInterfaces()) {
            VServer vs = iface.getAnnotation(VServer.class);
            if (vs == null) {
                continue;
            }
            // There should only be one constructor.
            if (vs.serverWrapper().getConstructors().length != 1) {
                throw new RuntimeException(
                        "Expected ServerWrapper to only have a single constructor");
            }
            Constructor<?> constructor = vs.serverWrapper().getConstructors()[0];

            try {
                stubs.add(constructor.newInstance(srv));
            } catch (InstantiationException e) {
                throw new RuntimeException("Invalid constructor. Problem instanciating.", e);
            } catch (IllegalAccessException e) {
                throw new RuntimeException("Invalid constructor. Illegal access.", e);
            } catch (InvocationTargetException e) {
                throw new RuntimeException("Invalid constructor. Problem invoking.", e);
            }
        }
        if (stubs.size() == 0) {
            throw new VException(
                    "Object does not implement a valid generated server interface.");
        }
        return stubs;
    }

    private static class ClassInfo {
        final Map<String, Method> methods = new HashMap<String, Method>();

        ClassInfo(Class<?> c) throws VException {
            Method[] methodList = c.getDeclaredMethods();
            for (int i = 0; i < methodList.length; i++) {
                Method method = methodList[i];
                Method oldval = null;
                try {
                    oldval = this.methods.put(method.getName(), method);
                } catch (IllegalArgumentException e) {
                } // method not an VDL method.
                if (oldval != null) {
                    throw new VException("Overloading of method " + method.getName()
                            + " not allowed on server wrapper");
                }
            }
        }
        Map<String, Method> getMethods() {
            return this.methods;
        }
    }
}
