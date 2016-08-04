// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.rpc;

import org.joda.time.DateTime;

import io.v.v23.verror.VException;

import java.util.Arrays;

/**
 * The changes made in response to a network setting change being received.
 */
public class NetworkChange {
    private final DateTime time;
    private final ServerState state;
    private final NetworkAddress[] addedAddrs;
    private final NetworkAddress[] removedAddrs;
    private final String[] changedEndpoints;
    private final VException error;

    /**
     * Creates a new {@link NetworkChange} object.
     *
     * @param  time             time of the last change
     * @param  state            current state of the server
     * @param  addedAddrs       addresses added since the last change
     * @param  removedAddrs     addresses removed since the last change
     * @param  changedEndpoints list of endpoints added/removed as a result of this change
     * @param  error            any error encountered
     */
    public NetworkChange(DateTime time, ServerState state, NetworkAddress[] addedAddrs,
            NetworkAddress[] removedAddrs, String[] changedEndpoints, VException error) {
        this.time = time;
        this.state = state;
        this.addedAddrs = Arrays.copyOf(addedAddrs, addedAddrs.length);
        this.removedAddrs = Arrays.copyOf(removedAddrs, removedAddrs.length);
        this.changedEndpoints = Arrays.copyOf(changedEndpoints, changedEndpoints.length);
        this.error = error;
    }

    /**
     * Returns the time of the last change.
     */
    public DateTime getTime() { return this.time; }

    /**
     * Returns the current state of the server.
     */
    public ServerState getState() { return this.state; }

    /**
     * Returns the addresses added since the last change.
     */
    public NetworkAddress[] getAddedAddresses() {
        return Arrays.copyOf(this.addedAddrs, this.addedAddrs.length);
    }

    /**
     * Returns the addresses removed since the last change.
     */
    public NetworkAddress[] getRemovedAddresses() {
        return Arrays.copyOf(this.removedAddrs, this.removedAddrs.length);
    }
    /**
     * Returns the list of endpoints added/removed as a result of this change.
     */
    public String[] getChangedEndpoints() {
        return Arrays.copyOf(this.changedEndpoints, this.changedEndpoints.length);
    }
    /**
     * Returns any error encountered.
     */
    public VException getError() { return this.error; }

    @Override
    public String toString() {
        return String.format("{Time: %s, State: %s, Added addrs: %s, Removed addrs: %s, " +
            "Changed EPs: %s, Error: %s}", this.time, this.state, Arrays.toString(this.addedAddrs),
            Arrays.toString(this.removedAddrs), Arrays.toString(this.changedEndpoints), this.error);
    }
}
