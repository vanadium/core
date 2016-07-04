// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"v.io/v23/context"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/internal/logger"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/services/wakeup/gcm"
	"v.io/x/ref/services/wakeup/wakeuplib"
)

var mountName, gcmProjectID, gcmApiKeyFile, sharedKeyFile, mtPersistDir string

func main() {
	cmdWD.Flags.StringVar(&mountName, "mount-name", "wakeup", `A non-empty mount point name for the wakeup server.  The name may be absolute for a remote mount table service (e.g. "/<remote mt address>//some/suffix") or could be relative to this process' default mount table (e.g. "some/suffix").`)
	cmdWD.Flags.StringVar(&mtPersistDir, "persist-dir", "", `Directory in which to persist the mounts.  If empty, mounts aren't persisted.`)
	cmdWD.Flags.StringVar(&gcmProjectID, "gcm-project-id", "", `GCE project identifier for outgoing GCM messages (e.g., 632758215260@gcm.googleapis.com).`)
	cmdWD.Flags.StringVar(&gcmApiKeyFile, "gcm-api-key-file", "", `File that stores a Google API key used for outgoing GCM messages.  Should correspond to gcm-project-id.`)
	cmdWD.Flags.StringVar(&sharedKeyFile, "shared-key-file", "", `File that stores a shared key used by the wakeup server.  Must be specified.`)
	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdWD)
}

var cmdWD = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runWakeUpD),
	Name:   "wakeupd",
	Short:  "Runs the wakeup daemon",
	Long: `
Command wakeupd runs the wake-up daemon, which implements the
v.io/v23/services/wakeup interfaces.
`,
}

func runWakeUpD(ctx *context.T, env *cmdline.Env, args []string) error {
	if len(gcmProjectID) == 0 {
		return fmt.Errorf("Empty GCM project id")
	}
	// Read the api key from the file.
	b, err := ioutil.ReadFile(gcmApiKeyFile)
	if err != nil {
		return fmt.Errorf("Couldn't open GCM API Key file %q: %v", gcmApiKeyFile, err)
	}
	if len(b) == 0 {
		return fmt.Errorf("Empty GCM API Key in file %s", gcmApiKeyFile)
	}
	gcmApiKey := string(b)

	// Read the shared key from the file and/or write it into that file.
	f, err := os.Open(sharedKeyFile)
	if os.IsNotExist(err) {
		f, err = createKeyFile(sharedKeyFile)
	}
	if err != nil {
		return fmt.Errorf("Couldn't open shared key file %q: %v", sharedKeyFile, err)
	}
	var sharedKey [32]byte
	if _, err := io.ReadFull(f, sharedKey[:]); err != nil {
		return fmt.Errorf("Couldn't read shared key from file %q: %v", sharedKeyFile, err)
	}
	// Start wakeup servers.
	w := gcm.NewGCMWakeup(gcmProjectID, gcmApiKey, logger.Global())
	stop, err := wakeuplib.StartServers(ctx, mountName, mtPersistDir, sharedKey, w)
	if err != nil {
		return fmt.Errorf("mounttablelib.StartServers failed: %v", err)
	}
	defer stop()
	// Wait until signal is received.
	ctx.Info("Received ]signal ", <-signals.ShutdownOnSignals(ctx))
	return nil
}

func createKeyFile(fileName string) (*os.File, error) {
	var sharedKey [32]byte
	if _, err := rand.Read(sharedKey[:]); err != nil {
		return nil, err
	}
	if err := ioutil.WriteFile(fileName, sharedKey[:], 0600); err != nil {
		return nil, err
	}
	return os.Open(fileName)
}
