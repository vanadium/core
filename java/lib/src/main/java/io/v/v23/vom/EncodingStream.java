// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vom;

import java.io.ByteArrayOutputStream;

/**
 * A stream to encode VDL values. Can discard a suffix of accumulated output.
 */
class EncodingStream extends ByteArrayOutputStream {
    int getCount() {
        return this.count;
    }

    void setCount(int count) {
        this.count = count;
    }
}
