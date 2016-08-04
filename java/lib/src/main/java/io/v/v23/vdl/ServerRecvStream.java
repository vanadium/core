// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

import io.v.v23.InputChannel;

/**
 * Represents the receiving side of the server bidirectional stream.
 *
 * @param <RecvT>   type of values that the server is receiving from the client
 */
public interface ServerRecvStream<RecvT> extends InputChannel<RecvT> {
}
