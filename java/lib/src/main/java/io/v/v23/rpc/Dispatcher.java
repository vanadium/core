// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.rpc;

import io.v.v23.verror.VException;

/**
 * Interface that a server must implement to handle method invocations on named objects.
 */
public interface Dispatcher {
    /**
     * Returns the container storing (1) the service object identified by the
     * given suffix on which methods will be served and (2) the authorizer which
     * allows control over authorization checks.  Returning a {@code null}
     * container indicates that this dispatcher does handle the object - the
     * framework should try other dispatchers.
     * <p>
     * Reflection is used to match requests to the service object's method set.
     * As a special-case, if the returned object implements the {@link Invoker}
     * interface, the invoker is used to invoke methods directly, without
     * reflection.
     * <p>
     * A thrown exception indicates the dispatch lookup has failed.  The error
     * will be delivered back to the client and no further dispatch lookups will
     * be performed.
     * <p>
     * This method may be invoked concurrently by the underlying RPC system and
     * hence must be thread-safe.
     *
     * @param  suffix          the object's name suffix
     * @return                 a container storing (1) the service object identified by the given
     *                         suffix and (2) the associated authorizer; {@code null} is returned
     *                         if this dispatcher doesn't handle the object
     * @throws VException      if the lookup error has occured
     */
    ServiceObjectWithAuthorizer lookup(String suffix) throws VException;
}
