// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"runtime"
	"runtime/pprof"
	"strconv"
	"testing"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/x/ref/examples/rps"
	"v.io/x/ref/test/testutil"
	"v.io/x/ref/test/v23test"
)

func startRockPaperScissors(t *testing.T, ctx *context.T) (*RPS, func()) {
	ctx, cancel := context.WithCancel(ctx)
	rpsService := NewRPS(ctx)
	names := []string{"rps/judge/test", "rps/player/test", "rps/scorekeeper/test"}
	_, server, err := v23.WithNewServer(ctx, names[0], rps.RockPaperScissorsServer(rpsService), nil)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	for _, n := range names[1:] {
		server.AddName(n)
	}
	testutil.WaitForServerPublished(server)

	return rpsService, func() {
		cancel()
		<-server.Closed()
	}
}

// TestV23RockPaperScissorsImpl runs one rock-paper-scissors game and verifies
// that all the counters are consistent.
func TestV23RockPaperScissorsImpl(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()
	ctx := sh.Ctx

	rpsService, rpsStop := startRockPaperScissors(t, ctx)
	defer rpsStop()

	const numGames = 100
	for x := 0; x < numGames; x++ {
		if err := rpsService.Player().InitiateGame(ctx); err != nil {
			t.Errorf("Failed to initiate game: %v", err)
		}
	}
	rpsService.Player().WaitUntilIdle()

	// For each game, the player plays twice. So, we expect the player to
	// show that it played 2×numGames, and won numGames.
	played, won := rpsService.Player().Stats()
	if want, got := int64(2*numGames), played; want != got {
		t.Errorf("Unexpected number of played games. Got %d, want %d", got, want)
	}
	if want, got := int64(numGames), won; want != got {
		t.Errorf("Unexpected number of won games. Got %d, want %d", got, want)
	}

	// The Judge ran every game.
	if want, got := int64(numGames), rpsService.Judge().Stats(); want != got {
		t.Errorf("Unexpected number of games run. Got %d, want %d", got, want)
	}

	// The Score Keeper received one score card per game.
	if want, got := int64(numGames), rpsService.ScoreKeeper().Stats(); want != got {
		t.Errorf("Unexpected number of score cards. Got %d, want %d", got, want)
	}

	// Check for leaked goroutines.
	if n := runtime.NumGoroutine(); n > 100 {
		t.Logf("Num goroutines: %d", n)
		if leak := reportLeakedGoroutines(t, n/3); len(leak) != 0 {
			t.Errorf("Potentially leaked goroutines:\n%s", leak)
		}
	}
}

// reportLeakedGoroutines reads the goroutine pprof profile and returns the
// goroutines that have more than 'threshold' instances.
func reportLeakedGoroutines(t *testing.T, threshold int) string {
	f, err := ioutil.TempFile("", "test-profile-")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer f.Close()
	defer os.Remove(f.Name())
	p := pprof.Lookup("goroutine")
	if p == nil {
		t.Fatalf("Failed to lookup the goroutine profile")
	}
	if err := p.WriteTo(f, 1); err != nil {
		t.Fatalf("Failed to write profile: %v", err)
	}

	re, err := regexp.Compile(`^([0-9]+) @`)
	if err != nil {
		t.Fatalf("Failed to compile regexp")
	}
	f.Seek(0, 0)
	r := bufio.NewReader(f)

	var buf bytes.Buffer
	var printing bool
	for {
		line, err := r.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Failed to read bytes: %v", err)
		}
		if len(line) <= 1 {
			printing = false
			continue
		}
		if !printing {
			if m := re.FindSubmatch(line); len(m) == 2 {
				count, _ := strconv.Atoi(string(m[1]))
				if count > threshold {
					printing = true
				}
			}
		}
		if printing {
			buf.Write(line)
		}
	}
	return buf.String()
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}
