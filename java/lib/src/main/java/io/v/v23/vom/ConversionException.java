// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vom;

import java.lang.reflect.Type;

/**
 * An exception occurred during value conversion.
 */
public class ConversionException extends Exception {
    private static final long serialVersionUID = 1L;

    public ConversionException(String msg) {
        super(msg);
    }

    public ConversionException(Throwable cause) {
        super(cause);
    }

    public ConversionException(String msg, Throwable cause) {
        super(msg, cause);
    }

    public ConversionException(Object value, Type targetType) {
        super("Can't convert from " + value + " to " + targetType);
    }

    public ConversionException(Object value, Type targetType, Throwable cause) {
        super("Can't convert from " + value + " to " + targetType, cause);
    }

    public ConversionException(Object value, Type targetType, String msg) {
        super("Can't convert from " + value + " to " + targetType + ": " + msg);
    }
}
