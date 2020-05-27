// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc . -help

package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/vtrace"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/examples/rps"
	"v.io/x/ref/examples/rps/internal"
	"v.io/x/ref/internal/logger"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"
)

var name, aclFile, mountPrefix string

func main() {
	cmdRoot.Flags.StringVar(&name, "name", "", "Identifier to publish as (defaults to principal's blessing names).")
	cmdRoot.Flags.StringVar(&aclFile, "acl-file", "", "File containing JSON-encoded Permissions.")
	cmdRoot.Flags.StringVar(&mountPrefix, "mount-prefix", "vlab", "The mount prefix to use. The published name will be <mount-prefix>/rps/player/<name>.")
	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdRoot)
}

var cmdRoot = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runPlayer),
	Name:   "rpsplayer",
	Short:  "Implements the Player interface",
	Long: `
Command rpsplayer implements the Player interface, which enables a human to play
the game.
`,
}

func runPlayer(rootctx *context.T, env *cmdline.Env, args []string) error {
	for {
		ctx, _ := vtrace.WithNewTrace(rootctx)
		if selectOne([]string{"Initiate Game", "Wait For Challenge"}) == 0 {
			initiateGame(ctx)
		} else {
			fmt.Println("Waiting to receive a challenge...")
			game := recvChallenge(ctx)
			playGame(ctx, game.address, game.id)
		}
		if selectOne([]string{"Play Again", "Quit"}) == 1 {
			break
		}
	}
	return nil
}

type gameChallenge struct {
	address string
	id      rps.GameId
	opts    rps.GameOptions
}

// impl is a PlayerServerMethods implementation that prompts the user to accept
// or decline challenges. While waiting for a reply from the user, any incoming
// challenges are auto-declined.
type impl struct {
	ch      chan gameChallenge
	decline bool
	lock    sync.Mutex
}

func (i *impl) setDecline(v bool) bool {
	i.lock.Lock()
	defer i.lock.Unlock()
	prev := i.decline
	i.decline = v
	return prev
}

func (i *impl) Challenge(ctx *context.T, call rpc.ServerCall, address string, id rps.GameId, opts rps.GameOptions) error {
	remote, _ := security.RemoteBlessingNames(ctx, call.Security())
	ctx.VI(1).Infof("Challenge (%q, %+v) from %v", address, id, remote)
	// When setDecline(true) returns, future challenges will be declined.
	// Whether the current challenge should be considered depends on the
	// previous state. If 'decline' was already true, we need to decline
	// this challenge. It 'decline' was false, this is the first challenge
	// that we should process.
	if i.setDecline(true) {
		return errors.New("player is busy")
	}
	fmt.Println()
	fmt.Printf("Challenge received from %v for a %d-round ", remote, opts.NumRounds)
	switch opts.GameType {
	case rps.Classic:
		fmt.Print("Classic ")
	case rps.LizardSpock:
		fmt.Print("Lizard-Spock ")
	default:
	}
	fmt.Println("Game.")
	if selectOne([]string{"Accept", "Decline"}) == 0 {
		i.ch <- gameChallenge{address, id, opts}
		return nil
	}
	// Start considering challenges again.
	i.setDecline(false)
	return errors.New("player declined challenge")
}

// recvChallenge runs a server until a game challenge is accepted by the user.
// The server is stopped afterwards.
func recvChallenge(ctx *context.T) gameChallenge {
	ch := make(chan gameChallenge)
	if name == "" {
		name = internal.CreateName(ctx)
	}
	fullname := naming.Join(mountPrefix, "rps", "player", name)
	service := rps.PlayerServer(&impl{ch: ch})
	auth := internal.NewAuthorizer(aclFile)
	ctx, cancel := context.WithCancel(ctx)
	ctx, server, err := v23.WithNewServer(ctx, fullname, service, auth)
	if err != nil {
		ctx.Fatalf("NewServer failed: %v", err)
	}
	ctx.Infof("Listening on endpoint /%s", server.Status().Endpoints[0])
	result := <-ch
	cancel()
	<-server.Closed()
	return result
}

// initiateGame initiates a new game by getting a list of judges and players,
// and asking the user to select one of each, to select the game options, what
// to play, etc.
func initiateGame(ctx *context.T) error {
	jChan := make(chan []string)
	oChan := make(chan []string)
	go findAll(ctx, "judge", jChan)
	go findAll(ctx, "player", oChan)

	fmt.Println("Looking for available participants...")
	judges := <-jChan
	opponents := <-oChan
	fmt.Println()
	if len(judges) == 0 || len(opponents) == 0 {
		return errors.New("no one to play with")
	}

	fmt.Println("Choose a judge:")
	j := selectOne(judges)
	fmt.Println("Choose an opponent:")
	o := selectOne(opponents)
	fmt.Println("Choose the type of rock-paper-scissors game would you like to play:")
	gameType := selectOne([]string{"Classic", "LizardSpock"})
	fmt.Println("Choose the number of rounds required to win:")
	numRounds := selectOne([]string{"1", "2", "3", "4", "5", "6"}) + 1
	gameOpts := rps.GameOptions{NumRounds: int32(numRounds), GameType: rps.GameTypeTag(gameType)}

	gameID, err := createGame(ctx, judges[j], gameOpts)
	if err != nil {
		ctx.Infof("createGame: %v", err)
		return err
	}
	for {
		err := sendChallenge(ctx, opponents[o], judges[j], gameID, gameOpts)
		if err == nil {
			break
		}
		fmt.Printf("Challenge was declined by %s (%v)\n", opponents[o], err)
		fmt.Println("Choose another opponent:")
		o = selectOne(opponents)
	}
	fmt.Println("Joining the game...")

	if _, err = playGame(ctx, judges[j], gameID); err != nil {
		ctx.Infof("playGame: %v", err)
		return err
	}
	return nil
}

