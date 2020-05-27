// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"v.io/x/ref/runtime/factories/library"
	"v.io/x/ref/test/v23test"
)

func init() {
	// Allow v23.Init to be called multiple times.
	library.AllowMultipleInitializations = true
}

func TestV23DebugGlob(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	binary := v23test.BuildGoPkg(sh, "v.io/x/ref/services/debug/debug")
	stdout := sh.Cmd(binary, "glob", "__debug/*").Stdout()

	var want string
	for _, entry := range []string{"logs", "pprof", "stats", "vtrace", "http"} {
		want += "__debug/" + entry + "\n"
	}
	if got := stdout; got != want {
		t.Fatalf("unexpected output, want %s, got %s", want, got)
	}
}

func TestV23DebugGlobLogs(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	// Create a temp dir before we list the logs.
	dirName := sh.MakeTempDir()
	binary := v23test.BuildGoPkg(sh, "v.io/x/ref/services/debug/debug")
	output := sh.Cmd(binary, "--timeout=1m", "glob", "__debug/logs/*").CombinedOutput()

	// The output should contain the dir name.
	want := "/logs/" + filepath.Base(dirName)
	if !strings.Contains(output, want) {
		fi, err := os.Stat(dirName)
		t.Logf("Stat(%q): (%v, %v)", dirName, fi, err)
		t.Logf("TempDir: %q", os.TempDir())
		t.Fatalf("output should contain %s but did not\n%s", want, output)
	}
}

func TestV23ReadHostname(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	path := "__debug/stats/system/hostname"
	binary := v23test.BuildGoPkg(sh, "v.io/x/ref/services/debug/debug")
	stdout := sh.Cmd(binary, "stats", "read", path).Stdout()
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatalf("Hostname() failed: %v", err)
	}
	if got, want := stdout, path+": "+hostname+"\n"; got != want {
		t.Fatalf("unexpected output, want %q, got %q", want, got)
	}
}

func createTestLogFile(t *testing.T, sh *v23test.Shell, content string) *os.File {
	file := sh.MakeTempFile()
	_, err := file.Write([]byte(content))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	return file
}

func TestV23LogSize(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	binary := v23test.BuildGoPkg(sh, "v.io/x/ref/services/debug/debug")
	testLogData := "This is a test log file"
	file := createTestLogFile(t, sh, testLogData)

	// Check to ensure the file size is accurate
	str := strings.TrimSpace(sh.Cmd(binary, "logs", "size", "__debug/logs/"+filepath.Base(file.Name())).Stdout())
	got, err := strconv.Atoi(str)
	if err != nil {
		t.Fatalf("Atoi(\"%q\") failed", str)
	}
	want := len(testLogData)
	if got != want {
		t.Fatalf("unexpected output, want %d, got %d", got, want)
	}
}

func TestV23StatsRead(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	binary := v23test.BuildGoPkg(sh, "v.io/x/ref/services/debug/debug")
	testLogData := "This is a test log file\n"
	file := createTestLogFile(t, sh, testLogData)
	logName := filepath.Base(file.Name())
	runCount := 12
	for c := 0; c < runCount; c++ {
		sh.Cmd(binary, "logs", "read", "__debug/logs/"+logName).Run()
	}

	readlogStatsEndpoint := "__debug/stats/rpc/server/routing-id/*/methods/ReadLog/latency-ms"
	got := sh.Cmd(binary, "stats", "read", readlogStatsEndpoint).Stdout()

	want := fmt.Sprintf("Count: %d", runCount)
	if !strings.Contains(got, want) {
		t.Fatalf("expected output %q to contain %q, but did not\n", got, want)
	}

	// Test "-json" format.
	jsonOutput := sh.Cmd(binary, "stats", "read", "-json", readlogStatsEndpoint).Stdout()
	var stats []struct {
		Name  string
		Value struct {
			Count int
		}
	}
	if err := json.Unmarshal([]byte(jsonOutput), &stats); err != nil {
		t.Fatalf("invalid json output:\n%s", jsonOutput)
	}
	if want, got := 1, len(stats); want != got {
		t.Fatalf("unexpected stats length, want %d, got %d", want, got)
	}
	if !strings.HasPrefix(stats[0].Name, "__debug/stats/rpc/server/routing-id") {
		t.Fatalf("unexpected Name field, want %q, got %q", want, got)
	}
	if want, got := runCount, stats[0].Value.Count; want != got {
		t.Fatalf("unexpected Value.Count field, want %d, got %d", want, got)
	}
}

