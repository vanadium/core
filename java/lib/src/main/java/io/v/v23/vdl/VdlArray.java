// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.vdl;

import java.util.Arrays;
import java.util.Collection;
import java.util.Iterator;
import java.util.List;
import java.util.ListIterator;
import java.util.NoSuchElementException;

/**
 * VdlArray is a representation of a VDL array. It has fixed length and it is
 * backed by an array.
 *
 * @param <T> The type of the array element.
 */
public class VdlArray<T> extends VdlValue implements List<T> {
    private static final long serialVersionUID = 1L;

    private final T[] backingArray;
    private final int start, end;

    /**
     * Creates a VDL array based on a backing array. The backing array
     * isn't copied.
     *
     * @param type runtime VDL type of the wrapped array
     * @param backingArray The array that backs the fixed length list.
     */
    public VdlArray(VdlType type, final T[] backingArray) {
        this(type, backingArray, 0, backingArray.length);
    }

    /**
     * Creates a VDL array based on a backing array. The backing array
     * isn't copied.
     *
     * @param type runtime VDL type of the wrapped array
     * @param backingArray The array that backs the fixed length list.
     * @param start The index in the array where the list should start
     *            (inclusive index)
     * @param end The index after the point in the array where the list should
     *            end (exclusive index).
     */
    public VdlArray(VdlType type, final T[] backingArray, final int start, final int end) {
        super(type);
        assertKind(Kind.ARRAY);
        if (type.getLength() != end - start) {
            throw new IllegalArgumentException(
                    "Length of the array should be the same as specified in VDL type");
        }
        if (start < 0 || end > backingArray.length) {
            throw new IllegalArgumentException("indexes out of range of backing array");
        }
        this.backingArray = backingArray;
        this.start = start;
        this.end = end;
    }

    @Override
    public int hashCode() {
        // This specific hash code algorithm is required by the Java spec and
        // defined in the List interface Javadoc.
        int hashCode = 1;
        for (T e : this) {
            hashCode = 31 * hashCode + (e == null ? 0 : e.hashCode());
        }
        return hashCode;
    }

    private static boolean elementsEqual(Object e1, Object e2) {
        if (e1 == null) {
            return e2 == null;
        }
        return e1.equals(e2);
    }

    @Override
    public boolean equals(Object obj) {
        if (this == obj)
            return true;
        if (!(obj instanceof List))
            return false;
        List<?> other = (List<?>) obj;
        if (other.size() != size()) {
            return false;
        }
        for (int i = 0; i < size(); i++) {
            Object e1 = other.get(i);
            Object e2 = get(i);
            if (!elementsEqual(e1, e2)) {
                return false;
            }
        }
        return true;
    }

    @Override
    public String toString() {
        return Arrays.deepToString(backingArray);
    }

    @Override
    public void add(int location, T object) {
        throw new UnsupportedOperationException("add() not supported");
    }

    @Override
    public boolean add(T object) {
        throw new UnsupportedOperationException("add() not supported");
    }

    @Override
    public boolean addAll(int location, Collection<? extends T> collection) {
        throw new UnsupportedOperationException("addAll() not supported");
    }

    @Override
    public boolean addAll(Collection<? extends T> collection) {
        throw new UnsupportedOperationException("addAll() not supported");
    }

    @Override
    public void clear() {
        throw new UnsupportedOperationException("clear() not supported");
    }

    @Override
    public boolean contains(Object obj) {
        return indexOf(obj) >= 0;
    }

    @Override
    public boolean containsAll(Collection<?> collection) {
        for (Object obj : collection) {
            if (!this.contains(obj)) {
                return false;
            }
        }
        return true;
    }

    @Override
    public T get(int location) {
        if (location < start || location >= end) {
            throw new IndexOutOfBoundsException("index " + location + " outside of range [" + start
                    + "," + end + ")");
        }
        return backingArray[location];
    }

    @Override
    public int indexOf(Object object) {
        for (int i = start; i < end; i++) {
            T t = backingArray[i];
            if (elementsEqual(t, object)) {
                return i - start;
            }
        }
        return -1;
    }

    @Override
    public int lastIndexOf(Object object) {
        for (int i = end - 1; i >= start; i--) {
            T t = backingArray[i];
            if (elementsEqual(t, object)) {
                return i - start;
            }
        }
        return -1;
    }

    @Override
    public boolean isEmpty() {
        return start == end;
    }

    @Override
    public Iterator<T> iterator() {
        return listIterator();
    }

    @Override
    public ListIterator<T> listIterator() {
        return listIterator(0);
    }

    @Override
    public ListIterator<T> listIterator(int location) {
        return new VdlArrayIterator(location);
    }

    @Override
    public T remove(int location) {
        throw new UnsupportedOperationException("remove() not supported");
    }

    @Override
    public boolean remove(Object object) {
        throw new UnsupportedOperationException("remove() not supported");
    }

    @Override
    public boolean removeAll(Collection<?> collection) {
        throw new UnsupportedOperationException("removeAll() not supported");
    }

    @Override
    public boolean retainAll(Collection<?> collection) {
        throw new UnsupportedOperationException("retainAll() not supported");
    }

    @Override
    public T set(int location, T object) {
        if (location < start || location >= end) {
            throw new IndexOutOfBoundsException("index " + location + " outside of range [" + start
                    + "," + end + ")");
        }
        T prev = backingArray[location];
        backingArray[location] = object;
        return prev;
    }

    @Override
    public int size() {
        return end - start;
    }

    @Override
    public List<T> subList(int start, int end) {
        VdlType.Builder builder = new VdlType.Builder();
        VdlType.PendingType subListType = builder.newPending(Kind.ARRAY)
                .setLength(end - start).setElem(vdlType().getElem());
        builder.build();
        return new VdlArray<T>(subListType.built(), backingArray, start, end);
    }

    @Override
    public Object[] toArray() {
        return Arrays.copyOfRange(backingArray, start, end);
    }

    @Override
    @SuppressWarnings("unchecked")
    public <ToType> ToType[] toArray(ToType[] array) {
        if (array.length < size()) {
            return Arrays.copyOfRange(backingArray, start, end, (Class<ToType[]>) array.getClass());
        }
        System.arraycopy(backingArray, start, array, 0, size());
        return array;
    }

    private class VdlArrayIterator implements ListIterator<T> {
        private int position;

        public VdlArrayIterator(int index) {
            if (index < 0 || index >= VdlArray.this.size()) {
                throw new IllegalArgumentException("Index out of bounds");
            }
            position = index + start;
        }

        @Override
        public void add(T object) {
            throw new UnsupportedOperationException("add() not supported");
        }

        @Override
        public boolean hasNext() {
            return position < end;
        }

        @Override
        public boolean hasPrevious() {
            return position > start;
        }

        @Override
        public T next() {
            if (position >= end) {
                throw new NoSuchElementException("beyond end of list");
            }
            return backingArray[position++];
        }

        @Override
        public int nextIndex() {
            if (position >= end) {
                return end - start;
            }
            return position - start;
        }

        @Override
        public T previous() {
            if (position < start) {
                throw new NoSuchElementException("before beginning of list");
            }
            return backingArray[position--];
        }

        @Override
        public int previousIndex() {
            if (position < start) {
                return 0;
            }
            return position - start;
        }

        @Override
        public void remove() {
            throw new UnsupportedOperationException("remove() not supported");
        }

        @Override
        public void set(T object) {
            throw new UnsupportedOperationException("set() not supported");
        }

    }
}
