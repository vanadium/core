// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.namespace;

import com.google.common.util.concurrent.ListenableFuture;

import org.joda.time.Duration;

import java.util.List;
import java.util.Map;

import javax.annotation.CheckReturnValue;

import io.v.v23.InputChannel;
import io.v.v23.Options;
import io.v.v23.context.VContext;
import io.v.v23.naming.GlobReply;
import io.v.v23.naming.MountEntry;
import io.v.v23.security.access.Permissions;
import io.v.v23.verror.VException;

/**
 * Translation from object names to server object addresses.  It represents the interface to a
 * client side library for the {@code MountTable} service.
 */
public interface Namespace {
    /**
     * A shortcut for {@link #mount(VContext, String, String, Duration, Options)} with a {@code
     * null} options parameter.
     */
    @CheckReturnValue
    ListenableFuture<Void> mount(VContext context, String name, String server, Duration ttl);

    /**
     * Mounts the server object address under the object name, expiring after {@code ttl};
     * {@code ttl} of zero implies an implementation-specific high value (essentially forever).
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
     * @param context a client context
     * @param name a Vanadium name, see also <a href="https://vanadium.github.io/glossary.html#object-name">the
     *             Name entry</a> in the glossary
     * @param server an object address, see also
     *               <a href="https://vanadium.github.io/concepts/naming.html#object-names">the Object names</a>
     *               section of the Naming Concepts document
     * @param ttl the duration for which the mount should live
     * @param options options to pass to the implementation as described above, or {@code null}
     */
    @CheckReturnValue
    ListenableFuture<Void> mount(VContext context, String name, String server, Duration ttl,
                                 Options options);

    /**
     * A shortcut for {@link #unmount(VContext, String, String, Options)} with a {@code null}
     * options parameter.
     */
    @CheckReturnValue
    ListenableFuture<Void> unmount(VContext context, String name, String server);

    /**
     * Unmounts the server object address from the object name, or if {@code server} is empty,
     * unmounts all server object addresses from the object name.
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
     * @param context a client context
     * @param name a Vanadium name, see also <a href="https://vanadium.github.io/glossary.html#object-name">the
     *             Name entry</a> in the glossary
     * @param server an object address, see also
     *               <a href="https://vanadium.github.io/concepts/naming.html#object-names">the Object names</a>
     *               section of the Naming Concepts document
     * @param options options to pass to the implementation as described above, or {@code null}
     */
    @CheckReturnValue
    ListenableFuture<Void> unmount(VContext context, String name, String server, Options options);

    /**
     * A shortcut for {@link #delete(VContext, String, boolean, Options)} with a {@code null}
     * options parameter.
     */
    @CheckReturnValue
    ListenableFuture<Void> delete(VContext context, String name, boolean deleteSubtree);

    /**
     * Deletes the name from a mount table. If the name has any children in its mount table, it
     * (and its children) will only be removed if {@code deleteSubtree} is true.
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
     * @param context a client context
     * @param name the Vanadium name to delete, see also
     *             <a href="https://vanadium.github.io/glossary.html#object-name">the Name entry</a> in the
     *             glossary
     * @param deleteSubtree whether the entire tree rooted at {@code name} should be deleted
     * @param options options to pass to the implementation as described above, or {@code null}
     */
    @CheckReturnValue
    ListenableFuture<Void> delete(VContext context, String name, boolean deleteSubtree,
                                  Options options);

    /**
     * A shortcut for {@link #resolve(VContext, String, Options)} with a {@code null} options
     * parameter.
     */
    @CheckReturnValue
    ListenableFuture<MountEntry> resolve(VContext context, String name);

    /**
     * Resolves the object name into its mounted servers.
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
     * @param context a client context
     * @param name the Vanadium name to resolve, see also
     *             <a href="https://vanadium.github.io/glossary.html#object-name">the Name entry</a> in the
     *             glossary
     * @param options options to pass to the implementation as described above, or {@code null}
     * @return a new {@link ListenableFuture} whose result is the {@link MountEntry} to which the
     *         name resolves, or {@code null} if it does not resolve
     */
    @CheckReturnValue
    ListenableFuture<MountEntry> resolve(VContext context, String name, Options options);

    /**
     * A shortcut for {@link #resolveToMountTable(VContext, String, Options)} with a {@code null}
     * options parameter.
     */
    @CheckReturnValue
    ListenableFuture<MountEntry> resolveToMountTable(VContext context, String name);

