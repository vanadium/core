// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

/**
 * Represents the server side of the established bidirectional stream.
 *
 * @param <SendT>   type of values that the server is sending to the client
 * @param <RecvT>   type of values that the server is receiving from the client
 */
public interface ServerStream<SendT, RecvT>
        extends ServerSendStream<SendT>, ServerRecvStream<RecvT> {}
