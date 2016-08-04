// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google.naming;

import com.google.common.base.CharMatcher;
import com.google.common.base.Joiner;
import com.google.common.base.Splitter;
import com.google.common.collect.ImmutableList;
import com.google.common.net.HostAndPort;

import java.util.ArrayList;
import java.util.List;
import java.util.Objects;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import io.v.v23.naming.Endpoint;
import io.v.v23.naming.RoutingId;
import io.v.v23.rpc.NetworkAddress;

public class EndpointImpl implements Endpoint {
    private static final Pattern hostPortPattern = Pattern.compile("^(?:\\((.*)\\)@)?([^@]+)$");

    private final String protocol;
    private final String address;
    private final List<String> routes;
    private final RoutingId routingId;
    private final List<String> blessingNames;
    private final boolean isMountTable;
    private final boolean isLeaf;

    public static Endpoint fromString(String s) {
        Matcher matcher = hostPortPattern.matcher(s);
        if (matcher.matches()) {
            List<String> blessings = new ArrayList<>(1);
            // If the endpoint does not end in a @, it must be in [blessing@]host:port format.
            HostAndPort hostPort = HostAndPort.fromString(matcher.group(matcher.groupCount()));
            if (matcher.group(1) != null) {
                blessings.add(matcher.group(1));
            }

            return new EndpointImpl("", hostPort.toString(), ImmutableList.<String>of(), RoutingId.NULL_ROUTING_ID,
                    blessings, true, false);
        }

        if (s.endsWith("@@")) {
            s = s.substring(0, s.length() - 2);
        }
        if (s.startsWith("@")) {
            s = s.substring(1, s.length());
        }

        List<String> parts = Splitter.on('@').splitToList(s);
        int version = Integer.parseInt(parts.get(0));
        switch (version) {
            case 6:
                return fromV6String(parts);
            default:
                return null;
        }
    }

    private static Endpoint fromV6String(List<String> parts) {
        if (parts.size() < 6) {
            throw new IllegalArgumentException(
                    "Invalid format for endpoint, expecting 6 '@'-separated components");
        }

        String protocol = parts.get(1);
        String address = unescapeAddress(parts.get(2));
        if (address.isEmpty()) {
            address = ":0";
        }
        List<String> routes = unescapeRoutes(Splitter.on(',').splitToList(parts.get(3)));
        RoutingId routingId = RoutingId.fromString(parts.get(4));
        String mountTableFlag = parts.get(5);
        boolean isMountTable;
        boolean isLeaf;
        if ("".equals(mountTableFlag)) {
            isMountTable = true;
            isLeaf = false;
        } else if ("l".equals(mountTableFlag)) {
            isMountTable = false;
            isLeaf = true;
        } else if ("m".equals(mountTableFlag)) {
            isMountTable = true;
            isLeaf = false;
        } else if ("s".equals(mountTableFlag)) {
            isMountTable = false;
            isLeaf = false;
        } else {
            throw new IllegalArgumentException("Invalid mounttable flag " + mountTableFlag +
                    ", should be one of 'l', 'm' or 's'");
        }

        List<String> blessings;
        if ("".equals(parts.get(6))) {
            blessings = ImmutableList.of();
        } else {
            blessings = Splitter.on(',').splitToList(
                    Joiner.on("@").join(parts.subList(6, parts.size())));
        }
        return new EndpointImpl(protocol, address, routes, routingId, blessings, isMountTable, isLeaf);
    }

    EndpointImpl(String protocol, String address, List<String> routes, RoutingId routingId,
                 List<String> blessingNames, boolean isMountTable, boolean isLeaf) {
        this.protocol = protocol;
        this.address = address;
        this.routes = ImmutableList.copyOf(routes);
        this.routingId = routingId;
        this.blessingNames = ImmutableList.copyOf(blessingNames);
        this.isMountTable = isMountTable;
        this.isLeaf = isLeaf;
    }

