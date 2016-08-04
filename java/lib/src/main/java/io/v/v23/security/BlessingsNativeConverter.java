// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import io.v.v23.vdl.NativeTypes;
import io.v.v23.vdl.VdlValue;

/**
 * Converter that translates blessings into their wire representation and vice-versa.
 * <p>
 * This class is used by the VOM encoder to automatically convert the wire
 * type into its Java native type and vice-versa.
 */
public final class BlessingsNativeConverter extends NativeTypes.Converter {
    /**
     * Singleton instance of the {@link BlessingsNativeConverter}.
     */
    public static final BlessingsNativeConverter INSTANCE = new BlessingsNativeConverter();

    private BlessingsNativeConverter() {
        super(WireBlessings.class);
    }

    @Override
    public VdlValue vdlValueFromNative(Object nativeValue) {
        assertInstanceOf(nativeValue, Blessings.class);
        return ((Blessings) nativeValue).wireFormat();
    }

    @Override
    public Object nativeFromVdlValue(VdlValue value) {
        assertInstanceOf(value, WireBlessings.class);
        return Blessings.create((WireBlessings) value);
    }
}
