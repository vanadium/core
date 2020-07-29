// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/vom"
	"v.io/x/lib/cmdline"
	seclib "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"
	"v.io/x/ref/services/debug/debug/browseserver"
)

func init() {
	cmdBrowse.Flags.StringVar(&flagBrowseAddr, "addr", "", "Address on which the interactive HTTP server will listen. For example, localhost:14141. If empty, defaults to localhost:<some random port>")
	cmdBrowse.Flags.BoolVar(&flagBrowseLog, "log", true, "If true, log debug data obtained so that if a subsequent refresh from the browser fails, previously obtained information is available from the log file")
	cmdBrowse.Flags.StringVar(&flagBrowseBlessings, "blessings", "", "If non-empty, points to the blessings required to debug the process. This is typically obtained via 'debug delegate' run by the owner of the remote process")
	cmdBrowse.Flags.StringVar(&flagBrowsePrivateKey, "key", "", "If non-empty, must be accompanied with --blessings with a value obtained via 'debug delegate' run by the owner of the remote process")
	cmdBrowse.Flags.StringVar(&flagBrowseAssets, "assets", "", "If non-empty, load assets from this directory.")
	cmdDelegate.Flags.StringVar(&flagDelegateAccess, "access", "resolve,debug", "Comma-separated list of access tags on methods that can be invoked by the delegate")
}

var (
	flagBrowseAddr       string
	flagBrowseLog        bool
	flagBrowseBlessings  string
	flagBrowsePrivateKey string
	flagBrowseAssets     string
	flagDelegateAccess   string
	cmdBrowse            = &cmdline.Command{
		Runner: v23cmd.RunnerFunc(runBrowse),
		Name:   "browse",
		Short:  "Starts an interactive interface for debugging",
		Long: `
Starts a webserver with a URL that when visited allows for inspection of a
remote process via a web browser.

This differs from browser.v.io in a few important ways:

  (a) Does not require a chrome extension,
  (b) Is not tied into the v.io cloud services
  (c) Can be setup with alternative different credentials,
  (d) The interface is more geared towards debugging a server than general purpose namespace browsing.

While (d) is easily overcome by sharing code between the two, (a), (b) & (c)
are not easy to work around.  Of course, the down-side here is that this
requires explicit command-line invocation instead of being just a URL anyone
can visit (https://browser.v.io).

A dump of some possible future features:
TODO(ashankar):?

  (1) Trace browsing: Browse traces at the remote server, and possible force
  the collection of some traces (avoiding the need to restart the remote server
  with flags like --v23.vtrace.collect-regexp for example). In the mean time,
  use the 'vtrace' command (instead of the 'browse' command) for this purpose.
  (2) Log offsets: Log files can be large and currently the logging endpoint
  of this interface downloads the full log file from the beginning. The ability
  to start looking at the logs only from a specified offset might be useful
  for these large files.
  (3) Signature: Display the interfaces, types etc. defined by any suffix in the
  remote process. in the mean time, use the 'vrpc signature' command for this purpose.
`,
		ArgsName: "<name> [<name>] [<name>]",
		ArgsLong: `
<name> is the vanadium object name of the remote process to inspect.
If multiple names are provided, they are considered equivalent and any one of them that can
is accessible is used.`,
	}

	cmdDelegate = &cmdline.Command{
		Runner: v23cmd.RunnerFunc(runDelegate),
		Name:   "delegate",
		Short:  "Create credentials to delegate debugging to another user",
		Long: `
Generates credentials (private key and blessings) required to debug remote
processes owned by the caller and prints out the command that the delegate can
run. The delegation limits the bearer of the token to invoke methods on the
remote process only if they have the "access.Debug" tag on them, and this token
is valid only for limited amount of time. For example, if Alice wants Bob to
debug a remote process owned by her for the next 2 hours, she runs:

  debug delegate my-friend-bob 2h myservices/myservice

And sends Bob the output of this command. Bob will then be able to inspect the
remote process as myservices/myservice with the same authorization as Alice.
`,
		ArgsName: "<to> <duration> [<name>]",
		ArgsLong: `
<to> is an identifier to provide to the delegate.

<duration> is the time period to delegate for (e.g., 1h for 1 hour)

<name> (optional) is the vanadium object name of the remote process to inspect
`,
	}
)

