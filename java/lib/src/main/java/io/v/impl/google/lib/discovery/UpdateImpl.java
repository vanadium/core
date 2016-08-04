// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google.lib.discovery;

import com.google.common.base.Function;
import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableMap;
import com.google.common.collect.Maps;
import com.google.common.util.concurrent.Futures;
import com.google.common.util.concurrent.ListenableFuture;

import java.util.Arrays;
import java.util.List;

import io.v.v23.VFutures;
import io.v.v23.context.VContext;
import io.v.v23.discovery.AdId;
import io.v.v23.discovery.Advertisement;
import io.v.v23.discovery.Attachments;
import io.v.v23.discovery.Attributes;
import io.v.v23.discovery.Update;
import io.v.v23.verror.VException;
import io.v.v23.vom.VomUtil;

import io.v.impl.google.ListenableFutureCallback;

class UpdateImpl implements Update {
    private final long nativeRef;

    private boolean lost;
    private Advertisement ad;
    private long timestampNs;

    private native void nativeAttachment(
            long nativeRef, VContext ctx, String name, ListenableFutureCallback<byte[]> callback)
            throws VException;

    private native void nativeFinalize(long nativeRef);

    private UpdateImpl(long nativeRef, boolean lost, Advertisement ad, long timestampNs) {
        this.nativeRef = nativeRef;
        this.lost = lost;
        this.ad = ad;
        this.timestampNs = timestampNs;
    }

    @Override
    public boolean isLost() {
        return lost;
    }

    @Override
    public AdId getId() {
        return ad.getId();
    }

    @Override
    public String getInterfaceName() {
        return ad.getInterfaceName();
    }

    @Override
    public List<String> getAddresses() {
        return ImmutableList.copyOf(ad.getAddresses());
    }

    @Override
    public String getAttribute(String name) {
        Attributes attributes = ad.getAttributes();
        if (attributes != null) {
            return attributes.get(name);
        }
        return null;
    }

    @Override
    public ListenableFuture<byte[]> getAttachment(VContext ctx, final String name)
            throws VException {
        synchronized (ad) {
            Attachments attachments = ad.getAttachments();
            if (attachments != null) {
                if (attachments.containsKey(name)) {
                    byte[] data = attachments.get(name);
                    return Futures.immediateFuture(Arrays.copyOf(data, data.length));
                }
            }
        }

        ListenableFutureCallback<byte[]> callback = new ListenableFutureCallback<>();
        nativeAttachment(nativeRef, ctx, name, callback);
        return VFutures.withUserLandChecks(
                ctx,
                Futures.transform(
                        callback.getVanillaFuture(),
                        new Function<byte[], byte[]>() {
                            @Override
                            public byte[] apply(byte[] data) {
                                synchronized (ad) {
                                    Attachments attachments = ad.getAttachments();
                                    if (attachments == null) {
                                        attachments = new Attachments();
                                        ad.setAttachments(attachments);
                                    }
                                    attachments.put(name, data);
                                    return Arrays.copyOf(data, data.length);
                                }
                            }
                        }));
    }

    @Override
    public Advertisement getAdvertisement() {
        return new Advertisement(
                ad.getId(),
                ad.getInterfaceName(),
                ImmutableList.copyOf(ad.getAddresses()),
                new Attributes(ImmutableMap.copyOf(ad.getAttributes())),
                new Attachments(
                        Maps.transformValues(
                                ad.getAttachments(),
                                new Function<byte[], byte[]>() {
                                    @Override
                                    public byte[] apply(byte[] data) {
                                        return Arrays.copyOf(data, data.length);
                                    }
                                })));
    }

    @Override
    public long getTimestampNs() {
        return timestampNs;
    }

    @Override
    public String toString() {
        return String.format(
                "{%b %s %s %s %s}",
                lost,
                VomUtil.bytesToHexString(ad.getId().toPrimitiveArray()),
                ad.getInterfaceName(),
                ad.getAddresses(),
                ad.getAttributes());
    }

    @Override
    protected void finalize() throws Throwable {
        super.finalize();
        nativeFinalize(nativeRef);
    }
}
