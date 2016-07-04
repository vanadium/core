// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/application"
	"v.io/v23/services/permissions"
	"v.io/v23/verror"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	"v.io/x/ref/services/internal/binarylib"
	"v.io/x/ref/services/repository"
)

// TODO(caprita): Add unit test.

// TODO(caprita): Extend to include env, args, packages.

var cmdPublish = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runPublish),
	Name:   "publish",
	Short:  "Publish the given application(s).",
	Long: `
Publishes the given application(s) to the binary and application servers.
The binaries should be in $JIRI_ROOT/release/go/bin/[<GOOS>_<GOARCH>] by default (can be overrriden with --from).
By default the binary name is used as the name of the application envelope, and as the
title in the envelope. However, <envelope-name> and <title> can be specified explicitly
using :<envelope-name> and @<title>.
The binary is published as <binserv>/<binary name>/<GOOS>-<GOARCH>/<TIMESTAMP>.
The application envelope is published as <appserv>/<envelope-name>/<TIMESTAMP>.
Optionally, adds blessing patterns to the Read and Resolve AccessLists.`,
	ArgsName: "<binary name>[:<envelope-name>][@<title>] ...",
}

var binaryService, applicationService, readBlessings, goarchFlag, goosFlag, fromFlag string
var addPublisher bool
var minValidPublisherDuration time.Duration

func init() {
	cmdPublish.Flags.StringVar(&binaryService, "binserv", "binaries", "Name of binary service.")
	cmdPublish.Flags.StringVar(&applicationService, "appserv", "applications", "Name of application service.")
	cmdPublish.Flags.StringVar(&goosFlag, "goos", runtime.GOOS, "GOOS for application.  The default is the value of runtime.GOOS.")
	cmdPublish.Flags.Lookup("goos").DefValue = "<runtime.GOOS>"
	cmdPublish.Flags.StringVar(&goarchFlag, "goarch", runtime.GOARCH, "GOARCH for application.  The default is the value of runtime.GOARCH.")
	cmdPublish.Flags.Lookup("goarch").DefValue = "<runtime.GOARCH>"
	cmdPublish.Flags.StringVar(&readBlessings, "readers", "dev.v.io", "If non-empty, comma-separated blessing patterns to add to Read and Resolve AccessList.")
	cmdPublish.Flags.BoolVar(&addPublisher, "add-publisher", true, "If true, add a publisher blessing to the application envelope")
	cmdPublish.Flags.DurationVar(&minValidPublisherDuration, "publisher-min-validity", 30*time.Hour, "Publisher blessings that are valid for less than this amount of time are considered invalid")
	cmdPublish.Flags.StringVar(&fromFlag, "from", "", "Location of binaries to be published.  Defaults to $JIRI_ROOT/release/go/bin/[<GOOS>_<GOARCH>]")
}

func setAccessLists(ctx *context.T, env *cmdline.Env, von string) error {
	if readBlessings == "" {
		return nil
	}
	perms, version, err := permissions.ObjectClient(von).GetPermissions(ctx)
	if err != nil {
		// TODO(caprita): This is a workaround until we sort out the
		// default AccessLists for applicationd (see issue #1317).  At that
		// time, uncomment the line below.
		//
		//   return err
		perms = make(access.Permissions)
	}
	for _, blessing := range strings.Split(readBlessings, ",") {
		for _, tag := range []access.Tag{access.Read, access.Resolve} {
			perms.Add(security.BlessingPattern(blessing), string(tag))
		}
	}
	if err := permissions.ObjectClient(von).SetPermissions(ctx, perms, version); err != nil {
		return err
	}
	fmt.Fprintf(env.Stdout, "Added patterns %q to Read,Resolve AccessList for %q\n", readBlessings, von)
	return nil
}

