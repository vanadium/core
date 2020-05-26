// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package internal defines common functions used by both rock paper scissors
// clients and servers.
package internal

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security"

	"v.io/x/ref/examples/rps"
)

// CreateName creates a name using the username and hostname.
func CreateName(ctx *context.T) string {
	p := v23.GetPrincipal(ctx)
	b, _ := p.BlessingStore().Default()
	return naming.EncodeAsNameElement(strings.Join(security.BlessingNames(p, b), ","))
}

// FindJudge returns a random rock-paper-scissors judge from the mount table.
func FindJudge(ctx *context.T, prefix string) (string, error) {
	judges, err := findAll(ctx, prefix, "judge")
	if err != nil {
		return "", err
	}
	if len(judges) > 0 {
		return judges[rand.Intn(len(judges))], nil
	}
	return "", errors.New("no judges")
}

// FindPlayer returns a random rock-paper-scissors player from the mount table.
func FindPlayer(ctx *context.T, prefix string) (string, error) {
	players, err := findAll(ctx, prefix, "player")
	if err != nil {
		return "", err
	}
	if len(players) > 0 {
		return players[rand.Intn(len(players))], nil
	}
	return "", errors.New("no players")
}

// FindScoreKeepers returns all the rock-paper-scissors score keepers from the
// mount table.
func FindScoreKeepers(ctx *context.T, prefix string) ([]string, error) {
	sKeepers, err := findAll(ctx, prefix, "scorekeeper")
	if err != nil {
		return nil, err
	}
	return sKeepers, nil
}

func findAll(ctx *context.T, prefix, t string) ([]string, error) {
	start := time.Now()
	ns := v23.GetNamespace(ctx)
	c, err := ns.Glob(ctx, naming.Join(prefix, "rps", t, "*"))
	if err != nil {
		ctx.Infof("mt.Glob failed: %v", err)
		return nil, err
	}
	var servers []string
	for e := range c {
		switch v := e.(type) {
		case *naming.GlobReplyError:
			ctx.VI(1).Infof("findAll(%q) error for %q: %v", t, v.Value.Name, v.Value.Error)
		case *naming.GlobReplyEntry:

			servers = append(servers, strings.TrimPrefix(v.Value.Name, naming.Clean(prefix)+"/"))
		}
	}
	ctx.VI(1).Infof("findAll(%q) elapsed: %s", t, time.Now().Sub(start))
	return servers, nil
}

// FormatScoreCard returns a string representation of a score card.
func FormatScoreCard(score rps.ScoreCard) string {
	buf := bytes.NewBufferString("")
	var gameType string
	switch score.Opts.GameType {
	case rps.Classic:
		gameType = "Classic"
	case rps.LizardSpock:
		gameType = "LizardSpock"
	default:
		gameType = "Unknown"
	}
	fmt.Fprintf(buf, "Game Type: %s\n", gameType)
	fmt.Fprintf(buf, "Number of rounds: %d\n", score.Opts.NumRounds)
	fmt.Fprintf(buf, "Judge: %s\n", score.Judge)
	fmt.Fprintf(buf, "Player 1: %s\n", score.Players[0])
	fmt.Fprintf(buf, "Player 2: %s\n", score.Players[1])
	for i, r := range score.Rounds {
		roundOffset := r.StartTime.Sub(score.StartTime)
		roundTime := r.EndTime.Sub(r.StartTime)
		fmt.Fprintf(buf, "Round %2d: Player 1 played %-10q. Player 2 played %-10q. Winner: %d %-28s [%-10s/%-10s]\n",
			i+1, r.Moves[0], r.Moves[1], r.Winner, r.Comment, roundOffset, roundTime)
	}
	fmt.Fprintf(buf, "Winner: %d\n", score.Winner)
	fmt.Fprintf(buf, "Time: %s\n", score.EndTime.Sub(score.StartTime))
	return buf.String()
}
