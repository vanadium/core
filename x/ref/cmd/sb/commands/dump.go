// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package commands

import (
	"fmt"
	"io"

	"v.io/v23/context"
	"v.io/v23/syncbase"
	"v.io/x/lib/cmdline"
)

var cmdDump = &cmdline.Command{
	Name:   "dump",
	Short:  "Print a dump of the database",
	Long:   `Print a dump of the database.`,
	Runner: SbRunner(dumpDatabase),
}

func dumpDatabase(ctx *context.T, d syncbase.Database, env *cmdline.Env, _ []string) error {
	w := env.Stdout
	var errors []error
	if err := dumpCollections(ctx, w, d); err != nil {
		errors = append(errors, fmt.Errorf("failed dumping collections: %v", err))
	}
	if err := dumpSyncgroups(ctx, w, d); err != nil {
		errors = append(errors, fmt.Errorf("failed dumping syncgroups: %v", err))
	}
	return mergeErrors(errors)
}

func dumpCollections(ctx *context.T, w io.Writer, d syncbase.Database) error {
	collections, err := d.ListCollections(ctx)
	if err != nil {
		return fmt.Errorf("failed listing collections: %v", err)
	}
	var errs []error
	for _, collection := range collections {
		fmt.Fprintf(w, "collection: %v\n", collection)
		// TODO(ivanpi): Queries currently support only the default user blessing.
		if err := queryExec(ctx, w, d, fmt.Sprintf("select k, v from %s", collection.Name)); err != nil {
			errs = append(errs, fmt.Errorf("> %v: %v", collection, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed dumping %d of %d collections:\n%v", len(errs), len(collections), mergeErrors(errs))
	}
	return nil
}

func dumpSyncgroups(ctx *context.T, w io.Writer, d syncbase.Database) error {
	sgIds, err := d.ListSyncgroups(ctx)
	if err != nil {
		return fmt.Errorf("failed listing syncgroups: %v", err)
	}
	var errs []error
	for _, sgId := range sgIds {
		fmt.Fprintf(w, "syncgroup: %+v\n", sgId)
		if spec, version, err := d.SyncgroupForId(sgId).GetSpec(ctx); err != nil {
			errs = append(errs, err)
		} else {
			fmt.Fprintf(w, "%+v (version: \"%s\")\n", spec, version)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed dumping %d of %d syncgroups:\n%v", len(errs), len(sgIds), mergeErrors(errs))
	}
	return nil
}

func mergeErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	err := errs[0]
	for _, e := range errs[1:] {
		err = fmt.Errorf("%v\n%v", err, e)
	}
	return err
}
