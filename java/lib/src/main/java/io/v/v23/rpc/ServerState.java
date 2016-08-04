// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.rpc;

/**
 * The state of the server.
 */
public enum ServerState {
    /**
     * Initial state of the server.
     */
    SERVER_INIT,
    /**
     * Server is active.
     */
    SERVER_ACTIVE,
    /**
     * Server has been asked to stop and is in the process of doing so.
     */
    SERVER_STOPPING,
    /**
     * Server has stopped.
     */
    SERVER_STOPPED
}
