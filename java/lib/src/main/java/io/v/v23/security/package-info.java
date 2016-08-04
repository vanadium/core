// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/**
 * Package security defines types and utilities associated with security.
 * <p><ul>
 *   <li>Concept: <a href="https://vanadium.github.io/concepts/security.html">https://vanadium.github.io/concepts/security.html</a></li>
 *   <li>Tutorial: (forthcoming)</li>
 * </ul><p>
 * The primitives and APIs defined in this package enable bi-directional,
 * end-to-end authentication between communicating parties; authorization based
 * on that authentication; and secrecy and integrity of all communication.
 * <p>
 * <strong>Overview</strong>
 * <p>
 * The Vanadium security model is centered around the concepts of principals
 * and blessings.
 * <p>
 * A principal in the Vanadium framework is a public and private key pair.
 * Every RPC is executed on behalf of a principal.
 * <p>
 * A blessing is a binding of a human-readable name to a principal, valid under
 * some caveats, given by another principal. A principal can have multiple
 * blessings bound to it. For instance, a television principal may have a
 * blessing from the manufacturer (e.g., {@code popularcorp/products/tv}) as well as
 * from the owner (e.g., {@code alice/devices/hometv}). Principals are authorized for
 * operations based on the blessings bound to them.
 * <p>
 * A principal can "bless" another principal by binding an extension of one of
 * its own blessings to the other principal. This enables delegation of
 * authority. For example, a principal with the blessing
 * {@code "johndoe"} can delegate to his phone by blessing the phone as
 * {@code "johndoe/phone"}, which in-turn can delegate to the headset by blessing it as
 * {@code "johndoe/phone/headset"}.
 * <p>
 * Caveats can be added to a blessing in order to restrict the contexts in which
 * it can be used. Amongst other things, caveats can restrict the duration of use and
 * the set of peers that can be communicated with using a blessing.
 * <p>
 * <strong>Navigating the interfaces</strong>
 * <p>
 * We recommend the following order in order to introduce yourself to the API:
 * <p><ul>
 *   <li>{@link io.v.v23.security.VPrincipal}</li>
 *   <li>{@link io.v.v23.security.Blessings}</li>
 *   <li>{@link io.v.v23.security.BlessingStore}</li>
 *   <li>{@link io.v.v23.security.BlessingRoots}</li>
 *   <li>{@link io.v.v23.security.VSecurity}</li>
 * </ul>
 * <p>
 * <strong>Examples</strong>
 * <p>
 * A principal can decide to name itself anything it wants:
 * <p><blockquote><pre>
 *  // (in process A)
 *  VPrincipal p1 = VSecurity.newPrincipal();
 *  Blessings alice := p1.blessSelf("alice");
 * </pre></blockquote><p>
 * This {@code alice} blessing can be presented to to another principal (typically a
 * remote process), but that other principal will not recognize this "self-proclaimed" authority:
 * <p><blockquote><pre>
 *  // (in process B)
 *  // context = current context
 *  // call = current security state
 *  VPrincipal p2 = VSecurity.newPrincipal();
 *  String[] names = VSecurity.getRemoteBlessingNames(ctx, call);
 *  System.out.println(Arrays.toString(names));  // Will print {@code ""}
 * </pre></blockquote><p>
 * However, {@code p2} can decide to trust the roots of the {@code "alice"} blessing and then it
 * will be able to recognize her delegates as well:
 * <p><blockquote><pre>
 *  // (in process B)
 *  VSecurity.addToRoots(p2, call.remoteBlessings());
 *  String[] names = VSecurity.getRemoteBlessingNames(ctx, call);
 *  System.out.println(Arrays.toString(names));  // Will print {@code "alice"}
 * </pre></blockquote><p>
 * Furthermore, {@code p2} can seek a blessing from {@code "alice"}:
 * <p><blockquote><pre>
 *  // (in process A)
 *  // call = call under which p2 is seeking a blessing from alice, call.localPrincipal() = p1
 *  ECPublicKey key2 = call.remoteBlessings().publicKey();
 *  Caveat onlyFor10Minutes = VSecurity.newExpiryCaveat(DateTime.now().plus(10000));
 *  Blessings aliceFriend = p1.bless(key2, alice, "friend", onlyFor10Minutes);
 *  sendBlessingToProcessB(aliceFriend);
 * </pre></blockquote><p>
 * {@code p2} can then add this blessing to its store such that this blessing will be
 * presented to {@code p1} anytime {@code p2} communicates with it in the future:
 * <p><blockquote><pre>
 *  // (in process B)
 *  p2.blessingStore().set(aliceFriend, new BlessingPattern("alice"));
 * </pre></blockquote><p>
 * {@code p2} can also choose to present multiple blessings to some servers:
 *  <p><blockquote><pre>
 *  // (in process B)
 *  Blessings charlieFriend = receiveBlessingFromSomewhere();
 *  Blessings union = VSecurity.unionOfBlessings(aliceFriend, charlieFriend);
 *  p2.blessingStore().set(union, new BlessingPattern("alice/mom"));
 * </pre></blockquote><p>
 * Thus, when communicating with a server that presents the blessing {@code "alice/mom"},
 * {@code p2} will declare that he is both "alice's friend" and "charlie's friend" and
 * the server may authorize actions based on this fact.
 * <p>
 * {@code p2} may also choose that it wants to present these two blessings when acting
 * as a "server", (i.e., when it does not know who the peer is):
 * <p><blockquote><pre>
 * // (in process B)
 * Blessings default = VSecurity.unionOfBlessings(aliceFriend, charlieFriend);
 * p2.blessingStore().SetDefault(default);
 * </pre></blockquote><p>
 */
package io.v.v23.security;
