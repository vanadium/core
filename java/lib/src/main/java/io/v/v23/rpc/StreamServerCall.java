// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.rpc;

/**
 * The in-flight call state on the server, including methods to stream args and results.
 */
public interface StreamServerCall extends Stream, ServerCall {}
