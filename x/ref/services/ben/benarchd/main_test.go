// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"v.io/x/ref/test/expect"
	"v.io/x/ref/test/v23test"
)

func TestV23Benarchd(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	// Startup benarchd
	dbfile := tempFile(t)
	defer os.Remove(dbfile)
	benarchd := sh.Cmd(
		v23test.BuildGoPkg(sh, "v.io/x/ref/services/ben/benarchd"),
		"--store=sqlite3:"+dbfile,
		"--v23.tcp.address=127.0.0.1:0",
		"--logtostderr").
		WithCredentials(sh.ForkCredentials("benarchd"))
	S := expect.NewSession(t, benarchd.StderrPipe(), time.Minute/2)
	benarchd.Start()
	S.SetVerbosity(testing.Verbose())
	var httpaddr, v23addr string
	if e := S.ExpectSetEventuallyRE(".*HTTP server at (.*)$"); len(e) > 0 && len(e[0]) > 1 {
		httpaddr = e[0][1]
	}
	if e := S.ExpectSetEventuallyRE(".*v23 server at (.*)$"); len(e) > 0 && len(e[0]) > 1 {
		v23addr = e[0][1]
	}

	// Fire up a few uploads from benup
	benup := sh.Cmd(v23test.BuildGoPkg(sh, "v.io/x/ref/services/ben/benup"), "--archiver", v23addr).WithCredentials(sh.ForkCredentials("benupuser"))
	benup.SetStdinReader(bytes.NewBufferString(`
BENDROIDCPU_ARCHITECTURE=arm64-v8a,armeabi-v7a,armeabi
BENDROIDCPU_DESCRIPTION=google Nexus 5X bullhead
BENDROIDOS_VERSION=6.0.1 (Build MMB29Q Release 2480792 SDK 23)
PASS
BenchmarkGood-4      	1111111111	         1 ns/op
BenchmarkThroughput-4	2222222222	         2 ns/op	3 MB/s
BenchmarkAllocs-4    	33333333	         4 ns/op	      5 B/op	       6 allocs/op
BenchmarkAllMetrics-4	44444444	       700 ns/op	  80 MB/s	      90 B/op	       10 allocs/op
BenchmarkBad-4       	--- FAIL: BenchmarkBad-4
	testdata_test.go:54: BenchmarkBad intentionally fails
ok  	path/to/some/package	4.728s

`))
	benup.Run()

	benup = benup.Clone()
	benup.SetStdinReader(bytes.NewBufferString(`
BENDROIDCPU_ARCHITECTURE=armeabi-v7a,armeabi
BENDROIDCPU_DESCRIPTION=google Nexus 7 razor
BENDROIDOS_VERSION=5.1.1 (Build LMY48I Release 2074855 SDK 22)
PASS
BenchmarkGood-4      	5555555555	         10 ns/op
BenchmarkThroughput-4	6666666666	         11 ns/op	12.13 MB/s
BenchmarkAllocs-4    	77777777	       140 ns/op	      150 B/op	       16 allocs/op
BenchmarkAllMetrics-4	88888888	       170 ns/op	  180 MB/s	      190 B/op	       20 allocs/op
BenchmarkBad-4       	--- FAIL: BenchmarkBad-4
	testdata_test.go:54: BenchmarkBad intentionally fails
ok  	path/to/some/package	4.728s
`))
	benup.Run()

	// Sanity check: HTTP requests made to the web ui.
	mkurl := func(query string) string {
		return fmt.Sprintf("%v/?q=%v", httpaddr, url.QueryEscape(query))
	}
	// Homepage:
	if err := testHTTP(httpaddr); err != nil {
		t.Error(err)
	}
	// Query that leads to empty results
	if err := testHTTP(mkurl("ThisHasNoResult"), "No results for [ThisHasNoResult]"); err != nil {
		t.Error(err)
	}
	// Query that is satisfied by one benchmark, so runs will be rendered
	if err := testHTTP(mkurl(`cpu:"Nexus 7 RAZOR" Throughput`),
		"path/to/some/package.BenchmarkThroughput",
		"android (5.1.1 (build lmy48i release 2074855 sdk 22))",
		"root:benupuser",
		"11</span>ns</div></td>", // ns/op
		"<td>12.13</td>",
		"<td>6666666666</td>"); err != nil {
		t.Error(err)
	}
	// Query that leads to multiple benchmarks/scenarios
	if err := testHTTP(mkurl(`cpu:nexus os:android Benchmark`),
		"path/to/some/package.BenchmarkGood",
		"path/to/some/package.BenchmarkThroughput",
		"path/to/some/package.BenchmarkAllocs",
		"path/to/some/package.BenchmarkAllMetrics",
		"google nexus 5x bullhead",
		"google nexus 7 razor"); err != nil {
		t.Error(err)
	}
}

func testHTTP(url string, expect ...string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("GET %v failed: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %v returned %v", url, resp.StatusCode)
	}
	if len(expect) == 0 {
		return nil
	}
	buf := bytes.NewBuffer(nil)
	io.Copy(buf, resp.Body) //nolint:errcheck
	body := buf.String()
	for _, e := range expect {
		if !strings.Contains(body, e) {
			return fmt.Errorf("failed to find [%v] in contents of %q: %v", e, url, body)
		}
	}
	return nil
}

func tempFile(t *testing.T) string {
	f, err := ioutil.TempFile(os.TempDir(), "TestV23Benarchd")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	return f.Name()
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}
