// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"fmt"
	"strings"
	"testing"

	"v.io/v23/naming"
	"v.io/v23/services/build"
	"v.io/x/ref/test/v23test"
)

func profileCommandOutput(t *testing.T, sh *v23test.Shell, profileBin string, expectError bool, command, name, suffix string) string {
	labelArgs := []string{
		command, naming.Join(name, suffix),
	}
	cmd := sh.Cmd(profileBin, labelArgs...)
	cmd.ExitErrorIsOk = true
	out := cmd.Stdout()
	if cmd.Err != nil && !expectError {
		t.Fatalf("%s %q failed: %v\n%v", profileBin, strings.Join(labelArgs, " "), cmd.Err, out)
	}
	if cmd.Err == nil && expectError {
		t.Fatalf("%s %q did not fail when it should", profileBin, strings.Join(labelArgs, " "))
	}
	return strings.TrimSpace(out)
}

func putProfile(sh *v23test.Shell, profileBin, name, suffix string) {
	putArgs := []string{
		"put", naming.Join(name, suffix),
	}
	sh.Cmd(profileBin, putArgs...).Run()
}

func removeProfile(sh *v23test.Shell, profileBin, name, suffix string) {
	removeArgs := []string{
		"remove", naming.Join(name, suffix),
	}
	sh.Cmd(profileBin, removeArgs...).Run()
}

func TestV23ProfileRepository(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	// Start the profile repository.
	profileRepoName := "test-profile-repo"
	profileRepoStore := sh.MakeTempDir()
	args := []string{
		"-name=" + profileRepoName, "-store=" + profileRepoStore,
		"-v23.tcp.address=127.0.0.1:0",
	}
	profiledBin := v23test.BuildGoPkg(sh, "v.io/x/ref/services/profile/profiled")
	sh.Cmd(profiledBin, args...).Start()

	clientBin := v23test.BuildGoPkg(sh, "v.io/x/ref/services/profile/profile")

	// Create a profile.
	const profile = "test-profile"
	putProfile(sh, clientBin, profileRepoName, profile)

	// Retrieve the profile label and check it matches the
	// expected label.
	profileLabel := profileCommandOutput(t, sh, clientBin, false, "label", profileRepoName, profile)
	if got, want := profileLabel, "example"; got != want {
		t.Fatalf("unexpected output: got %v, want %v", got, want)
	}

	// Retrieve the profile description and check it matches the
	// expected description.
	profileDesc := profileCommandOutput(t, sh, clientBin, false, "description", profileRepoName, profile)
	if got, want := profileDesc, "Example profile to test the profile manager implementation."; got != want {
		t.Fatalf("unexpected output: got %v, want %v", got, want)
	}

	// Retrieve the profile specification and check it matches the
	// expected specification.
	profileSpec := profileCommandOutput(t, sh, clientBin, false, "specification", profileRepoName, profile)
	if got, want := profileSpec, fmt.Sprintf(`profile.Specification{Label:"example", Description:"Example profile to test the profile manager implementation.", Arch:%d, Os:%d, Format:%d, Libraries:map[profile.Library]struct {}{profile.Library{Name:"foo", MajorVersion:"1", MinorVersion:"0"}:struct {}{}}}`, build.ArchitectureAmd64, build.OperatingSystemLinux, build.FormatElf); got != want {
		t.Fatalf("unexpected output: got %v, want %v", got, want)
	}

	// Remove the profile.
	removeProfile(sh, clientBin, profileRepoName, profile)

	// Check that the profile no longer exists.
	profileCommandOutput(t, sh, clientBin, true, "label", profileRepoName, profile)
	profileCommandOutput(t, sh, clientBin, true, "description", profileRepoName, profile)
	profileCommandOutput(t, sh, clientBin, true, "specification", profileRepoName, profile)
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}
