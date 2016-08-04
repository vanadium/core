// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.options;

import org.joda.time.Duration;

import javax.annotation.Nullable;

import io.v.v23.OptionDefs;
import io.v.v23.Options;

/**
 * Strongly-typed alternative to {@link io.v.v23.Options} for RPC servers, supporting more options.
 * See also
 * <a href="https://github.com/vanadium/go.v23/blob/master/options/options.go">v.io/v23/options</a>.
 */
public final class RpcServerOptions {
    private static final Duration DEFAULT_LAME_DUCK_TIMEOUT = Duration.standardSeconds(0);

    private static Duration migrateLameDuckTimeout(final Options opts) {
        if (!opts.has(OptionDefs.SERVER_LAME_DUCK_TIMEOUT)) {
            return DEFAULT_LAME_DUCK_TIMEOUT;
        }
        Object timeout = opts.get(OptionDefs.SERVER_LAME_DUCK_TIMEOUT);
        if (!(timeout instanceof Duration)) {
            throw new RuntimeException("SERVER_LAME_DUCK_TIMEOUT option if specified must " +
                    "contain an object of type org.joda.time.Duration");
        }
        return (Duration) timeout;
    }

    /**
     * @deprecated For migration purposes only; call overloads taking {@code RpcServerOptions}
     *  directly.
     */
    @Nullable
    public static RpcServerOptions migrateOptions(@Nullable final Options legacy) {
        return legacy == null ? null : new RpcServerOptions()
                .lameDuckTimeout(migrateLameDuckTimeout(legacy));
    }

    private boolean servesMountTable;
    // TODO(rosswang): This is a nullable type... it's unclear why it should default to a different
    // value than the default behavior in Go when omitted. (isLeaf defaults to true in Go, and could
    // as well be a nullable Boolean instead.)
    private Duration lameDuckTimeout = DEFAULT_LAME_DUCK_TIMEOUT;
    // Defaults to true:
    // https://github.com/vanadium/go.ref/blob/60698e6/runtime/internal/rpc/server.go#L97
    private boolean isLeaf = true;
    private Duration channelTimeout;

    public RpcServerOptions servesMountTable(final boolean servesMountTable) {
        this.servesMountTable = servesMountTable;
        return this;
    }

    public RpcServerOptions lameDuckTimeout(final Duration lameDuckTimeout) {
        this.lameDuckTimeout = lameDuckTimeout;
        return this;
    }

    public RpcServerOptions isLeaf(final boolean isLeaf) {
        this.isLeaf = isLeaf;
        return this;
    }

    public RpcServerOptions channelTimeout(final Duration channelTimeout) {
        this.channelTimeout = channelTimeout;
        return this;
    }

    public boolean servesMountTable() {
        return this.servesMountTable;
    }

    public Duration lameDuckTimeout() {
        return this.lameDuckTimeout;
    }

    public boolean isLeaf() {
        return this.isLeaf;
    }

    public Duration channelTimeout() {
        return this.channelTimeout;
    }
}
