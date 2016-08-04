// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vom;

import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableMap;
import com.google.common.reflect.TypeToken;

import junit.framework.TestCase;
import static com.google.common.truth.Truth.assertThat;

import org.joda.time.DateTime;
import org.joda.time.DateTimeZone;
import org.joda.time.Duration;

import io.v.v23.V;
import io.v.v23.vdl.Types;
import io.v.v23.vdl.VdlArray;
import io.v.v23.vdl.VdlType;
import io.v.v23.vdl.VdlValue;
import io.v.v23.verror.VException;

import java.lang.reflect.Type;
import java.util.Arrays;
import java.util.List;
import java.util.Map;

public class BinaryDecoderTest extends TestCase {
    static {
        V.init();
    }

    public void testDecode80() throws Exception {
        decodeTest(Constants.VERSION_80, io.v.v23.vom.testdata.data80.Constants.TESTS);
    }
    public void testDecode81() throws Exception {
        decodeTest(Constants.VERSION_81, io.v.v23.vom.testdata.data81.Constants.TESTS);
    }

    void decodeTest(Version version, List<io.v.v23.vom.testdata.types.TestCase> tests) throws Exception {
        for (io.v.v23.vom.testdata.types.TestCase test : tests) {
          byte[] bytes = TestUtil.hexStringToBytes(test.getHex());
          Object targetVal = test.getValue();
          if (test.getValue().getElem() != null) {
            targetVal = test.getValue().getElem();
          }
          Object value;
          if (test.getValue().getElem() != null && test.getValue().getElem().getClass().isArray()) {
              value = TestUtil.decode(bytes, test.getValue().getElem().getClass());
          } else {
              value = TestUtil.decode(bytes);
          }
          TestUtil.assertEqual(String.format("%s decode(%s) -> %s == %s", version, test.getName(), targetVal, value), targetVal, value);
        }
    }

    public void testDecodeEncode80() throws Exception {
        decodeEncodeTest(Constants.VERSION_80, io.v.v23.vom.testdata.data80.Constants.TESTS);
    }
    public void testDecodeEncode81() throws Exception {
        decodeEncodeTest(Constants.VERSION_81, io.v.v23.vom.testdata.data81.Constants.TESTS);
    }
    public void decodeEncodeTest(Version version, List<io.v.v23.vom.testdata.types.TestCase> tests) throws Exception {
        for (io.v.v23.vom.testdata.types.TestCase test : tests) {
          byte[] bytes = TestUtil.hexStringToBytes(test.getHex());
          VdlValue value = (VdlValue) TestUtil.decode(bytes, VdlValue.class);
          assertEquals(String.format("encode(%s) == %s", test.getName(), test.getHex()), test.getHex(), TestUtil.encode(version, value.vdlType(), value));
        }

        VdlType testsType = Types.getVdlTypeFromReflect(
                io.v.v23.vom.testdata.data80.Constants.class.getDeclaredField("TESTS").getGenericType());
        String encoded = TestUtil.encode(version, testsType, tests);
        VdlValue decoded = (VdlValue) TestUtil.decode(
                TestUtil.hexStringToBytes(encoded));
        assertEquals(encoded, TestUtil.encode(version, decoded.vdlType(), decoded));
    }

    public void testDecodeVdlArray() throws Exception {
        VdlArray<Byte> v = new VdlArray<Byte>(Types.arrayOf(4, Types.BYTE), new Byte[]{1, 2, 3, 4});
        byte[] encoded = TestUtil.hexStringToBytes(TestUtil.encode(Constants.DEFAULT_VERSION, v));
        Object decoded = TestUtil.decode(encoded, VdlValue.class);
        assertNotNull(decoded);
    }

    public void testDecodeVException() throws Exception {
        Object[] params = {
                1,
                "2",
                ImmutableList.<String>of("3"),
                ImmutableMap.<String, String>of("4", "")
        };
        Type[] paramTypes = {
                Integer.class,
                String.class,
                new TypeToken<List<String>>(){}.getType(),
                new TypeToken<Map<String, String>>(){}.getType()
        };
        VException.IDAction id = VException.register(
                "io.v.v23.vom.BinaryDecoderTest.testDecodeVException",
                VException.ActionCode.NO_RETRY, "{1} {2} {_}");
        VException v = new VException(id, "en", "test", "test", params, paramTypes);
        byte[] encoded = TestUtil.hexStringToBytes(TestUtil.encode(Constants.DEFAULT_VERSION, VException.class, v));
        Object decoded = TestUtil.decode(encoded);
        if (!v.deepEquals(decoded)) {
            fail(String.format("Expected error %s, got %s", v, decoded));
        }
    }

