// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.options;

import org.joda.time.Duration;

import javax.annotation.Nullable;

import io.v.v23.OptionDefs;
import io.v.v23.Options;
import io.v.v23.naming.MountEntry;
import io.v.v23.security.Authorizer;
import io.v.v23.security.VSecurity;

/**
 * Strongly-typed alternative to {@link io.v.v23.Options} for RPCs, supporting more options. See
 * also
 * <a href="https://github.com/vanadium/go.v23/blob/master/options/options.go">v.io/v23/options</a>.
 */
public final class RpcOptions {
    private static Authorizer migrateNameResolutionAuthorizer(final Options opts) {
        if (opts.has(OptionDefs.SKIP_SERVER_ENDPOINT_AUTHORIZATION) &&
                opts.get(OptionDefs.SKIP_SERVER_ENDPOINT_AUTHORIZATION, Boolean.class))
            return VSecurity.newAllowEveryoneAuthorizer();

        return !opts.has(OptionDefs.NAME_RESOLUTION_AUTHORIZER)
                ? null
                : opts.get(OptionDefs.NAME_RESOLUTION_AUTHORIZER, Authorizer.class);
    }

    private static Authorizer migrateServerAuthorizer(final Options opts) {
        if (opts.has(OptionDefs.SKIP_SERVER_ENDPOINT_AUTHORIZATION) &&
                opts.get(OptionDefs.SKIP_SERVER_ENDPOINT_AUTHORIZATION, Boolean.class))
            return VSecurity.newAllowEveryoneAuthorizer();

        return !opts.has(OptionDefs.SERVER_AUTHORIZER)
                ? null
                : opts.get(OptionDefs.SERVER_AUTHORIZER, Authorizer.class);
    }

    /**
     * @deprecated For migration purposes only; call overloads taking {@code RpcOptions} directly.
     */
    @Nullable public static RpcOptions migrateOptions(@Nullable final Options legacy) {
        return legacy == null ? null : new RpcOptions()
                .nameResolutionAuthorizer(migrateNameResolutionAuthorizer(legacy))
                .serverAuthorizer(migrateServerAuthorizer(legacy));
    }

    private Authorizer nameResolutionAuthorizer, serverAuthorizer;
    private MountEntry preresolved;
    private boolean noRetry;
    private Duration connectionTimeout, channelTimeout;

    public RpcOptions nameResolutionAuthorizer(final Authorizer nameResolutionAuthorizer) {
        this.nameResolutionAuthorizer = nameResolutionAuthorizer;
        return this;
    }

    public RpcOptions serverAuthorizer(final Authorizer serverAuthorizer) {
        this.serverAuthorizer = serverAuthorizer;
        return this;
    }

    public RpcOptions preresolved(final MountEntry preresolved) {
        this.preresolved = preresolved;
        return this;
    }

    public RpcOptions noRetry(final boolean noRetry) {
        this.noRetry = noRetry;
        return this;
    }

    public RpcOptions connectionTimeout(final Duration connectionTimeout) {
        this.connectionTimeout = connectionTimeout;
        return this;
    }

    public RpcOptions channelTimeout(final Duration channelTimeout) {
        this.channelTimeout = channelTimeout;
        return this;
    }

    public Authorizer nameResolutionAuthorizer() {
        return this.nameResolutionAuthorizer;
    }

    public Authorizer serverAuthorizer() {
        return this.serverAuthorizer;
    }

    public MountEntry preresolved() {
        return this.preresolved;
    }

    public boolean noRetry() {
        return this.noRetry;
    }

    public Duration connectionTimeout() {
        return this.connectionTimeout;
    }

    public Duration channelTimeout() {
        return this.channelTimeout;
    }

    public RpcOptions skipServerEndpointAuthorization() {
        nameResolutionAuthorizer = serverAuthorizer = VSecurity.newAllowEveryoneAuthorizer();
        return this;
    }
}