func createGame(ctx *context.T, judge string, opts rps.GameOptions) (rps.GameId, error) {
	return rps.JudgeClient(naming.Join(mountPrefix, judge)).CreateGame(ctx, opts)
}

func sendChallenge(ctx *context.T, opponent, judge string, gameID rps.GameId, gameOpts rps.GameOptions) error {
	return rps.PlayerClient(naming.Join(mountPrefix, opponent)).Challenge(ctx, judge, gameID, gameOpts)
}

func playGame(outer *context.T, judge string, gameID rps.GameId) (rps.PlayResult, error) {
	ctx, cancel := context.WithTimeout(outer, 10*time.Minute)
	defer cancel()
	fmt.Println()
	game, err := rps.JudgeClient(naming.Join(mountPrefix, judge)).Play(ctx, gameID)
	if err != nil {
		return rps.PlayResult{}, err
	}
	var playerNum int32
	rStream := game.RecvStream()
	for rStream.Advance() {
		in := rStream.Value()
		switch v := in.(type) {
		case rps.JudgeActionPlayerNum:
			playerNum = v.Value
			fmt.Printf("You are player %d\n", playerNum)
		case rps.JudgeActionOpponentName:
			fmt.Printf("Your opponent is %q\n", v.Value)
		case rps.JudgeActionRoundResult:
			rr := v.Value
			if playerNum != 1 && playerNum != 2 {
				ctx.Fatalf("invalid playerNum: %d", playerNum)
			}
			fmt.Printf("You played %q\n", rr.Moves[playerNum-1])
			fmt.Printf("Your opponent played %q\n", rr.Moves[2-playerNum])
			if len(rr.Comment) > 0 {
				fmt.Printf(">>> %s <<<\n", strings.ToUpper(rr.Comment))
			}
			if rr.Winner == 0 {
				fmt.Println("----- It's a draw -----")
			} else if rps.WinnerTag(playerNum) == rr.Winner {
				fmt.Println("***** You WIN *****")
			} else {
				fmt.Println("##### You LOSE #####")
			}
		case rps.JudgeActionMoveOptions:
			opts := v.Value
			fmt.Println()
			fmt.Println("Choose your weapon:")
			m := selectOne(opts)
			if err := game.SendStream().Send(rps.PlayerActionMove{Value: opts[m]}); err != nil {
				return rps.PlayResult{}, err
			}
		case rps.JudgeActionScore:
			score := v.Value
			fmt.Println()
			fmt.Println("==== GAME SUMMARY ====")
			fmt.Print(internal.FormatScoreCard(score))
			fmt.Println("======================")
			if rps.WinnerTag(playerNum) == score.Winner {
				fmt.Println("You won! :)")
			} else {
				fmt.Println("You lost! :(")
			}
		default:
			outer.Infof("unexpected message type: %T", in)
		}
	}
	if err := rStream.Err(); err == nil {
		fmt.Println("Game Ended")
	} else {
		outer.Infof("stream error: %v", err)
	}

	return game.Finish()
}

func selectOne(choices []string) (choice int) {
	if len(choices) == 0 {
		logger.Global().Fatal("No options to choose from!")
	}
	fmt.Println()
	for i, x := range choices {
		fmt.Printf("  %d. %s\n", i+1, x)
	}
	fmt.Println()
	for {
		if len(choices) == 1 {
			fmt.Print("Select one [1] --> ")
		} else {
			fmt.Printf("Select one [1-%d] --> ", len(choices))
		}
		fmt.Scanf("%d", &choice)
		if choice >= 1 && choice <= len(choices) {
			choice -= 1
			break
		}
	}
	fmt.Println()
	return
}

func findAll(ctx *context.T, t string, out chan []string) {
	ns := v23.GetNamespace(ctx)
	var result []string
	c, err := ns.Glob(ctx, naming.Join(mountPrefix, "rps", t, "*"))
	if err != nil {
		ctx.Infof("ns.Glob failed: %v", err)
		out <- result
		return
	}
	for e := range c {
		switch v := e.(type) {
		case *naming.GlobReplyError:
			fmt.Print("E")
		case *naming.GlobReplyEntry:
			fmt.Print(".")
			result = append(result, strings.TrimPrefix(v.Value.Name, naming.Clean(mountPrefix)+"/"))
		}
	}
	if len(result) == 0 {
		ctx.Infof("found no %ss", t)
		out <- result
		return
	}
	sort.Strings(result)
	out <- result
}