    /**
     * Resolves the object name into the mounttables directly responsible for the name.
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
     * @param context a client context
     * @param name the Vanadium name to resolve, see also
     *             <a href="https://vanadium.github.io/glossary.html#object-name">the Name entry</a> in the
     *             glossary
     * @param options options to pass to the implementation as described above, or {@code null}
     * @return a new {@link ListenableFuture} whose result is the {@link MountEntry} of the
     *         mounttable server directly responsible for {@code name}, or {@code null} if
     *         {@code name} does not resolve
     */
    @CheckReturnValue
    ListenableFuture<MountEntry> resolveToMountTable(VContext context, String name,
                                                     Options options);

    /**
     * Flushes resolution information cached for the given name. If anything was flushed it returns
     * {@code true}.
     * <p>
     * This is a non-blocking method.
     *
     * @param context a client context
     * @param name a Vanadium name, see also <a href="https://vanadium.github.io/glossary.html#object-name">the
     *             Name entry</a> in the glossary
     * @return {@code true} iff resolution information for the name was successfully flushed
     */
    boolean flushCacheEntry(VContext context, String name);

    /**
     * A shortcut for {@link #glob(VContext, String, Options)} with a {@code null} options
     * parameter.
     */
    InputChannel<GlobReply> glob(VContext context, String pattern);

    /**
     * Returns a channel over all names matching the provided pattern.
     * <p>
     * A particular implementation of this interface chooses which options to support, but at the
     * minimum it must handle the following pre-defined options:
     * <ul>
     *     <li>{@link io.v.v23.OptionDefs#SKIP_SERVER_ENDPOINT_AUTHORIZATION}</li>
     * </ul>
     * <p>
     * {@link io.v.v23.context.VContext#cancel Canceling} the provided context will
     * stop the glob operation and cause the channel to stop producing elements. Note that to avoid
     * memory leaks, the caller should drain the channel after cancelling the context.
     *
     * @param context a client context
     * @param pattern a pattern that should be matched
     * @param options options to pass to the implementation as described above, or {@code null}
     * @return        an {@link InputChannel} of {@link GlobReply} objects matching the provided
     *                pattern
     */
    InputChannel<GlobReply> glob(VContext context, String pattern, Options options);

    /**
     * Sets the roots that the local namespace is relative to.
     * <p>
     * All relative names passed to the other methods in this class will be interpreted as
     * relative to these roots. The roots will be tried in the order that they are specified in
     * {@code roots} list. Calling this method with an empty list will clear the currently
     * configured set of roots.
     * <p>
     * This is a non-blocking method.
     *
     * @param roots the roots that will be used to turn relative paths into absolute paths, or
     *              {@code null} to clear the currently configured set of roots. Each entry should
     *              be a Vanadium name, see also
     *              <a href="https://vanadium.github.io/glossary.html#object-name">
     *              the Name entry</a> in the glossary
     */
    void setRoots(List<String> roots) throws VException;

    /**
     * A shortcut for {@link #setPermissions(VContext, String, Permissions, String, Options)} with a
     * {@code null} options parameter.
     */
    @CheckReturnValue
    ListenableFuture<Void> setPermissions(VContext context, String name, Permissions permissions,
                                          String version);

    /**
     * Sets the permissions in a node in a mount table. If the caller tries to set a permission that
     * removes them from {@link io.v.v23.security.access.Constants#ADMIN}, the caller's original
     * admin blessings will be retained.
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
     * @param context a client context
     * @param name the name of the node receiving the new permissions
     * @param permissions the permissions to set on the node
     * @param version a string containing the current permissions version number (e.g. {@code "4"}.
     *                If this value does not match the version known to the receiving server,
     *                a {@link VException} is thrown indicating that this call had no effect. If the
     *                version number is not specified, no version check is performed
     * @param options options to pass to the implementation as described above, or {@code null}
     */
    @CheckReturnValue
    ListenableFuture<Void> setPermissions(VContext context, String name, Permissions permissions,
                                          String version, Options options);

    /**
     * A shortcut for {@link #getPermissions(VContext, String, Options)} with a {@code null} options
     * parameter.
     */
    @CheckReturnValue
    ListenableFuture<Map<String, Permissions>> getPermissions(VContext context, String name);

    /**
     * Returns a new {@link ListenableFuture} whose result are the Permissions in a node in a mount
     * table. The returned map will contain a single entry whose key is the permissions version
     * (see {@link #setPermissions(VContext, String, Permissions, String, Options)}) and whose value
     * is the permissions corresponding to that version.
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
     * @param context a client context
     * @param name the name of the node
     * @param options options to pass to the implementation as described above, or {@code null}
     * @return a single-entry map from permissions version to permissions for the named object
     */
    @CheckReturnValue
    ListenableFuture<Map<String, Permissions>> getPermissions(VContext context, String name,
                                                              Options options);
}
