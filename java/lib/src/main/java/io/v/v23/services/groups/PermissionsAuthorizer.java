// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.services.groups;

import io.v.v23.context.VContext;
import io.v.v23.security.Authorizer;
import io.v.v23.security.Call;
import io.v.v23.security.access.Permissions;
import io.v.v23.vdl.Types;
import io.v.v23.vdl.VdlType;
import io.v.v23.verror.VException;

import java.lang.reflect.Type;

/**
 * An authorizer that subscribes to an authorization policy where access is granted if
 * the remote end presents blessings included in the Access Control Lists (ACLs) associated with
 * the set of relevant tags.
 * <p>
 * This class handles authorization the same way as
 * {@link io.v.v23.security.access.PermissionsAuthorizer}, except that it supports groups.
 */
public class PermissionsAuthorizer implements Authorizer {
    private static native PermissionsAuthorizer nativeCreate(Permissions perms, VdlType type)
            throws VException;

    /**
     * Creates a new {@link PermissionsAuthorizer} authorizer.
     *
     * @param perms        ACLs containing authorization rules
     * @param tagType      type of the method tags that this authorizer checks
     * @return             a newly created authorizer
     * @throws VException  if the authorizer couldn't be created
     */
    public static PermissionsAuthorizer create(Permissions perms, Type tagType) throws VException {
        try {
            VdlType type = Types.getVdlTypeFromReflect(tagType);
            return nativeCreate(perms, type);
        } catch (IllegalArgumentException e) {
            throw new VException(String.format(
                    "Tag type %s does not have a corresponding VdlType: %s",
                    tagType, e.getMessage()));
        }
    }

    private final long nativeRef;

    private native void nativeAuthorize(long nativeRef, VContext ctx, Call call) throws VException;

    private PermissionsAuthorizer(long nativeRef) {
        this.nativeRef = nativeRef;
    }

    @Override
    public void authorize(VContext ctx, Call call) throws VException {
        nativeAuthorize(nativeRef, ctx, call);
    }
}
