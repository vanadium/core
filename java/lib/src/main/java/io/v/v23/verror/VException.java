// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.verror;

import io.v.v23.context.VContext;
import io.v.v23.i18n.Language;
import io.v.v23.vdl.Types;
import io.v.v23.vdl.VdlType;

import java.io.Serializable;
import java.lang.reflect.Type;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.concurrent.locks.ReadWriteLock;
import java.util.concurrent.locks.ReentrantReadWriteLock;

/**
 * Captures errors that occurred in the Vanadium environment, both in the core Vanadium code as
 * well as the user-defined clients and servers.
 * <p>
 * Each {@link VException} has an identifier associated with it that uniquely represents an error.
 * Two {@link VException}s are equal iff their identifiers are equal, regardless of the associated
 * messages.  This allows the user to throw {@link VException}s with different messages (e.g.,
 * multiple languages) yet have the caller interpret them all as the same error.
 * <p>
 * To define a new error identifier, for example {@code "someNewError"}, client code is
 * expected to declare a variable like this:
 * <p><blockquote><pre>
 * VException.IDAction someNewError = VException.register(
 *         "my/package/name.someNewError",
 *         VException.ActionCode.NO_RETRY,
 *         "{1} {2} English text for new error");
 * </pre></blockquote><p>
 * Error identifier strings should start with the package path to ensure uniqueness.  Note that the
 * package paths are separated with {@code "/"} delimiter; this is a chosen convention to make
 * the error uniquely identifiable across various programming languages.
 * <p>
 * The purpose of an {@link ActionCode} is so clients not familiar with an error can retry
 * appropriately.
 * <p>
 * Errors are registered with their English text, but the text for other languages can subsequently
 * be added to the default {@link io.v.v23.i18n.Catalog} using the returned error identifier.
 * <p>
 * {@link VException}s are given parameters when created.  Conventionally, the first parameter
 * is the name of the component (typically server or binary name), and the second is the name of the
 * operation (such as an RPC or sub-command) that encountered the error.  Other parameters typically
 * identify the object(s) on which the error occurred.  This convention is normally applied by
 * {@link #VException(IDAction,VContext,Object...)}, which fetches the language, component name
 * and operation name from the current context:
 * <p><blockquote><pre>
 * VException e = new VException(someNewError, ctx, "object_on_which_error_occurred");
 * </pre></blockquote><p>
 * The {@link #VException(IDAction,String,String,String,Object...)} constructor can be used
 * to specify these things explicitly:
 * <p><blockquote><pre>
 * VException e = new VException(
 *         someNewError, "en", "my_component", "op_name", "procedure_name", "object_name");
 * </pre></blockquote><p>
 * If the language, component and/or operation name are unknown, use an empty string.
 * <p>
 * Because of the convention for the first two parameters, error messages in the catalog typically
 * look like this (at least for left-to-right languages):
 * <p><blockquote><pre>
 *      {1} {2} The new error {_}
 * </pre></blockquote><p>
 * Tokens {@code {1}}, {@code {2}}, etc.  refer to the first and second positional
 * parameters respectively, while {@code {_}} is replaced by the positional parameters not
 * explicitly referred to elsewhere in the message.  Thus, given the parameters above, this would
 * lead to the output:
 * <p><blockquote><pre>
 *      my_component op_name The new error object_name
 * </pre></blockquote><p>
 * If a substring is of the form {@code {:<number>}, {<number>:}, {:<number>:},
 * {:_}, {_:}, or {:_:}} and the corresponding parameters are not the empty string, the parameter is
 * preceded by {@code ": "} or followed by {@code ":"} or both, respectively.
 */
public class VException extends Exception {
    private static final long serialVersionUID = 1L;
    private static VContext defaultContext = null;
    private static final ReadWriteLock lock = new ReentrantReadWriteLock();

