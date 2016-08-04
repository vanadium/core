// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security.access;

import io.v.v23.context.VContext;
import io.v.v23.security.Authorizer;
import io.v.v23.security.Call;
import io.v.v23.vdl.Types;
import io.v.v23.vdl.VdlType;
import io.v.v23.verror.VException;

import java.lang.reflect.Type;

/**
 * An authorizer that subscribes to an authorization policy where access is granted if
 * the remote end presents blessings included in the Access Control Lists (ACLs) associated with
 * the set of relevant tags.
 * <p>
 * The set of relevant tags is the subset of tags associated with the method
 * ({@link io.v.v23.security.Call#methodTags}) that have the same type as
 * the provided one.
 * Currently, {@code tagType.getKind()} must be {@link io.v.v23.vdl.Types#STRING} , i.e., only tags
 * that are named string types are supported.
 * <p>
 * If multiple tags of the provided type are associated with the method, then access is granted
 * if the peer presents blessings that match the ACLs of each one of those tags. If no tags of
 * the provided are associated with the method, then access is denied.
 * <p>
 * If the {@link Permissions} provided are {@code null}, then an authorizer that rejects all remote
 * ends is returned.
 * <p>
 * <string>Sample usage:</strong>
 * <p><ol>
 * <li>Attach tags to methods in the VDL (eg. myservice.vdl)
 * <p><blockquote><pre>
 *     package myservice
 *
 *     type MyTag string
 *     const (
 *         ReadAccess  = MyTag("R")
 *         WriteAccess = MyTag("W")
 *     )
 *
 *     type MyService interface {
 *         Get() ([]string, error)       {ReadAccess}
 *         GetIndex(int) (string, error) {ReadAccess}
 *
 *         Set([]string) error           {WriteAccess}
 *         SetIndex(int, string) error   {WriteAccess}
 *
 *         GetAndSet([]string) ([]string, error) {ReadAccess, WriteAccess}
 *     }
 * </pre></blockquote><p>
 * </li>
 * <li>Setup the dispatcher to use the {@link PermissionsAuthorizer}:
 * <p><blockquote><pre>
 *     public class MyDispatcher implements io.v.v23.rpc.Dispatcher {
 *         {@literal @}Override
 *         public ServiceObjectWithAuthorizer lookup(String suffix) throws VException {
 *             Permissions acls = new Permissions(ImmutableMap.of(
 *             "R", new AccessList(ImmutableList.of(new BlessingPattern("alice:friends:..."),
 *                                                  new BlessingPattern("alice:family:...")),
 *                                 null),
 *             "W", new AccessList(ImmutableList.of(new BlessingPattern("alice:family:..."),
 *                                                  new BlessingPattern("alice:colleagues:...")),
 *                                 null)));
 *             return new ServiceObjectWithAuthorizer(
 *                     newInvoker(), VSecurity.newPermissionsAuthorizer(acls, MyTag.class));
 *   }
 * </pre></blockquote><p>
 * </li>
 * </ol>
 * With the above dispatcher, the server will grant access to a peer with the blessing
 * {@code "alice:friend:bob"} access only to the {@code Get} and {@code GetIndex} methods.
 * A peer presenting the blessing {@code "alice:colleague:carol"} will get access only to the
 * {@code Set} and {@code SetIndex} methods. A peer presenting {@code "alice:family:mom"} will
 * get access to all methods, even {@code GetAndSet} - which requires that the blessing appear
 * in the ACLs for both the {@code ReadAccess} and {@code WriteAccess} tags.
 */
public class PermissionsAuthorizer implements Authorizer {
    private static native PermissionsAuthorizer nativeCreate(Permissions perms, VdlType type)
            throws VException;
    private native void nativeFinalize(long nativeRef);

    /**
     * Creates a new {@link PermissionsAuthorizer} authorizer.
     *
     * @param  perms      ACLs containing authorization rules
     * @param  tagType    type of the method tags this authorizer checks
     * @return            a newly created authorizer
     * @throws VException if the authorizer couldn't be created
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

    @Override
    protected void finalize() {
        nativeFinalize(this.nativeRef);
    }
}
