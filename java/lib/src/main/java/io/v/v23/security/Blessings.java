// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import com.google.common.base.Joiner;

import java.io.IOException;
import java.io.ObjectInputStream;
import java.io.ObjectOutputStream;
import java.io.Serializable;
import java.security.interfaces.ECPublicKey;
import java.util.ArrayList;
import java.util.List;

import io.v.v23.verror.VException;

/**
 * Encapsulator of all the cryptographic operations required to prove that a set of blessings
 * (human-readable strings) have been bound to a principal in a specific call.
 * <p>
 * {@link Blessings} objects are meant to be presented to other principals to authenticate
 * and authorize actions.  Functions {@link VSecurity#getLocalBlessingNames} and
 * {@link VSecurity#getRemoteBlessingNames} can be used to uncover the blessing names encapsulated
 * in these objects.
 * <p>
 * {@link Blessings} objects are immutable and multiple threads may invoke methods on
 * them simultaneously.
 * <p>
 * See also: <a href="https://vanadium.github.io/glossary.html#blessing">https://vanadium.github.io/glossary.html#blessing</a>.
 */
public final class Blessings implements Serializable {
    private static final long serialVersionUID = 1L;

    private static native long nativeCreate(WireBlessings wire) throws VException;
    private static native Blessings nativeCreateUnion(Blessings[] blessings) throws VException;

    public static Blessings create(WireBlessings wire) {
        try {
            return new Blessings(nativeCreate(wire), wire);
        } catch (VException e) {
            throw new RuntimeException("Couldn't create blessings from WireBlessings", e);
        }
    }

    static Blessings createUnion(Blessings... blessings) throws VException {
        return nativeCreateUnion(blessings);
    }

    private long nativeRef;
    private volatile WireBlessings wire;  // can be null

    private native ECPublicKey nativePublicKey(long nativeRef) throws VException;
    private native Blessings nativeSigningBlessings(long nativeRef) throws VException;
    private native WireBlessings nativeWireFormat(long nativeRef) throws VException;

    private native void nativeFinalize(long nativeRef);

    private Blessings(long nativeRef) {
        this.nativeRef = nativeRef;
    }

    private Blessings(long nativeRef, WireBlessings wire) {
        this.nativeRef = nativeRef;
        this.wire = wire;
    }

    /**
     * Returns the public key of the principal to which blessings in this object are bound.
     * The return value may be {@code null} if the blessings are empty.
     */
    public ECPublicKey publicKey() {
        try {
            return nativePublicKey(nativeRef);
        } catch (VException e) {
            throw new RuntimeException("Couldn't get public key", e);
        }
    }

    /**
     * Returns the signing blessings in this object. (Signing blessings are blessings whose
     * caveats are universally understood and verifiable without third party discharges.)
     * The return value may be {@code null} if the blessings are empty.
     */
    public Blessings signingBlessings() {
        try {
            return nativeSigningBlessings(nativeRef);
      } catch (VException e) {
          throw new RuntimeException("Couldn't get signing blessings", e);
      }
    }

    /**
     * Returns the blessings in the wire format.
     */
    public WireBlessings wireFormat() {
        // Check the cache first as nativeWireFormat() is an expensive operation.
        synchronized (this) {
            if (wire != null) {
                return wire;
            }
        }
        WireBlessings ret = null;
        try {
            ret = nativeWireFormat(nativeRef);
        } catch (VException e) {
            throw new RuntimeException("Couldn't get wire blessings representation.", e);
        }
        synchronized (this) {
            if (wire == null) {
                wire = ret;
            }
        }
        return ret;
    }

    /**
     * Returns {@code true} iff the blessings are empty.
     */
    public boolean isEmpty() {
        return wireFormat().getCertificateChains().isEmpty();
    }

    /**
     * Returns the certificate chains stored inside the blessings.
     */
    public List<List<VCertificate>> getCertificateChains() {
        return wireFormat().getCertificateChains();
    }

    private long nativeRef() {
        return nativeRef;
    }

    private void writeObject(ObjectOutputStream out) throws IOException {
        out.writeObject(wireFormat());
    }

    private void readObject(ObjectInputStream in) throws IOException, ClassNotFoundException {
        wire = (WireBlessings) in.readObject();
        try {
            nativeRef = nativeCreate(wire);
        } catch (VException e) {
            throw new IOException("Couldn't create native blessings.", e);
        }
    }

    @Override
    public boolean equals(Object obj) {
        if (this == obj) return true;
        if (obj == null) return false;
        if (!(obj instanceof Blessings)) return false;
        Blessings other = (Blessings) obj;
        return wireFormat().equals(other.wireFormat());
    }

    @Override
    public int hashCode() {
        return wireFormat().hashCode();
    }

    @Override
    public String toString() {
        List<String> chains = new ArrayList<String>(getCertificateChains().size());
        for (List<VCertificate> certificateChain : getCertificateChains()) {
            List<String> certificateNames =
                    new ArrayList<String>(certificateChain.size());
            for (VCertificate certificate : certificateChain) {
                // TODO(ashankar): Remove once
                // https://github.com/vanadium/issues/issues/739
                // has been resolved.
                String e = certificate.getExtension().replaceAll("/", Constants.CHAIN_SEPARATOR);
                certificateNames.add(e);
            }
            chains.add(Joiner.on(Constants.CHAIN_SEPARATOR).join(certificateNames));
        }
        return Joiner.on(",").join(chains);
    }
    @Override
    protected void finalize() {
        nativeFinalize(nativeRef);
    }
}
