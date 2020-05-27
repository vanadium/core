// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"runtime"
	"time"

	"v.io/v23/context"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	"v.io/x/ref/runtime/internal/rpc/stress"
	"v.io/x/ref/runtime/internal/rpc/stress/internal"
)

var (
	duration time.Duration

	workers        int
	maxChunkCnt    int
	maxPayloadSize int

	outFormat string
)

func init() {
	cmdStressTest.Flags.DurationVar(&duration, "duration", 1*time.Minute, "duration of the test to run")
	cmdStressTest.Flags.IntVar(&workers, "workers", 1, "number of test workers to run")
	cmdStressTest.Flags.IntVar(&maxChunkCnt, "max-chunk-count", 1000, "maximum number of chunks to send per streaming RPC")
	cmdStressTest.Flags.IntVar(&maxPayloadSize, "max-payload-size", 10000, "maximum size of payload in bytes")
	cmdStressTest.Flags.StringVar(&outFormat, "format", "text", "Stats output format; either text or json")

	cmdStressStats.Flags.StringVar(&outFormat, "format", "text", "Stats output format; either text or json")
}

var cmdStressTest = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runStressTest),
	Name:     "stress",
	Short:    "Run stress test",
	Long:     "Run stress test",
	ArgsName: "<server> ...",
	ArgsLong: "<server> ... A list of servers to connect to.",
}

func runStressTest(ctx *context.T, env *cmdline.Env, args []string) error {
	if len(args) == 0 {
		return env.UsageErrorf("no server specified")
	}
	if outFormat != "text" && outFormat != "json" {
		return env.UsageErrorf("invalid output format: %s\n", outFormat)
	}

	rand.Seed(time.Now().UnixNano())
	fmt.Fprintf(env.Stdout, "starting stress test against %d server(s) using %d core(s)...\n", len(args), runtime.NumCPU())
	fmt.Fprintf(env.Stdout, "workers: %d, maxChunkCnt: %d, maxPayloadSize: %d, duration: %v\n", workers, maxChunkCnt, maxPayloadSize, duration)

	start := time.Now()
	done := make(chan stress.SumStats)
	for i := 0; i < workers; i++ {
		go func() {
			var stats stress.SumStats
			timeout := time.After(duration)
		done:
			for {
				server := args[rand.Intn(len(args))]
				if rand.Intn(2) == 0 {
					internal.CallSum(ctx, server, maxPayloadSize, &stats)
				} else {
					internal.CallSumStream(ctx, server, maxChunkCnt, maxPayloadSize, &stats)
				}

				select {
				case <-timeout:
					break done
				default:
				}
			}
			done <- stats
		}()
	}
	var merged stress.SumStats
	for i := 0; i < workers; i++ {
		stats := <-done
		merged.SumCount += stats.SumCount
		merged.SumStreamCount += stats.SumStreamCount
		merged.BytesRecv += stats.BytesRecv
		merged.BytesSent += stats.BytesSent
	}
	elapsed := time.Since(start)
	fmt.Printf("done after %v\n", elapsed)
	return outSumStats(env.Stdout, outFormat, "client stats:", &merged)
}

var cmdStressStats = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runStressStats),
	Name:     "stats",
	Short:    "Print out stress stats of servers",
	Long:     "Print out stress stats of servers",
	ArgsName: "<server> ...",
	ArgsLong: "<server> ... A list of servers to connect to.",
}

func runStressStats(ctx *context.T, env *cmdline.Env, args []string) error {
	if len(args) == 0 {
		return env.UsageErrorf("no server specified")
	}
	if outFormat != "text" && outFormat != "json" {
		return env.UsageErrorf("invalid output format: %s\n", outFormat)
	}
	for _, server := range args {
		stats, err := stress.StressClient(server).GetSumStats(ctx)
		if err != nil {
			return err
		}
		title := fmt.Sprintf("server stats(%s):", server)
		if err := outSumStats(env.Stdout, outFormat, title, &stats); err != nil {
			return err
		}
	}
	return nil
}

func outSumStats(w io.Writer, format, title string, stats *stress.SumStats) error {
	switch format {
	case "text":
		fmt.Fprintf(w, "%s\n", title)
		fmt.Fprintf(w, "\tnumber of non-streaming RPCs:\t%d\n", stats.SumCount)
		fmt.Fprintf(w, "\tnumber of streaming RPCs:\t%d\n", stats.SumStreamCount)
		fmt.Fprintf(w, "\tnumber of bytes received:\t%d\n", stats.BytesRecv)
		fmt.Fprintf(w, "\tnumber of bytes sent:\t\t%d\n", stats.BytesSent)
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
