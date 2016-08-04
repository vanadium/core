// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.security;

import com.google.common.collect.ImmutableList;
import io.v.v23.context.VContext;
import io.v.v23.security.access.Permissions;
import io.v.v23.security.access.PermissionsAuthorizer;
import io.v.v23.verror.VException;
import io.v.v23.vom.VomUtil;
import org.joda.time.DateTime;

import java.lang.reflect.Type;
import java.security.InvalidKeyException;
import java.security.KeyPair;
import java.security.KeyPairGenerator;
import java.security.NoSuchAlgorithmException;
import java.security.PrivateKey;
import java.security.SignatureException;
import java.security.interfaces.ECPublicKey;
import java.util.Arrays;
import java.util.List;

/**
 * Various functions used for creating and managing Vanadium security primitives.
 */
public class VSecurity {
    private static native String[] nativeGetRemoteBlessingNames(VContext context, Call call)
            throws VException;
    private static native String[] nativeGetLocalBlessingNames(VContext context, Call call)
            throws VException;
    private static native String[] nativeGetBlessingNames(VPrincipal principal, Blessings blessings)
            throws VException;
    private static native String[] nativeGetSigningBlessingNames(VContext context,
            VPrincipal principal, Blessings blessings) throws VException;
    private static native void nativeAddToRoots(VPrincipal principal, Blessings blessings)
            throws VException;


    /**
     * Creates a new instance of {@link VSigner} using the provided public/private key pair.
     * <p>
     * The returned {@link VSigner} respests the location of the private key: if the key
     * is stored in a safe place (e.g., hardware token), it will remain secure.
     *
     * @param  privKey private key
     * @param  pubKey  corresponding public key
     * @return         new instance of {@link VSigner} that uses the provided key pair to
     *                 do the signing
     */
    public static VSigner newSigner(PrivateKey privKey, ECPublicKey pubKey) {
        return new ECDSASigner(privKey, pubKey);
    }

    /**
     * Mints a new private key and creates a {@link VSigner} based on this key.  The key is stored
     * in the clear in memory of the running process.
     */
    public static VSigner newInMemorySigner() throws VException {
        try {
            KeyPairGenerator keyGen = KeyPairGenerator.getInstance("EC");
            keyGen.initialize(256);
            KeyPair keyPair = keyGen.generateKeyPair();
            PrivateKey privKey = keyPair.getPrivate();
            ECPublicKey pubKey = (ECPublicKey) keyPair.getPublic();
            return new ECDSASigner(privKey, pubKey);
        } catch (NoSuchAlgorithmException e) {
            throw new VException("Couldn't mint private key: " + e.getMessage());
        }
    }

    /**
     * Mints a new private key and generates a principal based on this key, storing its
     * blessing roots and blessing store in memory.
     *
     * @return                 in-memory principal using the newly minted private key
     * @throws VException      if the principal couldn't be created
     */
    public static VPrincipal newPrincipal() throws VException {
        return VPrincipalImpl.create();
    }

    /**
     * Creates a principal using the provided signer, storing its blessing roots and
     * blessing store in memory.
     *
     * @param  signer          signer to be used by the new principal
     * @return                 in-memory principal using the provided signer
     * @throws VException      if the principal couldn't be created
     */
    public static VPrincipal newPrincipal(VSigner signer) throws VException {
        return VPrincipalImpl.create(signer);
    }

    /**
     * Creates a principal using the provided signer, blessing store, and blessing roots.  If the
     * provided store is {@code null}, the principal will use a store whose every opration will
     * fail.  If the provided roots are {@code null}, the principal will not trust any public keys
     * and all subsequent {@link VSecurity#addToRoots} operations with that principal will fail.
     *
     * @param  signer signer to be used by the principal
     * @param  store  blessing store to be used by the principal
     * @param  roots  blessing roots to be used by the principal
     * @return        newly created principal
     */
    public static VPrincipal newPrincipal(VSigner signer, BlessingStore store, BlessingRoots roots)
        throws VException {
        return VPrincipalImpl.create(signer, store, roots);
    }

