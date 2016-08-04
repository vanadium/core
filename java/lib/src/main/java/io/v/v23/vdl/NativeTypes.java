// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

import java.lang.reflect.Type;

/**
 * NativeTypes provides helpers to register conversion of values from Java native types
 * (like org.joda.time) to VDL wire representation.
 */
public class NativeTypes {
    /**
     * Converts java native values to VDL wire representation.
     */
    public abstract static class Converter {
        private final Type wireType;

        public Converter(Type wireType) {
            this.wireType = wireType;
        }

        /**
         * Converts a java native value to a VDL value.
         *
         * @throws IllegalArgumentException if the native value has incorrect type
         */
        public abstract VdlValue vdlValueFromNative(Object nativeValue);

        /**
         * Converts a VDL value to a corresponding java native value.
         *
         * @throws IllegalArgumentException if the VDL value has incorrect type
         */
        public abstract Object nativeFromVdlValue(VdlValue value);

        /**
         * Returns VDL wire type corresponding to the java native type.
         */
        public Type getWireType() {
            return wireType;
        }

        /**
        * @throws IllegalArgumentException if value is not an instance of expected class
        */
        protected static void assertInstanceOf(Object value, Class<?> expectedClass) {
            if (!expectedClass.isAssignableFrom(value.getClass())) {
                throw new IllegalArgumentException("Unexpected value class: expected "
                        + expectedClass + ", got " + value.getClass());
            }
        }
    }
}
