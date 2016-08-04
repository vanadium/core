// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

import io.v.v23.vom.testdata.data80.Constants;
import io.v.v23.vom.testdata.types.TestCase;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.ObjectInputStream;
import java.io.ObjectOutputStream;

/**
 * Tests that the VDL types are correctly parceled.
 */
public class SerializableTest extends junit.framework.TestCase {
    public void testSerializable() throws IOException, ClassNotFoundException {
        for (TestCase test : Constants.TESTS) {
            Object value = test.getValue().getElem();
            if (!(value instanceof VdlValue)) continue;

            // Write
            ByteArrayOutputStream data = new ByteArrayOutputStream();
            ObjectOutputStream out = new ObjectOutputStream(data);
            out.writeObject(value);
            out.close();

            // Read
            ObjectInputStream in =
                    new ObjectInputStream(new ByteArrayInputStream(data.toByteArray()));

            // Verify
            Object copy = in.readObject();
            assertEquals(String.format("serialization of %s", test.getName()), value, copy);
            assertEquals(String.format("original and deserialized %s", test.getName()), value.hashCode(), copy.hashCode());
        }
    }
}
