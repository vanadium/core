// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security.access;

import io.v.v23.context.VContext;
import io.v.v23.security.Authorizer;
import io.v.v23.security.BlessingPattern;
import io.v.v23.security.Call;
import io.v.v23.verror.VException;

import java.util.List;

/**
 * A wrapper around WireAccessList, providing additional functionality.
 */
public class AccessList extends WireAccessList implements Authorizer {
    private static final long serialVersionUID = 1L;

    private final long nativeRef;

    private native long nativeCreate() throws VException;
    private native boolean nativeIncludes(long nativeRef, String[] blessings) throws VException;
    private native void nativeAuthorize(long nativeRef, VContext context, Call call);
    private native void nativeFinalize(long nativeRef);

    /**
     * Creates a new {@link AccessList} object.
     *
     * @param  in    blessings that should be allowed
     * @param  notIn blessings that should be denied
     */
    public AccessList(List<BlessingPattern> in, List<String> notIn) {
        super(in, notIn);
        try {
            this.nativeRef = nativeCreate();
        } catch (VException e) {
            throw new RuntimeException("Couldn't create native AccessList", e);
        }
    }

    AccessList(WireAccessList wire) {
        this(wire.getIn(), wire.getNotIn());
    }

    /**
     * Returns {@code true} iff the access list grants access to a principal that presents
     * these blessings.
     *
     * @param  blessings blessings we are getting access for
     * @return           true iff the ACL grants access to a principal that presents these
     *                   blessings
     */
    public boolean includes(String... blessings) {
        try {
            return nativeIncludes(this.nativeRef, blessings);
        } catch (VException e) {
            throw new RuntimeException("Couldn't test for access list inclusion", e);
        }
    }

    /**
     * Authorizes only if the remote blessings are included in the access list.
     *
     * @param  context    vanadium context
     * @param  call       the call being authorized
     * @throws VException if the request is not authorized
     */
    @Override
    public void authorize(VContext context, Call call) throws VException {
        nativeAuthorize(this.nativeRef, context, call);
    }

    @Override
    protected void finalize() {
        nativeFinalize(this.nativeRef);
    }
}
