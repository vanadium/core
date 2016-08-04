// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google.lib.discovery;

import io.v.v23.context.VContext;
import io.v.v23.discovery.Discovery;
import io.v.v23.verror.VException;

import org.joda.time.Duration;

/**
 * Used for creating Vanadium global discovery instances.
 * <p>
 * TODO(jhahn): This is an experimental work to see its feasibility and set
 * the long-term goal, and can be changed without notice.
 */
public class GlobalDiscovery {
    private static native Discovery nativeNewDiscovery(
            VContext ctx, String path, Duration mountTtl, Duration scanInterval) throws VException;

    /**
     * Returns a new global {@link Discovery} instance that uses the Vanadium namespace
     * under {@code path} with default mount ttl (120s) and scan interval (90s).
     *
     * @param  ctx        current context
     * @param  path       a namespace path to use for global discovery
     * @throws VException if a new discovery instance cannot be created
     */
    public static Discovery newDiscovery(VContext ctx, String path) throws VException {
        return nativeNewDiscovery(ctx, path, Duration.ZERO, Duration.ZERO);
    }

    /**
     * Returns a new global {@link Discovery} instance that uses the Vanadium
     * namespace under {@code path}.
     *
     * @param  ctx          current context
     * @param  path         a namespace path to use for global discovery
     * @param  mountTtl     the duration for which the advertisement lives in a mounttable
     *                      after advertising is cancelled
     * @param  scanInterval the duration to scan a mounttable periodically
     * @throws VException   if a new discovery instance cannot be created
     */
    public static Discovery newDiscovery(
            VContext ctx, String path, Duration mountTtl, Duration scanInterval) throws VException {
        return nativeNewDiscovery(ctx, path, mountTtl, scanInterval);
    }
}
