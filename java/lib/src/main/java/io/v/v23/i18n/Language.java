// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.i18n;

import io.v.v23.context.VContext;

/**
 * Provides various language-related utilities.
 */
public class Language {
    private static Catalog defaultCatalog = new Catalog();

    private Language() {}

    /**
     * Returns a child of the given context that has the provided language attached to it.
     *
     * @param  base     base context
     * @param  language a language that is to be attached
     * @return          a child of the given context that has the provided language attached to it
     */
    public static VContext contextWithLanguage(VContext base, String language) {
        return base.withValue(new LanguageTagKey(), language);
    }

    /**
     * Returns the language associated with the provided context or the empty language if
     * there is none.
     *
     * @param  context context the language is associated with
     * @return         the language associated with the above context
     */
    public static String languageFromContext(VContext context) {
        if (context == null) return "";
        Object value = context.value(new LanguageTagKey());
        if (value == null || !(value instanceof String)) {
            return "";
        }
        return (String) value;
    }

    /**
     * Returns the default {@link Catalog}.
     */
    public static Catalog getDefaultCatalog() {
        return defaultCatalog;
    }

    /**
     * Returns a base language given a language.  It is the first hyphen-separated segment of an
     * IETF language tag.
     *
     * @param  language input language
     * @return          base language of the input language
     */
    public static String baseLanguage(String language) {
        int idx = language.indexOf('-');
        if (idx == -1) return language;
        return language.substring(0, idx);
    }

    private static class LanguageTagKey {
        @Override
        public int hashCode() {
            return 0;
        }
    }
}
