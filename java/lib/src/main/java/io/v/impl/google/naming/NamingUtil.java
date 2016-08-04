// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.impl.google.naming;

import java.io.ByteArrayOutputStream;
import java.util.Arrays;
import java.util.Iterator;
import java.util.List;

import com.google.common.base.CharMatcher;
import com.google.common.base.Joiner;
import com.google.common.base.Splitter;
import com.google.common.collect.ImmutableList;

/**
 * Utilities for dealing with Vanadium names.
 */
public class NamingUtil {
    /**
     * Takes an object name and returns the server address and the name relative to the server.
     * <p>
     * The name parameter may be a rooted name or a relative name; an empty string
     * address is returned for the latter case.
     * <p>
     * The returned address may be in endpoint format or {@code host:port} format.
     * <p>
     * The returned list is guaranteed to contain exactly two entries: the server address and
     * the name relative to the server.
     *
     * @param  name name from which the server address and relative name are extracted
     * @return      a list containing exactly two entries: the server address and
     *              the name relative to the server
     */
    public static List<String> splitAddressName(String name) {
        name = clean(name);
        if (!isRooted(name)) {
            return ImmutableList.of("", name);
        }
        name = name.substring(1, name.length()); // trim the beginning "/"
        if (name.isEmpty()) {
            return ImmutableList.of("", "");
        }
        // Could have used regular expressions, but that makes this function
        // 10x slower as per the benchmark.
        if (name.startsWith("@")) { // <endpoint>/<suffix>
            List<String> parts = splitInTwo(name, "@@/");
            String addr = parts.get(0), suffix = parts.get(1);
            if (!suffix.isEmpty()) { // The trailing "@@" was stripped, restore it
                addr += "@@";
            }
            return ImmutableList.of(addr, suffix);
        }
        if (name.startsWith("(")) { // (blessing)@host:[port]/suffix
            String tmp = splitInTwo(name, ")@").get(1);
            String suffix = splitInTwo(tmp, "/").get(1);
            String addr = trimSuffix(name, "/" + suffix);
            if (addr.endsWith("/" + suffix)) {
                addr = addr.substring(0, addr.length() - suffix.length() - 1);
            }
            return ImmutableList.of(addr, suffix);
        }
        // host:[port]/suffix
        List<String> parts = splitInTwo(name, "/");
        return ImmutableList.of(parts.get(0), parts.get(1));
    }

    private static List<String> splitInTwo(String str, String separator) {
       Iterator<String> iter = Splitter.on(separator).limit(2).split(str).iterator();
       return ImmutableList.of(
               iter.hasNext() ? iter.next() : "", iter.hasNext() ? iter.next() : "");
    }

    /**
     * Takes an address and a relative name and returns a rooted or relative name.
     * <p>
     * If a valid address is supplied then the returned name will always be a rooted name (i.e.
     * starting with {@code /}), otherwise it may be relative. {@code address} should not start
     * with a {@code /} and if it does, that prefix will be stripped.
     */
    public static String joinAddressName(String address, String name) {
        address = CharMatcher.is('/').trimLeadingFrom(address);
        if (address.isEmpty()) {
            return clean(name);
        }
        if (name.isEmpty()) {
            return clean("/" + address);
        }
        return clean("/" + address + "/" + name);
    }

    /**
     * Takes a variable number of name fragments and concatenates them together using {@code '/'}.
     * <p>
     * The returned name is cleaned of multiple adjacent {@code '/'}s.
     *
     * @param  names name fragments to be concatenated
     * @return       the concatenated (and cleaned) name
     */
    public static String join(String... names) {
        Iterator<String> iter = Arrays.asList(names).iterator();
        for (int i = 0; i < names.length && names[i].isEmpty(); ++i, iter.next());
        return clean(Joiner.on("/").join(iter));
    }

    /**
     * Splits the given name into fragments using {@code '/'} as the separator.
     * <p>
     * The returned list is cleaned of empty strings.
     */
    public static List<String> split(String name) {
        return Splitter.on("/").omitEmptyStrings().splitToList(name);
    }

    /**
     * Removes the suffix (and any connecting {@code /}) from the name.
     */
    public static String trimSuffix(String name, String suffix) {
        name = clean(name);
        suffix = clean(suffix);

        // Easy cases first.
        if (name.equals(suffix)) {
            return "";
        }
        if (suffix.length() >= name.length()) {
            return name;
        }

        // A suffix starting with a slash cannot be a partial match.
        if (suffix.startsWith("/")) {
            return name;
        }
        // At this point suffix is guaranteed not to start with a '/' and
        // suffix is shorter than name.
        if (name.endsWith(suffix)) {
            String prefix = name.substring(0, name.length() - suffix.length());
            if (prefix.endsWith("/")) {
                if (prefix.length() == 1) {
                    return name;
                }
                return prefix.substring(0, prefix.length() - 1);
            }
            return prefix;
        }
        return name;
    }

