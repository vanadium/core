// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vom;

import com.google.common.collect.ImmutableMultimap;
import com.google.common.reflect.TypeToken;

import junit.framework.TestCase;

import io.v.v23.vdl.Types;
import io.v.v23.vdl.VdlAny;
import io.v.v23.vdl.VdlInt32;
import io.v.v23.vdl.VdlOptional;
import io.v.v23.vdl.VdlType;
import io.v.v23.vdl.VdlTypeObject;
import io.v.v23.vdl.VdlUint32;
import io.v.v23.vdl.VdlValue;
import io.v.v23.vom.testdata.types.ConvertGroup;

import java.lang.reflect.Type;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * Tests for value conversions.
 */
public class ConversionTest extends TestCase {
    private final ImmutableMultimap<VdlValue, Type> tests =
            ImmutableMultimap.<VdlValue, Type>builder()
            .put(VdlOptional.of(new VdlUint32()), VdlInt32.class)
            .put(VdlOptional.of(new VdlUint32()),
                    new TypeToken<VdlOptional<VdlInt32>>(){}.getType())
            .put(new VdlUint32(), VdlInt32.class)
            .put(new VdlUint32(), new TypeToken<VdlOptional<VdlInt32>>(){}.getType())
            .put(new VdlAny(new VdlUint32()), VdlInt32.class)
            .put(new VdlAny(new VdlUint32()), VdlAny.class)
            .put(new VdlUint32(), VdlAny.class)
            .build();

    private static Object convert(Version version, VdlValue value, Type targetType) throws Exception {
        byte[] encoded = TestUtil.hexStringToBytes(TestUtil.encode(version, value));
        return TestUtil.decode(encoded, targetType);
    }

    public void testConversion() throws Exception {
        Version version = Constants.DEFAULT_VERSION;
        for (Map.Entry<VdlValue, Type> test : tests.entries()) {
            final byte[] encoded = TestUtil.hexStringToBytes(TestUtil.encode(version, test.getKey()));
            final Object decoded = TestUtil.decode(encoded, test.getValue());
            Class<?> targetClass = ReflectUtil.getRawClass(test.getValue());
            assertEquals(targetClass, decoded.getClass());
        }

        for (List<ConvertGroup> test : io.v.v23.vom.testdata.data80.Constants.CONVERT_TESTS.values()) {
            for (int i = 0; i < test.size(); i++) {
                ConvertGroup g = test.get(i);
                for (VdlAny value : g.getValues()) {
                    // Everything inside a convert group is convertible.
                    for (VdlAny otherValue : g.getValues()) {
                        Type targetType;
                        if (value.getElem().getClass().isArray()) {
                            targetType = value.getElem().getClass();
                        } else {
                            targetType = Types.getReflectTypeForVdl(value.getElemType());
                        }
                        TestUtil.assertEqual(String.format("convert %s -> %s == %s", otherValue, targetType, value.getElem()), value.getElem(), convert(version, otherValue, targetType));
                    }
                    // Element of one convert group can't be converted to primary type of
                    // previous groups.
                    for (int j = 0; j < i; j++) {
                        Type targetType = Types.getReflectTypeForVdl(
                                test.get(j).getPrimaryType().getTypeObject());
                        try {
                            convert(version, value, targetType);
                            fail("Converted " + value + " -> " + targetType);
                        } catch (Exception expected) {
                        }
                    }
                }
            }
        }
    }

    public void testTypeCompatibility() throws Exception {
        Map<VdlType, String> vdlTypes = new HashMap<VdlType, String>();
        for (Map.Entry<String, List<VdlTypeObject>> testGroup : io.v.v23.vom.testdata.data80.Constants.COMPAT_TESTS.entrySet()) {
            for (VdlTypeObject type : testGroup.getValue()) {
                vdlTypes.put(type.getTypeObject(), testGroup.getKey());
            }
        }
        for (Map.Entry<VdlType, String> i : vdlTypes.entrySet()) {
            for (Map.Entry<VdlType, String> j : vdlTypes.entrySet()) {
                boolean compatible = TypeCompatibility.compatible(i.getKey(), j.getKey());
                assertEquals("Types: " + i.getKey() + ", " + j.getKey(),
                        i.getValue().equals(j.getValue()), compatible);
            }
        }
    }
}
