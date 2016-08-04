// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.context;

import com.google.common.base.Preconditions;
import com.google.common.util.concurrent.ListenableFuture;

import io.v.impl.google.ListenableFutureCallback;
import io.v.v23.rpc.Callback;
import io.v.v23.verror.VException;
import org.joda.time.DateTime;
import org.joda.time.Duration;

/**
 * The mechanism for carrying deadlines, cancellation, as well as other arbitrary values.
 * <p>
 * Application code receives contexts in two main ways:
 * <p><ol>
 * <li> A {@link VContext} is returned by {@link io.v.v23.V#init V.init}.  This will generally be
 * used to set up servers in {@code main}, or for stand-alone client programs:
 * <p><blockquote><pre>
 *     public static void main(String[] args) {
 *         VContext ctx = V.init();
 *         doSomething(ctx);
 *     }
 * </pre></blockquote><p></li>
 * <li> A {@link VContext} is passed to every server method implementation as the first parameter:
 * <p><blockquote><pre>
 *     public class MyServerImpl implements MyServer {
 *         {@literal @}Override
 *         public void get(VContext ctx, ServerCall call) throws VException;
 *     }
 * </pre></blockquote><p></li></ol>
 * Further contexts can be derived from the original context to refine the environment.
 * For example if part of your operation requires a finer deadline you can create a new context to
 * handle that part:
 * <p><blockquote><pre>
 *    VContext ctx = V.init();
 *    // We'll use cacheCtx to lookup data in memcache; if it takes more than a
 *    // second to get data from memcache we should just skip the cache and
 *    // perform the slow operation.
 *    VContext cacheCtx = ctx.withTimeout(Duration.standardSeconds(1));
 *    try {
 *        fetchDataFromMemcache(cacheCtx, key);
 *    } catch (VException e) {
 *      if (e.is(DEADLINE_EXCEEDED)) {
 *        recomputeData(ctx, key);
 *      }
 *    }
 * </pre></blockquote><p>
 * {@link VContext}s form a tree where derived contexts are children of the contexts from which
 * they were derived.  Children inherit all the properties of their parent except for the property 
 * being replaced (the deadline in the example above).
 * <p>
 * {@link VContext}s are extensible.  The {@link #value value}/{@link #withValue withValue} methods
 * allow you to attach new information to the context and extend its capabilities. In the same way
 * we derive new contexts via the {@code with} family of functions you can create methods to attach
 * new data:
 * <p><blockquote><pre>
 *    public class Auth {
 *        private class AuthKey {}
 *
 *        private static final AuthKey KEY = new AuthKey();
 *
 *        public VContext withAuth(VContext parent, Auth data) {
 *          return parent.withValue(KEY, data);
 *        }
 *
 *        public Auth fromContext(VContext ctx) {
 *            return (Auth)ctx.Value(KEY);
 *        }
 *    }
 * </pre></blockquote><p>
 * Note that any type can be used as a key but the caller should preferably use a private or
 * protected type to prevent collisions (i.e., to prevent somebody else instantiating a key of the
 * same type).  Keys are tested for equality by comparing their value pairs
 * {@code (getClass(), hashCode())}.
 */
public class VContext {
    private static native VContext nativeCreate() throws VException;

    /**
     * The context cancellation reason.
     */
    public enum DoneReason {
        /**
         * Context was explicitly canceled by the user.
         */
        CANCELED,
        /**
         * Context has had its deadline exceeded.
         */
        DEADLINE_EXCEEDED,
    }

    /** Creates a new context with no data attached.
     * <p>
     * This function is meant for use in tests only - the preferred way of obtaining a fully
     * initialized context is through the Vanadium runtime.
     *
     * @return a new root context with no data attached
     */
    public static VContext create() {
        try {
            return nativeCreate();
        } catch (VException e) {
            throw new RuntimeException("Couldn't create new context", e);
        }
    }

    private long nativeRef;
    private long nativeCancelRef;  // may be 0

    private native void nativeCancel(long nativeCancelRef);
    private native boolean nativeIsCanceled(long nativeRef);
    private native DateTime nativeDeadline(long nativeRef) throws VException;
    private native void nativeOnDone(long nativeRef, Callback<DoneReason> callback);
    private native Object nativeValue(long nativeRef, String keySign) throws VException;
    private native VContext nativeWithCancel(long nativeRef) throws VException;
    private native VContext nativeWithDeadline(long nativeRef, DateTime deadline) throws VException;
    private native VContext nativeWithTimeout(long nativeRef, Duration timeout) throws VException;
    private native VContext nativeWithValue(long nativeRef, long nativeCancelRef, String keySign, Object value)
            throws VException;
    private native void nativeFinalize(long nativeRef, long nativeCancelRef);

    protected VContext(long nativeRef, long nativeCancelRef) {
        this.nativeRef = nativeRef;
        this.nativeCancelRef = nativeCancelRef;
    }

    /**
     * Returns {@code true} iff this context is cancelable.
     */
    public boolean isCancelable() {
        return nativeCancelRef != 0;
    }

    /**
     * Returns {@code true} iff the context has been canceled.
     * <p>
     * It is illegal to invoke this method on a context that isn't
     * {@link #isCancelable() cancelable}.
     */
    public boolean isCanceled() {
        return nativeIsCanceled(nativeRef);
    }