    /**
     * Reads the entire state for a principal (i.e., private key, blessing roots, blessing store)
     * from the provided directory {@code dir} and commits all state changes to the same directory.
     * <p>
     * If the directory does not contain state, a new private key is minted and all state of the
     * principal is committed to {@code dir}. If the directory does not exist, it is created.
     *
     * @param  passphrase      passphrase used to encrypt the private key; if empty, no encryption
     *                         is done
     * @param  dir             directory where the state for a principal is to be persisted
     * @return                 principal whose state is persisted in the provided directory
     * @throws VException      if the principal couldn't be created
     */
    public static VPrincipal newPersistentPrincipal(String passphrase, String dir)
            throws VException {
        return VPrincipalImpl.createPersistent(passphrase, dir);
    }

    /**
     * Creates a new principal using the provided signer and a partial state (i.e., blessing roots,
     * blessing store) that is read from the provided directory {@code dir}.  Changes to the
     * partial state are persisted and commited to the same directory.  The provided signer isn't
     * persisted: the caller is expected to persist it separately.
     * <p>
     * If the directory does not contain any partial state, a new partial state instances are
     * created and subsequently commited to {@code dir}.  If the directory does not exist, it
     * is created.
     *
     * @param  signer          signer to be used by the new principal
     * @param  dir             directory where the partial state for a principal is to be persisted
     * @return                 principal whose partial state is persisted in the provided directory
     * @throws VException      if the principal couldn't be created
     */
    public static VPrincipal newPersistentPrincipal(VSigner signer, String dir)
        throws VException {
        return VPrincipalImpl.createPersistent(signer, dir);
    }

    /**
     * Returns a {@link Blessings} object that carries the union of the provided blessings.
     * All provided blessings must have the same public key.  Returns {@code null} if invoked
     * without arguments.
     *
     * @param  blessings       blessings that will be merged
     * @return                 the union of the provided blessings
     * @throws VException      if there was an error creating an union
     */
    public static Blessings unionOfBlessings(Blessings... blessings) throws VException {
        return Blessings.createUnion(blessings);
    }

    /**
     * Returns a validated set of blessing names presented by the remote end of a call.
     * <p>
     * These returned blessings (strings) are guaranteed to:
     * <p><ol>
     *     <li>Satisfy all the caveats given the call.</li>
     *     <li>Be rooted in {@code call.localPrincipal().roots()}.</li>
     * </ol>
     * <p>
     * Caveats are considered satisfied in the given call if the {@link CaveatValidator}
     * implementation can be found in the address space of the caller and
     * {@link CaveatValidator#validate validate} doesn't throw an exception.
     *
     * @param  context  vanadium context
     * @param  call     security-related state associated with the call
     * @return          validated set of blessing names presented by the remote end of a call
     *                  (possibly empty, but never {@code null})
     */
    public static String[] getRemoteBlessingNames(VContext context, Call call) {
        try {
            return nativeGetRemoteBlessingNames(context, call);
        } catch (VException e) {
            throw new RuntimeException("Couldn't get blessings for call", e);
        }
    }

    /**
     * Returns the set of human-readable blessing names presented by the local end of the call.
     * <p>
     * This is merely a convenience over
     * {@code getBlessingNames(call.localPrincipal(), call.localBlessings())}
     *
     * @param  context vanadium context
     * @param  call    security-related state associated with the call
     * @return         set of blessing names presented by the local end of the call
     *                 (possibly empty, but never {@code null})
     */
    public static String[] getLocalBlessingNames(VContext context, Call call) {
        try {
            return nativeGetLocalBlessingNames(context, call);
        } catch (VException e) {
            throw new RuntimeException("Couldn't get blessings for call", e);
        }
    }

    /**
     * Returns the set of human-readable blessing names encapsulated in blessings.
     * <p>
     * The returned names are guaranteed to be rooted in {@link VPrincipal#roots}
     * though caveats may not be validated.
     * <p>
     * The blessings must be bound to the provided principal. There is
     * intentionally not API to obtain blessing names bound to other principals
     * by ignoring caveats.  This is to prevent accidental authorization based
     * on potentially invalid blessing names (since caveats are not validated).
     *
     * @param principal principal to which the provided blessings are bound
     * @param blessings the blessings whose names are to be extracted
     * @return          set of blessing names bound to principal, encapsulated
     *                  in blessings (possibly empty, but never {@code null})
     */
    public static String[] getBlessingNames(VPrincipal principal, Blessings blessings) {
        try {
            return nativeGetBlessingNames(principal, blessings);
        } catch (VException e) {
            throw new RuntimeException("Couldn't get blessing names", e);
        }
    }

