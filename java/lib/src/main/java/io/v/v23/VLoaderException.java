// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23;

import com.google.common.collect.ImmutableList;

import java.util.List;

/**
 * An error that has occurred while loading the Vanadium native shared library.
 */
public class VLoaderException extends RuntimeException {
    private static final long serialVersionUID = 1L;

    private final List<Throwable> exceptions;

    VLoaderException(List<Throwable> exceptions) {
        super(getMessages(exceptions));
        this.exceptions = ImmutableList.copyOf(exceptions);
    }

    /**
     * Returns the list of exceptions that were encountered when trying to load the native shared
     * library.
     */
    public List<Throwable> getExceptions() {
        return exceptions;
    }

    private static String getMessages(List<Throwable> exceptions) {
        String ret = "";
        for (Throwable e : exceptions) {
            if (!ret.isEmpty()) {
                ret += "; ";
            }
            ret += e.getMessage();
            if (e.getCause() != null) {
                ret += ": " + e.getCause().getMessage();
            }
        }
        return ret;
    }
}
