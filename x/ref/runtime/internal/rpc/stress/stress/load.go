// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"time"

	"v.io/v23/context"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	"v.io/x/ref/runtime/internal/rpc/stress/internal"
)

var (
	cpus        int
	payloadSize int
)

func init() {
	cmdLoadTest.Flags.DurationVar(&duration, "duration", 1*time.Minute, "duration of the test to run")
	cmdLoadTest.Flags.IntVar(&cpus, "cpu", 0, "number of cpu cores to use; if zero, use the number of servers to test")
	cmdLoadTest.Flags.IntVar(&payloadSize, "payload-size", 1000, "size of payload in bytes")
	cmdLoadTest.Flags.StringVar(&outFormat, "format", "text", "Stats output format; either text or json")
}

type loadStats struct {
	Iterations uint64
	MsecPerRPC float64
	QPS        float64
	QPSPerCore float64
}

var cmdLoadTest = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runLoadTest),
	Name:     "load",
	Short:    "Run load test",
	Long:     "Run load test",
	ArgsName: "<server> ...",
	ArgsLong: "<server> ... A list of servers to connect to.",
}

func runLoadTest(ctx *context.T, env *cmdline.Env, args []string) error {
	if len(args) == 0 {
		return env.UsageErrorf("no server specified")
	}
	if outFormat != "text" && outFormat != "json" {
		return env.UsageErrorf("invalid output format: %s\n", outFormat)
	}

	cores := cpus
	if cores == 0 {
		cores = len(args)
	}
	runtime.GOMAXPROCS(cores)

	fmt.Fprintf(env.Stdout, "starting load test against %d server(s) using %d core(s)...\n", len(args), cores)
	fmt.Fprintf(env.Stdout, "payloadSize: %d, duration: %v\n", payloadSize, duration)

	start := time.Now()
	done := make(chan loadStats)
	for _, server := range args {
		go func(server string) {
			var stats loadStats

			start := time.Now()
			stats.Iterations = internal.CallEcho(ctx, server, payloadSize, duration)
			elapsed := time.Since(start)

			stats.QPS = float64(stats.Iterations) / elapsed.Seconds()
			stats.MsecPerRPC = 1000 / stats.QPS
			done <- stats
		}(server)
	}
	var merged loadStats
	for i := 0; i < len(args); i++ {
		stats := <-done
		merged.Iterations += stats.Iterations
		merged.MsecPerRPC += stats.MsecPerRPC
		merged.QPS += stats.QPS
	}
	merged.MsecPerRPC /= float64(len(args))
	merged.QPSPerCore = merged.QPS / float64(cores)
	elapsed := time.Since(start)
	fmt.Printf("done after %v\n", elapsed)
	return outLoadStats(env.Stdout, outFormat, "load stats:", &merged)
}

func outLoadStats(w io.Writer, format, title string, stats *loadStats) error {
	switch format {
	case "text":
		fmt.Fprintf(w, "%s\n", title)
		fmt.Fprintf(w, "\tnumber of RPCs:\t\t%d\n", stats.Iterations)
		fmt.Fprintf(w, "\tlatency (msec/rpc):\t%.2f\n", stats.MsecPerRPC)
		fmt.Fprintf(w, "\tQPS:\t\t\t%.2f\n", stats.QPS)
		fmt.Fprintf(w, "\tQPS/core:\t\t%.2f\n", stats.QPSPerCore)
	case "json":
		b, err := json.Marshal(stats)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "%s%s\n", title, b)
	default:
		return fmt.Errorf("invalid output format: %s", format)
	}
	return nil
}