func runBrowse(ctx *context.T, env *cmdline.Env, args []string) error { //nolint:gocyclo
	if len(args) == 0 {
		return env.UsageErrorf("must provide at least a single vanadium object name")
	}
	// For now, require that either both --key and --blessings be set, or
	// neither be. This ensures that no persistent changes are made to the principal
	// embedded in 'ctx' at this point.
	if (len(flagBrowsePrivateKey) != 0) != (len(flagBrowseBlessings) != 0) {
		return env.UsageErrorf("either both --key and --blessings should be set, or neither should be")
	}
	if len(flagBrowsePrivateKey) != 0 {
		keybytes, err := base64.URLEncoding.DecodeString(flagBrowsePrivateKey)
		if err != nil {
			return fmt.Errorf("bad value for --key: %v", err)
		}
		key, err := x509.ParseECPrivateKey(keybytes)
		if err != nil {
			// Try with PKCS#8
			tmp, err := x509.ParsePKCS8PrivateKey(keybytes)
			if err != nil {
				return fmt.Errorf("bad value for --key: %v", err)
			}
			var ok bool
			if key, ok = tmp.(*ecdsa.PrivateKey); !ok {
				return fmt.Errorf("expected an ECDSA private in in --key, got %T", tmp)
			}
		}
		signer, err := security.NewInMemoryECDSASigner(key)
		if err != nil {
			return fmt.Errorf("failed to create an ECDSA signer: %v", err)
		}
		principal, err := seclib.NewPrincipalFromSigner(signer)
		if err != nil {
			return fmt.Errorf("unable to use --key: %v", err)
		}
		if ctx, err = v23.WithPrincipal(ctx, principal); err != nil {
			return err
		}
	}
	if len(flagBrowseBlessings) != 0 {
		blessingbytes, err := base64.URLEncoding.DecodeString(flagBrowseBlessings)
		if err != nil {
			return fmt.Errorf("bad value for --blessings: %v", err)
		}
		var blessings security.Blessings
		if err := vom.Decode(blessingbytes, &blessings); err != nil {
			return fmt.Errorf("bad value for --blessings: %v", err)
		}
		if _, err := v23.GetPrincipal(ctx).BlessingStore().Set(blessings, security.AllPrincipals); err != nil {
			if len(flagBrowsePrivateKey) == 0 {
				return fmt.Errorf("cannot use --blessings, mismatch with --key? (%v)", err)
			}
			return fmt.Errorf("cannot use --blessings: %v", err)
		}
		if err := security.AddToRoots(v23.GetPrincipal(ctx), blessings); err != nil {
			return fmt.Errorf("failed to add --blessings to the set of recognized roots: %v", err)
		}
	}
	name, err := selectName(ctx, args)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-signals.ShutdownOnSignals(ctx)
		cancel()
	}()
	return browseserver.Serve(ctx, flagBrowseAddr, name, timeout, flagBrowseLog, flagBrowseAssets)
}

func selectName(ctx *context.T, options []string) (string, error) {
	// TODO(ashankar,mattr): This is way more complicated than it should
	// be.  What we really want is to be able to have the RPC system take
	// in these many names and pick the best one (accessible, shorted path)
	// in every subsequent call. For now, just try them all in parallel and
	// return the first one that works.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	working := make(chan int, len(options))
	errorch := make(chan error, len(options))
	for i := range options {
		go func(idx int) {
			pc, err := v23.GetClient(ctx).PinConnection(ctx, options[idx])
			if err != nil {
				errorch <- err
				return
			}
			pc.Unpin()
			working <- idx
		}(i)
	}
	var errors []error
	for i := 0; i < len(options); i++ {
		select {
		case idx := <-working:
			return options[idx], nil
		case err := <-errorch:
			errors = append(errors, err)
		}
	}
	return "", fmt.Errorf("failed to contact server: %v", errors)
}

func delegateAccessCaveat() (security.Caveat, error) {
	strs := strings.Split(flagDelegateAccess, ",")
	var tags []access.Tag
	for _, s := range strs {
		ls := strings.ToLower(s)
		found := false
		for _, t := range access.AllTypicalTags() {
			lt := strings.ToLower(string(t))
			if ls == lt {
				tags = append(tags, t)
				found = true
				break
			}
		}
		if !found {
			return security.Caveat{}, fmt.Errorf("invalid --access: [%s] is not in %v", s, access.AllTypicalTags())
		}
	}
	if len(tags) == 0 {
		var all []string
		for _, t := range access.AllTypicalTags() {
			all = append(all, string(t))
		}
		return security.Caveat{}, fmt.Errorf("must specify a non-empty --access. To grant all access, use --access=%s", strings.Join(all, ","))
	}
	return access.NewAccessTagCaveat(tags...)
}

func runDelegate(ctx *context.T, env *cmdline.Env, args []string) error {
	if len(args) < 2 || len(args) > 3 {
		return env.UsageErrorf("got %d arguments, expecting 2 or 3", len(args))
	}
	extension := args[0]
	duration, err := time.ParseDuration(args[1])
	if err != nil {
		return env.UsageErrorf("failed to parse <duration>: %v", err)
	}
	name := "<name>"
	if len(args) == 3 {
		name = args[2]
	}
	// Create a new private key.
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	privbytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return err
	}
	// Create a blessings
	var caveats []security.Caveat
	c, err := delegateAccessCaveat()
	if err != nil {
		return err
	}
	caveats = append(caveats, c)

	c, err = security.NewExpiryCaveat(time.Now().Add(duration))
	if err != nil {
		return err
	}
	caveats = append(caveats, c)

	p := v23.GetPrincipal(ctx)
	b, _ := p.BlessingStore().Default()
	blessing, err := p.Bless(security.NewECDSAPublicKey(&priv.PublicKey), b, extension, caveats[0], caveats[1:]...)
	if err != nil {
		return err
	}
	blessingbytes, err := vom.Encode(blessing)
	if err != nil {
		return err
	}
	fmt.Printf(`Your delegate will be seen as %v at your server. Ask them to run:

debug browse \
  --key=%q \
  --blessings=%q \
  %q
`,
		blessing,
		base64.URLEncoding.EncodeToString(privbytes),
		base64.URLEncoding.EncodeToString(blessingbytes),
		name)
	return nil
}
