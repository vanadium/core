// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"v.io/v23"
	"v.io/v23/services/application"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/binary/tidy/appd"
	"v.io/x/ref/services/binary/tidy/binaryd"
	"v.io/x/ref/services/internal/servicetest"
	"v.io/x/ref/test"
)

func TestApplicationTidying(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	apptape := servicetest.NewTape()
	ctx, appserver, err := v23.WithNewDispatchingServer(ctx, "", appd.NewDispatcher(t, apptape))
	if err != nil {
		t.Fatalf("applicationd NewDispatchingServer failed: %v", err)
	}

	// Setup the command-line.
	var stdout, stderr bytes.Buffer
	env := &cmdline.Env{Stdout: &stdout, Stderr: &stderr}
	applicationName := appserver.Status().Endpoints[0].Name()

	apptape.SetResponses(
		// TidyNow()
		nil,
	)

	if err := v23cmd.ParseAndRunForTest(cmdApplicationTidy, ctx, env, []string{applicationName}); err != nil {
		t.Errorf("error: %v", err)
	}

	// Verify no output.
	if expected, got := "", strings.TrimSpace(stdout.String()); got != expected {
		t.Errorf("Unexpected output from tidy application. Got %q, expected %q", got, expected)
	}
	if expected, got := "", strings.TrimSpace(stderr.String()); got != expected {
		t.Errorf("Unexpected error from tidy application. Got %q, expected %q", got, expected)
	}

	// Verify application tape.
	if got, expected := apptape.Play(), []interface{}{
		"TidyNow",
	}; !reflect.DeepEqual(expected, got) {
		t.Errorf("apptape invalid call sequence. Got %#v, want %#v", got, expected)
	}
}

