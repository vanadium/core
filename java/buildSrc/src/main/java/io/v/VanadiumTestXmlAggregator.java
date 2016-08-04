// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v;

import com.google.common.base.Charsets;
import com.google.common.io.Files;
import io.v.generated.Testsuite;
import io.v.generated.Testsuites;

import javax.xml.bind.JAXBContext;
import javax.xml.bind.JAXBException;
import javax.xml.bind.Marshaller;
import javax.xml.bind.Unmarshaller;
import javax.xml.transform.stream.StreamSource;
import java.io.File;
import java.io.FilenameFilter;
import java.io.IOException;

/**
 * Provides utilities to convert JUnit XML output format to the format expected by the Vanadium presubmit tool.
 */
public class VanadiumTestXmlAggregator {
    /**
     * Takes all the JUnit XML files in the given {@code xmlDir} and converts them to a single XML file representing
     * the aggregated results of all the tests. The resulting XML file is suitable for use as an input to the {@code
     * v23} presubmit system.
     *
     * @param xmlDir the directory containing the input XML files
     * @param outputFile the output file to produce
     * @throws JAXBException if any of the input JUnit XML files were malformed
     * @throws IOException if there was an error performing file operations
     */
    public static void mergeAllTestXmlFiles(File xmlDir, File outputFile) throws JAXBException, IOException {
        JAXBContext context = JAXBContext.newInstance(Testsuite.class, Testsuites.class);
        Unmarshaller unmarshaller = context.createUnmarshaller();
        Testsuites mergedSuites = new Testsuites();
        int tests = 0;
        int failures = 0;
        int errors = 0;

        File[] files = xmlDir.listFiles(new FilenameFilter() {
            @Override
            public boolean accept(File file, String s) {
                return s.endsWith(".xml");
            }
        });

        for (File file : files) {
            Testsuite suite = (Testsuite) unmarshaller.unmarshal(file);
            tests += suite.getTests();
            failures += suite.getFailures();
            errors += suite.getErrors();
            mergedSuites.getTestsuite().add(suite);
        }

        mergedSuites.setTests(tests);
        mergedSuites.setFailures(failures);
        mergedSuites.setErrors(errors);
        Marshaller marshaller = context.createMarshaller();
        marshaller.setProperty(Marshaller.JAXB_FORMATTED_OUTPUT, true);
        marshaller.marshal(mergedSuites, outputFile);
    }
}
