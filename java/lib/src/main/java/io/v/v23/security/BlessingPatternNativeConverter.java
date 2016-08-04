// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import io.v.v23.vdl.NativeTypes;
import io.v.v23.vdl.VdlValue;

/**
 * Converts blessing patterns into their wire representations and vice-versa.
 * <p>
 * This class is used by the VOM encoder to automatically convert the wire
 * type into its Java native type and vice-versa.
 */
public final class BlessingPatternNativeConverter extends NativeTypes.Converter {
    /**
     * Singleton instance of the {@link BlessingPatternNativeConverter}.
     */
    public static final BlessingPatternNativeConverter INSTANCE =
            new BlessingPatternNativeConverter();

    private BlessingPatternNativeConverter() {
        super(WireBlessingPattern.class);
    }

    @Override
    public VdlValue vdlValueFromNative(Object nativeValue) {
        assertInstanceOf(nativeValue, BlessingPattern.class);
        // Can't simply cast here as the VOM encoder expects the returned
        // value's getClass() method to return WireBlessingPattern.  (With
        // casting it would return BlessingPattern.)
        return new WireBlessingPattern(((BlessingPattern) nativeValue).getValue());
    }

    @Override
    public Object nativeFromVdlValue(VdlValue value) {
        assertInstanceOf(value, WireBlessingPattern.class);
        return new BlessingPattern((WireBlessingPattern) value);
    }
}
