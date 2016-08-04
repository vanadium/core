// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security.access;

import io.v.v23.vdl.NativeTypes;
import io.v.v23.vdl.VdlValue;

/**
 * Converts access lists into their wire representations and vice-versa.
 * <p>
 * This class is used by the VOM encoder to automatically convert the wire
 * type into its Java native type and vice-versa.
 */
public final class AccessListNativeConverter extends NativeTypes.Converter {
    /**
     * Singleton instance of {@link AccessListNativeConverter}.
     */
    public static final AccessListNativeConverter INSTANCE = new AccessListNativeConverter();

    private AccessListNativeConverter() {
        super(WireAccessList.class);
    }

    @Override
    public VdlValue vdlValueFromNative(Object nativeValue) {
        assertInstanceOf(nativeValue, AccessList.class);
        // Can't simply cast here as the VOM encoder expects the returned
        // value's getClass() method to return WireAccessList.  (With casting
        // it would return AccessList.)
        AccessList acl = (AccessList) nativeValue;
        return new WireAccessList(acl.getIn(), acl.getNotIn());
    }

    @Override
    public Object nativeFromVdlValue(VdlValue value) {
        assertInstanceOf(value, WireAccessList.class);
        return new AccessList((WireAccessList) value);
    }
}
