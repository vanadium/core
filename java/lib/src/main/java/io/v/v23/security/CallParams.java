// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import com.google.common.collect.ImmutableMap;

import org.joda.time.DateTime;

import java.util.Map;

import io.v.v23.context.VContext;
import io.v.v23.vdl.VdlValue;

/**
 * Container for {@link Call} creation parameters.  Here is an example of a simple
 * call creation:
 * <p><blockquote><pre>
 *     Call call = VSecurity.newCall(new CallParams()
 *           .withLocalPrincipal(VSecurity.newPrincipal())
 *           .withMethodName("test")
 *           .withTimestamp(DateTime.now());
 * </pre></blockquote><p>
 * {@link CallParams} form a tree where derived params are children of the params from which they
 * were derived.  Children inherit all the properties of their parent except for the property being
 * replaced (the principal/method/timestamp in the example above).
 */
public class CallParams {
    final CallParams parent;

    private DateTime timestamp;
    private String method;
    private VdlValue[] methodTags;
    private String suffix;
    private Map<String, Discharge> localDischarges;
    private Map<String, Discharge> remoteDischarges;
    private String localEndpoint;
    private String remoteEndpoint;
    private VPrincipal principal;
    private Blessings localBlessings;
    private Blessings remoteBlessings;
    private io.v.v23.context.VContext context;

    /**
     * Creates a new (and empty) {@link CallParams} object.
     */
    public CallParams() {
        this.parent = null;
    }

    private CallParams(CallParams parent) {
        this.parent = parent;
    }

