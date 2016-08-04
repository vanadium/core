// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.rpc;

import io.v.v23.verror.VException;

import java.util.Arrays;

/**
 * Stores the information required to create a listening network endpoint for a server
 * and, optionally, the name of a proxy to use in conjunction with that listener.
 */
public class ListenSpec {
    /**
     * A pair of (network protocol, address) that the server should listen on.
     * For TCP, the address must be in {@code ip:port} format. The {@code ip} may be omitted, but
     * the {@code port} can not (choose a port of {@code 0} to have the system allocate one).
     */
    public static class Address {
        private final String protocol;
        private final String address;

        /**
         * Creates a new {@link Address} with the given protocol and address.
         *
         * @param  protocol network protocol (e.g., {@code "tcp"})
         * @param  address  network address (e.g., {@code "localhost:32786"})
         */
        public Address(String protocol, String address) {
            this.protocol = protocol;
            this.address = address;
        }

        /**
         * Returns the network protocol.
         */
        public String getProtocol() { return this.protocol; }

        /**
         * Returns the network address.
         */
        public String getAddress() { return this.address; }

        @Override
        public String toString() {
            return this.protocol + ":" + this.address;
        }
    }

    private final Address[] addrs;  // non-null
    private final String proxy;  // non-null
    private final AddressChooser chooser;  // non-null

    /**
     * Creates a new {@link ListenSpec} with the given addresses, proxy,
     * and {@link AddressChooser}.
     *
     * @param  addrs   {@link Address}es that the server should listen on
     * @param  proxy   proxy that the server should use to proxy the connection
     * @param  chooser {@link AddressChooser} used for selecting addresses to publish with the
     *                 mount table from a candidate set of addresses;  if {@code null}, a default
     *                 {@link AddressChooser} that chooses all the addresses will be used.
     */
    public ListenSpec(Address[] addrs, String proxy, AddressChooser chooser) {
        this.addrs = addrs == null ? new Address[0] : Arrays.copyOf(addrs, addrs.length);
        this.proxy = proxy == null ? "" : proxy;
        this.chooser = chooser != null ? chooser : new AddressChooser() {
            @Override
            public NetworkAddress[] choose(String protocol, NetworkAddress[] candidates)
                    throws VException {
                return candidates;
            }
        };
    }

    /**
     * Same as {@link #ListenSpec(Address[],String,AddressChooser)}, only with a
     * single listening {@link Address}.
     */
    public ListenSpec(Address addr, String proxy, AddressChooser chooser) {
        this(new Address[]{ addr }, proxy, chooser);
    }

    /**
     * Same as {@link #ListenSpec(Address[],String,AddressChooser)}, only with a
     * single listening {@link Address} that will be created using the given {@code protocol}
     * and {@code address}.
     */
    public ListenSpec(String protocol, String address) {
        this(new Address[]{ new Address(protocol, address)}, "", null);
    }

    /**
     * Returns a new copy of this {@link ListenSpec} with the {@link AddressChooser} replaced with
     * the given address chooser.
     */
    public ListenSpec withAddressChooser(AddressChooser newChooser) {
        return new ListenSpec(getAddresses(), proxy, newChooser);
    }

    /**
     * Returns a new copy of this {@link ListenSpec} whose {@link #getProxy} method will return the
     * given proxy.
     */
    public ListenSpec withProxy(String newProxy) {
        return new ListenSpec(getAddresses(), newProxy, chooser);
    }

    /**
     * Returns a new copy of this {@link ListenSpec} whose {@link #getAddresses} method will return
     * an array containing only the given address.
     */
    public ListenSpec withAddress(Address address) {
        return new ListenSpec(new Address[]{address}, proxy, chooser);
    }

    /**
     * Returns the addresses the server should listen on.
     */
    public Address[] getAddresses() {
        return Arrays.copyOf(this.addrs, this.addrs.length);
    }

    /**
     * Returns the name of the proxy.  If empty, the server isn't proxied.
     */
    public String getProxy() { return this.proxy; }

    /**
     * Returns the address chooser that is used to choose the preferred address
     * to publish with the mount table when one is not otherwise specified.
     */
    public AddressChooser getChooser() { return this.chooser; }

    @Override
    public String toString() {
        return Arrays.toString(this.addrs) +
                (this.proxy.isEmpty() ? "" : "proxy(" + this.proxy + ")");
    }
}