    /**
     * Returns the set of validated human-readable signing blessing names encapsulated in the
     * provided blessings object, as determined by the provided principal.  Only names that are
     * available for signing are returned by this method, even though the given blessings object
     * might have more blessing names that are valid at the time. (The name on a certificate chain
     * is valid if all the caveats on the chain are signing caveats, and the chain's validity can
     * be checked by the given principal).
     * <p>
     * The blessing names are guaranteed to be rooted in {@code call.localPrincipal().roots()}.
     * <p>
     * This method does not validate caveats on the blessing names.
     *
     * @param  context      vanadium context
     * @param  principal    vanadium principal
     * @param  blessings    vanadium blessings
     * @return              validated set of human-readable blessing names encapsulated in the
     *                      provided signing blessings object, as determined by the provided
     *                      {@link VPrincipal} (possibly empty, but never {@code null})
     */
    public static String[] getSigningBlessingNames(VContext context, VPrincipal principal,
            Blessings blessings) {
        try {
            return nativeGetSigningBlessingNames(context, principal, blessings);
        } catch (VException e) {
            throw new RuntimeException("Couldn't get signing blessing names", e);
        }
    }

    /**
     * Returns a caveat that requires validation by the validator corresponding to the
     * given descriptor and uses the provided parameter.
     *
     * @param  desc            caveat descriptor
     * @param  param           caveat parameter used by the associated validator
     * @return                 caveat that requires validation by the validator corresponding to the
     *                         given descriptor and uses the provided parameter
     * @throws VException      if the caveat couldn't be created
     */
    public static Caveat newCaveat(CaveatDescriptor desc, Object param) throws VException {
        byte[] paramVOM = VomUtil.encode(param, desc.getParamType().getTypeObject());
        return new Caveat(desc.getId(), paramVOM);
    }

    /**
     * Returns a caveat that validates iff the current time is before the provided time.
     *
     * @param  time            time before which the caveat validates
     * @return                 caveat that validates if the current time is before the provided time
     * @throws VException      if the caveat couldn't be created
     */
    public static Caveat newExpiryCaveat(DateTime time) throws VException {
        return newCaveat(Constants.EXPIRY_CAVEAT, time);
    }

    /**
     * Returns a caveat that validates iff the method being invoked by the peer is listed in an
     * argument to this function.
     *
     * @param  method            name of the method for which this caveat should validate
     * @param  additionalMethods additional method names for which this caveat should validate
     * @return                   caveat that validates iff the method being invoked by the peer is
     *                           one of the provided methods
     * @throws VException        if the caveat couldn't be created
     */
    public static Caveat newMethodCaveat(String method, String... additionalMethods)
            throws VException {
        List<String> methods = ImmutableList.<String>builder()
                .add(method)
                .add(additionalMethods)
                .build();
        return newCaveat(Constants.METHOD_CAVEAT, methods);
    }

    /**
     * Returns a caveat that never fails to validate. This is useful only for providing
     * unconstrained blessings to another principal.
     *
     * @return a caveat that never fails to validate
     */
    public static Caveat newUnconstrainedUseCaveat() throws VException {
        return newCaveat(Constants.CONST_CAVEAT, true);
    }

    /**
     * Returns a new security call that uses the provided params.
     *
     * @param params call params
     * @return       new security call that uses the provided params
     */
    public static Call newCall(CallParams params) {
        return new CallParamsImpl(params);
    }

    /**
     * Returns an authorizer that subscribes to an authorization policy where access is granted if
     * the remote end presents blessings included in the Access Control Lists (ACLs) associated with
     * the set of relevant tags.
     * <p>
     * See {@link io.v.v23.security.access.PermissionsAuthorizer} for a more detailed description
     * of this authorizer.
     *
     * @param  acls            ACLs containing authorization rules
     * @param  type            type of the method tags this authorizer checks
     * @return                 a newly created authorizer
     * @throws VException      if the authorizer couldn't be created
     */
    public static Authorizer newPermissionsAuthorizer(Permissions acls, Type type)
        throws VException {
        return PermissionsAuthorizer.create(acls, type);
    }

