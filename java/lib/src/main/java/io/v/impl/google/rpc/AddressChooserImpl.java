// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google.rpc;

import io.v.v23.rpc.AddressChooser;
import io.v.v23.rpc.NetworkAddress;
import io.v.v23.verror.VException;

/**
 * An implementation of {@link AddressChooser} interface that calls to native
 * code for most of its functionalities.
 */
public class AddressChooserImpl implements AddressChooser {
    private final long nativeRef;

    private native NetworkAddress[] nativeChoose(long nativeRef,
            String protocol, NetworkAddress[] candidates) throws VException;
    private native void nativeFinalize(long nativeRef);

    private AddressChooserImpl(long nativeRef) {
        this.nativeRef = nativeRef;
    }
    @Override
    public NetworkAddress[] choose(String protocol, NetworkAddress[] candidates) throws VException {
        return nativeChoose(this.nativeRef, protocol, candidates);
    }
    @Override
    protected void finalize() {
        nativeFinalize(this.nativeRef);
    }
}
