// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import org.joda.time.DateTime;

import java.util.Map;

import io.v.v23.vdl.VdlValue;

/**
 * The state available for authorizing a principal.
 */
public interface Call {
    /**
     * Returns the timestamp at which the authorization state is to be checked.
     */
    DateTime timestamp();

    /**
     * Returns the method being invoked.
     */
    String method();

    /**
     * Returns the tags attached to the method, typically through the interface specification
     * in VDL.  Returns empty array if no tags are attached.
     */
    VdlValue[] methodTags();

    /**
     * Returns the Vanadium name suffix for the request.
     */
    String suffix();

    /**
     * Returns the discharges for third-party caveats presented by the local end of the call,
     * mapping a third-party caveat identifier to the corresponding discharge.
     */
    Map<String, Discharge> localDischarges();

    /**
     * Returns the discharges for third-party caveats presented by the remote end of the call,
     * mapping a third-party caveat identifier to the corresponding discharge.
     */
    Map<String, Discharge> remoteDischarges();

    /**
     * Returns the principal used to authenticate to the remote end.
     */
    VPrincipal localPrincipal();

    /**
     * Returns the blessings sent to the remote end for authentication.
     */
    Blessings localBlessings();

    /**
     * Returns the blessings received from the remote end during authentication.
     */
    Blessings remoteBlessings();

    /**
     * Returns the endpoint of the principal at the local end of the request.
     */
    String localEndpoint();

    /**
     * Returns the endpoint of the principal at the remote end of the request.
     */
    String remoteEndpoint();

    // TODO(spetrovic): implement discharge methods.
}
