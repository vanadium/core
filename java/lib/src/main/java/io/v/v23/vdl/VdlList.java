// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

import java.util.Collection;
import java.util.Iterator;
import java.util.List;
import java.util.ListIterator;

/**
 * VdlList is a representation of a VDL list.
 * It is a wrapper around {@code java.util.List} that stores a VDL {@code Type}.
 *
 * @param <T> The type of the list element.
 */
public class VdlList<T> extends VdlValue implements List<T> {
    private static final long serialVersionUID = 1L;

    private final List<T> impl;

    /**
     * Wraps a list with a VDL value.
     *
     * @param type runtime VDL type of the wrapped list
     * @param impl wrapped list
     */
    public VdlList(VdlType type, List<T> impl) {
        super(type);
        assertKind(Kind.LIST);
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
        return (impl == null ? 0 : impl.hashCode());
    }

    @Override
    public String toString() {
        return impl.toString();
    }

    @Override
    public void add(int location, T object) {
        impl.add(location, object);
    }

    @Override
    public boolean add(T object) {
        return impl.add(object);
    }

    @Override
    public boolean addAll(int location, Collection<? extends T> collection) {
        return impl.addAll(location, collection);
    }

    @Override
    public boolean addAll(Collection<? extends T> collection) {
        return impl.addAll(collection);
    }

    @Override
    public void clear() {
        impl.clear();
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
    public T get(int location) {
        return impl.get(location);
    }

    @Override
    public int indexOf(Object object) {
        return impl.indexOf(object);
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
    public int lastIndexOf(Object object) {
        return impl.lastIndexOf(object);
    }

    @Override
    public ListIterator<T> listIterator() {
        return impl.listIterator();
    }

    @Override
    public ListIterator<T> listIterator(int location) {
        return impl.listIterator(location);
    }

    @Override
    public T remove(int location) {
        return impl.remove(location);
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
    public T set(int location, T object) {
        return impl.set(location, object);
    }

    @Override
    public int size() {
        return impl.size();
    }

    @Override
    public VdlList<T> subList(int start, int end) {
        return new VdlList<T>(vdlType(), impl.subList(start, end));
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
