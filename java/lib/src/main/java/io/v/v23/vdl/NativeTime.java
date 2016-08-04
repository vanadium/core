// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

import org.joda.time.DateTimeZone;

import io.v.v23.vdl.NativeTypes.Converter;

/**
 * NativeTime provides helpers to convert values of Java native types to VDL wire representation
 * for {@code org.joda.time.DateTime} and {@code org.joda.time.Duration}.
 */
public class NativeTime {
    static final long MILLIS_PER_SECOND = 1000;
    static final long NANOS_PER_MILLISECOND = 1000 * 1000;
    static final long SECONDS_PER_DAY = 24 * 60 * 60;

    /**
     * Represent the java epoch 1970-01-01 in terms of our epoch 0001-01-01 for easy conversions.
     * Note that we use a proleptic Gregorian calendar; there is a leap year every 4 years, except
     * for years divisible by 100, but including years divisible by 400.
     */
    static final long JAVA_EPOCH = (1969 * 365 + 1969 / 4 - 1969 / 100 + 1969 / 400)
            * SECONDS_PER_DAY * MILLIS_PER_SECOND;

    /**
     * Converts {@code org.joda.time.Duration}.
     */
    static final class DurationConverter extends Converter {
        static final DurationConverter INSTANCE = new DurationConverter();

        private DurationConverter() {
            super(io.v.v23.vdlroot.time.Duration.class);
        }

        @Override
        public VdlValue vdlValueFromNative(Object nativeValue) {
            assertInstanceOf(nativeValue, org.joda.time.Duration.class);
            long millis = ((org.joda.time.Duration) nativeValue).getMillis();
            return new io.v.v23.vdlroot.time.Duration(millis / MILLIS_PER_SECOND,
                    (int) ((millis % MILLIS_PER_SECOND) * NANOS_PER_MILLISECOND));
        }

        @Override
        public Object nativeFromVdlValue(VdlValue value) {
            assertInstanceOf(value, io.v.v23.vdlroot.time.Duration.class);
            io.v.v23.vdlroot.time.Duration wireDuration = (io.v.v23.vdlroot.time.Duration) value;
            return new org.joda.time.Duration(wireDuration.getSeconds() * MILLIS_PER_SECOND
                    + wireDuration.getNanos() / NANOS_PER_MILLISECOND);
        }
    }

    /**
     * Converts {@code org.joda.time.DateTime}.
     */
    static final class DateTimeConverter extends Converter {
        static final DateTimeConverter INSTANCE = new DateTimeConverter();

        private DateTimeConverter() {
            super(io.v.v23.vdlroot.time.Time.class);
        }

        @Override
        public VdlValue vdlValueFromNative(Object nativeValue) {
            assertInstanceOf(nativeValue, org.joda.time.DateTime.class);
            long millis = ((org.joda.time.DateTime) nativeValue).getMillis() + JAVA_EPOCH;
            return new io.v.v23.vdlroot.time.Time(millis / MILLIS_PER_SECOND,
                    (int) ((millis % MILLIS_PER_SECOND) * NANOS_PER_MILLISECOND));
        }

        @Override
        public Object nativeFromVdlValue(VdlValue value) {
            assertInstanceOf(value, io.v.v23.vdlroot.time.Time.class);
            io.v.v23.vdlroot.time.Time wireTime = (io.v.v23.vdlroot.time.Time) value;
            long millis = wireTime.getSeconds() * MILLIS_PER_SECOND - JAVA_EPOCH
                    + wireTime.getNanos() / NANOS_PER_MILLISECOND;
            return new org.joda.time.DateTime(millis, DateTimeZone.UTC);
        }
    }
}
