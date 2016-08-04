// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import io.v.v23.vdl.NativeTypes;
import io.v.v23.vdl.VdlValue;

/**
 * Converter that translates a {@link Discharge} into its wire type and vice-versa.
 * <p>
 * This class is used by the VOM encoder to automatically convert the wire type into its Java native
 * type and vice-versa.
 */
public class DischargeNativeConverter extends NativeTypes.Converter {
    private DischargeNativeConverter() {
        super(WireDischarge.class);
    }

    /**
     * A singleton instance of {@link DischargeNativeConverter}.
     */
    public static final DischargeNativeConverter INSTANCE = new DischargeNativeConverter();

    @Override
    public VdlValue vdlValueFromNative(Object nativeValue) {
        assertInstanceOf(nativeValue, Discharge.class);
        return ((Discharge) nativeValue).wireFormat();
    }

    @Override
    public Object nativeFromVdlValue(VdlValue value) {
        assertInstanceOf(value, WireDischarge.class);
        return new Discharge((WireDischarge) value);
    }
}
