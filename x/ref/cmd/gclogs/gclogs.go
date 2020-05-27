// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc . -help

package main

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"time"

	"v.io/x/lib/cmdline"
)

var (
	flagCutoff   time.Duration
	flagProgname string
	flagVerbose  bool
	flagDryrun   bool

	cmdGCLogs = &cmdline.Command{
		Runner: cmdline.RunnerFunc(garbageCollectLogs),
		Name:   "gclogs",
		Short:  "safely deletes old log files",
		Long: `
Command gclogs safely deletes old log files.

It looks for file names that match the format of files produced by the vlog
package, and deletes the ones that have not changed in the amount of time
specified by the --cutoff flag.

Only files produced by the same user as the one running the gclogs command
are considered for deletion.
`,
		ArgsName: "<dir> ...",
		ArgsLong: "<dir> ... A list of directories where to look for log files.",
	}
)

func init() {
	cmdGCLogs.Flags.DurationVar(&flagCutoff, "cutoff", 24*time.Hour, "The age cut-off for a log file to be considered for garbage collection.")
	cmdGCLogs.Flags.StringVar(&flagProgname, "program", ".*", `A regular expression to apply to the program part of the log file name, e.g ".*test".`)
	cmdGCLogs.Flags.BoolVar(&flagVerbose, "verbose", false, "If true, each deleted file is shown on stdout.")
	cmdGCLogs.Flags.BoolVar(&flagDryrun, "n", false, "If true, log files that would be deleted are shown on stdout, but not actually deleted.")
}

func main() {
	cmdline.Main(cmdGCLogs)
}

func garbageCollectLogs(env *cmdline.Env, args []string) error {
	if len(args) == 0 {
		return env.UsageErrorf("gclogs requires at least one argument")
	}
	timeCutoff := time.Now().Add(-flagCutoff)
	currentUser, err := user.Current()
	if err != nil {
		return err
	}
	programRE, err := regexp.Compile(flagProgname)
	if err != nil {
		return err
	}
	var lastErr error
	for _, logdir := range args {
		if err := processDirectory(env, logdir, timeCutoff, programRE, currentUser.Username); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func processDirectory(env *cmdline.Env, logdir string, timeCutoff time.Time, programRE *regexp.Regexp, username string) error { //nolint:gocyclo
	fmt.Fprintf(env.Stdout, "Processing: %q\n", logdir)

	f, err := os.Open(logdir)
	if err != nil {
		return err
	}
	defer f.Close()

	var lastErr error
	deleted := 0
	symlinks := []string{}
	for {
		fi, err := f.Readdir(100)
		if err == io.EOF {
			break
		}
		if err != nil {
			lastErr = err
			break
		}
		for _, file := range fi {
			fullname := filepath.Join(logdir, file.Name())
			if file.IsDir() {
				if flagVerbose {
					fmt.Fprintf(env.Stdout, "Skipped directory: %q\n", fullname)
				}
				continue
			}
			lf, err := parseFileInfo(logdir, file)
			if err != nil {
				if flagVerbose {
					fmt.Fprintf(env.Stdout, "Not a log file: %q\n", fullname)
				}
				continue
			}
			if lf.user != username {
				if flagVerbose {
					fmt.Fprintf(env.Stdout, "Skipped log file created by other user: %q\n", fullname)
				}
				continue
			}
			if !programRE.MatchString(lf.program) {
				if flagVerbose {
					fmt.Fprintf(env.Stdout, "Skipped log file doesn't match %q: %q\n", flagProgname, fullname)
				}
				continue
			}
			if lf.symlink {
				symlinks = append(symlinks, fullname)
				continue
			}
			if file.ModTime().Before(timeCutoff) {
				if flagDryrun {
					fmt.Fprintf(env.Stdout, "Would delete %q\n", fullname)
					continue
				}
				if flagVerbose {
					fmt.Fprintf(env.Stdout, "Deleting %q\n", fullname)
				}
				if err := os.Remove(fullname); err != nil {
					lastErr = err
				} else {
					deleted++
				}
			}
		}
	}
	// Delete broken links.
	for _, sl := range symlinks {
		if _, err := os.Stat(sl); err != nil && os.IsNotExist(err) {
			if flagDryrun {
				fmt.Fprintf(env.Stdout, "Would delete symlink %q\n", sl)
				continue
			}
			if flagVerbose {
				fmt.Fprintf(env.Stdout, "Deleting symlink %q\n", sl)
			}
			if err := os.Remove(sl); err != nil {
				lastErr = err
			} else {
				deleted++
			}
		}

	}
	fmt.Fprintf(env.Stdout, "Number of files deleted: %d\n", deleted)
	return lastErr
}
