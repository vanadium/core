// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google.lib.discovery;

import io.v.v23.context.VContext;
import io.v.v23.verror.VException;

/**
 * A utility function for using a mock plugin in discovery.
 * <p>
 * This is only for testing. We put this in main package to allow other platforms
 * like mojo to use it for testing.
 */
public class FactoryUtil {
    /**
     * Allows a runtime to use a mock discovery plugin.
     * <p>
     * This should be called before V.newDiscovery() is called.
     */
    public static native void injectMockPlugin(VContext ctx) throws VException;
}
