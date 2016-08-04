// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security.access;

import java.lang.reflect.Type;

/**
 * Various access-related utilities.
 */
public class Access {
    /**
     * Returns the type of the predefined tags in this access package.
     * <p>
     * Typical use of this method is to setup an {@link io.v.v23.security.Authorizer} that uses
     * these predefined tags:
     * <p><blockquote><pre>
     * Authorizer authorizer = PermissionsAuthorizer.create(permissions, Access.typicalTagType());
     * </pre></blockquote><p>
     */
    public static Type typicalTagType() {
        return Tag.class;
    }

    private Access() {}
}
