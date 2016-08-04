// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vom;

import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.BlockJUnit4ClassRunner;
import org.junit.runners.model.InitializationError;

import java.net.URLClassLoader;

import io.v.testing.SkipOnAndroid;
import io.v.v23.vdl.Types;
import io.v.v23.vdl.VdlBool;
import io.v.v23.vdl.VdlType;
import io.v.v23.vom.testdata.types.NBool;

import static org.junit.Assert.assertEquals;
import static org.junit.Assert.fail;

/**
 * These tests test that BinaryDecoder automatically registers type generated from VDL.
 */
// Since this test class exercises static initializers, we run it in a SeparateClassloaderTestRunner
// to ensure that tests that have run before this test didn't cause the static initializers to run
// already.
@RunWith(BinaryDecoderTypeRegistryTest.SeparateClassloaderTestRunner.class)
@SkipOnAndroid
public class BinaryDecoderTypeRegistryTest {

    @Test
    public void testGuessType() throws Exception {
        VdlType vdlType = Types.named("v.io/v23/vom/testdata/types.NBool", Types.BOOL);
        String encoded = TestUtil.encode(Constants.DEFAULT_VERSION, new VdlBool(vdlType, true));
        // Make sure that the class NBool is not loaded yet.
        try {
            Types.getReflectTypeForVdl(vdlType);
            fail("Class NBool is already loaded");
        } catch (IllegalArgumentException expected) {
        }
        Object value = TestUtil.decode(TestUtil.hexStringToBytes(encoded));
        assertEquals(NBool.class, value.getClass());
        assertEquals(new NBool(true), value);
    }

    public static class SeparateClassloaderTestRunner extends BlockJUnit4ClassRunner {
        public SeparateClassloaderTestRunner(Class<?> clazz) throws InitializationError {
            super(getFromTestClassloader(clazz));
        }

        private static Class<?> getFromTestClassloader(Class<?> clazz) throws InitializationError {
            try {
                ClassLoader testClassLoader = new TestClassLoader();
                return Class.forName(clazz.getName(), true, testClassLoader);
            } catch (ClassNotFoundException e) {
                throw new InitializationError(e);
            }
        }

        public static class TestClassLoader extends URLClassLoader {
            public TestClassLoader() {
                super(((URLClassLoader) getSystemClassLoader()).getURLs());
            }

            @Override
            public Class<?> loadClass(String name) throws ClassNotFoundException {
                if (name.startsWith("io.v.")) {
                    return super.findClass(name);
                }
                return super.loadClass(name);
            }
        }
    }
}
