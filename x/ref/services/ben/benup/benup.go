// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"golang.org/x/tools/benchmark/parse"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"v.io/v23/context"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/services/ben"
	"v.io/x/ref/services/ben/archive"
)

var (
	flagLabel    string
	flagDryRun   bool
	flagArchiver string
	cmdRoot      = &cmdline.Command{
		Name:     "benup",
		Runner:   v23cmd.RunnerFunc(runUpload),
		Short:    "Upload microbenchmark results to an archival service",
		ArgsName: "[<input file>]",
		ArgsLong: `
[input file] Is the name of the file from which to extract benchmark results
(or empty for standard input).`,
		Long: `
Command benup extracts microbenchmark results from the provided input (standard
input for file) and uploads them to an archival service.

For example:
  go test -run X -bench crypto/sha256 | benup
will upload the benchmark results, along with a description of the scenario
(operating system, CPU, compiler etc.) under which the benchmarks were run
to the archival service.

To run tests on an Android device, use bendroid (https://godoc.org/v.io/x/devtools/bendroid):
  bendroid -bench crypto/sha256 | benup

In the fullness of time, benup may support benchmark formats other than those
from "go test" (such as Java benchmarks written using Caliper).

CAVEAT: At this time, benup assumes the benchmarks were either run on the same
machine (operating system, CPU etc.) as the one where benup is being run, OR
run by bendroid and the state of the source code is the same as that on which
benup is being run.
`,
	}
)

func runUpload(ctx *context.T, env *cmdline.Env, args []string) error {
	code, err := detectSourceCode(env)
	if err != nil {
		return err
	}
	// Set input
	var input io.Reader
	switch nargs := len(args); nargs {
	case 0:
		if flagDryRun {
			input = env.Stdin
			break
		}
		// Make the command "transparent"
		// e.g., "go test | benup" will dump test results to standard
		// output just like "go test" would.
		input = io.TeeReader(env.Stdin, env.Stdout)
	case 1:
		f, err := os.Open(args[0])
		if err != nil {
			return err
		}
		defer f.Close()
		input = f
	default:
		return env.UsageErrorf("expected 0 or 1 arguments, got %d", nargs)
	}

	// Goroutines:
	// (1) Parse input to extract ben.Scenario and ben.Run objects
	// (2) Send ben.Run objects to destination
	// (3) (This one) Wait for both to finish
	var (
		runs = make(chan ben.Run)
		errs = make(chan error)
		scn  = make(chan ben.Scenario)
	)
	go parseRuns(scn, runs, input, errs)
	if flagDryRun {
		go printRuns(env, scn, code, runs, errs)
	} else {
		go sendRuns(env, ctx, scn, code, runs, errs)
	}
	// Wait for both to terminate (two sends on errch)
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			return err
		}
	}
	return nil
}

func parseRuns(scn chan<- ben.Scenario, out chan<- ben.Run, in io.Reader, errch chan<- error) {
	// Parse the input file. The ben.Scenario under which the benchmarks
	// were run should be specified in the input file (bendroid takes care
	// of that). If none is found, then assume that the benchmarks were run
	// on the same machine as this function.
	var (
		scanner = bufio.NewScanner(in)
		pkgRuns []ben.Run // Runs for all benchmarks in a particular package
		write   = func(pkg string) {
			// Prepend pkg to the name of all benchmarks in pkgRuns,
			// send them on put and clear the slice.
			if len(pkgRuns) == 0 {
				return
			}
			for _, r := range pkgRuns {
				r.Name = pkg + "." + r.Name
				out <- r
			}
			pkgRuns = pkgRuns[:0]
		}
	)
	defer func() {
		if err := scanner.Err(); err != nil {
			errch <- fmt.Errorf("error parsing input: %v", err)
		} else {
			errch <- nil
		}
		close(out)
		close(scn)
	}()
	// Fill in a ben.Scenario: It will be in the input before any benchmark
	// runs.
	var scan bool
	for scan = scanner.Scan(); scan; scan = scanner.Scan() {
		line := scanner.Text()
		if _, err := parse.ParseLine(line); err == nil {
			scn <- detectScenario()
			break
		}
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "BENDROID") {
			continue
		}
		var s ben.Scenario
		s.Os.Name = "android"
		s.Label = flagLabel
		bendroid := true
		for bendroid {
			// https://github.com/vanadium/go.devtools/blob/4fd6edf469d84a5be8d00a11283fe7febeafe30f/bendroid/templates.go#L149
			if ok, v := trimPrefix(line, "BENDROIDOS_VERSION="); ok {
				s.Os.Version = v
			} else if ok, v := trimPrefix(line, "BENDROIDCPU_ARCHITECTURE="); ok {
				s.Cpu.Architecture = v
			} else if ok, v := trimPrefix(line, "BENDROIDCPU_DESCRIPTION="); ok {
				s.Cpu.Description = v
			}
			if bendroid = scanner.Scan(); bendroid {
				line = scanner.Text()
				_, err := parse.ParseLine(line)
				line = strings.TrimSpace(line)
				bendroid = (err != nil) && strings.HasPrefix(line, "BENDROID")
			}
		}
		scn <- s
		break
	}
	for ; scan; scan = scanner.Scan() {
		line := scanner.Text()
		if b, err := parse.ParseLine(line); err == nil {
			// Translate b into a ben.Run
			parallelism := 0
			name := b.Name
			if idx := strings.LastIndex(name, "-"); idx > 0 {
				if p, err := strconv.Atoi(name[idx+1:]); err == nil {
					parallelism = p
					name = name[0:idx]
				}
			}
			pkgRuns = append(pkgRuns, ben.Run{
				Name:              name,
				Iterations:        uint64(b.N),
				NanoSecsPerOp:     b.NsPerOp,
				AllocsPerOp:       b.AllocsPerOp,
				AllocedBytesPerOp: b.AllocedBytesPerOp,
				MegaBytesPerSec:   b.MBPerS,
				Parallelism:       uint32(parallelism),
			})
			continue
		}
		if strings.HasPrefix(line, "ok") || strings.HasPrefix(line, "FAIL") {
			// Lines which tell us the package name, like:
			// ok  	v.io/v23/security	24.588s
			// OR
			// FAIL v.io/v23/security        0.046s
			if fields := strings.Fields(line); len(fields) == 3 {
				write(fields[1])
			}
		}
	}
}

