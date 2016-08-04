// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.discovery;

import com.google.common.util.concurrent.ListenableFuture;

import java.util.List;

import javax.annotation.CheckReturnValue;

import io.v.v23.InputChannel;
import io.v.v23.context.VContext;
import io.v.v23.security.BlessingPattern;
import io.v.v23.verror.VException;

/**
 * An interface for discovery operations; it is the client-side library for the discovery service.
 */
public interface Discovery {
    /**
     * Broadcasts the advertisement to be discovered by {@link #scan scan} operations.
     * <p>
     * Visibility is used to limit the principals that can see the advertisement. An
     * empty list means that there are no restrictions on visibility (i.e, equivalent
     * to {@link io.v.v23.security.Constants#ALL_PRINCIPALS}).
     * <p>
     * If {@link Advertisement#id} is not specified, a random unique a random unique
     * identifier will be assigned. Any change to service will not be applied after
     * advertising starts.
     * <p>
     * It is an error to have simultaneously active advertisements for two identical
     * instances (i.e., {@link Advertisement#id}s).
     * <p>
     * Advertising will continue until the context is canceled or exceeds its deadline
     * and the returned {@link ListenableFuture} will complete once it stops.
     * <p>
     * The returned future is guaranteed to be executed on an {@link java.util.concurrent.Executor}
     * specified in {@code context} (see {@link io.v.v23.V#withExecutor}).
     * <p>
     *
     * @param context     a context that will be used to stop advertising
     * @param ad          an advertisement to advertises; this may be update with a random unique
     *                    identifier if ad.id is not specified.
     * @param visibility  a set of blessing patterns for whom this advertisement is meant; any entity
     *                    not matching a pattern here won't know what the advertisement is
     * @return            a new {@link ListenableFuture} that completes once advertising stops
     * @throws VException if advertising couldn't be started
     */
    @CheckReturnValue
    ListenableFuture<Void> advertise(
            VContext context, Advertisement ad, List<BlessingPattern> visibility) throws VException;

    /**
     * Scans advertisements that match the query and returns an {@link InputChannel} of updates.
     * <p>
     * Scan excludes the advertisements that are advertised from the same discovery instance.
     * <p>
     * The query is a {@code WHERE} expression of a {@code syncQL} query against advertisements,
     * where keys are {@link Advertisement#id}s and values are {@link Advertisement}s.
     * <p>
     * Examples:
     * <p><blockquote><pre>
     *     v.InterfaceName = "v.io/i"
     *     v.InterfaceName = "v.io/i" AND v.Attributes["a"] = "v"
     *     v.Attributes["a"] = "v1" OR v.Attributes["a"] = "v2"
     * </pre></blockquote><p>
     * You can find the {@code SyncQL} tutorial at:
     *     https://vanadium.github.io/tutorials/syncbase/syncql-tutorial.html
     * <p>
     * Scanning will continue until the context is canceled or exceeds its deadline. Note that
     * to avoid memory leaks, the caller should drain the channel after cancelling the context.
     *
     * @param context     a context that will be used to stop scanning
     * @param query       a WHERE expression of {@code syncQL query} against scanned advertisements
     * @return            a (potentially-infite) {@link InputChannel} of updates
     * @throws VException if scanning couldn't be started
     */
    @CheckReturnValue
    InputChannel<Update> scan(VContext context, String query) throws VException;
}
