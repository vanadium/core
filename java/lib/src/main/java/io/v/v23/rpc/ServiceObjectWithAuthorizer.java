// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.rpc;

import io.v.v23.security.Authorizer;

/**
 * A pair of (1) a service object that has invokable methods, and (2) an {@link Authorizer} that
 * allows control over authorization checks.
 */
public class ServiceObjectWithAuthorizer {
    private final Object service;
    private final Authorizer auth;

    /**
     * Creates a new {@link ServiceObjectWithAuthorizer} object.
     *
     * @param  service  service object that has invokable methods
     * @param  auth     {@link Authorizer} that allows control over authorization checks
     */
    public ServiceObjectWithAuthorizer(Object service, Authorizer auth) {
        this.service = service;
        this.auth = auth;
    }

    /**
     * Returns the service object.
     */
    public Object getServiceObject() { return this.service; }

    /**
     * Returns the {@link Authorizer}.
     */
    public Authorizer getAuthorizer() { return this.auth; }
}
