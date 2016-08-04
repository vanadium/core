// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"reflect"
	"runtime"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/x/lib/cmdline"
	"v.io/x/ref"
	"v.io/x/ref/lib/v23cmd"

	_ "v.io/x/ref/runtime/factories/generic"
)

func newKey() security.PublicKey {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	return security.NewECDSAPublicKey(&k.PublicKey)
}

var blessingName string

func main() {
	// A pristine agent has a single blessing "agent_principal" (from agentd/main.go).
	cmdTestPrincipal.Flags.StringVar(&blessingName, "expect-blessing", "agent_principal", "The name of the blessing in the provided credentials")
	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdTestPrincipal)
}

var cmdTestPrincipal = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runTestPrincipal),
	Name:   "test_principal",
	Short:  "Runs tests against a principal",
	Long:   "Command test_principal runs tests against a principal.",
}

func runTestPrincipal(ctx *context.T, env *cmdline.Env, args []string) error {
	var errors []string
	errorf := func(format string, args ...interface{}) {
		_, file, line, _ := runtime.Caller(1)
		errors = append(errors, fmt.Sprintf("%v:%d: %v", file, line, fmt.Sprintf(format, args...)))
	}
	p := v23.GetPrincipal(ctx)
	// Make sure we're running under a pristine agent to begin with.
	// The agent aims to be transparent, so use a collection of heuristics
	// to detect this.
	if got := env.Vars[ref.EnvCredentials]; len(got) != 0 {
		errorf("%v environment variable is unexpectedly set", ref.EnvCredentials)
	}
	if got := env.Vars[ref.EnvAgentPath]; len(got) == 0 {
		errorf("%v environment variable is not set", ref.EnvAgentPath)
	}
	def, defCh := p.BlessingStore().Default()
	if got, want := security.BlessingNames(p, def), []string{blessingName}; !reflect.DeepEqual(got, want) {
		errorf("Got %v want %v", got, want)
	}

	// BlessSelf
	b, err := p.BlessSelf("batman")
	if err != nil {
		errorf("BlessSelf: %v", err)
	}
	// Bless
	if _, err := p.Bless(newKey(), b, "delegate", security.UnconstrainedUse()); err != nil {
		errorf("Bless: %v", err)
	}
	// Sign & PublicKey
	signature, err := p.Sign([]byte("bugs bunny"))
	if err != nil {
		errorf("Sign: %v", err)
	}
	if !signature.Verify(p.PublicKey(), []byte("bugs bunny")) {
		errorf("signature.Verify: %v", err)
	}
	// MintDischarge
	cav, err := security.NewMethodCaveat("method")
	if err != nil {
		errorf("security.MethodCaveat: %v", err)
	}
	tpcav, err := security.NewPublicKeyCaveat(p.PublicKey(), "location", security.ThirdPartyRequirements{}, cav)
	if err != nil {
		errorf("security.NewPublicKeyCaveat: %v", err)
	}
	dis, err := p.MintDischarge(tpcav, cav)
	if err != nil {
		errorf("MintDischarge: %v", err)
	}
	// BlessingRoots
	keybytes, err := p.PublicKey().MarshalBinary()
	if err != nil {
		errorf("Failed to marshal public key: %v", err)
	}
	if err := p.Roots().Recognized(keybytes, "batman"); err == nil {
		errorf("Roots().Recognized returned nil")
	}
	if err := security.AddToRoots(p, b); err != nil {
		errorf("AddToRoots: %v", err)
	}
	if err := p.Roots().Recognized(keybytes, "batman"); err != nil {
		errorf("Roots().Recognized: %v", err)
	}
	// BlessingStore: Defaults
	if err := p.BlessingStore().SetDefault(security.Blessings{}); err != nil {
		errorf("BlessingStore().SetDefault: %v", err)
	}
	<-defCh // SetDefault should trigger a notification
	if def, defCh = p.BlessingStore().Default(); !def.IsZero() {
		errorf("BlessingStore().Default returned %v, want empty", def)
	}
	if err := p.BlessingStore().SetDefault(b); err != nil {
		errorf("BlessingStore().SetDefault: %v", err)
	}
	<-defCh
	if def, defCh = p.BlessingStore().Default(); !reflect.DeepEqual(def, b) {
		errorf("BlessingStore().Default returned [%v], want [%v]", def, b)
	}
	// BlessingStore: Set & ForPeer
	// First, clear out the self-generated default of the blessing store.
	if _, err := p.BlessingStore().Set(security.Blessings{}, security.AllPrincipals); err != nil {
		errorf("BlessingStore().Set(nil, %q): %v", security.AllPrincipals, err)
	}
	if forpeer := p.BlessingStore().ForPeer("superman:friend"); !forpeer.IsZero() {
		errorf("BlessingStore().ForPeer unexpectedly returned %v", forpeer)
	}
	if old, err := p.BlessingStore().Set(b, "superman"); err != nil {
		errorf("BlessingStore().Set returned (%v, %v)", old, err)
	}
	if forpeer := p.BlessingStore().ForPeer("superman:friend"); !reflect.DeepEqual(forpeer, b) {
		errorf("BlessingStore().ForPeer returned %v and not %v", forpeer, b)
	}
	p.BlessingStore().CacheDischarge(dis, tpcav, security.DischargeImpetus{})
	if got, _ := p.BlessingStore().Discharge(tpcav, security.DischargeImpetus{}); !dis.Equivalent(got) {
		errorf("BlessingStore().Discharges returned %#v want %#v", got, dis)
	}
	p.BlessingStore().ClearDischarges(dis)
	if got, _ := p.BlessingStore().Discharge(tpcav, security.DischargeImpetus{}); got.ID() != "" {
		errorf("BlessingStore().Discharges returned %#v want empty", got)
	}

	if len(errors) > 0 {
		// Print out all errors and exit with failure.
		for _, e := range errors {
			fmt.Fprintln(env.Stderr, e)
		}
		return cmdline.ErrExitCode(1)
	}
	return nil
}
