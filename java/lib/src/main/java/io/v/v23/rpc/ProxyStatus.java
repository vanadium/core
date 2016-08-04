// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.rpc;

import io.v.v23.verror.VException;

/**
 * The status of a proxy connection maintained by a server.
 */
public class ProxyStatus {
    private final String proxy;
    private final String endpoint;
    private final VException error;

    /**
     * Creates a new {@link ProxyStatus} object.
     *
     * @param  proxy    name of the proxy
     * @param  endpoint name of the endpoint that the server is using to receive proxied requests on
     * @param  error    any error status of the connection to the proxy
     */
    public ProxyStatus(String proxy, String endpoint, VException error) {
        this.proxy = proxy;
        this.endpoint = endpoint;
        this.error = error;
    }

    /**
     * Returns the name of the proxy.
     */
    public String getProxy() {
        return this.proxy;
    }

    /**
     * Returns the name of the endpoint that the server is using to receive proxied
     * requests on. The endpoint of the proxy itself can be obtained by resolving its name.
     */
    public String getEndpoint() {
        return this.endpoint;
    }

    /**
     * Returns the error status of the connection to the proxy.  It returns {@code null} if the
     * connection is currently correctly established, or the most recent error otherwise.
     */
    public VException getError() {
        return this.error;
    }

    @Override
    public String toString() {
        return String.format(
            "Proxy: %s, Endpoint: %s, Error: %s", this.proxy, this.endpoint, this.error);
    }
}
