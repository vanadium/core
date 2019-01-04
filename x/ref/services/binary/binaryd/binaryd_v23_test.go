// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"v.io/v23/naming"
	"v.io/x/ref/test/testutil"
	"v.io/x/ref/test/v23test"
)

func checkFileType(t *testing.T, infoFile, typeString string) {
	var catOut bytes.Buffer
	catCmd := exec.Command("cat", infoFile)
	catCmd.Stdout = &catOut
	catCmd.Stderr = &catOut
	if err := catCmd.Run(); err != nil {
		t.Fatalf("%q failed: %v\n%v", strings.Join(catCmd.Args, " "), err, catOut.String())
	}
	if got, want := strings.TrimSpace(catOut.String()), typeString; got != want {
		t.Fatalf("unexpect file type: got %v, want %v", got, want)
	}
}

func readFileOrDie(t *testing.T, path string) []byte {
	result, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) failed: %v", path, err)
	}
	return result
}

func compareFiles(t *testing.T, f1, f2 string) {
	if !bytes.Equal(readFileOrDie(t, f1), readFileOrDie(t, f2)) {
		t.Fatalf("the contents of %s and %s differ when they should not", f1, f2)
	}
}

func deleteFile(t *testing.T, sh *v23test.Shell, creds *v23test.Credentials, bin, name, suffix string) {
	args := []string{"delete", naming.Join(name, suffix)}
	sh.Cmd(bin, args...).WithCredentials(creds).Run()
}

func downloadAndInstall(t *testing.T, sh *v23test.Shell, creds *v23test.Credentials, bin, name, path, suffix string) {
	args := []string{"download", naming.Join(name, suffix), path}
	sh.Cmd(bin, args...).WithCredentials(creds).Run()
}

func downloadFile(t *testing.T, sh *v23test.Shell, creds *v23test.Credentials, bin, name, path, suffix string) (string, string) {
	args := []string{"download", "--install=false", naming.Join(name, suffix), path}
	stdout := sh.Cmd(bin, args...).WithCredentials(creds).Stdout()
	match := regexp.MustCompile(`Binary downloaded to (.+) \(media info (.+)\)`).FindStringSubmatch(stdout)
	if len(match) != 3 {
		t.Fatalf("Failed to match download stdout: %s", stdout)
	}
	return match[1], match[2]
}

func downloadFileExpectError(t *testing.T, sh *v23test.Shell, creds *v23test.Credentials, bin, name, path, suffix string) {
	args := []string{"download", naming.Join(name, suffix), path}
	cmd := sh.Cmd(bin, args...).WithCredentials(creds)
	cmd.ExitErrorIsOk = true
	if cmd.Run(); cmd.Err == nil {
		t.Fatalf("%s %v: did not fail when it should", bin, args)
	}
}

func downloadURL(t *testing.T, path, rootURL, suffix string) {
	url := fmt.Sprintf("http://%v/%v", rootURL, suffix)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Get(%q) failed: %v", url, err)
	}
	output, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("ReadAll() failed: %v", err)
	}
	if err = ioutil.WriteFile(path, output, 0600); err != nil {
		t.Fatalf("WriteFile() failed: %v", err)
	}
}

func rootURL(t *testing.T, sh *v23test.Shell, creds *v23test.Credentials, bin, name string) string {
	args := []string{"url", name}
	stdout := sh.Cmd(bin, args...).WithCredentials(creds).Stdout()
	return strings.TrimSpace(stdout)
}

func uploadFile(t *testing.T, sh *v23test.Shell, creds *v23test.Credentials, bin, name, path, suffix string) {
	args := []string{"upload", naming.Join(name, suffix), path}
	sh.Cmd(bin, args...).WithCredentials(creds).Run()
}

