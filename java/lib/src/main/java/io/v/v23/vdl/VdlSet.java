// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

import java.util.Collection;
import java.util.Iterator;
import java.util.Set;

/**
 * VdlSet is a representation of a VDL set.
 * It is a wrapper around {@code java.util.Set} that stores a VDL {@code Type}.
 *
 * @param <T> The type of the set element.
 */
public class VdlSet<T> extends VdlValue implements Set<T> {
    private static final long serialVersionUID = 1L;

    private final Set<T> impl;

    /**
     * Wraps a set with a VDL value.
     *
     * @param type runtime VDL type of the wrapped set
     * @param impl wrapped set
     */
    public VdlSet(VdlType type, Set<T> impl) {
        super(type);
        assertKind(Kind.SET);
        this.impl = impl;
    }

    @Override
    public boolean equals(Object obj) {
        if (this == obj) return true;
        if (obj == null) return false;
        return impl.equals(obj);
    }

    @Override
    public int hashCode() {
        return (impl == null) ? 0 : impl.hashCode();
    }

    @Override
    public String toString() {
        return impl.toString();
    }

    @Override
    public void clear() {
        impl.clear();
    }

    @Override
    public boolean add(T object) {
        return impl.add(object);
    }

    @Override
    public boolean addAll(Collection<? extends T> collection) {
        return impl.addAll(collection);
    }

    @Override
    public boolean contains(Object object) {
        return impl.contains(object);
    }

    @Override
    public boolean containsAll(Collection<?> collection) {
        return impl.containsAll(collection);
    }

    @Override
    public boolean isEmpty() {
        return impl.isEmpty();
    }

    @Override
    public Iterator<T> iterator() {
        return impl.iterator();
    }

    @Override
    public boolean remove(Object object) {
        return impl.remove(object);
    }

    @Override
    public boolean removeAll(Collection<?> collection) {
        return impl.removeAll(collection);
    }

    @Override
    public boolean retainAll(Collection<?> collection) {
        return impl.retainAll(collection);
    }

    @Override
    public int size() {
        return impl.size();
    }

    @Override
    public Object[] toArray() {
        return impl.toArray();
    }

    @Override
    public <E> E[] toArray(E[] array) {
        return impl.toArray(array);
    }
}
