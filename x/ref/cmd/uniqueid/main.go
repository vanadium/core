// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc .

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"regexp"

	"v.io/v23/uniqueid"
	"v.io/x/lib/cmdline"
)

func main() {
	cmdline.Main(cmdUniqueID)
}

var cmdUniqueID = &cmdline.Command{
	Name:  "uniqueid",
	Short: "generates unique identifiers",
	Long: `
Command uniqueid generates unique identifiers.
It also has an option of automatically substituting unique ids with placeholders in files.
`,
	Children: []*cmdline.Command{cmdGenerate, cmdInject},
	Topics:   []cmdline.Topic{},
}

var cmdGenerate = &cmdline.Command{
	Runner: cmdline.RunnerFunc(runGenerate),
	Name:   "generate",
	Short:  "Generates UniqueIds",
	Long: `
Generates unique ids and outputs them to standard out.
`,
	ArgsName: "",
	ArgsLong: "",
}

var cmdInject = &cmdline.Command{
	Runner: cmdline.RunnerFunc(runInject),
	Name:   "inject",
	Short:  "Injects UniqueIds into existing files",
	Long: `
Injects UniqueIds into existing files.
Strings of the form "$UNIQUEID$" will be replaced with generated ids.
`,
	ArgsName: "<filenames>",
	ArgsLong: "<filenames> List of files to inject unique ids into",
}

// runGenerate implements the generate command which outputs generated ids to stdout.
func runGenerate(env *cmdline.Env, args []string) error {
	if len(args) > 0 {
		return env.UsageErrorf("expected 0 args, got %d", len(args))
	}
	id, err := uniqueid.Random()
	if err != nil {
		return err
	}
	fmt.Printf("%#v\n", id)
	return nil
}

// runInject implements the inject command which replaces $UNIQUEID$ strings with generated ids.
func runInject(env *cmdline.Env, args []string) error {
	if len(args) == 0 {
		return env.UsageErrorf("expected at least one file arg, got 0")
	}
	for _, arg := range args {
		if err := injectIntoFile(arg); err != nil {
			return err
		}
	}
	return nil
}

var uniqueRE = regexp.MustCompile("[$]UNIQUEID")

// injectIntoFile replaces $UNIQUEID$ strings when they exist in the specified file.
func injectIntoFile(filename string) error {
	inbytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	// Replace $UNIQUEID$ with generated ids.
	replaced := uniqueRE.ReplaceAllFunc(inbytes, func(match []byte) []byte {
		id, randErr := uniqueid.Random()
		if randErr != nil {
			err = randErr
		}
		return []byte(fmt.Sprintf("%#v", id))
	})
	if err != nil {
		return err
	}

	// If the file with injections is different, write it to disk.
	if !bytes.Equal(inbytes, replaced) {
		fmt.Printf("Updated: %s\n", filename)
		return ioutil.WriteFile(filename, replaced, 0)
	}
	return nil
}
