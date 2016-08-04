// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.rpc;

import com.google.common.base.Preconditions;
import com.google.common.collect.AbstractIterator;

import java.lang.reflect.Type;
import java.util.Iterator;

import io.v.v23.VFutures;
import io.v.v23.VIterable;
import io.v.v23.verror.CanceledException;
import io.v.v23.verror.EndOfFileException;
import io.v.v23.verror.VException;

/**
 * Implements a {@link VIterable} from the provided {@link Stream} by repeatedly invoking
 * {@link Stream#recv} methods on the stream.
 */
public class StreamIterable<T> implements VIterable<T> {
    private final Stream stream;
    private final Type type;
    private boolean isCreated;
    private volatile VException error;

    public StreamIterable(Stream stream, Type type) {
        this.stream = stream;
        this.type = type;
    }
    @Override
    public synchronized Iterator<T> iterator() {
        Preconditions.checkState(!isCreated, "Can only create one iterator.");
        isCreated = true;
        return new AbstractIterator<T>() {
            @Override
            protected T computeNext() {
                try {
                    return (T) VFutures.sync(stream.recv(type));
                } catch (EndOfFileException e) {  // legitimate end of stream
                    return endOfData();
                } catch (CanceledException e) {  // context canceled
                    return endOfData();
                } catch (VException e) {
                    error = e;
                    return endOfData();
                }
            }
        };
    }
    @Override
    public VException error() {
        return error;
    }
}