    /**
     * Action expected to be performed by a typical client receiving an error that perhaps
     * it does not understand.
     */
    public static enum ActionCode {
        /**
         * Do not retry.
         */
        NO_RETRY         (0),
        /**
         * Renew high-level connection/context.
         */
        RETRY_CONNECTION (1),
        /**
         * Refetch and retry (e.g., out of date HTTP ETag).
         */
        RETRY_REFETCH    (2),
        /**
         * Backoff and retry a finite number of times.
         */
        RETRY_BACKOFF    (3);

        /**
         * Returns an {@link ActionCode} corresponding to the given integer value.
         */
        public static ActionCode fromValue(int value) {
            switch (value) {
                case 0: return NO_RETRY;
                case 1: return RETRY_CONNECTION;
                case 2: return RETRY_REFETCH;
                case 3: return RETRY_BACKOFF;
                default: return NO_RETRY;
            }
        }

        private final int value;

        private ActionCode(int value) {
            this.value = value;
        }

        /**
         * Returns the integer value corresponding to this {@link ActionCode}.
         */
        public int getValue() { return this.value; }
    }

    /**
     * A pair of (error identifier, {@link ActionCode}).  The error identifier allows stable error
     * checking across different error messages and different address spaces.
     * <p>
     * By convention the format for the identifier is {@code "PKGPATH.NAME"} - e.g. {@code errIDFoo}
     * defined in the {@code io.v.v23.verror} package has id {@code io.v.v23.verror.errIDFoo}.
     * It is unwise ever to create two {@link IDAction}s that associate different
     * {@link ActionCode}s with the same id.
    */
    public static class IDAction {
        private final String id;
        private final ActionCode action;

        public IDAction(String id, ActionCode action) {
            this.id = id == null ? "" : id;
            this.action = action;
        }

        public String getID() { return this.id; }

        public ActionCode getAction() { return this.action; }

        @Override
        public boolean equals(Object obj) {
            if (this == obj) return true;
            if (obj == null) return false;
            if (this.getClass() != obj.getClass()) return false;
            IDAction other = (IDAction) obj;
            if (!this.id.equals(other.id)) return false;
            if (this.action != other.action) return false;
            return true;
        }

        @Override
        public int hashCode() {
            int result = 1;
            int prime = 31;
            result = prime * result + id.hashCode();
            result = prime * result + action.hashCode();
            return result;
        }

        @Override
        public String toString() {
            return String.format("{ID: %s, Action: %s}", this.id, this.action);
        }
    }

    /**
     * Returns an {@link IDAction} with the given identifier and action fields and
     * inserts a message into the default i18n catalog in US English.  Other languages can be
     * added by directly modifying the catalog.
     *
     * @param  id          error identifier
     * @param  action      error action
     * @param  englishText english message associated with the error
     * @return             {@code IDAction} with the given identifier and action fields
     */
    public static IDAction register(String id, ActionCode action, String englishText) {
        Language.getDefaultCatalog().setWithBase("en-US", id, englishText);
        return new IDAction(id, action);
    }

    /**
     * Sets the default context that is used whenever a user passes in a {@code null} context
     * to the {@link VException} constructors.
     *
     * @param ctx default context
     */
    public static void setDefaultContext(VContext ctx) {
        lock.writeLock().lock();
        defaultContext = ctx;
        lock.writeLock().unlock();
    }

    /**
     * Returns a child of the given context that has the provided component name attached to it.
     *
     * @param  base          base context
     * @param  componentName a component name that is to be attached
     */
    public static VContext contextWithComponentName(VContext base, String componentName) {
        return base.withValue(new ComponentNameKey(), componentName);
    }