func publishOne(ctx *context.T, env *cmdline.Env, binPath, binary string) error {
	binaryName, envelopeName, title := binary, binary, binary
	binaryRE := regexp.MustCompile(`^([^:@]+)(:[^@]+)?(@.+)?$`)
	if parts := binaryRE.FindStringSubmatch(binary); len(parts) == 4 {
		binaryName = parts[1]
		envelopeName, title = binaryName, binaryName
		if len(parts[2]) > 1 {
			envelopeName = parts[2][1:]
		}
		if len(parts[3]) > 1 {
			title = parts[3][1:]
		}
	} else {
		return fmt.Errorf("invalid binary spec (%v)", binary)
	}

	// Step 1, upload the binary to the binary service.

	// TODO(caprita): Instead of the current timestamp, use each binary's
	// BuildTimestamp from the buildinfo.
	timestamp := time.Now().UTC().Format(time.RFC3339)
	binaryVON := naming.Join(binaryService, binaryName, fmt.Sprintf("%s-%s", goosFlag, goarchFlag), timestamp)
	binaryFile := filepath.Join(binPath, binaryName)
	var binarySig *security.Signature
	var err error
	for i := 0; ; i++ {
		binarySig, err = binarylib.UploadFromFile(ctx, binaryVON, binaryFile)
		if verror.ErrorID(err) == verror.ErrExist.ID {
			newTS := fmt.Sprintf("%s-%d", timestamp, i+1)
			binaryVON = naming.Join(binaryService, binaryName, fmt.Sprintf("%s-%s", goosFlag, goarchFlag), newTS)
			continue
		}
		if err == nil {
			break
		}
		return err
	}
	fmt.Fprintf(env.Stdout, "Binary %q uploaded from file %s\n", binaryVON, binaryFile)

	// Step 2, set the perms for the uploaded binary.

	if err := setAccessLists(ctx, env, binaryVON); err != nil {
		return err
	}

	// Step 3, download existing envelope (or create a new one), update, and
	// upload to application service.

	// TODO(caprita): use the profile detection machinery and/or let user
	// specify the profile by hand.
	profile := fmt.Sprintf("%s-%s", goosFlag, goarchFlag)
	appVON := naming.Join(applicationService, envelopeName)
	appClient := repository.ApplicationClient(appVON)
	envelope, err := appClient.Match(ctx, []string{profile})
	// TODO(caprita): Fix https://github.com/vanadium/issues/issues/679
	if errID := verror.ErrorID(err); errID == verror.ErrNoExist.ID || errID == "v.io/x/ref/services/application/applicationd.InvalidSuffix" {
		// There was nothing published yet, create a new envelope.
		envelope = application.Envelope{Title: title}
	} else if err != nil {
		return err
	} else {
		// We are going to be updating an existing envelope

		// Complain if a title was specified explicitly and does not match the one in the
		// envelope, because we are not going to update the one in the envelope
		if title != binaryName && title != envelope.Title {
			return fmt.Errorf("Specified title (%v) does not match title in existing envelope (%v)", title, envelope.Title)
		}
	}

	envelope.Binary.File = binaryVON
	if addPublisher {
		publisher, err := getPublisherBlessing(ctx, strings.Join([]string{"apps", "published", title}, security.ChainSeparator))
		if err != nil {
			return err
		}
		envelope.Publisher = publisher
		envelope.Binary.Signature = *binarySig
	} else {
		// We must explicitly clear these fields because we might be trying to update
		// an envelope that previously pointed at a signed binary.
		envelope.Binary.Signature = security.Signature{}
		envelope.Publisher = security.Blessings{}
	}
	appVON = naming.Join(appVON, timestamp)
	appClient = repository.ApplicationClient(appVON)
	if err := appClient.Put(ctx, profile, envelope, false); err != nil {
		// NOTE(caprita): We don't retry if an envelope already exists
		// at the versioned name, as we do when uploading binaries.  In
		// the case of binaries, it's likely that the same binary is
		// uploaded more than once in a given second, due to apps
		// sharing the same binary.  The scenarios where the same app is
		// published repeatedly in a short time-frame are expected to be
		// rare, and the operator can retry manually in such cases.
		return err
	}
	fmt.Fprintf(env.Stdout, "Published %q\n", appVON)

	// Step 4, set the perms for the uploaded envelope.

	if err := setAccessLists(ctx, env, appVON); err != nil {
		return err
	}
	return nil
}

func runPublish(ctx *context.T, env *cmdline.Env, args []string) error {
	if expectedMin, got := 1, len(args); got < expectedMin {
		return env.UsageErrorf("publish: incorrect number of arguments, expected at least %d, got %d", expectedMin, got)
	}
	binaries := args
	binPath := fromFlag
	if binPath == "" {
		vroot := env.Vars["JIRI_ROOT"]
		if vroot == "" {
			return env.UsageErrorf("publish: $JIRI_ROOT environment variable should be set")
		}
		binPath = filepath.Join(vroot, "release/go/bin")
		if goosFlag != runtime.GOOS || goarchFlag != runtime.GOARCH {
			binPath = filepath.Join(binPath, fmt.Sprintf("%s_%s", goosFlag, goarchFlag))
		}
	}
	if fi, err := os.Stat(binPath); err != nil {
		return env.UsageErrorf("publish: failed to stat %v: %v", binPath, err)
	} else if !fi.IsDir() {
		return env.UsageErrorf("publish: %v is not a directory", binPath)
	}
	if binaryService == "" {
		return env.UsageErrorf("publish: --binserv must point to a binary service name")
	}
	if applicationService == "" {
		return env.UsageErrorf("publish: --appserv must point to an application service name")
	}
	var lastErr error
	for _, b := range binaries {
		if err := publishOne(ctx, env, binPath, b); err != nil {
			fmt.Fprintf(env.Stderr, "Failed to publish %q: %v\n", b, err)
			lastErr = err
		}
	}
	return lastErr
}

func getPublisherBlessing(ctx *context.T, extension string) (security.Blessings, error) {
	p := v23.GetPrincipal(ctx)
	bDef, _ := p.BlessingStore().Default()
	b, err := p.Bless(p.PublicKey(), bDef, extension, security.UnconstrainedUse())
	if err != nil {
		return security.Blessings{}, err
	}

	// We need to make sure that the blessing is usable as a publisher blessing -- in
	// practice this current means that it has no caveats other than expiration. We
	// test this by putting it into a call object and verifying that the blessing will
	// not be rejected
	call := security.NewCall(&security.CallParams{
		RemoteBlessings: b,
		LocalBlessings:  bDef,
		LocalPrincipal:  p,
		Timestamp:       time.Now().Add(minValidPublisherDuration),
	})
	accepted, rejected := security.RemoteBlessingNames(ctx, call)
	if len(accepted) == 0 {
		return security.Blessings{}, fmt.Errorf("All blessings are invalid: %v", rejected)
	}
	if len(rejected) > 0 {
		fmt.Fprintf(os.Stderr, "Warning: Some invalid blessings are present: %v", rejected)
	}
	return b, nil
}
