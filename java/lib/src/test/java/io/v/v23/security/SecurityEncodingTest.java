// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import com.google.common.collect.ImmutableList;
import io.v.v23.vdl.VdlType;
import io.v.v23.vdl.VdlValue;
import io.v.v23.vom.BinaryDecoder;
import io.v.v23.vom.BinaryEncoder;
import junit.framework.TestCase;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.IOException;

import static com.google.common.truth.Truth.assertThat;

/**
 * Tests that package-private classes such as {@link Nonce} can be encoded and decoded.
 */
public class SecurityEncodingTest extends TestCase {
    public void testNonceEncoding() throws Exception {
        Nonce n = new Nonce();
        n.set(0, (byte) 0x01);
        new BinaryDecoder(new ByteArrayInputStream(vdlEncode(n))).decodeValue(Nonce.class);
    }

    public void testPublicKeyThirdPartyCaveatParamEncoding() throws Exception {
        PublicKeyThirdPartyCaveatParam param = new PublicKeyThirdPartyCaveatParam();
        param.setDischargerKey(new byte[]{1, 2, 3, 4});
        param.setDischargerLocation("Hello");
        ThirdPartyRequirements requirements = new ThirdPartyRequirements();
        requirements.setReportServer(true);
        param.setDischargerRequirements(requirements);
        param.setCaveats(ImmutableList.of(new Caveat()));
        assertThat(new BinaryDecoder(new ByteArrayInputStream(vdlEncode(param))).decodeValue
                (PublicKeyThirdPartyCaveatParam.class)).isEqualTo(param);
    }

    private byte[] vdlEncode(VdlValue value) throws IOException {
        return vdlEncode(value.vdlType(), value);
    }

    private byte[] vdlEncode(VdlType type, Object value) throws IOException {
        ByteArrayOutputStream stream = new ByteArrayOutputStream();
        new BinaryEncoder(stream).encodeValue(type, value);
        return stream.toByteArray();
    }
}