    private enum AuthorizerTypes {
        ALLOW_EVERYONE_AUTHORIZER,
        ENDPOINT_AUTHORIZER,
        DEFAULT_AUTHORIZER,
        PUBLIC_KEY_AUTHORIZER
    }
    private static native Authorizer nativeCreateAuthorizer(int type, ECPublicKey key);

    /**
     * Returns an authorizer that allows all requests.
     */
    public static Authorizer newAllowEveryoneAuthorizer() {
        return nativeCreateAuthorizer(AuthorizerTypes.ALLOW_EVERYONE_AUTHORIZER.ordinal(), null);
    }

    /**
     * Returns an authorizer that authorizes principals iff they present blessings that
     * match those specified in {@link Call#remoteEndpoint()}.
     */
    public static Authorizer newEndpointAuthorizer() {
        return nativeCreateAuthorizer(AuthorizerTypes.ENDPOINT_AUTHORIZER.ordinal(), null);
    }

    /**
     * Returns an authorizer that implements a "reasonably secure"
     * authorization policy that can be used whenever in doubt.
     * <p>
     * It has the conservative policy that requires one end of the RPC to have a
     * blessing that is extended from the blessing presented by the other end.
     */
    public static Authorizer newDefaultAuthorizer() {
        return nativeCreateAuthorizer(AuthorizerTypes.DEFAULT_AUTHORIZER.ordinal(), null);
    }

    /**
     * Returns an authorizer that only authorizes principals with a specific public key.
     * <p>
     * Normally, authorizations in Vanadium should be based on blessing names and not
     * public keys, since the former are resilient to key rotations and process
     * replication. However, in rare circumstances it may be possible that blessing names
     * cannot be used (for example, if the local end does not recognize the remote end's
     * blessing root), and the public key might be usable instead.
     */
    public static Authorizer newPublicKeyAuthorizer(ECPublicKey key) {
        return nativeCreateAuthorizer(AuthorizerTypes.PUBLIC_KEY_AUTHORIZER.ordinal(), key);
    }

    /**
     * Verifies the provides signature of the given message, using the supplied public key.
     *
     * @param  sig             signature in the Vanadium format
     * @param  key             public key
     * @param  message         message whose signature is verified
     * @throws VException      iff the signature doesn't verify
     */
    public static void verifySignature(VSignature sig, ECPublicKey key, byte[] message)
        throws VException {
        String vHashAlgorithm = sig.getHash().getValue();
        String verifyAlgorithm = CryptoUtil.javaSigningAlgorithm(vHashAlgorithm);
        try {
            message = CryptoUtil.messageDigest(vHashAlgorithm, message, sig.getPurpose(), key);
            byte[] jSig = CryptoUtil.javaSignature(sig);
            java.security.Signature verifier = java.security.Signature.getInstance(verifyAlgorithm);
            verifier.initVerify(key);
            verifier.update(message);
            if (!verifier.verify(jSig)) {
                throw new VException("Signature doesn't verify.");
            }
        } catch (NoSuchAlgorithmException e) {
            throw new VException("Verifying algorithm " + verifyAlgorithm +
                    " not supported by the runtime: " + e.getMessage());
        } catch (InvalidKeyException e) {
            throw new VException("Invalid private key: " + e.getMessage());
        } catch (SignatureException e) {
            throw new VException(
                "Invalid signing data [ " + Arrays.toString(message) + " ]: " + e.getMessage());
        }
    }

    /**
     * Marks the root principals of all blessing chains represented by {@code blessings} as an
     * authority on blessing chains beginning at that root.
     * <p>
     * For example, if {@code blessings} represents the blessing chains
     * {@code ["alice/friend/spouse", "charlie/family/daughter"]} then {@code addToRoots(blessing)}
     * will mark the root public key of the chain {@code "alice/friend/bob"} as the as authority on
     * all blessings that match the pattern {@code "alice/..."}, and root public key of the chain
     * {@code "charlie/family/daughter"} as an authority on all blessings that match the pattern
     * {@code "charlie/..."}.
     *
     * @param  principal       the principal whose {@link BlessingRoots} object should be edited
     * @param  blessings       blessings to be used as authorities on blessing chains beginning at
     *                         those roots
     * @throws VException      if there was an error assigning the said authorities
     */
    public static void addToRoots(VPrincipal principal, Blessings blessings) throws VException {
        nativeAddToRoots(principal, blessings);
    }

    private VSecurity() {}
}
