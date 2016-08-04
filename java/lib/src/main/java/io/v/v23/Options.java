// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23;

import com.google.common.collect.ImmutableMap;

import java.util.HashMap;
import java.util.Map;

/**
 * Placeholder for Vanadium options.  Each option has a {@link java.lang.String} key
 * and an arbitrary option value.
 */
public class Options {
    private Map<String, Object> options = new HashMap<String, Object>();

    /**
     * Returns {@code true} iff the option with the provided key exists.  (Note that if the
     * option exists but its value is {@code null}, this method will return {@code true}.)
     *
     * @param  key key of the option whose existance is checked
     * @return     {@code true} iff the option with the above key exists
     */
    public boolean has(String key) {
        return this.options.containsKey(key);
    }

    /**
     * Returns the option with the provided key.
     *
     * @param  key key of the option we want to get
     * @return     an option with the provided key, or {@code null} if no such option exists
     */
    public Object get(String key) {
        return this.options.get(key);
    }

    /**
     * Returns the option of the provided type with the given key.
     *
     * @param key              key of the option we want to get
     * @param type             type of the option we want to get
     * @return                 an option with the provided key, or {@code null} if no such
     *                         option exists
     */
    public <T> T get(String key, Class<T> type) {
        Object opt = this.options.get(key);
        if (opt == null) return null;
        if (!type.isAssignableFrom(opt.getClass())) {
            throw new RuntimeException(String.format(
                "Expected type %s for option %s, got: %s", type, key, opt.getClass()));
        }
        return type.cast(opt);
    }

    /**
     * Associates option value with the provided key.  If an option with the same key already
     * exists, its value will be overwritten.
     *
     * @param key   key of the option we are setting
     * @param value an option we are setting
     * @return this {@code Options} object
     */
    public Options set(String key, Object value) {
        options.put(key, value);
        return this;
    }

    /**
     * Removes the option with a given key.  This method is a no-op if the option doesn't exist.
     *
     * @param key a key of an option to be removed
     * @return this {@code Options} object
     */
    public Options remove(String key) {
        options.remove(key);
        return this;
    }

    /**
     * Returns an immutable copy of this {@code Options} object represented as a map.
     */
    public Map<String, Object> asMap() {
        return ImmutableMap.copyOf(options);
    }
}
