// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.i18n;

import java.io.BufferedReader;
import java.io.BufferedWriter;
import java.io.IOException;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.io.OutputStreamWriter;
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.locks.ReadWriteLock;
import java.util.concurrent.locks.ReentrantReadWriteLock;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import io.v.v23.verror.VException;

/**
 * Catalog maps language names and message identifiers to message format strings.  The intent is
 * to provide a primitive form of String.format() in a way where the format string can depend upon
 * the language.
 * <p>
 * Languages are specified in the IETF language tag format.  Language tags can be obtained easily
 * by calling the {@link java.util.Locale#toLanguageTag} method.  Empty string indicates that the
 * default locale's language should be used.
 * <p>
 * Message identifier is a string that identifies a set of message format strings that have the
 * same meaning, but may be available in multiple languages.
 * <p>
 * A message format string is a string containing substrings of the form {@code {<number>}}
 * which are replaced by the corresponding position parameter (numbered from 1), or
 * {@code {_}}, which is replaced by all otherwise unused parameters.  If a substring is of the
 * form {@code {:<number>}}, {@code {<number>:}}, {@code {:<number>:}}, {@code {:_}}, {@code {_:}},
 * or {@code {:_:}} and the corresponding parameters are not the empty string, the parameter is
 * preceded by {@code ": "} or followed by {@code ":"}  or both, respectively.  For example, if the
 * format:
 * <p><blockquote><pre>
 * {3:} foo {2} bar{:_} ({3})
 * </pre></blockquote><p>
 * is used with arguments:
 * <p><blockquote><pre>
 * "1st", "2nd", "3rd", "4th"
 * </pre></blockquote><p>
 * it yields:
 * <p><blockquote><pre>
 * 3rd: foo 2nd bar: 1st 4th (3rd)
 * </pre></blockquote><p>
 *
 * The positional parameters may have any type and are printed in their default formatting.
 * If a particular formatting is desired, the parameter should be converted to a string first.
 * In principle, the default formating for a parameter may depend on a language tag.
 *
 * A typical usage pattern of this class goes like:
 * <p><blockquote><pre>
 * Catalog cat = Language.getDefaultCatalog();
 * String output = cat.format(language, msgID, "1st", "2nd", "3rd", "4th");
 * </pre></blockquote><p>
 */
public class Catalog {
    private static native String nativeFormatParams(String format, String[] params)
            throws VException;

    /**
     * Returns a copy of format with instances of {@code {1}}, {@code {2}}, etc replaced by the
     * default string representation of {@code params[0]}, {@code params[1]}, etc.  The last
     * instance of the string {@code {_}} is replaced with a space-separated list of positional
     * parameters unused by other {@code {...}} sequences.  Missing parameters are replaced
     * with {@code "?"}.
     *
     * @param  format message format
     * @param  params message parameters
     * @return        the result of applying the parameters to the given format
     */
    public static String formatParams(String format, Object... params) {
        try {
            return nativeFormatParams(format, convertParamsToStr(params));
        } catch (VException e) {
            throw new RuntimeException("Couldn't format params.", e);
        }
    }

    private static String[] convertParamsToStr(Object... params) {
        String[] ret = new String[params.length];
        for (int i = 0; i < params.length; ++i) {
            ret[i] = "" + params[i];
        }
        return ret;
    }

    // language -> (msgID -> format)
    private final Map<String, Map<String, String>> formats;
    private final ReadWriteLock lock;

    /**
     * Creates a new empty {@link Catalog}.
     */
    public Catalog() {
        this.formats = new HashMap<String, Map<String, String>>();
        this.lock = new ReentrantReadWriteLock();
    }

    /**
     * Returns the format corresponding to a given language and message identifier.
     * If no such message is known, any message for a base language is retrieved.
     * If no such message exists, empty string is returned.
     *
     * @param  language language that is looked up
     * @param  msgID    message identifier that is looked up
     * @return          a format corresponding to the given language and message identifier
     */
    public String lookup(String language, String msgID) {
        this.lock.readLock().lock();
        String fmt = lookupUnlocked(language, msgID);
        if (fmt.isEmpty()) {
            fmt = lookupUnlocked(Language.baseLanguage(language), msgID);
        }
        this.lock.readLock().unlock();
        return fmt;
    }

    private String lookupUnlocked(String language, String msgID) {
        Map<String, String> msgFmtMap = this.formats.get(language);
        if (msgFmtMap == null) {
            return "";
        }
        String fmt = msgFmtMap.get(msgID);
        return fmt == null ? "" : fmt;
    }