    private static VException create(IDAction idAction, String language, String componentName,
            String opName, Object[] params, VdlType[] paramTypes) {
        if (params == null) {
            params = new Object[0];
        }
        if (paramTypes == null) {
            paramTypes = new VdlType[0];
        }
        if (params.length != paramTypes.length) {
            System.err.println(String.format(
                    "Passed different number of types (%s) than parameters (%s) to VException. " +
                    "Some params may be dropped.",
                    Arrays.toString(paramTypes), Arrays.toString(params)));
            int length = params.length <= paramTypes.length ? params.length : paramTypes.length;
            params = Arrays.copyOf(params, length);
            paramTypes = Arrays.copyOf(paramTypes, length);
        }
        // Remove non-serializable params and params with null types.
        List<Serializable> newParams = new ArrayList<>(params.length + 2);
        List<VdlType> newParamTypes = new ArrayList<>(paramTypes.length + 2);
        for (int i = 0; i < params.length; ++i) {
            if (!(params[i] instanceof Serializable)) {
                System.err.println(String.format(
                        "Dropping parameter #%d (%s) that isn't serializable", i, params[i]));
                continue;
            }
            if (paramTypes[i] == null) {
                System.err.println(String.format(
                        "Dropping parameter #%d (%s) whose type doesn't have a matching VdlType.",
                        i, params[i]));
                continue;
            }
            newParams.add((Serializable)params[i]);
            newParamTypes.add(paramTypes[i]);
        }

        // Append componentName and opName to params.
        newParams.add(0, componentName);
        newParamTypes.add(0, Types.STRING);
        newParams.add(1, opName);
        newParamTypes.add(1, Types.STRING);
        String msg = Language.getDefaultCatalog().format(
                    language, idAction.getID(), newParams.toArray());
        return new VException(idAction, msg, newParams.toArray(new Serializable[newParams.size()]),
                newParamTypes.toArray(new VdlType[newParamTypes.size()]));
    }

    private static String componentNameFromContext(VContext ctx) {
        if (ctx == null) {
            lock.readLock().lock();
            ctx = defaultContext;
            lock.readLock().unlock();
        }
        String componentName = "";
        if (ctx != null) {
            Object value = ctx.value(new ComponentNameKey());
            if (value != null && value instanceof String) {
                componentName = (String) value;
            }
        }
        if (componentName.isEmpty()) {
            componentName = System.getProperty("program.name", "");
        }
        if (componentName.isEmpty()) {
            componentName = System.getProperty("user.name", "");
        }
        return componentName;
    }

    private static String languageFromContext(VContext ctx) {
        if (ctx == null) {
            lock.readLock().lock();
            ctx = defaultContext;
            lock.readLock().unlock();
        }
        String language = "";
        if (ctx != null) {
            language = Language.languageFromContext(ctx);
        }
        if (language.isEmpty()) {
            language = "en-US";
        }
        return language;
    }

    private static Type[] createParamTypes(Object[] params) {
        Type[] ret = new Type[params.length];
        for (int i = 0; i < params.length; ++i) {
            ret[i] = params[i] == null ? String.class : params[i].getClass();
        }
        return ret;
    }

    private static VdlType[] convertParamTypes(Type[] types) {
        if (types == null) {
            return null;
        }
        VdlType[] vdlTypes = new VdlType[types.length];
        for (int i = 0; i < types.length; ++i) {
            try {
                vdlTypes[i] = Types.getVdlTypeFromReflect(types[i]);
            } catch (IllegalArgumentException e) {
                System.err.println(String.format(
                        "Couldn't determine VDL type for param reflect type %s.  This param will " +
                        "be dropped if ever VOM-encoded", types[i]));
                vdlTypes[i] = null;
            }
        }
        return vdlTypes;
    }

    private final IDAction id;  // non-null
    private final Object[] params;  // non-null
    private final VdlType[] paramTypes;  // non-null, same length as params

    /**
     * Creates a new {@link UnknownException} error in English and empty
     * component/operation names.
     *
     * @param  msg error message
     */
    public VException(String msg) {
        this(UnknownException.ID_ACTION, (VContext) null, msg);
    }

    /**
     * Same as {@link #VException(IDAction,String,String,String,Object...)} but
     * obtains the language, component name, and operation name from the given context.  If the
     * provided context is {@code null}, default values are used.
     */
    public VException(IDAction idAction, VContext ctx, Object... params) {
        this(idAction, ctx, params, createParamTypes(params));
    }