func TestBinaryTidying(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	binarytape := servicetest.NewTape()
	ctx, binserver, err := v23.WithNewDispatchingServer(ctx, "", binaryd.NewDispatcher(t, binarytape))
	if err != nil {
		t.Fatalf("binaryd NewDispatchingServer failed: %v", err)
	}

	apptape := servicetest.NewTape()
	ctx, appserver, err := v23.WithNewDispatchingServer(ctx, "", appd.NewDispatcher(t, apptape))
	if err != nil {
		t.Fatalf("applicationd NewDispatchingServer failed: %v", err)
	}

	// Setup the command-line.
	var stdout, stderr bytes.Buffer
	env := &cmdline.Env{Stdout: &stdout, Stderr: &stderr}
	binaryName := binserver.Status().Endpoints[0].Name()
	applicationName := appserver.Status().Endpoints[0].Name()

	binarytape.SetResponses(
		// Glob for all binary names
		binaryd.GlobResponse{[]string{
			"binaries",
			"binaries/applicationd",
			"binaries/applicationd/darwin-amd64",
			"binaries/applicationd/darwin-amd64/app-darwin-amd-1",
			"binaries/applicationd/darwin-amd64/app-darwin-amd-2",
			"binaries/applicationd/linux-amd64",
			"binaries/applicationd/linux-amd64/app-linux-amd-1",
			"binaries/applicationd/linux-amd64/app-linux-amd-2",
			"binaries/binaryd",
			"binaries/binaryd/linux-amd64",
			"binaries/binaryd/linux-amd64/bind-linux-amd-1",
			"binaries/binaryd/linux-amd64/bind-linux-amd-2",
			"binaries/binaryd/linux-amd64/bind-linux-amd-3",
			"binaries/libraries",
			"binaries/libraries/linux-amd64",
			"binaries/libraries/linux-amd64/extra-goo-1",
		},
			nil,
		},

		// Stat calls
		fmt.Errorf("binaries"),
		fmt.Errorf("binaries/applicationd"),
		fmt.Errorf("binaries/applicationd/darwin-amd64"),
		nil, // binaries/applicationd/darwin-amd64/app-darwin-amd-1
		fmt.Errorf("binaries/applicationd/linux-amd64"),
		nil, // binaries/applicationd/linux-amd64/app-linux-amd-1
		fmt.Errorf("binaries/binaryd"),
		fmt.Errorf("binaries/binaryd/linux-amd64"),
		nil, // binaries/binaryd/linux-amd64/bind-linux-amd-1
		nil, // binaries/binaryd/linux-amd64/bind-linux-amd-2
		fmt.Errorf("binaries/libraries"),
		fmt.Errorf("binaries/libraries/linux-amd64"),

		// Deletion of five binaries.
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	apptape.SetResponses(
		// Glob for all versions of all apps
		binaryd.GlobResponse{[]string{
			"applications",
			"applications/applicationd",
			"applications/applicationd/0",
			"applications/binaryd",
			"applications/binaryd/1",
		},
			nil,
		},

		// applications.Match(linux-amd64)
		appd.MatchResult{
			application.Envelope{},
			fmt.Errorf("no applications.Match(linux-amd64)"),
		},
		// applications.Match(linux-386)
		appd.MatchResult{
			application.Envelope{},
			fmt.Errorf("no applications.Match(linux-386)"),
		},
		// applications.Match(linux-arm)
		appd.MatchResult{
			application.Envelope{},
			fmt.Errorf("no applications.Match(linux-arm)"),
		},
		// applications.Match(darwin-amd64)
		appd.MatchResult{
			application.Envelope{},
			fmt.Errorf("no applications.Match(darwin-amd64)"),
		},
		// applications.Match(android-arm)
		appd.MatchResult{
			application.Envelope{},
			fmt.Errorf("no applications.Match(android-arm)"),
		},
		// applications/applicationd.Match(linux-amd64)
		appd.MatchResult{
			application.Envelope{
				Binary: application.SignedFile{
					File: "binaries/applicationd/linux-amd64/app-linux-amd-2",
				},
			},
			nil,
		},
		// applications/applicationd.Match(linux-386)
		appd.MatchResult{
			application.Envelope{},
			fmt.Errorf("no applications/applicationd.Match(linux-386)"),
		},
		// applications/applicationd.Match(linux-arm)
		appd.MatchResult{
			application.Envelope{},
			fmt.Errorf("no applications/applicationd.Match(linux-arm)"),
		},
		// applications/applicationd.Match(darwin-amd64)
		appd.MatchResult{
			application.Envelope{
				Binary: application.SignedFile{
					File: "binaries/applicationd/darwin-amd64/app-darwin-amd-2",
				},
			},
			nil,
		},
		// applications/applicationd.Match(android-arm)
		appd.MatchResult{
			application.Envelope{},
			fmt.Errorf("no applications/applicationd.Match(android-arm)"),
		},
		// applications/applicationd/0.Match(linux-amd64)
		appd.MatchResult{
			application.Envelope{
				Binary: application.SignedFile{
					File: "binaries/applicationd/linux-amd64/app-linux-amd-2",
				},
			},
			nil,
		},
		// applications/applicationd/0.Match(linux-386)
		appd.MatchResult{
			application.Envelope{},
			fmt.Errorf("no applications/applicationd/0.Match(linux-386)"),
		},
		// applications/applicationd/0.Match(linux-arm)
		appd.MatchResult{
			application.Envelope{},
			fmt.Errorf("no applications/applicationd/0.Match(linux-arm)"),
		},
		// applications/applicationd/0.Match(darwin-amd64)
		appd.MatchResult{
			application.Envelope{
				Binary: application.SignedFile{
					File: "binaries/applicationd/darwin-amd64/app-darwin-amd-2",
				},
			},
			nil,
		},
		// applications/applicationd/0.Match(android-arm)
		appd.MatchResult{
			application.Envelope{},
			fmt.Errorf("no applications/applicationd/0.Match(android-arm)"),
		},
		// applications/binaryd.Match(linux-amd64)
		appd.MatchResult{
			application.Envelope{
				Binary: application.SignedFile{
					File: "binaries/binaryd/linux-amd64/bind-linux-amd-3",
				},
				Packages: application.Packages{
					"somewhere": {
						File: "binaries/libraries/linux-amd64/extra-goo-1",
					},
				},
			},
			nil,
		},
		// applications/binaryd.Match(linux-386)
		appd.MatchResult{
			application.Envelope{},
			fmt.Errorf("no applications/binaryd.Match(linux-386)"),
		},
		// applications/binaryd.Match(linux-arm)
		appd.MatchResult{
			application.Envelope{},
			fmt.Errorf("no applications/binaryd.Match(linux-arm)"),
		},
		// applications/binaryd.Match(darwin-amd64)
		appd.MatchResult{
			application.Envelope{
				Binary: application.SignedFile{
					// Deliberately doesn't exist to show that this case is correctly handled.
					File: "binaries/binaryd/darwin-amd64/bind-darwin-amd-2",
				},
			},
			nil,
		},
		// applications/binaryd.Match(android-arm)
		appd.MatchResult{
			application.Envelope{},
			fmt.Errorf("no applications/binaryd.Match(android-arm)"),
		},
		// applications/binaryd/1.Match(linux-amd64)
		appd.MatchResult{
			application.Envelope{
				Binary: application.SignedFile{
					File: "binaries/binaryd/linux-amd64/bind-linux-amd-3",
				},
				Packages: application.Packages{
					"somewhere": {
						File: "binaries/libraries/linux-amd64/extra-goo-1",
					},
				},
			},
			nil,
		},
		// applications/binaryd/1.Match(linux-386)
		appd.MatchResult{
			application.Envelope{},
			fmt.Errorf("no applications/binaryd/1.Match(linux-386)"),
		},
		// applications/binaryd/1.Match(linux-arm)
		appd.MatchResult{
			application.Envelope{},
			fmt.Errorf("no applications/binaryd/1.Match(linux-arm)"),
		},
		// applications/binaryd/1.Match(darwin-amd64)
		appd.MatchResult{
			application.Envelope{
				Binary: application.SignedFile{
					// Deliberately doesn't exist to show that this case is correctly handled.
					File: "binaries/binaryd/darwin-amd64/bind-darwin-amd-2",
				},
			},
			nil,
		},
		// applications/binaryd/1.Match(android-arm)
		appd.MatchResult{
			application.Envelope{},
			fmt.Errorf("no applications/binaryd/1.Match(android-arm)"),
		},
	)

	if err := v23cmd.ParseAndRunForTest(cmdBinaryTidy, ctx, env, []string{applicationName, binaryName}); err != nil {
		t.Errorf("error: %v", err)
	}

	// Verify no output.
	if expected, got := "", strings.TrimSpace(stdout.String()); got != expected {
		t.Errorf("Unexpected output from tidy binary. Got %q, expected %q", got, expected)
	}
	if expected, got := "", strings.TrimSpace(stderr.String()); got != expected {
		t.Errorf("Unexpected error from tidy binary. Got %q, expected %q", got, expected)
	}

	// Verify binaryd tape.
	if got, expected := binarytape.Play(), []interface{}{
		binaryd.GlobStimulus{Pattern: "..."},

		binaryd.StatStimulus{Op: "Stat", Suffix: "binaries"},
		binaryd.StatStimulus{Op: "Stat", Suffix: "binaries/applicationd"},
		binaryd.StatStimulus{Op: "Stat", Suffix: "binaries/applicationd/darwin-amd64"},
		binaryd.StatStimulus{Op: "Stat", Suffix: "binaries/applicationd/darwin-amd64/app-darwin-amd-1"},
		binaryd.StatStimulus{Op: "Stat", Suffix: "binaries/applicationd/linux-amd64"},
		binaryd.StatStimulus{Op: "Stat", Suffix: "binaries/applicationd/linux-amd64/app-linux-amd-1"},
		binaryd.StatStimulus{Op: "Stat", Suffix: "binaries/binaryd"},
		binaryd.StatStimulus{Op: "Stat", Suffix: "binaries/binaryd/linux-amd64"},
		binaryd.StatStimulus{Op: "Stat", Suffix: "binaries/binaryd/linux-amd64/bind-linux-amd-1"},
		binaryd.StatStimulus{Op: "Stat", Suffix: "binaries/binaryd/linux-amd64/bind-linux-amd-2"},
		binaryd.StatStimulus{Op: "Stat", Suffix: "binaries/libraries"},
		binaryd.StatStimulus{Op: "Stat", Suffix: "binaries/libraries/linux-amd64"},

		binaryd.DeleteStimulus{Op: "Delete", Suffix: "binaries/applicationd/darwin-amd64/app-darwin-amd-1"},
		binaryd.DeleteStimulus{Op: "Delete", Suffix: "binaries/applicationd/linux-amd64/app-linux-amd-1"},
		binaryd.DeleteStimulus{Op: "Delete", Suffix: "binaries/binaryd/linux-amd64/bind-linux-amd-1"},
		binaryd.DeleteStimulus{Op: "Delete", Suffix: "binaries/binaryd/linux-amd64/bind-linux-amd-2"},
	}; !reflect.DeepEqual(expected, got) {
		t.Errorf("binarytape invalid call sequence. Got %#v, want %#v", got, expected)
	}

	// Verify application tape.
	if got, expected := apptape.Play(), []interface{}{
		binaryd.GlobStimulus{"..."},
		appd.MatchStimulus{Name: "Match", Suffix: "applications", Profiles: []string{"linux-amd64"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications", Profiles: []string{"linux-386"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications", Profiles: []string{"linux-arm"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications", Profiles: []string{"darwin-amd64"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications", Profiles: []string{"android-arm"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications/applicationd", Profiles: []string{"linux-amd64"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications/applicationd", Profiles: []string{"linux-386"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications/applicationd", Profiles: []string{"linux-arm"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications/applicationd", Profiles: []string{"darwin-amd64"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications/applicationd", Profiles: []string{"android-arm"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications/applicationd/0", Profiles: []string{"linux-amd64"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications/applicationd/0", Profiles: []string{"linux-386"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications/applicationd/0", Profiles: []string{"linux-arm"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications/applicationd/0", Profiles: []string{"darwin-amd64"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications/applicationd/0", Profiles: []string{"android-arm"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications/binaryd", Profiles: []string{"linux-amd64"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications/binaryd", Profiles: []string{"linux-386"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications/binaryd", Profiles: []string{"linux-arm"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications/binaryd", Profiles: []string{"darwin-amd64"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications/binaryd", Profiles: []string{"android-arm"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications/binaryd/1", Profiles: []string{"linux-amd64"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications/binaryd/1", Profiles: []string{"linux-386"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications/binaryd/1", Profiles: []string{"linux-arm"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications/binaryd/1", Profiles: []string{"darwin-amd64"}},
		appd.MatchStimulus{Name: "Match", Suffix: "applications/binaryd/1", Profiles: []string{"android-arm"}},
	}; !reflect.DeepEqual(expected, got) {
		t.Errorf("apptape invalid call sequence. Got %#v, want %#v", got, expected)
	}

}
