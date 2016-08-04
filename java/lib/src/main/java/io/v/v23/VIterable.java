// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23;

import io.v.v23.verror.VException;

/**
 * An interface for iterating through a collection of elements.
 * <p>
 * The {@link java.util.Iterator} returned by the {@link java.lang.Iterable#iterator iterator}:
 * <p><ul>
 *     <li>can be created <strong>only</strong> once,
 *     <li>does not support {@link java.util.Iterator#remove remove}</li>,
 *     <li>terminates early if the underlying iteration step fails; in that case, {@link #error}
 *         method returns the error that caused the iteration step to fail.
 * </ul>
 */
public interface VIterable<T> extends Iterable<T> {
    /**
     * Returns an error (if any) that caused the iterator to terminate early.  Returns {@code null}
     * if the iterator terminated gracefully.
     */
    VException error();
}
