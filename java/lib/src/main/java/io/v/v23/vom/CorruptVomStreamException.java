// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vom;

import java.io.IOException;

/**
 * CorruptVomStreamException indicates that the VOM stream is incorrectly
 * formatted.
 */
public class CorruptVomStreamException extends IOException {
    private static final long serialVersionUID = 1L;

    public CorruptVomStreamException(String message) {
        super(message);
    }
}
