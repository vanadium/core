// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

import static io.v.v23.vdl.NativeTime.MILLIS_PER_SECOND;
import static io.v.v23.vdl.NativeTime.NANOS_PER_MILLISECOND;
import static org.joda.time.DateTimeZone.UTC;

import com.google.common.collect.ImmutableMap;

import junit.framework.TestCase;

import org.joda.time.DateTime;

import io.v.v23.vdl.NativeTypes.Converter;
import io.v.v23.vdlroot.time.Duration;
import io.v.v23.vdlroot.time.Time;

import java.util.Map;

/**
 * Tests conversion of java native time to its wire representation.
 */
public class NativeTimeTest extends TestCase {
    private final int NPMS = (int) NANOS_PER_MILLISECOND;
    private final int NPS = (int) (NANOS_PER_MILLISECOND * MILLIS_PER_SECOND);

    public void testTime() {
        final Map<Time, org.joda.time.DateTime> tests =
                ImmutableMap.<Time, org.joda.time.DateTime>builder()
                // DateTime(year, month, day, hour, minute, second, millisecond, timezone)
                .put(new Time(0, NPMS), new DateTime(1, 1, 1, 0, 0, 0, 1, UTC))
                .put(new Time(0, -NPMS), new DateTime(0, 12, 31, 23, 59, 59, 999, UTC))
                .put(new Time(1, 0),new DateTime(1, 1, 1, 0, 0, 1, 0, UTC))
                .put(new Time(1, NPMS), new DateTime(1, 1, 1, 0, 0, 1, 1, UTC))
                .put(new Time(1, -NPMS), new DateTime(1, 1, 1, 0, 0, 0, 999, UTC))
                .put(new Time(-1, 0), new DateTime(0, 12, 31, 23, 59, 59, 0, UTC))
                .put(new Time(-1, NPMS), new DateTime(0, 12, 31, 23, 59, 59, 1, UTC))
                .put(new Time(-1, -NPMS), new DateTime(0, 12, 31, 23, 59, 58, 999, UTC))
                .build();
        final Converter conv = NativeTime.DateTimeConverter.INSTANCE;
        for (Map.Entry<Time, DateTime> test : tests.entrySet()) {
            Time wireTime = test.getKey();
            assertEquals(test.getValue(), conv.nativeFromVdlValue(wireTime));
            if (wireTime.getSeconds() < 0 && wireTime.getNanos() > 0) {
                wireTime = new Time(wireTime.getSeconds() + 1, wireTime.getNanos() - NPS);
            } else if (wireTime.getSeconds() > 0 && wireTime.getNanos() < 0) {
                wireTime = new Time(wireTime.getSeconds() - 1, wireTime.getNanos() + NPS);
            }
            assertEquals(wireTime, conv.vdlValueFromNative(test.getValue()));
        }
    }

    public void testDuration() {
        final Map<Duration, org.joda.time.Duration> tests =
                ImmutableMap.<Duration, org.joda.time.Duration>builder()
                .put(new Duration(), new org.joda.time.Duration(0))
                .put(new Duration(0, NPMS), new org.joda.time.Duration(1))
                .put(new Duration(0, -NPMS), new org.joda.time.Duration(-1))
                .put(new Duration(1, 0), new org.joda.time.Duration(MILLIS_PER_SECOND))
                .put(new Duration(-1, 0), new org.joda.time.Duration(-MILLIS_PER_SECOND))
                .put(new Duration(1, NPMS), new org.joda.time.Duration(MILLIS_PER_SECOND + 1))
                .put(new Duration(-1, -NPMS), new org.joda.time.Duration(-MILLIS_PER_SECOND - 1))
                .build();
        final Converter conv = NativeTime.DurationConverter.INSTANCE;
        for (Map.Entry<Duration, org.joda.time.Duration> test : tests.entrySet()) {
            assertEquals(test.getValue(), conv.nativeFromVdlValue(test.getKey()));
            assertEquals(test.getKey(), conv.vdlValueFromNative(test.getValue()));
        }
    }
}