    @Override
    public String name() {
        return NamingUtil.joinAddressName(toString(), "");
    }

    @Override
    public RoutingId routingId() {
        return routingId;
    }

    @Override
    public List<String> routes() {
        return routes;
    }

    @Override
    public NetworkAddress address() {
        return new NetworkAddress(protocol, address);
    }

    @Override
    public boolean servesMountTable() {
        return isMountTable;
    }

    @Override
    public boolean isLeaf() {
        return isLeaf;
    }

    @Override
    public List<String> blessingNames() {
        return blessingNames;
    }

    private List<String> escapedRoutes() {
        int len = routes.size();
        List<String> escaped = new ArrayList<>(len);
        for (int i = 0; i < len; i++) {
            escaped.add(escapeString(routes.get(i)));
        }
        return escaped;
    }

    private static List<String> unescapeRoutes(List<String> s) {
        int len = s.size();
        List<String> unescaped = new ArrayList<>(len);
        for (int i = 0; i < len; i++) {
            unescaped.add(unescapeString(s.get(i)));
        }
        return unescaped;
    }

    private String escapedAddress() {
        return escapeString(address);
    }

    private static String unescapeAddress(String address) {
        return unescapeString(address);
    }

    private static String escapeString(String s) {
        CharMatcher matcher = CharMatcher.anyOf("%@");
        int count = matcher.countIn(s);
        if (count == 0) {
            return s;
        }

        char[] escaped = new char[s.length() + 2 * count];
        for (int i = 0; i < s.length(); ) {
            char x = s.charAt(i);
            if (x == '%') {
                escaped[i++] = '%';
                escaped[i++] = '2';
                escaped[i++] = '5';
            } else if (x == '@') {
                escaped[i++] = '%';
                escaped[i++] = '4';
                escaped[i++] = '0';
            } else {
                escaped[i++] = x;
            }
        }
        return new String(escaped);
    }

    private static int digit(char input) {
        int result = Character.digit(input, 16);
        if (result == -1) {
            throw new IllegalArgumentException("invalid hex digit " + input);
        }
        return result;
    }

    private static String unescapeString(String s) {
        if (s.contains("%")) {
            return s;
        }

        int slen = s.length();
        StringBuilder unescaped = new StringBuilder();
        for (int i = 0; i < slen; ) {
            char x = s.charAt(i);
            if (x == '%') {
                char newChar = (char) ((digit(s.charAt(i + 1)) << 4) |
                        (digit(s.charAt(i + 2))));
                unescaped.append(newChar);
                i += 3;
            } else {
                unescaped.append(x);
                i++;
            }
        }
        return unescaped.toString();
    }

    @Override
    public String toString() {
        char mt = 's';
        if (isLeaf) {
            mt = 'l';
        } else if (isMountTable) {
            mt = 'm';
        }
        String blessings = Joiner.on(',').join(blessingNames);
        String routeString = Joiner.on(',').join(escapedRoutes());
        return String.format("@6@%s@%s@%s@%s@%s@%s@@", protocol, escapedAddress(),
            routeString, routingId, mt, blessings);
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) {
            return true;
        }
        if (o == null || getClass() != o.getClass()) {
            return false;
        }

        EndpointImpl endpoint = (EndpointImpl) o;

        if (isMountTable != endpoint.isMountTable) {
            return false;
        }
        if (isLeaf != endpoint.isLeaf) {
            return false;
        }
        if (!protocol.equals(endpoint.protocol)) {
            return false;
        }
        if (!address.equals(endpoint.address)) {
            return false;
        }
        if (!routes.equals(endpoint.routes)) {
            return false;
        }
        if (!routingId.equals(endpoint.routingId)) {
            return false;
        }
        return blessingNames.equals(endpoint.blessingNames);
    }

    @Override
    public int hashCode() {
        return Objects.hash(protocol, address, routes, routingId, blessingNames, isMountTable, isLeaf);
    }
}