    /**
     * Same as {@link #VException(IDAction,VContext,Object...)} but explicitly provides the
     * types for all parameters.  This is necessary as parameters may need to be VOM-encoded (to be
     * shipped across the wire), and Java doesn't always have the ability to deduce the right type
     * from the value (e.g., generic parameters like {@code List<String>} or
     * {@code Map<String, Integer>}).
     */
    public VException(IDAction idAction, VContext ctx, Object[] params, Type[] paramTypes) {
        // TODO(spetrovic): implement the opName support.
        this(idAction, languageFromContext(ctx), componentNameFromContext(ctx),
                "", params, paramTypes);
    }

    /**
     * Returns an error with the given identifier and an error string in the chosen language.
     * The component and operation name are included the first and second parameters of the error.
     * Other parameters are taken from {@code params}. The parameters are formatted into the message
     * according to the format described in the {@link VException} documentation.
     *
     * @param  idAction      error identifier
     * @param  language      error language, in IETF format
     * @param  componentName error component name
     * @param  opName        error operation name
     * @param  params        error message parameters
     */
    public VException(IDAction idAction, String language, String componentName, String opName,
            Object... params) {
        this(idAction, language, componentName, opName, params, createParamTypes(params));
    }

    /**
     * Same as {@link #VException(IDAction,String,String,String,Object...)} but
     * explicitly provides the types for all parameters.  This is necessary as parameters may need
     * to be VOM-encoded (to be shipped across the wire), and Java doesn't always have the ability
     * to deduce the right type from the value (e.g., generic parameters like {@code List<String>}
     * or {@code Map<String, Integer>}).
     */
    public VException(IDAction idAction, String language, String componentName, String opName,
            Object[] params, Type[] paramTypes) {
        this(create(idAction, language, componentName,
                opName, params, convertParamTypes(paramTypes)));
    }

    VException(IDAction id, String msg, Object[] params, VdlType[] paramTypes) {
        super(msg);
        this.id = id;
        this.params = params;
        this.paramTypes = paramTypes;
    }

    protected VException(VException other) {
        this(other.id, other.getMessage(), other.params, other.paramTypes);
    }

    /**
     * Returns the error identifier associated with this {@link VException}.
     */
    public String getID() {
        return this.id.getID();
    }

    /**
     * Returns the action associated with this {@link VException}.
     */
    public ActionCode getAction() {
        return this.id.getAction();
    }

    /**
     * Returns true iff the error identifier associated with this {@link VException} is equal to the provided
     * identifier.
     *
     * @param  id the error identifier we're comparing with this error
     */
    public boolean is(String id) {
        return getID().equals(id);
    }

    /**
     * Returns true iff the error identifier associated with this {@link VException} is equal to the provided
     * identifier.
     *
     * @param  idAction the error identifier we're comparing with this error
     */
    public boolean is(IDAction idAction) {
        return is(idAction.getID());
    }

    /**
     * Returns true if this {@link VException} is deeply equal to the provided {@link Object}.
     * Unlike {@link #equals equals}, this method, in addition to comparing identifiers, also
     * compares {@link ActionCode}s and parameters.
     *
     * @param  obj the other object we are testing for equality
     */
    public boolean deepEquals(Object obj) {
        if (!equals(obj)) return false;
        VException other = (VException) obj;
        // equals() has already compared the IDs.
        if (!getAction().equals(other.getAction())) return false;
        if (!Arrays.deepEquals(getParams(), other.getParams())) return false;
        return Arrays.equals(getParamTypes(), other.getParamTypes());
    }

    private static class ComponentNameKey {
        @Override
        public int hashCode() {
            return 0;
        }
    }

    @Override
    public boolean equals(Object obj) {
        if (this == obj) return true;
        if (obj == null) return false;
        if (!(obj instanceof VException)) return false;
        VException other = (VException) obj;
        return this.getID().equals(other.getID());
    }

    @Override
    public int hashCode() {
        return this.id.hashCode();
    }

    Object[] getParams() { return this.params; }

    VdlType[] getParamTypes() { return this.paramTypes; }
}