    /**
     * Reduces multiple adjacent slashes to a single slash and removes any trailing slash.
     */
    public static String clean(String name) {
        CharMatcher slashMatcher = CharMatcher.is('/');
        name = slashMatcher.collapseFrom(name, '/');
        if ("/".equals(name)) {
            return name;
        }
        return slashMatcher.trimTrailingFrom(name);
    }

    /**
     * Returns {@code true} iff the provided name is rooted.
     * <p>
     * A rooted name is one that starts with a single {@code /} followed by
     * a non-{@code /}.
     * <p>
     * {@code /} on its own is considered rooted.
     */
    public static boolean isRooted(String name) {
        return name.startsWith("/");
    }


    /**
     * Returns a string representable as a name element by escaping {@code /}.
     *
     * @param name Name to encode.
     * @return Encoded name.
     */
    public static String encodeAsNameElement(String name) {
        return escape(name, new char[]{'/'});
    }

    /**
     * Decodes an encoded name element.
     * <p>
     * Note that this is more than the inverse of {@link NamingUtil#encodeAsNameElement} since it
     * can handle more hex encodings than {@code /} and {@code %}.
     * This is intentional since we'll most likely want to add other letters to the set to be encoded.
     *
     * @param name Name to decode.
     * @return Decoded name.
     * @throws IllegalArgumentException if {@code name} is truncated or malformed.
     */
    public static String decodeFromNameElement(String name) {
        return unescape(name);
    }

    private static final String hexDigits = "0123456789ABCDEF";

    /**
     * Encodes a string replacing the characters in {@code special} and {@code %} with a {@code %<hex>} escape.
     *
     * @param text Text to escape.
     * @param special Collection of special characters to escape in {@code text}.
     * @return Encoded text.
     */
    public static String escape(String text, char[] special) {
        /*
         * Note that this function is a modified version of Android's URLEncoder in
         * Android SDK java/net/URLEncoder.java (https://goo.gl/61z5ZV).
         * The biggest difference is that, unlike URLEncoder, our code only encodes characters
         * in {@code special} rather than all non-ASCII characters.
         * We also do not convert space to +.
         */
        String specialStr = new String(special) + '%';

        // Avoid copying the string if it does not have any of the special characters.
        boolean hasSpecial = false;
        for (int i = 0; i < text.length(); i++) {
            char ch = text.charAt(i);
            if (specialStr.indexOf(ch) >= 0) {
                hasSpecial = true;
                break;
            }
        }

        if (!hasSpecial) {
            return text;
        }

        StringBuffer buf = new StringBuffer();
        for (int i = 0; i < text.length(); i++) {
            char ch = text.charAt(i);
            if (specialStr.indexOf(ch) < 0) {
                buf.append(ch);
            } else {
                byte[] bytes = new String(new char[] { ch }).getBytes();
                for (int j = 0; j < bytes.length; j++) {
                    buf.append('%');
                    buf.append(hexDigits.charAt((bytes[j] & 0xf0) >> 4));
                    buf.append(hexDigits.charAt(bytes[j] & 0xf));
                }
            }
        }
        return buf.toString();
    }

    /**
     * Decodes {@code}%<hex>} encodings in a string into the relevant character.
     *
     * @param text
     * @return Decoded text.
     * @throws IllegalArgumentException if the {@code text} is trucated or malformed.
     */
    public static String unescape(String text) {
        /*
         * Note that this function is a slightly modified version of Android's URLDecoder in
         * Android SDK java/net/URLDecoder.java (https://goo.gl/TFrx7E).
         * The biggest difference is that, unlike URLDecoder, our code does not convert + to space.
         * We also avoid string copying if text does not encoded to start with.
         */

        // Avoid string copying if text is not encoded to start with.
        if (text.indexOf('%') < 0){
            return text;
        }

        StringBuffer result = new StringBuffer(text.length());
        ByteArrayOutputStream out = new ByteArrayOutputStream();
        for (int i = 0; i < text.length();) {
            char c = text.charAt(i);
            if (c == '%') {
                out.reset();
                do {
                    if (i + 2 >= text.length()) {
                        throw new IllegalArgumentException("Truncated or malformed encoded string");
                    }
                    int d1 = Character.digit(text.charAt(i + 1), 16);
                    int d2 = Character.digit(text.charAt(i + 2), 16);
                    if (d1 == -1 || d2 == -1) {
                        throw new IllegalArgumentException("Truncated or malformed encoded string");
                    }
                    out.write((byte) ((d1 << 4) + d2));
                    i += 3;
                } while (i < text.length() && text.charAt(i) == '%');
                result.append(out.toString());
                continue;
            } else {
                result.append(c);
            }
            i++;
        }
        return result.toString();

    }

    private NamingUtil() {}
}
