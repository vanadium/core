// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.naming;

import java.io.ByteArrayOutputStream;
import java.io.DataOutputStream;
import java.io.IOException;
import java.util.Arrays;

/**
 * A globally unique ID for routing packets to a given {@link Endpoint}.
 * <p>
 * Global uniqueness is required in order to support comparing {@link Endpoint endpoints} for
 * equality; this is required for sharing connections, for proxying (though global uniqueness is
 * not strictly required) and determining if different names resolve to the same endpoint.
 */
public class RoutingId {
    private static final int ROUTING_ID_LENGTH = 16;
    private static final int FIRST_UNRESERVED_ROUTING_ID = 1024;

    /**
     * The null route.
     */
    public static final RoutingId NULL_ROUTING_ID = new RoutingId(new byte[ROUTING_ID_LENGTH]);

    private final byte[] value;

    /**
     * Creates a new routing id given its serialized string (obtained using {@link #toString}).
     * If {@code id} is empty, returns {@link #NULL_ROUTING_ID}.
     */
    public static RoutingId fromString(String id) {
        if (id.isEmpty()) {
            return NULL_ROUTING_ID;
        }
        return new RoutingId(hexStringToByteArray(id));
    }

    /**
     * Returns a fixed routing ID for the given @{code long} constant.
     */
    public static RoutingId fixedRoutingId(long id) {
        try (ByteArrayOutputStream stream = new ByteArrayOutputStream(ROUTING_ID_LENGTH);
             DataOutputStream output = new DataOutputStream(stream)) {
            output.writeLong(0);
            output.writeLong(id);
            return new RoutingId(stream.toByteArray());
        } catch (IOException e) {
            // We never expect to see this.
            throw new RuntimeException(e);
        }
    }

    private RoutingId(byte[] value) {
        if (value.length != ROUTING_ID_LENGTH) {
            throw new IllegalArgumentException(
                    "input has unexpected length (was " + value.length + ", want " +
                            ROUTING_ID_LENGTH + ")");

        }
        this.value = new byte[ROUTING_ID_LENGTH];
        System.arraycopy(value, 0, this.value, 0, ROUTING_ID_LENGTH);
    }

    private static byte[] hexStringToByteArray(String s) {
        // TODO(suharshs): We should really be using some Hex library for this
        // function and toString.
        int len = s.length();
        byte[] data = new byte[len / 2];
        for (int i = 0; i < len; i += 2) {
            data[i / 2] = (byte) ((Character.digit(s.charAt(i), 16) << 4)
                    + Character.digit(s.charAt(i + 1), 16));
        }
        return data;
    }

    /**
     * Returns a string representation of this {@code RoutingId} that may be used as a parameter to
     * {@link #fromString}.
     */
    @Override
    public String toString() {
        char[] hexChars = new char[value.length * 2];
        for ( int j = 0; j < value.length; j++ ) {
            int v = value[j] & 0xFF;
            hexChars[j * 2] = Character.forDigit(v >>> 4, 16);
            hexChars[j * 2 + 1] = Character.forDigit(v & 0x0F, 16);
        }
        return new String(hexChars);
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) {
            return true;
        }
        if (o == null || getClass() != o.getClass()) {
            return false;
        }

        RoutingId routingId = (RoutingId) o;

        return Arrays.equals(value, routingId.value);

    }

    @Override
    public int hashCode() {
        return Arrays.hashCode(value);
    }
}
