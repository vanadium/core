// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

/**
 * This class represents a "proof" required for satisfying a {@code ThirdPartyCaveat}.
 * <p>
 * A discharge may have caveats of its own (including {@code ThirdPartyCaveats}) that restrict the
 * context in which the discharge is usable.
 * <p>
 * {@code Discharge} objects are immutable and are therefore threadsafe.
 * <p>
 * @see <a href="https://vanadium.github.io/glossary.html#discharge">the Discharge glossary entry</a>
 */
public class Discharge {
    private final WireDischarge wire;

    /**
     * Creates a new {@link Discharge} from the wire representation.
     */
    public Discharge(WireDischarge wire) {
        this.wire = wire;
    }

    WireDischarge wireFormat() {
        return wire;
    }
}
