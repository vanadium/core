// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rt

import (
	gocontext "context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/x/ref"
	vsecurity "v.io/x/ref/lib/security"
)

func (r *Runtime) initPrincipal(ctx *context.T, credentials string) (security.Principal, func(), error) {
	if principal, _ := ctx.Value(principalKey).(security.Principal); principal != nil {
		return principal, func() {}, nil
	}
	if len(credentials) > 0 {
		// Explicitly specified credentials, load them from the credentials
		// location without the ability to write them back to persistent
		// storage, but rather reloading them periodically or on a signal.
		readonly := true // should this be an option?
		reloadPeriod := 5 * time.Minute
		if update := os.Getenv(ref.EnvCredentialsReloadInterval); len(update) > 0 {
			if tmp, err := time.ParseDuration(update); err == nil {
				reloadPeriod = tmp
			}
		}
		goctx, cancel := gocontext.WithCancel(gocontext.Background())
		principal, err := vsecurity.LoadPersistentPrincipalDaemon(
			goctx,
			credentials,
			nil, // no passphrase.
			readonly,
			reloadPeriod,
		)
		if err != nil {
			cancel()
			return nil, nil, fmt.Errorf("failed to initialize credentials, perhaps you need to create them with 'principal create %v: %v", credentials, err)
		}
		return principal, func() { cancel() }, nil
	}

	// No agent, no explicit credentials specified: create a new principal
	// and blessing in memory.
	principal, err := vsecurity.NewPrincipal()
	if err != nil {
		return principal, nil, err
	}
	return principal, func() {}, vsecurity.InitDefaultBlessings(principal, defaultBlessingName())
}

func defaultBlessingName() string {
	options := []string{
		"apple", "banana", "cherry", "dragonfruit", "elderberry", "fig", "grape", "honeydew",
	}
	name := fmt.Sprintf("anonymous-%s-%d",
		options[rand.New(rand.NewSource(time.Now().Unix())).Intn(len(options))],
		os.Getpid())
	host, _ := os.Hostname()
	// (none) is a common default hostname and contains parentheses,
	// which are invalid blessings characters.
	if host == "(none)" || len(host) == 0 {
		return name
	}
	return name + "@" + host
}