func TestV23StatsWatch(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	binary := v23test.BuildGoPkg(sh, "v.io/x/ref/services/debug/debug")
	testLogData := "This is a test log file\n"
	file := createTestLogFile(t, sh, testLogData)
	logName := filepath.Base(file.Name())
	sh.Cmd(binary, "logs", "read", "__debug/logs/"+logName).Run()

	cmd := sh.Cmd(binary, "stats", "watch", "-raw", "__debug/stats/rpc/server/routing-id/*/methods/ReadLog/latency-ms")
	stdoutPipe := cmd.StdoutPipe()
	cmd.Start()

	lineChan := make(chan string)
	errCh := make(chan error)
	// Go off and read the invocation's stdout.
	go func() {
		line, err := bufio.NewReader(stdoutPipe).ReadString('\n')
		if err != nil {
			errCh <- fmt.Errorf("Could not read line from invocation")
		}
		lineChan <- line
	}()

	// Wait up to 10 seconds for some stats output. Either some output
	// occurs or the timeout expires without any output.
	select {
	case <-time.After(10 * time.Second):
		t.Errorf("Timed out waiting for output")
	case err := <-errCh:
		t.Error(err)
	case got := <-lineChan:
		// Expect one ReadLog call to have occurred.
		want := "}}{Count: 1"
		if !strings.Contains(got, want) {
			t.Errorf("wanted but could not find %q in output\n%s", want, got)
		}
	}

}

func performTracedRead(sh *v23test.Shell, debugBinary, path string) string {
	return sh.Cmd(debugBinary, "--v23.vtrace.sample-rate=1", "logs", "read", path).Stdout()
}

func TestV23VTrace(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	binary := v23test.BuildGoPkg(sh, "v.io/x/ref/services/debug/debug")
	logContent := "Hello, world!\n"
	logPath := "__debug/logs/" + filepath.Base(createTestLogFile(t, sh, logContent).Name())
	// Create a log file with tracing, read it and check that the resulting trace exists.
	got := performTracedRead(sh, binary, logPath)
	if logContent != got {
		t.Fatalf("unexpected output: want %s, got %s", logContent, got)
	}

	// Grab the ID of the first and only trace.
	traceContent, want := sh.Cmd(binary, "vtrace", "__debug/vtrace").Stdout(), 1
	if count := strings.Count(traceContent, "Trace -"); count != want {
		t.Fatalf("unexpected trace count, want %d, got %d: %s", want, count, traceContent)
	}
	fields := strings.Split(traceContent, " ")
	if len(fields) < 3 {
		t.Fatalf("expected at least 3 space-delimited fields, got %d: %v", len(fields), traceContent)
	}
	traceID := fields[2]

	// Do a sanity check on the trace ID: it should be a 32-character hex ID prefixed with 0x
	if match, _ := regexp.MatchString("0x[0-9a-f]{32}", traceID); !match {
		t.Fatalf("wanted a 32-character hex ID prefixed with 0x, got %s", traceID)
	}

	// Do another traced read, this will generate a new trace entry.
	performTracedRead(sh, binary, logPath)

	// Read vtrace, we should have 2 traces now.
	output, want := sh.Cmd(binary, "vtrace", "__debug/vtrace").Stdout(), 2
	if count := strings.Count(output, "Trace -"); count != want {
		t.Fatalf("unexpected trace count, want %d, got %d\n%s", want, count, output)
	}

	// Now ask for a particular trace. The output should contain exactly
	// one trace whose ID is equal to the one we asked for.
	got, want = sh.Cmd(binary, "vtrace", "__debug/vtrace", traceID).Stdout(), 1
	if count := strings.Count(got, "Trace -"); count != want {
		t.Fatalf("unexpected trace count, want %d, got %d\n%s", want, count, got)
	}
	fields = strings.Split(got, " ")
	if len(fields) < 3 {
		t.Fatalf("expected at least 3 space-delimited fields, got %d: %v", len(fields), got)
	}
	got = fields[2]
	if traceID != got {
		t.Fatalf("unexpected traceID, want %s, got %s", traceID, got)
	}
}

func TestV23Pprof(t *testing.T) {
	t.Skip("https://github.com/vanadium/build/issues/49")
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	binary := v23test.BuildGoPkg(sh, "v.io/x/ref/services/debug/debug")
	stdout := sh.Cmd(binary, "pprof", "run", "__debug/pprof", "heap", "--text").Stdout()

	// Assert that a profile indicating the heap size was written out.
	want, got := "(.*) of (.*) total", stdout
	var groups []string
	if groups = regexp.MustCompile(want).FindStringSubmatch(got); len(groups) < 3 {
		t.Fatalf("could not find regexp %q in output\n%s", want, got)
	}
	t.Logf("got a heap profile showing a heap size of %s", groups[2])
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}
