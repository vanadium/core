// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package io.v;

import com.google.common.base.Function;
import com.google.common.collect.Lists;
import com.google.common.io.Files;
import com.google.common.io.Resources;
import io.v.generated.Testsuite;
import io.v.generated.Testsuites;
import org.junit.After;
import org.junit.Before;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

import javax.xml.bind.JAXBContext;
import java.io.File;
import java.io.IOException;
import java.util.ArrayList;
import java.util.List;
import java.util.logging.Logger;

import static com.google.common.truth.Truth.assertThat;
import static com.google.common.truth.Truth.assertWithMessage;

@RunWith(JUnit4.class)
public class VanadiumTestXmlAggregatorTest {
    private static final Logger logger = Logger.getLogger(VanadiumTestXmlAggregatorTest.class.getName());

    private File tempDir;
    private List<File> tempFiles = new ArrayList<>();

    @Before
    public void extractTestResources() throws IOException {
        tempDir = Files.createTempDir();
        Files.write(Resources.toByteArray(Resources.getResource("test-a.xml")), addTempFile("test-a.xml"));
        Files.write(Resources.toByteArray(Resources.getResource("test-b.xml")), addTempFile("test-b.xml"));
        Files.write(Resources.toByteArray(Resources.getResource("test-c.xml")), addTempFile("test-c.xml"));
    }

    private File addTempFile(String name) throws IOException {
        File tempFile = new File(tempDir, name);
        tempFiles.add(tempFile);
        return tempFile;
    }

    @After
    public void deleteTempDir() {
        for (File file : tempFiles) {
            if (!file.delete()) {
                logger.warning("Could not clean up " + file);
            }
        }
        assertWithMessage("Couldn't clean up " + tempDir).that(tempDir.delete()).isTrue();
    }

    @Test
    public void testSimpleCase() throws Exception {
        File outputFile = new File(tempDir, "out.xml");
        tempFiles.add(outputFile);
        VanadiumTestXmlAggregator.mergeAllTestXmlFiles(tempDir, outputFile);

        // Read the file back in.
        JAXBContext context = JAXBContext.newInstance(Testsuites.class);
        Testsuites result = (Testsuites) context.createUnmarshaller().unmarshal(outputFile);

        // There should be 3 suites:
        assertThat(result.getTestsuite()).named("test suite list").hasSize(3);

        Function<Testsuite, String> testSuiteNames = new Function<Testsuite, String>() {
            @Override
            public String apply(Testsuite testsuite) {
                return testsuite.getName();
            }
        };

        assertThat(Lists.transform(result.getTestsuite(), testSuiteNames)).containsAllOf("io.v.v23.security" +
                ".CallTest", "io.v.v23.security.CaveatTest", "io.v.v23.vom.ConvertUtilTest");

        assertThat(result.getTests()).named("total test count").isEqualTo(6);
        assertThat(result.getFailures()).named("failure count").isEqualTo(1);
        assertThat(result.getErrors()).named("error count").isEqualTo(0);
    }
}