    /**
     * Returns a child of the current params with the given timestamp attached.
     *
     * @param  time timestamp
     * @return      a child of the current params with the given timestamp attached
     */
    public CallParams withTimestamp(DateTime time) {
        CallParams ret = new CallParams(this);
        ret.timestamp = time;
        return ret;
    }
    /**
     * Returns a child of the current params with the given method name attached.
     *
     * @param  method method name
     * @return        a child of the current params with the given method name attached
     */
    public CallParams withMethod(String method) {
        CallParams ret = new CallParams(this);
        ret.method = method;
        return ret;
    }
    /**
     * Returns a child of the current params with the given method tags attached.
     *
     * @param  tags method tags
     * @return      a child of the current params with the given method tags attached
     */
    public CallParams withMethodTags(VdlValue... tags) {
        CallParams ret = new CallParams(this);
        ret.methodTags = tags;
        return ret;
    }
    /**
     * Returns a child of the current params with the given vanadium name suffix attached.
     *
     * @param  suffix vanadium name suffix
     * @return        a child of the current params with the given vanadium name suffix attached
     */
    public CallParams withSuffix(String suffix) {
        CallParams ret = new CallParams(this);
        ret.suffix = suffix;
        return ret;
    }
    /**
     * Returns a child of the current params with the given local discharges.
     */
    public CallParams withLocalDischarges(Map<String, Discharge> localDischarges) {
        CallParams ret = new CallParams(this);
        ret.localDischarges = ImmutableMap.copyOf(localDischarges);
        return ret;
    }
    /**
     * Returns a child of the current params with the given remote discharges.
     */
    public CallParams withRemoteDischarges(Map<String, Discharge> remoteDischarges) {
        CallParams ret = new CallParams(this);
        ret.remoteDischarges = ImmutableMap.copyOf(remoteDischarges);
        return ret;
    }
    /**
     * Returns a child of the current params with the given local endpoint attached.
     *
     * @param  endpoint local endpoint
     * @return          a child of the current params with the given local endpoint attached
     */
    public CallParams withLocalEndpoint(String endpoint) {
        CallParams ret = new CallParams(this);
        ret.localEndpoint = endpoint;
        return ret;
    }
    /**
     * Returns a child of the current params with the given remote endpoint attached.
     *
     * @param  endpoint remote endpoint
     * @return          a child of the current params with the given remote endpoint attached
     */
    public CallParams withRemoteEndpoint(String endpoint) {
        CallParams ret = new CallParams(this);
        ret.remoteEndpoint = endpoint;
        return ret;
    }
    /**
     * Returns a child of the current params with the given local principal attached.
     *
     * @param  principal local principal
     * @return           a child of the current params with the given local principal attached
     */
    public CallParams withLocalPrincipal(VPrincipal principal) {
        CallParams ret = new CallParams(this);
        ret.principal = principal;
        return ret;
    }
    /**
     * Returns a child of the current params with the given local blessings attached.
     *
     * @param  blessings local blessings
     * @return           a child of the current params with the given local blessings attached
     */
    public CallParams withLocalBlessings(Blessings blessings) {
        CallParams ret = new CallParams(this);
        ret.localBlessings = blessings;
        return ret;
    }
    /**
     * Returns a child of the current params with the given remote blessings attached.
     *
     * @param  blessings remote blessings
     * @return           a child of the current params with the given remote blessings attached
     */
    public CallParams withRemoteBlessings(Blessings blessings) {
        CallParams ret = new CallParams(this);
        ret.remoteBlessings = blessings;
        return ret;
    }
    /**
     * Returns a child of the current params with the given Vanadium context attached.
     *
     * @param  context Vanadium context
     * @return         a child of the current params with the given Vanadium context attached
     */
    public CallParams withContext(VContext context) {
        CallParams ret = new CallParams(this);
        ret.context = context;
        return ret;
    }
    /**
     * Returns a timestamp attached to the params, or {@code null} if no timestamp is attached.
     */
    public DateTime getTimestamp() {
        if (this.timestamp != null) return this.timestamp;
        if (this.parent != null) return this.parent.getTimestamp();
        return null;
    }
    /**
     * Returns a method name attached to the params, or {@code null} if no method name is attached.
     */
    public String getMethod() {
        if (this.method != null) return this.method;
        if (this.parent != null) return this.parent.getMethod();
        return null;
    }
    /**
     * Returns method tags attached to the params, or {@code null} if no method tags are attached.
     */
    public VdlValue[] getMethodTags() {
        if (this.methodTags != null) return this.methodTags;
        if (this.parent != null) return this.parent.getMethodTags();
        return null;
    }
    /**
     * Returns a name suffix attached to the params, or {@code null} if no name suffix is
     * attached.
     */
    public String getSuffix() {
        if (this.suffix != null) return this.suffix;
        if (this.parent != null) return this.parent.getSuffix();
        return null;
    }
    /**
     * Returns the discharges for third-party caveats presented by the local end of the call. It
     * maps a third-party caveat identifier to the corresponding discharge.
     */
    public Map<String, Discharge> getLocalDischarges() {
        return localDischarges;
    }
    /**
     * Returns the discharges for third-party caveats presented by the remote end of the call. It
     * maps a third-party caveat identifier to the corresponding discharge.
     */
    public Map<String, Discharge> getRemoteDischarges() {
        return remoteDischarges;
    }
    /**
     * Returns a local endpoint attached to the params, or {@code null} if no local endpoint is
     * attached.
     */
    public String getLocalEndpoint() {
        if (this.localEndpoint != null) return this.localEndpoint;
        if (this.parent != null) return this.parent.getLocalEndpoint();
        return null;
    }
    /**
     * Returns a remote endpoint attached to the params, or {@code null} if no remote endpoint
     * is attached.
     */
    public String getRemoteEndpoint() {
        if (this.remoteEndpoint != null) return this.remoteEndpoint;
        if (this.parent != null) return this.parent.getRemoteEndpoint();
        return null;
    }
    /**
     * Returns a local principal attached to the params, or {@code null} if no local principal is
     * attached.
     */
    public VPrincipal getLocalPrincipal() {
        if (this.principal != null) return this.principal;
        if (this.parent != null) return this.parent.getLocalPrincipal();
        return null;
    }
    /**
     * Returns local blessings attached to the params, or {@code null} if no local blessings are
     * attached.
     */
    public Blessings getLocalBlessings() {
        if (this.localBlessings != null) return this.localBlessings;
        if (this.parent != null) return this.parent.getLocalBlessings();
        return null;
    }
    /**
     * Returns remote blessings attached to the params, or {@code null} if no remote blessings are
     * attached.
     */
    public Blessings getRemoteBlessings() {
        if (this.remoteBlessings != null) return this.remoteBlessings;
        if (this.parent != null) return this.parent.getRemoteBlessings();
        return null;
    }
    /**
     * Returns Vanadium context attached to the params, or {@code null} if no Vanadium context is
     * attached.
     */
    public VContext getContext() {
        if (this.context != null) return this.context;
        if (this.parent != null) return this.parent.getContext();
        return null;
    }
}