func TestV23BinaryRepositoryIntegration(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	testutil.InitRandGenerator(t.Logf)

	// Build the required binaries.
	// The client must run as a "delegate" of the server in order to pass
	// the default authorization checks on the server.
	var (
		binaryRepoBin   = v23test.BuildGoPkg(sh, "v.io/x/ref/services/binary/binaryd")
		clientBin       = v23test.BuildGoPkg(sh, "v.io/x/ref/services/binary/binary")
		binaryRepoCreds = sh.ForkCredentials("binaryd")
		clientCreds     = sh.ForkCredentials("binaryd:client")
	)

	// Start the build server.
	binaryRepoName := "test-binary-repository"
	sh.Cmd(binaryRepoBin,
		"-name="+binaryRepoName,
		"-http=127.0.0.1:0",
		"-v23.tcp.address=127.0.0.1:0").WithCredentials(binaryRepoCreds).Start()

	// Upload a random binary file.
	binFile := sh.MakeTempFile()
	if _, err := binFile.Write(testutil.RandomBytes(16 * 1000 * 1000)); err != nil {
		t.Fatalf("Write() failed: %v", err)
	}
	binSuffix := "test-binary"
	uploadFile(t, sh, clientCreds, clientBin, binaryRepoName, binFile.Name(), binSuffix)

	// Upload a compressed version of the binary file.
	tarFile := binFile.Name() + ".tar.gz"
	var tarOut bytes.Buffer
	tarCmd := exec.Command("tar", "zcvf", tarFile, binFile.Name())
	tarCmd.Stdout = &tarOut
	tarCmd.Stderr = &tarOut
	if err := tarCmd.Run(); err != nil {
		t.Fatalf("%q failed: %v\n%v", strings.Join(tarCmd.Args, " "), err, tarOut.String())
	}
	defer os.Remove(tarFile)
	tarSuffix := "test-compressed-file"
	uploadFile(t, sh, clientCreds, clientBin, binaryRepoName, tarFile, tarSuffix)

	// Download the binary file and check that it matches the
	// original one and that it has the right file type.
	downloadName := binFile.Name() + "-downloaded"
	downloadedFile, infoFile := downloadFile(t, sh, clientCreds, clientBin, binaryRepoName, downloadName, binSuffix)
	defer os.Remove(downloadedFile)
	defer os.Remove(infoFile)
	if downloadedFile != downloadName {
		t.Fatalf("expected %s, got %s", downloadName, downloadedFile)
	}
	compareFiles(t, binFile.Name(), downloadedFile)
	checkFileType(t, infoFile, `{"Type":"application/octet-stream","Encoding":""}`)

	// Download and install and make sure the file is as expected.
	installName := binFile.Name() + "-installed"
	downloadAndInstall(t, sh, clientCreds, clientBin, binaryRepoName, installName, binSuffix)
	defer os.Remove(installName)
	compareFiles(t, binFile.Name(), installName)

	// Download the compressed version of the binary file and
	// check that it matches the original one and that it has the
	// right file type.
	downloadTarName := binFile.Name() + "-compressed-downloaded"
	downloadedTarFile, infoFile := downloadFile(t, sh, clientCreds, clientBin, binaryRepoName, downloadTarName, tarSuffix)
	defer os.Remove(downloadedTarFile)
	defer os.Remove(infoFile)
	if downloadedTarFile != downloadTarName {
		t.Fatalf("expected %s, got %s", downloadTarName, downloadedTarFile)
	}
	compareFiles(t, tarFile, downloadedTarFile)
	checkFileType(t, infoFile, `{"Type":"application/x-tar","Encoding":"gzip"}`)

	// Download and install and make sure the un-archived file is as expected.
	installTarName := binFile.Name() + "-compressed-installed"
	downloadAndInstall(t, sh, clientCreds, clientBin, binaryRepoName, installTarName, tarSuffix)
	defer os.Remove(installTarName)
	compareFiles(t, binFile.Name(), filepath.Join(installTarName, binFile.Name()))

	// Fetch the root URL of the HTTP server used by the binary
	// repository to serve URLs.
	root := rootURL(t, sh, clientCreds, clientBin, binaryRepoName)

	// Download the binary file using the HTTP protocol and check
	// that it matches the original one.
	downloadedBinFileURL := binFile.Name() + "-downloaded-url"
	defer os.Remove(downloadedBinFileURL)
	downloadURL(t, downloadedBinFileURL, root, binSuffix)
	compareFiles(t, downloadName, downloadedBinFileURL)

	// Download the compressed version of the binary file using
	// the HTTP protocol and check that it matches the original
	// one.
	downloadedTarFileURL := binFile.Name() + "-downloaded-url.tar.gz"
	defer os.Remove(downloadedTarFileURL)
	downloadURL(t, downloadedTarFileURL, root, tarSuffix)
	compareFiles(t, downloadedTarFile, downloadedTarFileURL)

	// Delete the files.
	deleteFile(t, sh, clientCreds, clientBin, binaryRepoName, binSuffix)
	deleteFile(t, sh, clientCreds, clientBin, binaryRepoName, tarSuffix)

	// Check the files no longer exist.
	downloadFileExpectError(t, sh, clientCreds, clientBin, binaryRepoName, downloadName, binSuffix)
	downloadFileExpectError(t, sh, clientCreds, clientBin, binaryRepoName, downloadTarName, tarSuffix)
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}
