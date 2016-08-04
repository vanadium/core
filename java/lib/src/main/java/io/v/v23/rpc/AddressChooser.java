// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.rpc;

import io.v.v23.verror.VException;

/**
 * Interface for choosing addresses to publish with the mount table from a candidate
 * set of addresses.
 */
public interface AddressChooser {
    /**
     * Returns preferred addresses to publish with the mount table from the given candidate set.
     *
     * @param  protocol   network protocol
     * @param  candidates candidate set of addresses
     * @return            preferred addresses to publish with the mount table
     * @throws VException if there was an error choosing the addresses
     */
    NetworkAddress[] choose(String protocol, NetworkAddress[] candidates) throws VException;
}
