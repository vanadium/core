// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v.v23.verror;

import junit.framework.TestCase;

import io.v.v23.V;
import io.v.v23.context.VContext;
import io.v.v23.i18n.Catalog;
import io.v.v23.i18n.Language;

/**
 * VExceptionTest tests the {@code VException} implementation.
 */
public class VExceptionTest extends TestCase {
    // Some languages
    private static final String EN = "en";
    private static final String FR = "fr";
    private static final String DE = "de";

    private static final VException.IDAction idActionA;
    private static final VException.IDAction idActionB;
    private static final VException.IDAction idActionC;

    private static final VException aEN0;
    private static final VException aEN1;
    private static final VException aFR0;
    private static final VException aFR1;
    private static final VException aDE0;
    private static final VException aDE1;

    private static final VException bEN0;
    private static final VException bEN1;
    private static final VException bFR0;
    private static final VException bFR1;
    private static final VException bDE0;
    private static final VException bDE1;

    private static final VException nEN0;
    private static final VException nEN1;
    private static final VException nFR0;
    private static final VException nFR1;
    private static final VException nDE0;
    private static final VException nDE1;

    static {
        V.init();

        idActionA = VException.register(
            "A", VException.ActionCode.NO_RETRY, "{1} {2} error A {_}");
        idActionB = VException.register(
            "B", VException.ActionCode.RETRY_BACKOFF, "{1} {2} problem B {_}");
        idActionC = VException.register(
            "C", VException.ActionCode.NO_RETRY, "");

        {
            VContext ctx = VContext.create();
            ctx = Language.contextWithLanguage(ctx, EN);
            ctx = VException.contextWithComponentName(ctx, "VExceptionTest");
            VException.setDefaultContext(ctx);
        }

        final Catalog cat = Language.getDefaultCatalog();
        // Add messages for French.  Do not set messages for German, to test the case where the
        // messages are not present.
        cat.set(FR, idActionA.getID(), "{1} {2} erreur A {_}");
        cat.set(FR, idActionB.getID(), "{1} {2} problème B {_}");

        // Set English and French messages for UNKNOWN and NO_EXIST to ones the test can predict.
        // Delete any German messages that may be present.
        cat.set(EN, UnknownException.ID_ACTION.getID(), "{1} {2} unknown error {_}");
        cat.set(FR, UnknownException.ID_ACTION.getID(), "{1} {2} erreur inconnu {_}");
        cat.set(DE, UnknownException.ID_ACTION.getID(), "");

        cat.set(EN, NoExistException.ID_ACTION.getID(), "{1} {2} not found {_}");
        cat.set(FR, NoExistException.ID_ACTION.getID(), "{1} {2} pas trouvé {_}");
        cat.set(DE, NoExistException.ID_ACTION.getID(), "");

        VContext ctx = VContext.create();
        ctx = Language.contextWithLanguage(ctx, FR);
        ctx = VException.contextWithComponentName(ctx, "FooServer");

        // A first IDAction in various languages.
        aEN0 = new VException(idActionA, EN, "server", "aEN0", 0);
        aEN1 = new VException(idActionA, EN, "server", "aEN1", 1, 2);
        aFR0 = new VException(idActionA, FR, "server", "aFR0", 0);
        aFR1 = new VException(idActionA, ctx, 1, 2);
        aDE0 = new VException(idActionA, DE, "server", "aDE0", 0);
        aDE1 = new VException(idActionA, DE, "server", "aDE1", 1, 2);

        // A second IDAction in various languages.
        bEN0 = new VException(idActionB, EN, "server", "bEN0", 0);
        bEN1 = new VException(idActionB, EN, "server", "bEN1", 1, 2);
        bFR0 = new VException(idActionB, FR, "server", "bFR0", 0);
        bFR1 = new VException(idActionB, FR, "server", "bFR1", 1, 2);
        bDE0 = new VException(idActionB, DE, "server", "bDE0", 0);
        bDE1 = new VException(idActionB, DE, "server", "bDE1", 1, 2);

        // The NoExist error in various languages.
        nEN0 = new VException(NoExistException.ID_ACTION, EN, "server", "nEN0", 0);
        nEN1 = new VException(NoExistException.ID_ACTION, EN, "server", "nEN1", 1, 2);
        nFR0 = new VException(NoExistException.ID_ACTION, FR, "server", "nFR0", 0);
        nFR1 = new VException(NoExistException.ID_ACTION, FR, "server", "nFR1", 1, 2);
        nDE0 = new VException(NoExistException.ID_ACTION, DE, "server", "nDE0", 0);
        nDE1 = new VException(NoExistException.ID_ACTION, DE, "server", "nDE1", 1, 2);
    }

