// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import org.joda.time.DateTime;

import java.util.Map;

import io.v.v23.vdl.VdlValue;

class CallParamsImpl implements Call {
    private final CallParams params;

    CallParamsImpl(CallParams params) {
        this.params = params;
    }

    @Override
    public DateTime timestamp() {
        return this.params.getTimestamp();
    }
    @Override
    public String method() {
        return this.params.getMethod();
    }
    @Override
    public VdlValue[] methodTags() {
        return this.params.getMethodTags();
    }
    @Override
    public String suffix() {
        return this.params.getSuffix();
    }
    @Override
    public Map<String, Discharge> localDischarges() {
        return params.getLocalDischarges();
    }
    @Override
    public Map<String, Discharge> remoteDischarges() {
        return params.getRemoteDischarges();
    }
    @Override
    public String localEndpoint() {
        return this.params.getLocalEndpoint();
    }
    @Override
    public String remoteEndpoint() {
        return this.params.getRemoteEndpoint();
    }
    @Override
    public VPrincipal localPrincipal() {
        return this.params.getLocalPrincipal();
    }
    @Override
    public Blessings localBlessings() {
        return this.params.getLocalBlessings();
    }
    @Override
    public Blessings remoteBlessings() {
        return this.params.getRemoteBlessings();
    }
}
