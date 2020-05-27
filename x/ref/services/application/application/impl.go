// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc .

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	"v.io/v23/context"
	"v.io/v23/services/application"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/services/repository"
)

func main() {
	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdRoot)
}

func getEnvelopeJSON(ctx *context.T, app repository.ApplicationClientMethods, profiles []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	env, err := app.Match(ctx, profiles)
	if err != nil {
		return nil, err
	}
	j, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("MarshalIndent(%v) failed: %v", env, err)
	}
	return j, nil
}

func putEnvelopeJSON(ctx *context.T, env *cmdline.Env, app repository.ApplicationClientMethods, profiles []string, j []byte, overwrite bool) error {
	var envelope application.Envelope
	if err := json.Unmarshal(j, &envelope); err != nil {
		return fmt.Errorf("Unmarshal(%v) failed: %v", string(j), err)
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	results := make(chan struct {
		profile string
		err     error
	}, len(profiles))
	for _, profile := range profiles {
		go func(profile string) {
			results <- struct {
				profile string
				err     error
			}{profile, app.Put(ctx, profile, envelope, overwrite)}
		}(profile)
	}
	resultsMap := make(map[string]error, len(profiles))
	for i := 0; i < len(profiles); i++ {
		result := <-results
		resultsMap[result.profile] = result.err
	}
	nErrors := 0
	for _, p := range profiles {
		if err := resultsMap[p]; err == nil {
			fmt.Fprintf(env.Stdout, "Application envelope added for profile %s.\n", p)
		} else {
			fmt.Fprintf(env.Stderr, "Failed adding application envelope for profile %s: %v.\n", p, err)
			nErrors++
		}
	}
	if nErrors > 0 {
		return fmt.Errorf("encountered %d errors", nErrors)
	}
	return nil
}

func parseProfiles(s string) (ret []string) {
	profiles := strings.Split(s, ",")
	seen := make(map[string]bool)
	for _, p := range profiles {
		if p != "" && !seen[p] {
			seen[p] = true
			ret = append(ret, p)
		}
	}
	return
}

func promptUser(env *cmdline.Env, msg string) string {
	fmt.Fprint(env.Stdout, msg)
	var answer string
	if _, err := fmt.Scanf("%s", &answer); err != nil {
		return ""
	}
	return answer
}

var cmdMatch = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runMatch),
	Name:     "match",
	Short:    "Shows the first matching envelope that matches the given profiles.",
	Long:     "Shows the first matching envelope that matches the given profiles.",
	ArgsName: "<application> <profiles>",
	ArgsLong: `
<application> is the full name of the application.
<profiles> is a non-empty comma-separated list of profiles.`,
}

func runMatch(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 2, len(args); expected != got {
		return env.UsageErrorf("match: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name, profiles := args[0], parseProfiles(args[1])
	app := repository.ApplicationClient(name)
	j, err := getEnvelopeJSON(ctx, app, profiles)
	if err != nil {
		return err
	}
	fmt.Fprintln(env.Stdout, string(j))
	return nil
}

var cmdProfiles = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runProfiles),
	Name:     "profiles",
	Short:    "Shows the profiles supported by the given application.",
	Long:     "Returns a comma-separated list of profiles supported by the given application.",
	ArgsName: "<application>",
	ArgsLong: `
<application> is the full name of the application.`,
}

func runProfiles(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 1, len(args); expected != got {
		return env.UsageErrorf("profiles: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name := args[0]
	app := repository.ApplicationClient(name)
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	profiles, err := app.Profiles(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintln(env.Stdout, strings.Join(profiles, ","))
	return nil
}

var cmdPut = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runPut),
	Name:     "put",
	Short:    "Add the given envelope to the application for the given profiles.",
	Long:     "Add the given envelope to the application for the given profiles.",
	ArgsName: "<application> <profiles> [<envelope>]",
	ArgsLong: `
<application> is the full name of the application.
<profiles> is a comma-separated list of profiles.
<envelope> is the file that contains a JSON-encoded envelope. If this file is
not provided, the user will be prompted to enter the data manually.`,
}

var overwriteFlag bool

func init() {
	cmdPut.Flags.BoolVar(&overwriteFlag, "overwrite", false, "If true, put forces an overwrite of any existing envelope")
}

func runPut(ctx *context.T, env *cmdline.Env, args []string) error {
	if got := len(args); got != 2 && got != 3 {
		return env.UsageErrorf("put: incorrect number of arguments, expected 2 or 3, got %d", got)
	}
	name, profiles := args[0], parseProfiles(args[1])
	if len(profiles) == 0 {
		return env.UsageErrorf("put: no profiles specified")
	}
	app := repository.ApplicationClient(name)
	if len(args) == 3 {
		envelope := args[2]
		j, err := ioutil.ReadFile(envelope)
		if err != nil {
			return fmt.Errorf("ReadFile(%v): %v", envelope, err)
		}
		if err = putEnvelopeJSON(ctx, env, app, profiles, j, overwriteFlag); err != nil {
			return err
		}
		return nil
	}
	envelope := application.Envelope{Args: []string{}, Env: []string{}, Packages: application.Packages{}}
	j, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("MarshalIndent() failed: %v", err)
	}
	if err := editAndPutEnvelopeJSON(ctx, env, app, profiles, j, overwriteFlag); err != nil {
		return err
	}
	return nil
}

