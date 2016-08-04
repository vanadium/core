// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.naming;

import java.util.List;

import io.v.v23.rpc.NetworkAddress;

/**
 * Unique identifiers for entities communicating over a network. End users
 * don't use endpoints - they deal solely with object names, with the MountTable providing
 * translation of object names to endpoints.
 * <p>
 * In order to be compatible with other implementations of the Vanadium framework, the {@code
 * toString} implementation of this class must return a String in the following format:
 *
 * <p><blockquote><pre>
 *     &#64;&lt;version&gt;&#64;&lt;version specific fields&gt;&#64;&#64;
 * </pre></blockquote><p>
 * where version is an unsigned integer.
 * <p>
 * Version 5 is the current version for RPC:
 * <p><blockquote><pre>
 *     &#64;5&#64;&lt;protocol&gt;&#64;&lt;address&gt;&#64;&lt;routingid&gt;&#64;m|s&#64;[&lt;blessing&gt;[,<blessing>]...]&#64;&#64;
 * </pre></blockquote>
 */
public interface Endpoint {
    /**
     * Returns a string representation of this endpoint that can be used as a name with
     * {@link io.v.v23.rpc.Client#startCall}.
     */
    String name();

    /**
     * Returns the {@link RoutingId} associated with this endpoint.
     */
    RoutingId routingId();

    /**
     * Returns the Routes associated with this endpoint.
     */
    List<String> routes();

    /**
     * Returns the {@link NetworkAddress} associated with this endpoint.
     */
    NetworkAddress address();

    /**
     * Returns {@code true} if this endpoint serves a mount table.
     */
    boolean servesMountTable();

    /**
     * Returns {@code true} if this endpoint serves a leaf server.
     */
    boolean isLeaf();

    /**
     * Returns the blessings that the process associated with this endpoint will present.
     */
    List<String> blessingNames();
}
