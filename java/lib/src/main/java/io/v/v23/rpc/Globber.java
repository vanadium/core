// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.rpc;

import com.google.common.util.concurrent.ListenableFuture;

import io.v.v23.context.VContext;
import io.v.v23.naming.GlobReply;
import io.v.v23.vdl.ServerSendStream;

/**
 * Interface that allows the object to enumerate the the entire namespace below the receiver object.
 * <p>
 * Every object that implements it must be able to handle glob requests that could match any object
 * below itself. E.g. {@code "a/b".glob("* /*")}, {@code "a/b".glob("c/...")}, etc.
 */
public interface Globber {
    /**
     * Handles a glob request. The implementing class may respond by writing zero or more
     * {@link GlobReply} instances to the given {@code stream}. Once the replies are
     * written, the returned future should complete.
     *
     * @param call         in-flight call information
     * @param pattern      the glob pattern from the client
     * @param stream       the stream to which the responses must be written
     * @return             a new {@link ListenableFuture} that completes when the server is
     *                     done writing responses on the provided {@code stream}
     */
    ListenableFuture<Void> glob(VContext ctx, ServerCall call, String pattern,
                                ServerSendStream<GlobReply> stream);
}