    /**
     * Cancels the context.  After this method is invoked, all {@link ListenableFuture}s
     * returned by {@link VContext#onDone done} will get a chance to complete.
     * <p>
     * It is illegal to invoke this method on a context that isn't cancelable.
     */
    public void cancel() {
        Preconditions.checkState(isCancelable(), "Context isn't cancelable.");
        nativeCancel(nativeCancelRef);
    }

    /**
     * Returns a new {@link ListenableFuture} that completes when this context is canceled (either
     * explicitly or via an expired deadline).
     * <p>
     * It is illegal to invoke this method on a context that isn't cancelable.
     * <p>
     * The returned future is guaranteed to be executed on an {@link java.util.concurrent.Executor}
     * specified in this context (see {@link io.v.v23.V#withExecutor}).
     */
    public ListenableFuture<DoneReason> onDone() {
        ListenableFutureCallback<DoneReason> callback = new ListenableFutureCallback<>();
        nativeOnDone(nativeRef, callback);
        return callback.getFutureOnExecutor(this);
    }

    /**
     * Returns the time at which this context will be automatically canceled, or {@code null}
     * if this context doesn't have a deadline.
     */
    public DateTime deadline() {
        try {
            return nativeDeadline(nativeRef);
        } catch (VException e) {
            throw new RuntimeException("Couldn't get deadline", e);
        }
    }

    /**
     * Returns data inside the context associated with the provided key.  See {@link #withValue}
     * for more information on associating data with contexts.
     *
     * @param  key a key value is associated with.
     * @return     value associated with the key.
     */
    public Object value(Object key) {
        try {
            return nativeValue(nativeRef, keySignature(key));
        } catch (VException e) {
            throw new RuntimeException("Couldn't get value: ", e);
        }
    }

    /**
     * Returns a child of the current context that can be canceled.  If the current context
     * is already cancelable, this method will be a no-op.
     * <p>
     * Canceling the returned child context will not affect the current context (unlike
     * {@link #withValue}).
     */
    public VContext withCancel() {
        try {
            return nativeWithCancel(nativeRef);
        } catch (VException e) {
            throw new RuntimeException("Couldn't create cancelable context", e);
        }
    }

    /**
     * Returns a child of the current context that is automatically canceled after the provided
     * deadline is reached.  The returned context can also be canceled manually; this is in fact
     * encouraged when the context is no longer needed in order to free up the timer resources.
     * <p>
     * Canceling the returned child context will not affect the current context (unlike
     * {@link #withValue}).
     *
     * @param  deadline an absolute time after which the context will be canceled.
     * @return          a child of the current context that is automatically canceled after the
     *                  provided deadline is reached
     */
    public VContext withDeadline(DateTime deadline) {
        try {
            return nativeWithDeadline(nativeRef, deadline);
        } catch (VException e) {
            throw new RuntimeException("Couldn't create context with deadline", e);
        }
    }

    /**
     * Returns a child of the current context that is automatically canceled after the provided
     * duration of time.  The returned context can also be canceled manually; this is in fact
     * encouraged when the context is no longer needed in order to free up the timer resources.
     * <p>
     * Canceling the returned child context will not affect the current context (unlike
     * {@link #withValue}).
     *
     * @param  timeout a duration of time after which the context will be canceled.
     * @return         a child of the current context that is automatically canceled after the
     *                 provided duration of time
     */
    public VContext withTimeout(Duration timeout) {
        try {
            return nativeWithTimeout(nativeRef, timeout);
        } catch (VException e) {
            throw new RuntimeException("Couldn't create context with timeout", e);
        }
    }

    /**
     * Returns a child of the current context that additionally contains the given key and its
     * associated value.  A subsequent call to {@link #value value} will return the provided
     * value.  This method should be used only for data that is relevant across multiple API
     * boundaries and not for passing extra parameters to functions and methods.
     * <p>
     * Any type can be used as a key but the caller should preferably use a private or protected
     * type to prevent collisions (i.e., to prevent somebody else instantiating a key of the
     * same type).  Keys are tested for equality by comparing their value pairs:
     * {@code (getClass(), hashCode())}.  The caller shouldn't count on implementation
     * maintaining a reference to the provided key.
     * <p>
     * If the current context is {@link #isCancelable() cancelable}, its child will also be
     * cancelable.  Canceling the child will also cancel the current context (in addition to all
     * the children of that context).
     *
     * @param  key   a key value is associated with.
     * @param  value a value associated with the key.
     * @return       a child of the current context that additionally contains the given key and its
     *               associated value
     */
    public VContext withValue(Object key, Object value) {
        try {
            return nativeWithValue(nativeRef, nativeCancelRef, keySignature(key), value);
        } catch (VException e) {
            throw new RuntimeException("Couldn't create context with data:", e);
        }
    }

    private static String keySignature(Object key) {
        if (key == null) {
            return "";
        }
        return key.getClass().getName() + ":" + key.hashCode();
    }

    private long nativeRef() {
        return nativeRef;
    }

    private long nativeCancelRef() {
        return nativeCancelRef;
    }

    @Override
    protected void finalize() {
        nativeFinalize(nativeRef, nativeCancelRef);
    }
}