    /**
     * Finds the format corresponding to a given language and message identifier and then applies
     * {@link #formatParams formatParams} to it with the given params.  If the format lookup fails,
     * the result is the text of the message followed by a colon and then the parameters.
     *
     * @param  language language used for the format lookup
     * @param  msgID    message identifier used for the format lookup
     * @param  params   message parameters
     * @return          the result of applying the parameters to the looked-up format
     */
    public String format(String language, String msgID, Object... params) {
        String formatStr = lookup(language, msgID);
        if (formatStr.isEmpty()) {
            formatStr = msgID;
            if (params.length > 0) {
                formatStr += "{:_}";
            }
        }
        return formatParams(formatStr, params);
    }

    /**
     * Sets the format corresponding to the given language and message identifier.  If the format
     * string is empty, the corresponding entry is removed.  Any previous format string is returned.
     *
     * @param  language  language to be associated with the format
     * @param  msgID     message identifier to be associated with the format
     * @param  newFormat format assigned to the given language and message identifier
     * @return           previous format associated with the given language and message identifier
     */
    public String set(String language, String msgID, String newFormat) {
        this.lock.writeLock().lock();
        String oldFormat = setUnlocked(language, msgID, newFormat);
        this.lock.writeLock().unlock();
        return oldFormat;
    }

    /**
     * Just like {@link #set set} but additionaly sets the format for the base language if not
     * already set.  Note that if the new format is empty, the entry for the base language WILL NOT
     * be removed.
     *
     * @param  language  language to be associated with the format
     * @param  msgID     message identifier to be associated with the format
     * @param  newFormat format assigned to the given language and message identifier
     * @return           previous format associated with the given language and message identifier
     */
    public String setWithBase(String language, String msgID, String newFormat) {
        this.lock.writeLock().lock();
        String oldFormat = setUnlocked(language, msgID, newFormat);
        String baseLang = Language.baseLanguage(language);
        String baseFmt = lookupUnlocked(baseLang, msgID);
        if (baseFmt.isEmpty() && !newFormat.isEmpty() && !baseLang.equals(language)) {
            setUnlocked(baseLang, msgID, newFormat);
        }
        this.lock.writeLock().unlock();
        return oldFormat;
    }

    private String setUnlocked(String language, String msgID, String newFormat) {
        Map<String, String> msgFmtMap = this.formats.get(language);
        if (msgFmtMap == null) {
            msgFmtMap = new HashMap<String, String>();
            this.formats.put(language, msgFmtMap);
        }
        String oldFormat = msgFmtMap.get(msgID);
        if (newFormat != null && !newFormat.isEmpty()) {
            msgFmtMap.put(msgID, newFormat);
        } else {
            msgFmtMap.remove(msgID);
            if (msgFmtMap.isEmpty()) {
                this.formats.remove(language);
            }
        }
        return oldFormat == null ? "" : oldFormat;
    }

    /**
     * Merges the data in the lines from the provided input stream into the catalog.
     * Each line from the input stream is expected to be in the following format:
     * <p><blockquote><pre>
     *      {@literal <language> <message ID> "<escaped format string>"}
     * </pre></blockquote><p>
     * If a line starts with a {@code #} or cannot be parsed, the line is ignored.
     *
     * @param  in          input stream containing lines in the above format
     * @throws IOException if there was an error reading the input stream
     */
    public void merge(InputStream in) throws IOException {
        BufferedReader reader = new BufferedReader(new InputStreamReader(in));
        String line = null;
        Pattern pattern =
                Pattern.compile("^\\s*([^\\s\"]+)\\s+([^\\s\"]+)\\s+\"((?:[^\"]|\\\")*)\".*$");
        while ((line = reader.readLine()) != null) {
            Matcher matcher = pattern.matcher(line);
            if (matcher.matches() &&
                matcher.groupCount() == 3 && !matcher.group(1).startsWith("#")) {
                String language = matcher.group(1);
                String msgID = matcher.group(2);
                String format = matcher.group(3);
                set(language, msgID, format);
            }
        }
        reader.close();
    }

    /**
     * Emits the contents of the catalog into the provided output stream, in the format
     * expected by {@link #merge merge}.
     *
     * @param  out         output stream into which the catalog is emitted
     * @throws IOException if there was an error writing to the output stream
     */
    public void output(OutputStream out) throws IOException {
        BufferedWriter writer = new BufferedWriter(new OutputStreamWriter(out));
        this.lock.readLock().lock();
        for (Map.Entry<String, Map<String, String>> entry : this.formats.entrySet()) {
            String language = entry.getKey();
            for (Map.Entry<String, String> idFmt : entry.getValue().entrySet()) {
                String msgID = idFmt.getKey();
                String format = idFmt.getValue();
                writer.write(String.format("%s %s \"%s\"\n", language, msgID, format));
            }
        }
        this.lock.readLock().unlock();
        writer.close();
    }
}