    public void testDecodeVExceptionBadParams() throws Exception {
        Object[] params = {
                ImmutableList.<String>of("3"),
                ImmutableMap.<String, String>of("4", "")
        };
        Type[] paramTypes = {
                List.class,
                Map.class
        };
        VException.IDAction id = VException.register(
                "io.v.v23.vom.BinaryDecoderTest.testDecodeVExceptionBadParams",
                VException.ActionCode.NO_RETRY, "{1} {2} {_}");
        VException v = new VException(id, "en", "test", "test", params, paramTypes);
        byte[] encoded = TestUtil.hexStringToBytes(TestUtil.encode(Constants.DEFAULT_VERSION, VException.class, v));
        Object decoded = TestUtil.decode(encoded);
        VException expected = new VException(id, "en", "test", "test");
        if (!expected.deepEquals(decoded)) {
            fail(String.format("Expected error %s, got %s", v, decoded));
        }
    }

    public void testDecodeEncodeVException() throws Exception {
        Object[] params = {
                1,
                "2",
                ImmutableList.<String>of("3"),
                ImmutableMap.<String, String>of("4", "")
        };
        Type[] paramTypes = {
                Integer.class,
                String.class,
                new TypeToken<List<String>>(){}.getType(),
                new TypeToken<Map<String, String>>(){}.getType()
        };
        VException.IDAction id = VException.register(
                "io.v.v23.vom.BinaryDecoderTest.testDecodeEncodeVException",
                VException.ActionCode.NO_RETRY, "{1} {2} {_}");
        VException v = new VException(id, "en", "test", "test", params, paramTypes);
        byte[] encoded = TestUtil.hexStringToBytes(TestUtil.encode(Constants.DEFAULT_VERSION, VException.class, v));
        Object decoded = TestUtil.decode(encoded);
        if (!(decoded instanceof VException)) {
            fail(String.format("Decoded into %s, wanted %s", decoded.getClass(), VException.class));
        }
        VException decodedV = (VException) decoded;
        byte[] reEncoded = TestUtil.hexStringToBytes(
                TestUtil.encode(Constants.DEFAULT_VERSION, VException.class, decodedV));
        assertEquals(Arrays.toString(encoded), Arrays.toString(reEncoded));
    }

    public void testDecodeSubVException() throws Exception {
        SubVException v = new SubVException();
        byte[] encoded = TestUtil.hexStringToBytes(TestUtil.encode(Constants.DEFAULT_VERSION, SubVException.class, v));
        Object decoded = TestUtil.decode(encoded);
        VException expected = new SubVException();
        assertThat(decoded).isInstanceOf(SubVException.class);
        if (!expected.deepEquals(decoded)) {
            fail(String.format("Expected error %s, got %s", v, decoded));
        }
    }

    private static class SubVException extends VException {
        static final io.v.v23.verror.VException.IDAction ID_ACTION =
                VException.register("v.io/v23/vom.BinaryDecoderTest$SubVException",
                        VException.ActionCode.NO_RETRY, "{1} {2} {_}");

        SubVException(VException e) {
            super(e);
        }

        SubVException() {
            super(ID_ACTION, "en", "test", "test", new Object[]{ 5 },
                    new Type[]{ Integer.class });
        }
    }

    private void assertDecodeEncode(Object value, Version version) throws Exception {
        byte[] encoded = TestUtil.hexStringToBytes(TestUtil.encode(version, value.getClass(), value));
        Object decoded = TestUtil.decode(encoded);
        assertEquals(value, decoded);
    }

    public void testDecodeEncodeTime() throws Exception {
        Version version = Constants.DEFAULT_VERSION;
        assertDecodeEncode(new DateTime(2015, 2, 18, 20, 34, 35, 997, DateTimeZone.UTC), version);
        assertDecodeEncode(new Duration(239017), version);
    }

    public void testDecodeEncodeJavaObject() throws Exception {
        Version version = Constants.DEFAULT_VERSION;
        assertDecodeEncode(new JavaObject(
                5, "boo", ImmutableList.of(new JavaObject(7, "foo", null))), version);
    }

    private static class JavaObject  {
        private int i;
        private final String s;
        private List<JavaObject> j;

        public JavaObject() {
            this.i = 0;
            this.s = "";
            this.j = null;
        }

        public JavaObject(int i, String s, List<JavaObject> j) {
            this.i = i;
            this.s = s;
            this.j = j;
        }

        @Override
        public boolean equals(Object o) {
            if (this == o) return true;
            if (o == null || getClass() != o.getClass()) return false;

            JavaObject that = (JavaObject) o;

            if (i != that.i) return false;
            if (s != null ? !s.equals(that.s) : that.s != null) return false;
            return !(j != null ? !j.equals(that.j) : that.j != null);
        }
    }
}
