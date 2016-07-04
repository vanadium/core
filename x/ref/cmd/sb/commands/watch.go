// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package commands

import (
	"fmt"
	"strings"

	"v.io/v23/context"
	wire "v.io/v23/services/syncbase"
	"v.io/v23/services/watch"
	"v.io/v23/syncbase"
	"v.io/v23/vdl"
	"v.io/x/lib/cmdline"
)

var cmdWatch = &cmdline.Command{
	Name:     "watch",
	Short:    "Watch for updates to the database",
	Long:     "Watch for updates to the database.",
	ArgsName: "(<collection_blessing>,<collection_name>[,<row_key>] )*",
	ArgsLong: `
If no patterns are specified, watch defaults to watching for updates to any row
of any collection.
`,
	Runner: SbRunner(runWatch),
}

func runWatch(ctx *context.T, db syncbase.Database, env *cmdline.Env, args []string) error {
	// Get list of patterns to watch for.
	var patterns []wire.CollectionRowPattern
	if len(args) == 0 {
		patterns = []wire.CollectionRowPattern{
			wire.CollectionRowPattern{"%", "%", "%"},
		}
	} else {
		patterns = make([]wire.CollectionRowPattern, len(args))
		for i, arg := range args {
			patterns[i] = parsePattern(arg)
		}
	}

	// Watch for and print out changes.
	ws := db.Watch(ctx, watch.ResumeMarker("now"), patterns)
	for ws.Advance() {
		ch := ws.Change()
		switch ch.EntityType {
		case syncbase.EntityRow:
			fmt.Printf("change on row %s of collection %s:\n", ch.Row, ch.Collection)

			switch ch.ChangeType {
			case syncbase.PutChange:
				var v vdl.Value
				if err := ch.Value(&v); err != nil {
					fmt.Fprintf(env.Stderr, "\terror reading new value: %v\n", err)
				} else {
					fmt.Printf("\tnew value: %s\n", v.String())
				}
			case syncbase.DeleteChange:
				fmt.Printf("\trow removed\n")
			default:
				fmt.Fprintf(env.Stderr, "\tbad change type\n")
			}

		case syncbase.EntityCollection:
			fmt.Printf("change on collection %s:\n", ch.Collection)
			switch ch.ChangeType {
			case syncbase.PutChange:
				fmt.Printf("\tnew info: %v\n", ch.CollectionInfo())
			case syncbase.DeleteChange:
				fmt.Printf("\tcollection deleted\n")
			default:
				fmt.Fprintf(env.Stderr, "\tbad change type\n")
			}

		default:
			fmt.Fprintln(env.Stderr, "malformed change detected")
		}
	}

	return ws.Err()
}

func parsePattern(p string) wire.CollectionRowPattern {
	fields := strings.SplitN(p, ",", 3)
	return wire.CollectionRowPattern{fields[0], fields[1], fields[2]}
}
