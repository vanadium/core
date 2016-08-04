// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/**
 * Package rpc defines interfaces for communication via remote procedure call.
 * <p><ul>
 *   <li>Concept: <a href="https://vanadium.github.io/concepts/rpc.html">https://vanadium.github.io/concepts/rpc.html</a>.</li>
 *   <li>Tutorial: (forthcoming)</li>
 * </ul><p>
 * There are two actors in the system, clients and servers.  {@link io.v.v23.rpc.Client}s invoke
 * methods on {@link io.v.v23.rpc.Server}s, using the
 * {@link io.v.v23.rpc.Client#startCall startCall} method. {@link io.v.v23.rpc.Server}s implement
 * methods on named objects.  The named object is found using a {@link io.v.v23.rpc.Dispatcher} and
 * the method is invoked using an {@link io.v.v23.rpc.Invoker}.
 * <p>
 * Instances of the {@link io.v.v23.VRuntime} host {@link io.v.v23.rpc.Client}s and
 * {@link io.v.v23.rpc.Server}s; such instances may simultaneously host both
 * {@link io.v.v23.rpc.Client}s and {@link io.v.v23.rpc.Server}s.  The {@link io.v.v23.VRuntime}
 * allows multiple names to be simultaneously supported via the {@link io.v.v23.rpc.Dispatcher}
 * interface.
 * <p>
 * The {@code naming} package provides a rendezvous mechanism for {@link io.v.v23.rpc.Client}s and
 * {@link io.v.v23.rpc.Server}s.  In particular, it allows {@link io.v.v23.VRuntime} hosting
 * {@link io.v.v23.rpc.Server}s to share endpoints with {@link io.v.v23.rpc.Client}s that enables
 * communication between them.  Endpoints encode sufficient addressing information to enable
 * communication.
 */
package io.v.v23.rpc;
