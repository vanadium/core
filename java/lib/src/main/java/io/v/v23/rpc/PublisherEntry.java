// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.rpc;

import org.joda.time.DateTime;
import org.joda.time.Duration;

import io.v.v23.verror.VException;

/**
 * The status of the last mount or unmount operation for a server.
 */
public class PublisherEntry {
    private final String name;
    private final String server;
    private final DateTime lastMount;
    private final VException lastMountError;
    private final Duration ttl;
    private final DateTime lastUnmount;
    private final VException lastUnmountError;

    /**
     * Creates a new {@link PublisherEntry} object.
     *
     * @param  name             name under which server is mounted
     * @param  server           address under which server is mounted
     * @param  lastMount        time of the last attempted mount request
     * @param  lastMountError   any error reported by the last attempted mount
     * @param  ttl              TTL supplied for the last mount request
     * @param  lastUnmount      time of the last attempted unmount request
     * @param  lastUnmountError any error reported by the last attempted unmount
     */
    public PublisherEntry(String name, String server, DateTime lastMount,
        VException lastMountError, Duration ttl, DateTime lastUnmount,
        VException lastUnmountError) {
        this.name = name;
        this.server = server;
        this.lastMount = lastMount;
        this.lastMountError = lastMountError;
        this.ttl = ttl;
        this.lastUnmount = lastUnmount;
        this.lastUnmountError = lastUnmountError;
    }

    /**
     * Returns the name under which server is mounted.
     */
    public String getName() {
        return this.name;
    }

    /**
     * Returns the address under which server is mounted.
     */
    public String getServer() {
        return this.server;
    }

    /**
     * Returns the time of the last attempted mount request.
     */
    public DateTime getLastMount() {
        return this.lastMount;
    }

    /**
     * Returns any error reported by the last attempted mount.
     */
    public VException getLastMountError() {
        return this.lastMountError;
    }

    /**
     * Returns the TTL supplied for the last mount request.
     */
    public Duration getTTL() {
        return this.ttl;
    }

    /**
     * Returns the time of the last attempted unmount request.
     */
    public DateTime getLastUnmount() {
        return this.lastUnmount;
    }

    /**
     * Returns any error reported by the last attempted unmount.
     */
    public VException getLastUnmountError() {
        return this.lastUnmountError;
    }

    @Override
    public String toString() {
        return String.format(
            "Name: %s, Server: %s, Mount: %s, Mount Err: %s, TTL: %s, Unmount: %s, Unmount Err: %s",
            this.name, this.server, this.lastMount, this.lastMountError, this.ttl,
            this.lastUnmount, this.lastUnmountError);
    }
}
