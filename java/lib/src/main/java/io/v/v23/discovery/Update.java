// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.discovery;

import com.google.common.util.concurrent.ListenableFuture;

import java.util.List;

import javax.annotation.CheckReturnValue;

import io.v.v23.context.VContext;
import io.v.v23.verror.VException;

/**
 * Update is the interface for an update from Vanadium discovery scanning.
 */
public interface Update {
    /**
     * Returns true if this update represents a service that is lost during scan.
     */
    boolean isLost();

    /**
     * Returns the universal unique identifier of the discovered advertisement.
     */
    AdId getId();

    /**
     * Returns the interface name of the service.
     */
    String getInterfaceName();

    /**
     * Returns the addresses (vanadium object names) that the service is served on.
     */
    List<String> getAddresses();

    /**
     * Returns the named attribute of the service.
     * <p>
     * Returns null if the named attribute does not exist.
     *
     * @param name  the attachment name
     * @return      the data of the named attribute
     */
    String getAttribute(String name);

    /**
     * Returns a new {@link ListenableFuture} whose result is the data for the named attachment.
     * <p>
     * The returned future completes immediately if the named attachment is already available;
     * otherwise it completes once attachment fetching over RPC is done.
     * <p>
     * The result of future will be null if the named attachment does not exist.
     * <p>
     * The returned future is guaranteed to be executed on an {@link java.util.concurrent.Executor}
     * specified in {@code context} (see {@link io.v.v23.V#withExecutor}).
     * <p>
     * The returned future will fail if the row doesn't exist.
     * <p>
     * The returned {@link ListenableFuture} will fail if there was an error fetching the
     * attachments or {@code context} gets canceled.
     *
     * @param context     a context that will be used to stop fetching; the fetching will end
     *                    when the context is cancelled or timed out
     * @param name        the attachment name
     * @return            a new {@link ListenableFuture} whose result is the data for the named
     *                    attachment
     * @throws VException if attachment couldn't be fetched
     */
    @CheckReturnValue
    ListenableFuture<byte[]> getAttachment(VContext context, String name) throws VException;

    /**
     * Returns the advertisement that this update corresponds to.
     * <p>
     * The returned advertisement may not include all attachments.
     */
    Advertisement getAdvertisement();

    /**
     * Timestamp returns the time when advertising began for the corresponding Advertisement.
     */
    long getTimestampNs();
}