var cmdRemove = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runRemove),
	Name:     "remove",
	Short:    "removes the application envelope for the given profile.",
	Long:     "removes the application envelope for the given profile.",
	ArgsName: "<application> <profile>",
	ArgsLong: `
<application> is the full name of the application.
<profile> is a profile.  If specified as '*', all profiles are removed.`,
}

func runRemove(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 2, len(args); expected != got {
		return env.UsageErrorf("remove: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name, profile := args[0], args[1]
	app := repository.ApplicationClient(name)
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if err := app.Remove(ctx, profile); err != nil {
		return err
	}
	fmt.Fprintln(env.Stdout, "Application envelope removed successfully.")
	return nil
}

var cmdEdit = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(runEdit),
	Name:     "edit",
	Short:    "edits the application envelope for the given profile.",
	Long:     "edits the application envelope for the given profile.",
	ArgsName: "<application> <profile>",
	ArgsLong: `
<application> is the full name of the application.
<profile> is a profile.`,
}

func runEdit(ctx *context.T, env *cmdline.Env, args []string) error {
	if expected, got := 2, len(args); expected != got {
		return env.UsageErrorf("edit: incorrect number of arguments, expected %d, got %d", expected, got)
	}
	name, profiles := args[0], parseProfiles(args[1])
	if numProfiles := len(profiles); numProfiles != 1 {
		return env.UsageErrorf("edit: incorrect number of profiles, expected 1, got %d", numProfiles)
	}
	app := repository.ApplicationClient(name)

	envData, err := getEnvelopeJSON(ctx, app, profiles)
	if err != nil {
		return err
	}
	if err := editAndPutEnvelopeJSON(ctx, env, app, profiles, envData, true); err != nil {
		return err
	}
	return nil
}

func editAndPutEnvelopeJSON(ctx *context.T, env *cmdline.Env, app repository.ApplicationClientMethods, profiles []string, envData []byte, overwrite bool) error {
	f, err := ioutil.TempFile("", "application-edit-")
	if err != nil {
		return fmt.Errorf("TempFile() failed: %v", err)
	}
	fileName := f.Name()
	f.Close()
	defer os.Remove(fileName)
	if err = ioutil.WriteFile(fileName, envData, os.FileMode(0644)); err != nil {
		return err
	}
	editor := env.Vars["EDITOR"]
	if len(editor) == 0 {
		editor = "nano"
	}
	for {
		c := exec.Command("sh", "-c", fmt.Sprintf("%s %s", editor, fileName))
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("failed to run %s %s", editor, fileName)
		}
		newData, err := ioutil.ReadFile(fileName)
		if err != nil {
			fmt.Fprintf(env.Stdout, "Error: %v\n", err)
			if ans := promptUser(env, "Try again? [y/N] "); strings.ToUpper(ans) == "Y" {
				continue
			}
			return errors.New("aborted")
		}
		if bytes.Equal(envData, newData) {
			fmt.Fprintln(env.Stdout, "Nothing changed")
			return nil
		}
		if err = putEnvelopeJSON(ctx, env, app, profiles, newData, overwrite); err != nil {
			fmt.Fprintf(env.Stdout, "Error: %v\n", err)
			if ans := promptUser(env, "Try again? [y/N] "); strings.ToUpper(ans) == "Y" {
				continue
			}
			return errors.New("aborted")
		}
		break
	}
	fmt.Fprintln(env.Stdout, "Application envelope updated successfully.")
	return nil
}

var cmdRoot = &cmdline.Command{
	Name:  "application",
	Short: "manages the Vanadium application repository",
	Long: `
Command application manages the Vanadium application repository.
`,
	Children: []*cmdline.Command{cmdMatch, cmdProfiles, cmdPut, cmdRemove, cmdEdit},
}