    public static void testBasic() {
        expectBasic(aEN0, idActionA, "server aEN0 error A 0", 1);
        expectBasic(aEN1, idActionA, "server aEN1 error A 1 2", 2);
        expectBasic(aFR0, idActionA, "server aFR0 erreur A 0", 3);
        expectBasic(aFR1, idActionA, "FooServer  erreur A 1 2", 4);
        expectBasic(aDE0, idActionA, "A: server aDE0 0", 5);
        expectBasic(aDE1, idActionA, "A: server aDE1 1 2", 6);

        expectBasic(bEN0, idActionB, "server bEN0 problem B 0", 7);
        expectBasic(bEN1, idActionB, "server bEN1 problem B 1 2", 8);
        expectBasic(bFR0, idActionB, "server bFR0 problème B 0", 9);
        expectBasic(bFR1, idActionB, "server bFR1 problème B 1 2", 10);
        expectBasic(bDE0, idActionB, "B: server bDE0 0", 11);
        expectBasic(bDE1, idActionB, "B: server bDE1 1 2", 12);

        expectBasic(nEN0, NoExistException.ID_ACTION, "server nEN0 not found 0", 13);
        expectBasic(nEN1, NoExistException.ID_ACTION, "server nEN1 not found 1 2", 14);
        expectBasic(nFR0, NoExistException.ID_ACTION, "server nFR0 pas trouvé 0", 15);
        expectBasic(nFR1, NoExistException.ID_ACTION, "server nFR1 pas trouvé 1 2", 16);
        expectBasic(nDE0, NoExistException.ID_ACTION, "v.io/v23/verror.NoExist: server nDE0 0", 17);
        expectBasic(nDE1, NoExistException.ID_ACTION, "v.io/v23/verror.NoExist: server nDE1 1 2", 18);
    }

    public static void testEqual() {
        assertAllEqual(aEN0, aEN1, aDE0, aDE1, aDE0, aDE1);
        assertAllEqual(bEN0, bEN1, bDE0, bDE1, bDE0, bDE1);
        assertAllEqual(nEN0, nEN1, nDE0, nDE1, nDE0, nDE1);
    }

    private static void expectBasic(
        VException error, VException.IDAction idAction, String msg, int tag) {
        assertEquals(String.format("%d: (%s).getID(); got %s, want %s",
                tag, error, error.getID(), idAction.getID()),
                idAction.getID(), error.getID());
        assertEquals(String.format("%d: (%s).getAction(); got %s, want %s",
                tag, error, error.getAction(), idAction.getAction()),
                idAction.getAction(), error.getAction());
        assertEquals(String.format(
                "%d: (%s).getMessage(); got %s, want %s", tag, error, error.getMessage(), msg),
                msg, error.getMessage());
        assertTrue(String.format(
                "%d: (%s).is(%s) == false, want true",tag, error, idAction.getID()),
                error.is(idAction.getID()));   
        assertTrue(String.format("%d: (%s).is(%s) == false, want true", tag, error, idAction),
                error.is(idAction));
        assertTrue(String.format(
                "%d: (%s).is(%s) == true, want false", tag, error, idActionC.getID()),
                !error.is(idActionC.getID()));
        assertTrue(String.format("%d: (%s).is(%s) == true, want false", tag, error, idActionC),
                !error.is(idActionC));
    }

    private static void assertAllEqual(VException... errors) {
        for (VException error1 : errors) {
            for (VException error2 : errors) {
                assertEquals(error1, error2);
            }
        }
    }
}
