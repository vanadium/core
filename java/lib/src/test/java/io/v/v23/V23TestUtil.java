// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23;

import com.google.common.truth.Truth;
import com.google.common.util.concurrent.AsyncFunction;
import com.google.common.util.concurrent.Futures;
import com.google.common.util.concurrent.ListenableFuture;

import io.v.v23.context.VContext;
import io.v.v23.naming.Endpoint;
import io.v.v23.rpc.ListenSpec;
import io.v.v23.rpc.Server;
import io.v.v23.rpc.ServerCall;
import io.v.v23.security.VSecurity;
import io.v.v23.security.access.Permissions;
import io.v.v23.services.permissions.ObjectServer;
import io.v.v23.verror.VException;

import static com.google.common.truth.Truth.assertThat;

/**
 * Various Vanadium test utilities.
 */
public class V23TestUtil {
    public static VContext withDummyServer(VContext ctx) throws Exception {
        ctx = V.withListenSpec(ctx, V.getListenSpec(ctx).withAddress(
                new ListenSpec.Address("tcp", "localhost:0")));
        return V.withNewServer(ctx, "", new ObjectServer() {
                    @Override
                    public ListenableFuture<Void> setPermissions(VContext ctx, ServerCall call,
                                                                 Permissions permissions,
                                                                 String version) {
                        return Futures.immediateFailedFuture(new VException("Unimplemented!"));
                    }
                    @Override
                    public ListenableFuture<ObjectServer.GetPermissionsOut> getPermissions(
                            VContext ctx, ServerCall call) {
                        return Futures.immediateFailedFuture(new VException("Unimplemented!"));
                    }
                },
                VSecurity.newAllowEveryoneAuthorizer());
    }

    /**
     * Returns the first available endpoint for a server attached to the given context.
     */
    public static Endpoint getServerEndpoint(VContext ctx) {
        Server server = V.getServer(ctx);
        assertThat(server).isNotNull();
        assertThat(server.getStatus()).isNotNull();
        assertThat(server.getStatus().getEndpoints()).isNotEmpty();
        return server.getStatus().getEndpoints()[0];
    }

    /**
     * Asserts that the given future is executed on the provided thread.
     * <p>
     * Note that this check is inherently racey: if the future has already completed, there is
     * no way to check on which thread it got executed.  Therefore, this method should be executed
     * as close possible to the future's creation, to reduce the chance of the race happening.
     * Additionally, the future could be created with an executor that delays execution for a time
     * period that sufficient to all but prohibit races (e.g., 100ms).
     */
    public static <T> void assertRunsOnThread(ListenableFuture<T> future, final Thread thread) {
        try {
            assertThat(future.isDone()).isFalse();
            Futures.transform(future, new AsyncFunction<T, Void>() {
                @Override
                public ListenableFuture<Void> apply(T input) throws Exception {
                    assertThat(Thread.currentThread()).isEqualTo(thread);
                    return Futures.immediateFuture(null);
                }
            }).get();
        } catch (Exception e) {
            Truth.assertWithMessage("error asserting executor").that(e.getMessage()).isEmpty();
        }
    }
}