func parseBendroidLine(output *string, prefix, line string) bool {
	if strings.HasPrefix(line, prefix) {
		*output = strings.TrimPrefix(line, prefix)
		return true
	}
	return false
}

func printRuns(env *cmdline.Env, scn <-chan ben.Scenario, code ben.SourceCode, in <-chan ben.Run, errch chan<- error) {
	fmt.Fprintln(env.Stdout, "SOURCE CODE:")
	fmt.Fprintln(env.Stdout, code)
	fmt.Fprintf(env.Stdout, "SCENARIO:")
	prettyPrint(env, <-scn)
	fmt.Fprintln(env.Stdout)
	fmt.Fprintln(env.Stdout, "RUNS:")
	n := 0
	for r := range in {
		n++
		fmt.Fprintf(env.Stdout, "%d: %+v\n", n, r)
	}
	errch <- nil
}

func sendRuns(env *cmdline.Env, ctx *context.T, scn <-chan ben.Scenario, code ben.SourceCode, in <-chan ben.Run, errch chan<- error) {
	var (
		scenario = <-scn
		runs     []ben.Run
		uploaded int
		urls     []string
		timer    = time.After(time.Minute)
		archive  = func() error {
			if len(runs) == 0 {
				return nil
			}
			url, err := archive.BenchmarkArchiverClient(flagArchiver).Archive(ctx, scenario, code, runs)
			if err != nil {
				errch <- err
				return err
			}
			urls = append(urls, url)
			uploaded += len(runs)
			runs = runs[:0]
			return nil
		}
	)
	// Collect runs, every minute flush results out to the archiver.
	done := false
	for !done {
		select {
		case r, more := <-in:
			if !more {
				done = true
				break
			}
			runs = append(runs, r)
		case <-timer:
			if archive() != nil {
				return
			}
			timer = time.After(time.Minute)
		}
	}
	if archive() != nil {
		return
	}
	if uploaded == 0 {
		fmt.Fprintln(env.Stdout, "No benchmarks found to upload")
		errch <- nil
		return
	}
	fmt.Fprintf(env.Stdout, "Uploaded %d benchmark(s) to %v\n", uploaded, flagArchiver)
	for _, url := range urls {
		fmt.Fprintln(env.Stdout, "Results at:", url)
	}
	errch <- nil
}

func detectScenario() ben.Scenario {
	return ben.Scenario{
		Cpu: ben.Cpu{
			Architecture:  runtime.GOARCH,
			Description:   detectCPUDescription(),
			ClockSpeedMhz: detectCPUClockSpeedMHz(),
		},
		Os: ben.Os{
			Name:    runtime.GOOS,
			Version: detectOSVersion(),
		},
		Label: flagLabel,
	}
}

func trimPrefix(s, prefix string) (bool, string) {
	return strings.HasPrefix(s, prefix), strings.TrimPrefix(s, prefix)
}

func prettyPrint(env *cmdline.Env, v interface{}) error {
	// Use JSON only because json.MarshalIndent pretty-prints in multiple
	// lines.
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintf(env.Stdout, "%s\n", out)
	return nil
}

func main() {
	cmdline.HideGlobalFlagsExcept()
	cmdRoot.Flags.StringVar(&flagLabel, "label", "", "label to assign the uploaded benchmark results")
	cmdRoot.Flags.BoolVar(&flagDryRun, "n", false, "If true, results will not be uploaded but the results that would have been uploaded will be dumped out")
	cmdRoot.Flags.StringVar(&flagArchiver, "archiver", "/TODO(ashankar):Fill_this_in_when_something_is_setup", "Vanadium object name of the BenchmarkArchiver service")
	cmdline.Main(cmdRoot)
}
